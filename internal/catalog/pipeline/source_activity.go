package pipeline

import (
	"context"
	"slices"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// sourceActivityRun belongs to one Prepare call, including its concurrent collectors.
type sourceActivityRun struct {
	mu                sync.Mutex
	states            []sources.SourceActivity
	providerExpected  int
	providerCompleted int
	providerUnknown   bool
	providerEligible  bool
	providerAttempts  []sources.ProviderAttempt
}

func newSourceActivityRun(options *pkgsync.Options) *sourceActivityRun {
	return &sourceActivityRun{states: SourceConfiguration(options)}
}

// SourceConfiguration reports built-in support and parsed selection without I/O.
// A nil options value leaves selection unknown.
func SourceConfiguration(options *pkgsync.Options) []sources.SourceActivity {
	report := make([]sources.SourceActivity, 0, 4)
	for _, id := range []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID, sources.ModelsDevGitID, sources.ProvidersID} {
		row := sources.SourceActivity{Source: id, Supported: true, Eligibility: sources.EligibilityUnknown, SelectionUnknown: options == nil}
		if options != nil {
			row.Enabled = slices.Contains(options.Sources, id)
			if len(options.Sources) == 0 {
				row.Enabled = id != sources.ModelsDevGitID
			}
		}
		report = append(report, row)
	}
	return report
}

func (r *sourceActivityRun) resolved(available []sources.Source) {
	for _, source := range available {
		if source.ID() == sources.ProvidersID {
			r.providerExpected++
		}
	}
	for i := range r.states {
		state := &r.states[i]
		if !state.Enabled {
			continue
		}
		state.Eligibility = sources.EligibilityIneligible
		for _, source := range available {
			if source.ID() == state.Source {
				state.Eligibility = sources.EligibilityEligible
				// Provider credential preflight occurs within its collector.
				if state.Source == sources.ProvidersID {
					state.Eligibility = sources.EligibilityUnknown
				}
				break
			}
		}
	}
}

func (r *sourceActivityRun) attempted(id sources.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.states {
		if r.states[i].Source == id {
			r.states[i].Attempted = true
		}
	}
}

func (r *sourceActivityRun) snapshot() []sources.SourceActivity {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.states)
}

type activitySource struct {
	sources.Source
	run *sourceActivityRun
}

func (s activitySource) Observe(ctx context.Context, options ...sources.Option) (sources.Observation, error) {
	s.run.attempted(s.ID())
	if detailed, ok := s.Source.(interface {
		ObserveAttempts(context.Context, ...sources.Option) (sources.Observation, []sources.ProviderAttempt, error)
	}); ok && s.ID() == sources.ProvidersID {
		observation, attempts, err := detailed.ObserveAttempts(ctx, options...)
		s.run.recordProviderEligibility(attempts, err)
		return observation, err
	}
	return s.Source.Observe(ctx, options...)
}

func (s activitySource) providerBinding() sources.ProviderAcquisitionBinding {
	if scoped, ok := s.Source.(interface {
		providerBinding() sources.ProviderAcquisitionBinding
	}); ok {
		return scoped.providerBinding()
	}
	return sources.ProviderAcquisitionBinding{}
}

// Prepare records source activity without retaining mutable reports on the pipeline.
func (p *Pipeline) Prepare(ctx context.Context, existing *catalogs.Catalog, opts ...pkgsync.Option) (*Prepared, error) {
	if p == nil || existing == nil {
		return p.prepare(ctx, existing, nil)
	}
	options := pkgsync.Defaults().Apply(opts...)
	run := newSourceActivityRun(options)
	preparedPipeline := *p
	preparedPipeline.resolveDependencies = func(ctx context.Context, input []sources.Source, options *pkgsync.Options) ([]sources.Source, []error, error) {
		available, failures, err := p.resolveDependencies(ctx, input, options)
		if err == nil {
			run.resolved(available)
		} else {
			for _, failed := range sourceFailureSummaries(append(slices.Clone(failures), err)) {
				for i := range run.states {
					if run.states[i].Source == failed.Source {
						run.states[i].Eligibility = sources.EligibilityIneligible
					}
				}
			}
		}
		return available, failures, err
	}
	preparedPipeline.observe = func(ctx context.Context, input []sources.Source, options []sources.Option) ([]sources.Observation, error) {
		tracked := make([]sources.Source, 0, len(input))
		for _, source := range input {
			tracked = append(tracked, activitySource{Source: source, run: run})
		}
		return p.observe(ctx, tracked, options)
	}
	prepared, err := preparedPipeline.prepare(ctx, existing, options)
	report := run.snapshot()
	if err != nil {
		return nil, &sources.ActivityError{Activities: report, ProviderAttempts: run.providerReport(), Err: err}
	}
	prepared.Result.SourceActivities = report
	prepared.Result.ProviderAttempts = run.providerReport()
	return prepared, nil
}
