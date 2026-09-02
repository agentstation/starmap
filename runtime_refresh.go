package starmap

import (
	"context"
	stderrors "errors"
	"sync"
	"time"

	"github.com/agentstation/starmap/internal/fleet"
	"github.com/agentstation/starmap/internal/sources/github"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// runKind names the three refresh operations. Each one changes a distinct
// layer, so a report says exactly what moved.
type runKind string

const (
	// runKindRefresh reads the source and then observes the providers.
	runKindRefresh runKind = "refresh"

	// runKindSource reads the selected upstream source only.
	runKindSource runKind = "source"

	// runKindAcquisition observes providers only.
	runKindAcquisition runKind = "acquisition"
)

// SourceRefreshReport says what one upstream source read produced.
type SourceRefreshReport struct {
	// RunID is the identity of the run that produced this report.
	RunID string

	// StartedAt and CompletedAt bound the run.
	StartedAt   time.Time
	CompletedAt time.Time

	// SourceIdentity is the safe identity of the source that answered.
	SourceIdentity string

	// Changed reports whether the upstream generation moved.
	Changed bool

	// Published reports whether the runtime published a new effective catalog.
	Published bool

	// GenerationID identifies the upstream generation the runtime retained.
	GenerationID string

	// PublishedAt is the upstream publication time.
	PublishedAt time.Time

	// Health is what this runtime observed while it read the source. It grades
	// the transfer only.
	Health Health

	// UpstreamHealth is the health the upstream reported about itself. It stays
	// independent of Health, so a healthy transfer still carries a degraded
	// upstream report.
	UpstreamHealth Health

	// Reason is the safe reason code of a failed read.
	Reason string

	// Chain is the sanitized upstream source chain.
	Chain []SourceHop
}

// AcquisitionReport says what one provider acquisition run produced. A partial
// failure still publishes: the report names the providers that kept their own
// last-known-good observation.
type AcquisitionReport struct {
	// RunID is the identity of the run that produced this report.
	RunID string

	// StartedAt and CompletedAt bound the run.
	StartedAt   time.Time
	CompletedAt time.Time

	// Eligible is the number of providers the run considered.
	Eligible int

	// Succeeded, Skipped, and Failed count the terminal attempts.
	Succeeded int
	Skipped   int
	Failed    int

	// Attempts holds one terminal attempt per eligible provider.
	Attempts []sources.ProviderAttempt

	// Published reports whether the runtime published a new effective catalog.
	Published bool

	// GenerationID identifies the published effective catalog.
	GenerationID string

	// Retained names the providers that kept their previous last-known-good
	// observation because this run did not replace it.
	Retained []catalogs.ProviderID

	// Health grades the run. A failed provider degrades the run.
	Health Health
}

// RefreshReport says what one whole refresh produced. A source-only run leaves
// the acquisition report empty, and an acquisition-only run leaves the source
// report empty.
type RefreshReport struct {
	// RunID is the identity of the run.
	RunID string

	// Kind names the work the run did.
	Kind string

	// StartedAt and CompletedAt bound the run.
	StartedAt   time.Time
	CompletedAt time.Time

	// Source reports the upstream read.
	Source SourceRefreshReport

	// Acquisition reports the provider observations.
	Acquisition AcquisitionReport

	// Published reports whether the runtime published a new effective catalog.
	Published bool

	// GenerationID identifies the published effective catalog.
	GenerationID string
}

// activeRun is one refresh in flight. A second caller of the same kind joins it
// instead of starting a second run.
type activeRun struct {
	id   string
	kind runKind

	// epoch is the lease epoch the run started under. It fences every commit
	// the run makes, so a run that loses the lease cannot publish.
	epoch uint64

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	report RefreshReport
	err    error
}

// join waits for the run and returns its report. The caller keeps its own
// cancellation: leaving the join never cancels the run.
func (a *activeRun) join(ctx context.Context) (RefreshReport, error) {
	select {
	case <-a.done:
		return a.report, a.err
	case <-ctx.Done():
		return RefreshReport{}, errors.WrapResource("join", "refresh run", a.id, ctx.Err())
	}
}

// runGroup keeps refresh single-flight. One run of each kind exists at a time,
// and a caller of a different kind waits for the slot.
type runGroup struct {
	mu     sync.Mutex
	active *activeRun
}

// start returns the run this caller must use. The second result reports
// whether this caller owns the run and must execute it.
func (g *runGroup) start(
	ctx, parent context.Context,
	kind runKind,
	id string,
	epoch uint64,
) (*activeRun, bool, error) {
	for {
		g.mu.Lock()
		if g.active == nil {
			runCtx, cancel := context.WithCancel(parent)
			run := &activeRun{
				id:     id,
				kind:   kind,
				epoch:  epoch,
				ctx:    runCtx,
				cancel: cancel,
				done:   make(chan struct{}),
			}
			g.active = run
			g.mu.Unlock()
			return run, true, nil
		}
		existing := g.active
		g.mu.Unlock()
		if existing.kind == kind {
			return existing, false, nil
		}
		select {
		case <-existing.done:
		case <-ctx.Done():
			return nil, false, errors.WrapResource("await", "refresh run", existing.id, ctx.Err())
		}
	}
}

// finish publishes the result of one run and frees the slot.
func (g *runGroup) finish(run *activeRun, report RefreshReport, err error) {
	g.mu.Lock()
	if g.active == run {
		g.active = nil
	}
	g.mu.Unlock()
	run.report = report
	run.err = err
	run.cancel()
	close(run.done)
}

// cancelActive cancels the run in flight. The runtime calls it on close and
// after it loses the lease.
func (g *runGroup) cancelActive() {
	g.mu.Lock()
	run := g.active
	g.mu.Unlock()
	if run != nil {
		run.cancel()
	}
}

// Refresh reads the upstream source and then observes every eligible provider.
// It changes the source layer and the provider layers in one run.
func (r *Runtime) Refresh(ctx context.Context) (RefreshReport, error) {
	return r.execute(ctx, runKindRefresh, func(runCtx context.Context, report *RefreshReport, epoch uint64) error {
		sourceErr := r.readSource(runCtx, report, epoch)
		if r.config.acquirer == nil {
			return sourceErr
		}
		acquireErr := r.acquireProviders(runCtx, report, nil, epoch)
		return stderrors.Join(sourceErr, acquireErr)
	})
}

// RefreshSource reads the upstream source only. It changes the source layer.
func (r *Runtime) RefreshSource(ctx context.Context) (SourceRefreshReport, error) {
	report, err := r.execute(ctx, runKindSource, func(runCtx context.Context, report *RefreshReport, epoch uint64) error {
		return r.readSource(runCtx, report, epoch)
	})
	return report.Source, err
}

// Sync observes providers only. It changes the provider layers and returns the
// acquisition report. An empty provider list observes every eligible provider.
func (r *Runtime) Sync(ctx context.Context, providers ...catalogs.ProviderID) (AcquisitionReport, error) {
	report, err := r.execute(ctx, runKindAcquisition, func(runCtx context.Context, report *RefreshReport, epoch uint64) error {
		if r.config.acquirer == nil {
			return &errors.ConfigError{
				Component: "acquirer",
				Message:   "provider acquisition needs an injected acquirer",
			}
		}
		return r.acquireProviders(runCtx, report, providers, epoch)
	})
	return report.Acquisition, err
}

// execute runs one refresh under the single-flight group. A second caller of
// the same kind joins the run in flight and reads its report.
func (r *Runtime) execute(
	ctx context.Context,
	kind runKind,
	work func(context.Context, *RefreshReport, uint64) error,
) (RefreshReport, error) {
	if r == nil {
		return RefreshReport{}, &errors.ValidationError{Field: "runtime", Message: "is required"}
	}
	if ctx == nil {
		return RefreshReport{}, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	id, err := r.client.nextID()
	if err != nil {
		return RefreshReport{}, err
	}
	run, owner, err := r.runs.start(ctx, r.ctx, kind, id, r.lease.epoch())
	if err != nil {
		return RefreshReport{}, err
	}
	if !owner {
		return run.join(ctx)
	}

	// The run belongs to the runtime, not to one caller. Caller cancellation
	// still stops the run. A caller deadline adds no deadline of its own, so
	// the configured refresh timeout is the only deadline a run carries.
	stopPropagation := context.AfterFunc(ctx, run.cancel)
	defer stopPropagation()

	runCtx := run.ctx
	if timeout := r.config.refreshTimeout; timeout > 0 {
		bounded, cancel := context.WithTimeout(runCtx, timeout)
		defer cancel()
		runCtx = bounded
	}

	report := RefreshReport{RunID: run.id, Kind: string(kind), StartedAt: r.config.now()}
	workErr := work(runCtx, &report, run.epoch)
	report.CompletedAt = r.config.now()

	r.mu.Lock()
	r.report.lastRunID = run.id
	r.mu.Unlock()

	r.runs.finish(run, report, workErr)
	return report, workErr
}

// readSource reads the upstream source, retains the generation, and publishes
// the rebuilt effective catalog.
func (r *Runtime) readSource(ctx context.Context, report *RefreshReport, epoch uint64) error {
	source := r.source
	if source == nil {
		return &errors.ConfigError{Component: "catalog source", Message: "is not selected"}
	}
	result := SourceRefreshReport{
		RunID:          report.RunID,
		StartedAt:      r.config.now(),
		SourceIdentity: source.Identity(),
	}

	read, err := r.readWithRetry(ctx, source)
	result.CompletedAt = r.config.now()
	if err != nil {
		result.Health = HealthUnavailable
		result.Reason = safeSourceReason(err)
		r.recordSourceRead(result, false)
		report.Source = result
		return err
	}

	// A completed read grades the transfer healthy. The upstream report stays
	// separate, so a degraded upstream never hides a working transfer, and a
	// working transfer never hides a degraded upstream.
	result.Health = HealthOK
	result.UpstreamHealth = orUnknown(read.Health)
	result.Chain = read.Chain
	result.Changed = read.Changed
	result.PublishedAt = read.PublishedAt
	if !read.Changed {
		r.recordSourceRead(result, true)
		report.Source = result
		return nil
	}

	layer := sourceLayer{
		Identity:         source.Identity(),
		GenerationID:     read.Generation.Manifest.GenerationID,
		Checksum:         read.Generation.Manifest.Payload.Checksum,
		Payload:          read.Generation.Payload,
		PublishedAt:      read.PublishedAt,
		ChannelUpdatedAt: read.ChannelUpdatedAt,
		ObservedAt:       result.CompletedAt,
		Chain:            read.Chain,
	}
	if err := r.store.saveSource(layer); err != nil {
		result.Health = HealthDegraded
		result.Reason = "retention_failed"
		r.recordSourceRead(result, false)
		report.Source = result
		return err
	}
	r.mu.Lock()
	r.layers.source = &layer
	r.mu.Unlock()

	state, err := r.rebuild(ctx, epoch)
	if err != nil {
		result.Health = HealthDegraded
		result.Reason = "publication_failed"
		r.recordSourceRead(result, false)
		report.Source = result
		return err
	}
	result.Published = true
	result.GenerationID = state.GenerationID
	report.Published = true
	report.GenerationID = state.GenerationID
	r.recordSourceRead(result, true)
	report.Source = result
	return nil
}

// readWithRetry reads the source under the fleet retry policy. It honors a
// declared not-before boundary and warns while the request budget nears its
// bound, so a refused fleet does not retry at one instant.
func (r *Runtime) readWithRetry(ctx context.Context, source Source) (SourceRead, error) {
	policy := fleet.DefaultRetryPolicy()
	random := fleet.Random(r.config.random)
	delay := time.Duration(0)
	var lastErr error
	for retries := 0; ; retries++ {
		read, err := source.Read(ctx)
		if err == nil {
			return read, nil
		}
		lastErr = err
		if ctx.Err() != nil || !policy.Allows(retries) {
			return SourceRead{}, lastErr
		}

		var refusal *github.RefusalError
		if stderrors.As(err, &refusal) {
			if refusal.Budget.Warn() {
				logging.Warn().
					Int("used_percent", refusal.Budget.UsedPercent()).
					Str("resource", refusal.Resource).
					Msg("Catalog source request budget is nearly spent")
			}
			now := r.config.now()
			boundary := fleet.NotBefore(now, refusal.NotBefore, random)
			delay = boundary.Sub(now)
			if delay < 0 {
				delay = 0
			}
		} else {
			delay, err = policy.Next(delay, random)
			if err != nil {
				return SourceRead{}, err
			}
		}
		if !waitFor(ctx, delay) {
			return SourceRead{}, lastErr
		}
	}
}

// waitFor sleeps for the delay. It reports false when the context ended first.
func waitFor(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// recordSourceRead stores what Status reports about the source layer.
func (r *Runtime) recordSourceRead(result SourceRefreshReport, reached bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.report.sourceCheckedAt = result.CompletedAt
	r.report.sourceHealth = result.Health
	r.report.sourceReason = result.Reason
	if reached {
		r.report.upstreamHealth = orUnknown(result.UpstreamHealth)
	}
	if len(result.Chain) > 0 {
		r.report.upstreamChain = result.Chain
		r.report.upstreamHealth = worseHealth(r.report.upstreamHealth, result.Chain[0].Health)
	}
	if reached && result.Changed {
		r.report.sourceChangedAt = result.CompletedAt
	}
}

// acquireProviders observes providers, retains every completed observation, and
// publishes. A failed provider keeps its own last-known-good layer, so one
// failure never removes records from the effective catalog.
func (r *Runtime) acquireProviders(
	ctx context.Context,
	report *RefreshReport,
	providers []catalogs.ProviderID,
	epoch uint64,
) error {
	result := AcquisitionReport{RunID: report.RunID, StartedAt: r.config.now()}
	current := r.State().Catalog
	observed, err := r.config.acquirer.AcquireProviders(ctx, AcquisitionRequest{
		RunID:          report.RunID,
		Current:        current,
		Providers:      providers,
		CoalesceWindow: r.config.coalesceWindow,
	})
	result.CompletedAt = r.config.now()
	result.Eligible = observed.Eligible
	result.Attempts = observed.Attempts
	for _, attempt := range observed.Attempts {
		switch attempt.Outcome {
		case sources.ProviderOutcomeSucceeded:
			result.Succeeded++
		case sources.ProviderOutcomeSkippedNotConfigured:
			result.Skipped++
		case sources.ProviderOutcomeFailed:
			result.Failed++
		}
	}

	answered := make(map[catalogs.ProviderID]bool, len(observed.Layers))
	for _, layer := range observed.Layers {
		if saveErr := r.store.saveProvider(layer); saveErr != nil {
			return stderrors.Join(err, saveErr)
		}
		answered[layer.ProviderID] = true
		r.mu.Lock()
		r.layers.setProvider(layer)
		r.mu.Unlock()
	}
	r.mu.RLock()
	for _, id := range r.layers.providerOrder() {
		if !answered[id] {
			result.Retained = append(result.Retained, id)
		}
	}
	r.mu.RUnlock()

	result.Health = HealthOK
	switch {
	case err != nil:
		result.Health = HealthUnavailable
	case result.Failed > 0 || result.Skipped > 0:
		result.Health = HealthDegraded
	}

	// A partial failure still publishes. The layers that answered move forward
	// and the layers that did not keep their retained records.
	if len(observed.Layers) > 0 {
		state, publishErr := r.rebuild(ctx, epoch)
		if publishErr != nil {
			result.Health = HealthDegraded
			report.Acquisition = result
			return stderrors.Join(err, publishErr)
		}
		result.Published = true
		result.GenerationID = state.GenerationID
		report.Published = true
		report.GenerationID = state.GenerationID
	}

	r.mu.Lock()
	r.report.acquisitionStartedAt = result.StartedAt
	r.report.acquisitionHealth = result.Health
	r.report.attempts = result.Attempts
	if err == nil {
		r.report.acquisitionSucceededAt = result.CompletedAt
	}
	r.mu.Unlock()

	report.Acquisition = result
	return err
}

// safeSourceReason maps one source failure onto a safe reason code. The code
// names no URL, no host, and no credential.
func safeSourceReason(err error) string {
	var refusal *github.RefusalError
	if stderrors.As(err, &refusal) {
		if refusal.Status == 429 {
			return string(sources.ProviderReasonRateLimited)
		}
		return string(sources.ProviderReasonCredentialRejected)
	}
	return string(sources.ClassifyProviderReason(err))
}
