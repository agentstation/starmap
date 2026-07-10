// Package pipeline owns catalog sync orchestration behind *starmap.Client.Sync.
package pipeline

import (
	"context"

	"github.com/google/uuid"

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
	Apply(context.Context, *catalogs.Builder, *pkgsync.Options, *differ.Changeset, []sources.Observation) (Publication, error)
}

// Publication identifies the durable generation produced by Apply.
type Publication struct {
	GenerationID string
	SyncRunID    string
}

type loadLocalFunc func(string) (*catalogs.Builder, error)
type sourcesFunc func(*pkgsync.Options, *catalogs.Catalog) []sources.Source
type resolveDependenciesFunc func(context.Context, []sources.Source, *pkgsync.Options) ([]sources.Source, error)
type cleanupFunc func(context.Context, []sources.Source) error
type observeFunc func(context.Context, []sources.Source, []sources.Option) ([]sources.Observation, error)
type reconcileFunc func(context.Context, *catalogs.Catalog, []sources.Observation) (*reconciler.Result, error)

// Pipeline executes catalog sync through source observation, reconciliation, and persistence.
type Pipeline struct {
	store Store

	loadLocal           loadLocalFunc
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
		loadLocal:           catalogs.NewLocal,
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

	if ctx == nil {
		ctx = context.Background()
	}
	if logging.RunID(ctx) == "" {
		runID, runErr := uuid.NewRandom()
		if runErr != nil {
			return nil, pkgerrors.WrapResource("generate", "source run ID", "", runErr)
		}
		ctx = logging.WithRunID(ctx, "source-run-"+runID.String())
	}

	options := pkgsync.Defaults().Apply(opts...)

	var cancel context.CancelFunc
	if options.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
	} else {
		cancel = func() {}
	}
	defer cancel()

	local, err := p.loadLocal(options.OutputPath)
	if err != nil {
		return nil, pkgerrors.WrapResource("load", "catalog", "local", err)
	}

	if err = options.Validate(local.Providers()); err != nil {
		return nil, err
	}

	localSnapshot, err := local.Build()
	if err != nil {
		return nil, pkgerrors.WrapResource("publish", "local catalog snapshot", "", err)
	}
	srcs := p.createSources(options, localSnapshot)

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

	observations, err := p.observe(ctx, srcs, options.SourceOptions())
	if err != nil {
		return nil, err
	}

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

	result, err := p.reconcile(ctx, existing, observations)
	if err != nil {
		return nil, err
	}

	logChanges(result)

	syncResult := pkgsync.ChangesetToResultWithProvenance(
		result.Changeset,
		options.DryRun,
		options.OutputPath,
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

	if shouldSave(options, result.Changeset) {
		changeset := result.Changeset
		if changeset == nil {
			changeset = &differ.Changeset{}
		}
		publication, err := p.store.Apply(ctx, result.Catalog, options, changeset, observations)
		if err != nil {
			return nil, err
		}
		syncResult.GenerationID = publication.GenerationID
		syncResult.SyncRunID = publication.SyncRunID
	}

	return syncResult, nil
}

func activeSourceIDs(observations []sources.Observation) []sources.ID {
	ids := make([]sources.ID, 0, len(observations))
	for _, observation := range observations {
		ids = append(ids, observation.SourceID)
	}
	return ids
}

func shouldSave(options *pkgsync.Options, changeset *differ.Changeset) bool {
	if options.Reformat || options.Fresh {
		if changeset == nil || !changeset.HasChanges() {
			logging.Info().
				Bool("reformat", options.Reformat).
				Bool("force", options.Fresh).
				Msg("Forcing save due to reformat/force flag")
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
