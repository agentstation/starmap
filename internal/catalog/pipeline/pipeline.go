// Package pipeline owns acquisition orchestration behind the public
// acquisition package.
package pipeline

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/agentstation/starmap/internal/catalog/reconciler"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// Prepared is one complete acquisition result ready for optional publication.
// It remains internal because callers compose publication through starmap.Client.
type Prepared struct {
	Result         *pkgsync.Result
	Catalog        *catalogs.Builder
	Changeset      *differ.Changeset
	Observations   []sources.Observation
	Options        *pkgsync.Options
	WorkspaceInput workspace.InputExpectation
	Publish        bool
}

// Store is retained as an internal test adapter while acquisition migrates to
// Prepare. Production composition publishes through starmap.Client.Update.
type Store interface {
	Catalog() (*catalogs.Catalog, error)
	Apply(
		context.Context,
		*catalogs.Builder,
		*pkgsync.Options,
		*differ.Changeset,
		[]sources.Observation,
		[]catalogmeta.ReviewCandidate,
		workspace.InputExpectation,
	) (Publication, error)
}

// Publication identifies a generation produced by the internal Store adapter.
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
	store               Store
	loadWorkspace       loadWorkspaceFunc
	loadEmbedded        loadEmbeddedFunc
	createSources       sourcesFunc
	resolveDependencies resolveDependenciesFunc
	cleanup             cleanupFunc
	observe             observeFunc
	reconcile           reconcileFunc
}

// New creates the internal Store-backed adapter used by pipeline tests.
func New(store Store) *Pipeline {
	pipeline := newPipeline(nil, nil)
	pipeline.store = store
	return pipeline
}

// NewAcquisition creates a prepare-only pipeline with the provider factory
// injected by the opt-in acquisition composition.
func NewAcquisition(
	providerFactory sources.ProviderClientFactory,
	credentialResolver sources.ProviderCredentialResolver,
) *Pipeline {
	return newPipeline(providerFactory, credentialResolver)
}

func newPipeline(
	providerFactory sources.ProviderClientFactory,
	credentialResolver sources.ProviderCredentialResolver,
) *Pipeline {
	return &Pipeline{
		loadWorkspace: loadHumanWorkspace,
		loadEmbedded:  catalogs.NewEmbedded,
		createSources: func(options *pkgsync.Options, inputs catalogInputs) []sources.Source {
			return filterSources(options, inputs, providerSourceComposition{
				clientFactory:      providerFactory,
				credentialResolver: credentialResolver,
			})
		},
		resolveDependencies: resolveDependencies,
		cleanup:             cleanup,
		observe:             observe,
		reconcile:           reconcile,
	}
}

// Sync exercises the historical internal Store adapter through Prepare. New
// production callers use the explicit acquisition package.
func (p *Pipeline) Sync(ctx context.Context, opts ...pkgsync.Option) (*pkgsync.Result, error) {
	if p == nil || p.store == nil {
		return nil, &pkgerrors.ConfigError{
			Component: "pipeline",
			Message:   "store is required",
		}
	}
	existing, err := p.store.Catalog()
	if err != nil {
		return nil, err
	}
	options := pkgsync.Defaults().Apply(opts...)
	bounded, cancel, err := prepareSyncContext(ctx, options.Timeout)
	if err != nil {
		return nil, err
	}
	defer cancel()
	prepared, err := p.Prepare(bounded, existing, opts...)
	if err != nil {
		return nil, err
	}
	if !prepared.Publish {
		return prepared.Result, nil
	}
	changeset := prepared.Changeset
	if changeset == nil {
		changeset = &differ.Changeset{}
	}
	publication, err := p.store.Apply(
		ctx,
		prepared.Catalog,
		prepared.Options,
		changeset,
		prepared.Observations,
		prepared.Result.ReviewCandidates,
		prepared.WorkspaceInput,
	)
	if err != nil {
		return nil, err
	}
	prepared.Result.GenerationID = publication.GenerationID
	prepared.Result.SyncRunID = publication.SyncRunID
	prepared.Result.Projection = publication.Projection
	return prepared.Result, nil
}

// Prepare observes and reconciles sources against existing without mutating
// shared state. Publication remains the root client's responsibility.
func (p *Pipeline) Prepare(
	ctx context.Context,
	existing *catalogs.Catalog,
	opts ...pkgsync.Option,
) (*Prepared, error) {
	if p == nil {
		return nil, &pkgerrors.ValidationError{
			Field:   "pipeline",
			Message: "is required",
		}
	}
	if existing == nil {
		return nil, &pkgerrors.ValidationError{
			Field:   "pipeline.existing_catalog",
			Message: "is required",
		}
	}

	options := pkgsync.Defaults().Apply(opts...)
	if err := options.ValidateFilesystemLayout(); err != nil {
		return nil, err
	}
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
	syncResult.ReviewCandidates = append(
		[]catalogmeta.ReviewCandidate(nil),
		result.ReviewCandidates...,
	)

	if options.DryRun {
		logging.Info().Bool("dry_run", true).Msg("Dry run completed - no changes applied")
		return &Prepared{
			Result:         syncResult,
			Catalog:        result.Catalog,
			Changeset:      result.Changeset,
			Observations:   observations,
			Options:        options,
			WorkspaceInput: inputs.workspaceInput,
		}, nil
	}

	return &Prepared{
		Result:         syncResult,
		Catalog:        result.Catalog,
		Changeset:      result.Changeset,
		Observations:   observations,
		Options:        options,
		WorkspaceInput: inputs.workspaceInput,
		Publish: shouldPublish(
			options,
			result.Changeset,
			result.ReviewCandidates,
			inputs.workspaceInput.RequiresSeed(),
		),
	}, nil
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

func shouldPublish(
	options *pkgsync.Options,
	changeset *differ.Changeset,
	reviewCandidates []catalogmeta.ReviewCandidate,
	seedWorkspace bool,
) bool {
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
	if len(reviewCandidates) > 0 {
		logging.Info().
			Int("review_candidates", len(reviewCandidates)).
			Msg("Publishing durable review candidates without catalog changes")
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
