package runtime

import (
	"context"
	stderrors "errors"
	"slices"
	"sync"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// SourceAcquisitionRequest selects non-provider observations for one owned run.
// Current is immutable. Providers restricts model scope when the caller supplies IDs.
type SourceAcquisitionRequest struct {
	Current   *catalogs.Catalog
	Providers []catalogs.ProviderID
}

// SourceAcquirer collects configured non-provider observations without publication.
// It must preserve source receipts and report failed or partial observations.
// Runtime owns cancellation, lease fencing, retained evidence, and publication.
type SourceAcquirer interface {
	AcquireSources(context.Context, SourceAcquisitionRequest) ([]sources.Observation, error)
}

// WithSourceAcquirer adds non-provider acquisition beside the provider role.
// This option starts no source reads. Scheduled and explicit Sync runs use this role.
func WithSourceAcquirer(acquirer SourceAcquirer) Option {
	return func(config *options) error {
		if acquirer == nil {
			return &errors.ValidationError{Field: "source_acquirer", Message: "is required"}
		}
		config.sourceAcquirer = acquirer
		return nil
	}
}

func (r *Runtime) hasAcquisition() bool {
	return r.config.acquirer != nil || r.config.sourceAcquirer != nil
}

// Independent source groups share the same operation and lease.
// A slow metadata source does not hold completed provider publication windows.
func (r *Runtime) acquire(ctx context.Context, report *RefreshReport, providers []catalogs.ProviderID, epoch uint64) error {
	if r.config.sourceAcquirer == nil {
		err := r.acquireProviders(ctx, report, providers, epoch)
		r.recordAcquisition(report.Acquisition, err)
		return err
	}
	if r.config.providerBindings != nil {
		if _, err := r.config.providerBindings.selected(providers); err != nil {
			return err
		}
	}
	started := r.config.now()
	providerReport := RefreshReport{RunID: report.RunID}
	var providerErr error
	var work sync.WaitGroup
	if r.config.acquirer != nil {
		work.Go(func() { providerErr = r.acquireProviders(ctx, &providerReport, providers, epoch) })
	}
	observations, sourceErr := r.config.sourceAcquirer.AcquireSources(ctx, SourceAcquisitionRequest{Current: r.State().Catalog, Providers: slices.Clone(providers)})
	sourcePublished := false
	if len(observations) != 0 {
		for _, observation := range observations {
			if observation.SourceID == sources.ProvidersID || observation.SourceID == sources.EmbeddedCatalogID || observation.SourceID == sources.ReleaseArtifactID {
				sourceErr = stderrors.Join(sourceErr, &errors.ValidationError{Field: "source_acquirer.observation", Message: "must contain only non-provider acquisition sources"})
				observations = nil
				break
			}
		}
		if len(observations) != 0 {
			prepared, err := prepareManualObservations(ctx, observations)
			if err != nil {
				observations = nil
			}
			if err == nil {
				before := r.State().GenerationID
				state, publishErr := r.publishInputs(ctx, nil, nil, prepared, epoch, nil)
				sourcePublished = state.GenerationID != "" && state.GenerationID != before
				err = publishErr
			}
			sourceErr = stderrors.Join(sourceErr, err)
		}
	}
	work.Wait()
	result := providerReport.Acquisition
	result.RunID, result.StartedAt, result.CompletedAt = report.RunID, started, r.config.now()
	if result.Health == "" {
		result.Health = HealthOK
	}
	for _, observation := range observations {
		result.SourceObservations = append(result.SourceObservations, observation.Link())
		if observation.Status != sources.ObservationStatusSucceeded || observation.Completeness != sources.ObservationCompletenessComplete {
			result.Health = HealthDegraded
		}
	}
	combined := stderrors.Join(providerErr, sourceErr)
	if combined != nil {
		result.Health = HealthUnavailable
	}
	result.Published = result.Published || sourcePublished
	if result.Published {
		result.GenerationID = r.State().GenerationID
		report.Published, report.GenerationID = true, result.GenerationID
	}
	r.recordAcquisition(result, combined)
	report.Acquisition = result
	return combined
}
