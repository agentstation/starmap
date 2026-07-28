// Package pipeline owns catalog sync orchestration behind *starmap.Client.Sync.
package pipeline

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/differ"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// Store is the catalog boundary required by the sync pipeline.
type Store interface {
	Catalog() (*catalogs.Catalog, error)
	Apply(
		context.Context,
		*catalogs.Builder,
		*pkgsync.Options,
		*differ.Changeset,
		[]sources.Observation,
		workspace.InputExpectation,
	) (Publication, error)
}

// Publication identifies the durable generation produced by Apply.
type Publication struct {
	GenerationID    string
	PayloadChecksum string
	SyncRunID       string
	Projection      *pkgsync.ProjectionResult
}

type loadWorkspaceFunc func(string) (*catalogs.Builder, error)
type loadEmbeddedFunc func() (*catalogs.Builder, error)
type sourcesFunc func(*pkgsync.Options, catalogInputs) []sources.Source
type resolveDependenciesFunc func(context.Context, []sources.Source, *pkgsync.Options) ([]sources.Source, error)
type cleanupFunc func(context.Context, []sources.Source) error
type observeFunc func(context.Context, []sources.Source, []sources.Option) ([]sources.Observation, error)
type reconcileFunc func(context.Context, *catalogs.Catalog, []sources.Observation) (*reconciler.Result, error)

// Pipeline executes catalog sync through source observation, reconciliation, and persistence.
type Pipeline struct {
	store Store

	loadWorkspace       loadWorkspaceFunc
	loadEmbedded        loadEmbeddedFunc
	createSources       sourcesFunc
	resolveDependencies resolveDependenciesFunc
	cleanup             cleanupFunc
	observe             observeFunc
	reconcile           reconcileFunc
}

// New creates a catalog sync pipeline with production dependencies.
func New(store Store) *Pipeline {
	return &Pipeline{
		store:               store,
		loadWorkspace:       loadHumanWorkspace,
		loadEmbedded:        catalogs.NewEmbedded,
		createSources:       filterSources,
		resolveDependencies: resolveDependencies,
		cleanup:             cleanup,
		observe:             observe,
		reconcile:           reconcile,
	}
}

