package acquisition

import (
	"context"
	"slices"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/pipeline"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/runtime"
)

// NewForRuntime composes explicit acquisition with retained runtime publication.
// Construction reads no source. The caller owns the connected runtime's lifecycle.
func NewForRuntime(connected *runtime.Runtime, opts ...Option) (*Syncer, error) {
	if connected == nil {
		return nil, &errors.ValidationError{Field: "acquisition.runtime", Message: "is required"}
	}
	syncer, err := New(connected.Client(), opts...)
	if err != nil {
		return nil, err
	}
	syncer.connected = connected
	return syncer, nil
}

func (s *Syncer) syncRuntime(ctx context.Context, effective []pkgsync.Option, parsed *pkgsync.Options) (*pkgsync.Result, error) {
	if selected, present := s.connected.AcquisitionSources(); present {
		requested := slices.Clone(parsed.Sources)
		if len(requested) == 0 {
			requested = selected
		}
		if len(requested) == 0 {
			return nil, &errors.ConfigError{Component: "acquisition", Message: "no acquisition sources are enabled"}
		}
		for _, id := range requested {
			if id != sources.EmbeddedCatalogID && id != sources.ReleaseArtifactID && !slices.Contains(selected, id) {
				return nil, &errors.ConfigError{Component: "acquisition", Message: "requested source is excluded by the runtime source selection"}
			}
		}
		effective = append(slices.Clone(effective), pkgsync.WithSources(requested...))
		parsed = pkgsync.Defaults().Apply(effective...)
	}
	if parsed.ModelsDevGitCommit == "" && slices.Contains(parsed.Sources, sources.ModelsDevGitID) {
		if commit, present := s.connected.ModelsDevGitCommit(); present {
			effective = append(slices.Clone(effective), pkgsync.WithModelsDevGitCommit(commit))
			parsed = pkgsync.Defaults().Apply(effective...)
		}
	}
	if parsed.Fresh {
		effective = append(slices.Clone(effective), pkgsync.WithRequireAllSources(true))
	}
	var prepared *pipeline.Prepared
	var before *catalogs.Catalog
	var update runtime.ObservationUpdate
	prepare := func(ctx context.Context, inputs runtime.ObservationInputs) (runtime.ObservationUpdate, error) {
		before = inputs.Current.Catalog
		baseline := before
		if parsed.Fresh {
			baseline = inputs.Baseline.Catalog
		}
		var err error
		prepared, err = s.pipeline.Prepare(ctx, baseline, effective...)
		if err != nil {
			return runtime.ObservationUpdate{}, err
		}
		for _, observation := range prepared.Observations {
			if observation.SourceID != sources.EmbeddedCatalogID && observation.SourceID != sources.ReleaseArtifactID {
				update.Observations = append(update.Observations, observation)
			}
		}
		if parsed.Fresh {
			update.Resets = acquisitionResets(update.Observations, parsed, inputs.Baseline.Catalog)
		}
		return update, nil
	}
	var state starmap.CatalogState
	var err error
	if parsed.DryRun {
		state, err = s.connected.PreviewAcquisition(ctx, prepare)
	} else {
		state, err = s.connected.UpdateAcquisition(ctx, prepare)
	}
	if err != nil {
		return nil, err
	}
	if prepared == nil {
		return nil, &errors.ValidationError{Field: "acquisition.result", Message: "preparation did not complete"}
	}
	changes := differ.New().Catalogs(before, state.Catalog)
	counts := make(map[catalogs.ProviderID]int)
	for _, observation := range update.Observations {
		if observation.SourceID == sources.ProvidersID {
			for _, provider := range observation.Catalog.Providers().List() {
				counts[provider.ID] += len(provider.Models)
			}
		}
	}
	models := make(map[string]catalogs.ProviderID)
	for _, provider := range state.Catalog.Providers().List() {
		for id := range provider.Models {
			models[id] = provider.ID
		}
	}
	result := pkgsync.ChangesetToResultWithProvenance(changes, parsed.DryRun, parsed.CatalogPath, counts, models, state.Catalog.Provenance().Map(), prepared.Result.Sources...)
	result.Fresh, result.ResetCount = parsed.Fresh, len(update.Resets)
	result.SourceObservations = prepared.Result.SourceObservations
	result.Partial = prepared.Result.Partial
	result.SourceFailures = slices.Clone(prepared.Result.SourceFailures)
	result.SourceActivities = slices.Clone(prepared.Result.SourceActivities)
	result.ProviderAttempts = slices.Clone(prepared.Result.ProviderAttempts)
	result.ReviewCandidates = prepared.Result.ReviewCandidates
	result.SyncRunID = logging.RunID(ctx)
	if !parsed.DryRun {
		result.GenerationID = state.GenerationID
		if prepared.Options.CatalogPath != "" {
			result.Projection = projectCommittedCatalog(ctx, state.Catalog, prepared.Options.CatalogPath, starmap.Publication{Published: true, GenerationID: state.GenerationID, PayloadChecksum: state.PayloadChecksum, SyncRunID: result.SyncRunID}, prepared.WorkspaceInput, prepared.Options)
		}
	}
	return result, nil
}

func acquisitionResets(observations []sources.Observation, options *pkgsync.Options, baseline *catalogs.Catalog) []runtime.ObservationReset {
	var resets []runtime.ObservationReset
	for _, observation := range observations {
		switch observation.SourceID {
		case sources.ProvidersID:
			for _, provider := range observation.Catalog.Providers().List() {
				reset := runtime.ObservationReset{ProviderID: provider.ID}
				if binding := observation.ProviderBinding; binding != nil {
					reset.BindingID, reset.BindingRevision = binding.ID, binding.Revision
				}
				resets = append(resets, reset)
			}
		case sources.ModelsDevHTTPID, sources.ModelsDevGitID:
			if options.ProviderID == nil {
				resets = append(resets, runtime.ObservationReset{SourceID: observation.SourceID})
				continue
			}
			providers := []catalogs.ProviderID{*options.ProviderID}
			if provider, err := baseline.Provider(*options.ProviderID); err == nil {
				providers = append(providers, provider.Aliases...)
			}
			slices.Sort(providers)
			for _, provider := range slices.Compact(providers) {
				resets = append(resets, runtime.ObservationReset{SourceID: observation.SourceID, ProviderID: provider})
			}
		}
	}
	return resets
}
