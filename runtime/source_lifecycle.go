package runtime

import (
	"context"
	stderrors "errors"
	"slices"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

// OwnedSource supplies a source and its shutdown operation.
// Shutdown cancels and joins source-owned work unless its context ends first.
// With a live context, it must finish joining before returning any cleanup error.
// Repeated calls must be safe.
type OwnedSource interface {
	Source
	Shutdown(context.Context) error
}

// WithOwnedSource transfers the selected source to the runtime during Open.
// Open failure and runtime shutdown close it. Only the final selected source transfers.
// WithSource keeps ownership with the caller and replaces this selection.
func WithOwnedSource(source OwnedSource) Option {
	return func(config *options) error {
		if err := WithSource(source)(config); err != nil {
			return err
		}
		config.ownedSource = source
		return nil
	}
}

// SourceDescriber declares metadata capabilities and default selection without acquisition.
// The runtime calls it once when it opens. It must not read external data.
// The runtime treats omitted metadata sources as unsupported. Eligibility remains unknown until acquisition.
type SourceDescriber interface {
	SourceConfiguration() []sources.SourceActivity
}

// sourceReportAcquirer supplies activity owned by one acquisition call.
// Collectors without this optional role leave activity unknown.
type sourceReportAcquirer interface {
	AcquireSourceReport(context.Context, SourceAcquisitionRequest) ([]sources.Observation, []sources.SourceActivity, error)
}

// prepareSourceStartup applies startup policy before scheduled work starts.
// require_source reads once under the Open context when this runtime owns the lease.
// Authority observation binds after other startup checks, before any source worker can read a manifest.
func (r *Runtime) prepareSourceStartup(ctx context.Context) error {
	if r.config.generationPin != "" {
		return r.bindSourceAuthority()
	}
	if r.config.source.StartupPolicy == StartupRequireSource {
		if !r.automaticSourceReads() {
			r.mu.RLock()
			retained := r.layers.source != nil
			r.mu.RUnlock()
			if !retained {
				return &errors.ConfigError{Component: "catalog startup policy", Message: "require_source needs retained source state when automatic source reads are disabled"}
			}
		} else if r.lease.status() == leaseLost {
			logging.Info().
				Str("holder", r.schedule.identity.Instance).
				Msg("The lease owner supplies the source state; require_source reads nothing here")
		} else if _, err := r.RefreshSource(ctx); err != nil {
			return errors.WrapResource("read", "catalog source", r.source.Identity(), err)
		}
	}
	return r.bindSourceAuthority()
}

func describeSources(config *options) ([]sources.SourceActivity, error) {
	metadata := []sources.ID{sources.LocalCatalogID, sources.ModelsDevHTTPID, sources.ModelsDevGitID}
	configured := make(map[sources.ID]sources.SourceActivity)
	if descriptor, ok := config.sourceAcquirer.(SourceDescriber); ok {
		for _, row := range descriptor.SourceConfiguration() {
			if _, duplicate := configured[row.Source]; duplicate || !row.Valid() || row.Source == sources.ProvidersID || row.Attempted || row.Eligibility != sources.EligibilityUnknown {
				return nil, &errors.ValidationError{Field: "source_acquirer.configuration", Message: "must declare unique passive metadata source states"}
			}
			configured[row.Source] = row
		}
	} else if config.sourceAcquirer != nil {
		for _, id := range metadata {
			configured[id] = sources.SourceActivity{Source: id, SupportUnknown: true, SelectionUnknown: true, Eligibility: sources.EligibilityUnknown}
		}
	}
	result := make([]sources.SourceActivity, 0, len(metadata)+1)
	for _, id := range append(metadata, sources.ProvidersID) {
		row, present := configured[id]
		if !present {
			row = sources.SourceActivity{Source: id, Eligibility: sources.EligibilityUnknown}
		}
		if id == sources.ProvidersID {
			row.Supported = config.acquirer != nil
			row.Enabled = row.Supported
		}
		if config.acquisitionSources != nil {
			row.Enabled = slices.Contains(config.acquisitionSources.ids, id)
			row.SelectionUnknown = false
		}
		result = append(result, row)
	}
	return result, nil
}

func (r *Runtime) acquireSourceReport(ctx context.Context, request SourceAcquisitionRequest) ([]sources.Observation, []sources.SourceActivity, error) {
	detailed, ok := r.config.sourceAcquirer.(sourceReportAcquirer)
	if !ok {
		observations, err := r.config.sourceAcquirer.AcquireSources(ctx, request)
		return observations, nil, err
	}
	observations, activity, err := detailed.AcquireSourceReport(ctx, request)
	var owned []sources.SourceActivity
	seen := make(map[sources.ID]bool)
	invalid := false
	for _, row := range activity {
		if !row.Valid() || row.Source == sources.ProvidersID || seen[row.Source] {
			invalid = true
			continue
		}
		seen[row.Source] = true
		owned = append(owned, row)
	}
	if invalid {
		err = stderrors.Join(err, &errors.ValidationError{Field: "source_acquirer.activity", Message: "must contain unique valid metadata source states"})
	}
	return observations, owned, err
}

func providerSourceActivity(result AcquisitionResult, attempted bool, err error) sources.SourceActivity {
	row := sources.SourceActivity{Source: sources.ProvidersID, Supported: true, Enabled: true, Attempted: attempted, Eligibility: sources.EligibilityUnknown}
	unknown := len(result.Attempts) < result.Eligible || (len(result.Attempts) == 0 && (err != nil || len(result.Layers) > 0))
	for _, attempt := range result.Attempts {
		if attempt.Validate() != nil {
			unknown = true
			continue
		}
		if attempt.Requested || attempt.Outcome == sources.ProviderOutcomeSucceeded {
			row.Eligibility = sources.EligibilityEligible
			return row
		}
		switch attempt.Reason {
		case sources.ProviderReasonCredentialUnavailable, sources.ProviderReasonCredentialReferenceInvalid, sources.ProviderReasonCredentialExpired, sources.ProviderReasonInsufficientScope, sources.ProviderReasonDependencyUnavailable:
		default:
			unknown = true
		}
	}
	if !unknown {
		row.Eligibility = sources.EligibilityIneligible
	}
	return row
}