// Sync synchronizes the catalog through source observation, reconciliation, and optional persistence.
func (p *Pipeline) Sync(ctx context.Context, opts ...pkgsync.Option) (*pkgsync.Result, error) {
	if p.store == nil {
		return nil, &pkgerrors.ConfigError{
			Component: "pipeline",
			Message:   "store is required",
		}
	}

	options := pkgsync.Defaults().Apply(opts...)
	if err := options.ValidateFilesystemLayout(); err != nil {
		return nil, err
	}
	ctx, cancel, err := prepareSyncContext(ctx, options.Timeout)
	if err != nil {
		return nil, err
	}
	defer cancel()

	workspaceInput, err := workspace.ObserveInput(options.CatalogPath)
	if err != nil {
		return nil, err
	}
	inputs, err := p.loadCatalogInputs(options.CatalogPath, workspaceInput)
	if err != nil {
		return nil, err
	}
	if err = options.Validate(inputs.providerConfig.Providers()); err != nil {
		return nil, err
	}

	srcs := p.createSources(options, inputs)

	srcs, err = p.resolveDependencies(ctx, srcs, options)
	if err != nil {
		return nil, err
	}

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), constants.SyncCleanupTimeout)
		defer cleanupCancel()

		if cleanupErr := p.cleanup(cleanupCtx, srcs); cleanupErr != nil {
			logging.Warn().Err(cleanupErr).Msg("Source cleanup errors occurred")
		}
	}()

	existing, err := p.store.Catalog()
	if err != nil {
		empty := catalogs.NewEmpty()
		existing, err = empty.Build()
		if err != nil {
			return nil, pkgerrors.WrapResource("publish", "empty baseline snapshot", "", err)
		}
		logging.Debug().Msg("No existing catalog found, using empty baseline")
	}
	if options.Fresh {
		empty := catalogs.NewEmpty()
		existing, err = empty.Build()
		if err != nil {
			return nil, pkgerrors.WrapResource("publish", "fresh baseline snapshot", "", err)
		}
		logging.Info().Msg("Fresh sync uses an empty reconciliation baseline")
	}

	observations, observeErr := p.observe(ctx, srcs, options.SourceOptions())
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if observeErr != nil && options.RequireAllSources {
		return nil, observeErr
	}
	if observeErr != nil {
		logging.Warn().
			Err(observeErr).
			Msg("Continuing with degraded source observations and last-known-good data")
	}
	observations, err = guardObservationHealth(existing, observations)
	if err != nil {
		return nil, pkgerrors.WrapResource("guard", "source observations", "", err)
	}
	if options.RequireAllSources {
		if err := requireHealthyObservations(srcs, observations); err != nil {
			return nil, err
		}
	}
	if options.Fresh && hasDegradedObservation(observations) {
		return nil, &pkgerrors.SyncError{
			Provider: "all",
			Err: &pkgerrors.ValidationError{
				Field:   "fresh",
				Message: "cannot publish from an empty baseline while any source observation is degraded or partial",
			},
		}
	}

	result, err := p.reconcile(ctx, existing, observations)
	if err != nil {
		return nil, err
	}

	logChanges(result)

	syncResult := pkgsync.ChangesetToResultWithProvenance(
		result.Changeset,
		options.DryRun,
		options.CatalogPath,
		result.ProviderAPICounts,
		result.ModelProviderMap,
		result.Provenance,
		activeSourceIDs(observations)...,
	)
	syncResult.Fresh = options.Fresh
	syncResult.SourceObservations = make([]catalogs.SourceObservationLink, 0, len(observations))
	for _, observation := range observations {
		syncResult.SourceObservations = append(syncResult.SourceObservations, observation.Link())
	}

	if options.DryRun {
		logging.Info().Bool("dry_run", true).Msg("Dry run completed - no changes applied")
		return syncResult, nil
	}

	if shouldSave(options, result.Changeset, workspaceInput.RequiresSeed()) {
		changeset := result.Changeset
		if changeset == nil {
			changeset = &differ.Changeset{}
		}
		publication, err := p.store.Apply(
			ctx,
			result.Catalog,
			options,
			changeset,
			observations,
			workspaceInput,
		)
		if err != nil {
			return nil, err
		}
		syncResult.GenerationID = publication.GenerationID
		syncResult.SyncRunID = publication.SyncRunID
		syncResult.Projection = publication.Projection
	}

	return syncResult, nil
}

func prepareSyncContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if logging.RunID(ctx) == "" {
		runID, err := uuid.NewRandom()
		if err != nil {
			return nil, nil, pkgerrors.WrapResource("generate", "source run ID", "", err)
		}
		ctx = logging.WithRunID(ctx, "source-run-"+runID.String())
	}
	if timeout <= 0 {
		return ctx, func() {}, nil
	}
	bounded, cancel := context.WithTimeout(ctx, timeout)
	return bounded, cancel, nil
}

func hasDegradedObservation(observations []sources.Observation) bool {
	for _, observation := range observations {
		if observation.Status != sources.ObservationStatusSucceeded ||
			observation.Completeness != sources.ObservationCompletenessComplete {
			return true
		}
	}
	return false
}

func activeSourceIDs(observations []sources.Observation) []sources.ID {
	ids := make([]sources.ID, 0, len(observations))
	for _, observation := range observations {
		ids = append(ids, observation.SourceID)
	}
	return ids
}

func shouldSave(options *pkgsync.Options, changeset *differ.Changeset, seedWorkspace bool) bool {
	if options.Reformat || options.Fresh || seedWorkspace {
		if changeset == nil || !changeset.HasChanges() {
			logging.Info().
				Bool("reformat", options.Reformat).
				Bool("force", options.Fresh).
				Bool("seed_workspace", seedWorkspace).
				Msg("Forcing save due to explicit materialization policy")
		}
		return true
	}

	return changeset != nil && changeset.HasChanges()
}

func logChanges(result *reconciler.Result) {
	if result.Changeset != nil && result.Changeset.HasChanges() {
		logging.Info().
			Int("added", result.Changeset.Summary.ModelsAdded).
			Int("updated", result.Changeset.Summary.ModelsUpdated).
			Int("removed", result.Changeset.Summary.ModelsRemoved).
			Msg("Changes detected")
		return
	}

	logging.Info().Msg("No changes detected")
}
