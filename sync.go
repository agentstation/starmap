package starmap

import (
	"context"

	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/reconciler"
	"github.com/agentstation/starmap/pkg/save"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/pkg/sync"
)

// Sync synchronizes the catalog with provider APIs using staged source execution.
func (s *starmap) Sync(ctx context.Context, opts ...sync.Option) (*sync.Result, error) {

	// Step 0: Check and set context if nil
	if ctx == nil {
		ctx = context.Background()
	}

	// Step 1: Parse options with defaults
	options := sync.Defaults().Apply(opts...)

	// Step 2: Setup context with timeout
	var cancel context.CancelFunc
	if options.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
	} else {
		cancel = func() {} // No-op cancel if no timeout
	}
	defer cancel()

	// Step 3: Load embedded catalog for validation and base provider info
	embedded, err := catalogs.NewEmbedded()
	if err != nil {
		return nil, errors.WrapResource("load", "catalog", "embedded", err)
	}

	// Step 4: Validate options upfront with embedded catalog
	if err = options.Validate(embedded.Providers()); err != nil {
		return nil, err
	}

	// Step 5: filter sources by options
	srcs := s.filterSources(options)

	// Step 6: Cleanup sources
	defer func() {
		if cleanupErr := cleanup(srcs); cleanupErr != nil {
			logging.Warn().Err(cleanupErr).Msg("Source cleanup errors occurred")
		}
	}()

	// Step 7: Fetch catalogs from all sources
	if err = fetch(ctx, srcs, options.SourceOptions()); err != nil {
		return nil, err
	}

	// Step 8: Get existing catalog for baseline comparison
	existing, err := s.Catalog()
	if err != nil {
		// If we can't get existing catalog, use empty one
		existing, _ = catalogs.New()
		logging.Debug().Msg("No existing catalog found, using empty baseline")
	}

	// Step 9: Reconcile catalogs from all sources with baseline
	result, err := update(ctx, existing, srcs)
	if err != nil {
		return nil, err
	}

	// Step 10: Log change summary if changes detected
	if result.Changeset != nil && result.Changeset.HasChanges() {
		logging.Info().
			Int("added", len(result.Changeset.Models.Added)).
			Int("updated", len(result.Changeset.Models.Updated)).
			Int("removed", len(result.Changeset.Models.Removed)).
			Msg("Changes detected")
	} else {
		logging.Info().Msg("No changes detected")
	}

	// Step 11: Create sync result directly from reconciler's changeset
	syncResult := sync.ChangesetToResult(
		result.Changeset,
		options.DryRun,
		options.OutputPath,
		result.ProviderAPICounts,
		result.ModelProviderMap,
	)

	// Step 12: Apply changes if not dry run
	shouldSave := result.Changeset != nil && result.Changeset.HasChanges()

	// Force save if reformat or fresh flag is set (even without changes)
	if options.Reformat || options.Fresh {
		shouldSave = true
		if result.Changeset == nil || !result.Changeset.HasChanges() {
			logging.Info().
				Bool("reformat", options.Reformat).
				Bool("force", options.Fresh).
				Msg("Forcing save due to reformat/force flag")
		}
	}

	if !options.DryRun && shouldSave {
		// Create empty changeset if nil but we're forcing save
		changeset := result.Changeset
		if changeset == nil {
			changeset = &differ.Changeset{}
		}
		if err := s.save(result.Catalog, options, changeset); err != nil {
			return nil, err
		}
	} else if options.DryRun {
		logging.Info().Bool("dry_run", true).Msg("Dry run completed - no changes applied")
	}

	return syncResult, nil
}

// update reconciles the catalog with an optional baseline for comparison.
func update(ctx context.Context, baseline catalogs.Catalog, srcs []sources.Source) (*reconciler.Result, error) {

	// create reconciler options
	opts := []reconciler.Option{
		reconciler.WithStrategy(reconciler.NewAuthorityStrategy(authority.New())),
	}

	// Add baseline if provided
	if baseline != nil {
		opts = append(opts, reconciler.WithBaseline(baseline))
	}

	// create a new reconciler
	reconcile, err := reconciler.New(opts...)
	if err != nil {
		return nil, errors.WrapResource("create", "reconciler", "", err)
	}

	// reconcile the sources catalogs into a single result
	result, err := reconcile.Sources(ctx, sources.ProvidersID, srcs)
	if err != nil {
		return nil, &errors.SyncError{
			Provider: "all",
			Err:      err,
		}
	}

	return result, nil
}

// ============================================================================
// Helper Methods for Sync
// ============================================================================

// save applies the catalog changes if not in dry-run mode.
func (s *starmap) save(result catalogs.Catalog, options *sync.Options, changeset *differ.Changeset) error {

	// Update internal catalog first
	s.mu.Lock()
	oldCatalog := s.catalog
	s.catalog = result
	s.mu.Unlock()

	// Save to output path if specified
	if options.OutputPath != "" {
		// Debug: check what providers have models
		providers := result.Providers().List()
		for _, p := range providers {
			modelCount := 0
			if p.Models != nil {
				modelCount = len(p.Models)
			}
			logging.Info().
				Str("provider", string(p.ID)).
				Int("models", modelCount).
				Msg("Provider model count before save")
		}

		if saveable, ok := result.(catalogs.Persistence); ok {
			if err := saveable.Save(save.WithPath(options.OutputPath)); err != nil {
				return errors.WrapIO("write", options.OutputPath, err)
			}

			// Copy models.dev logos after successful save
			// Extract provider IDs from the saved catalog
			providerIDs := make([]catalogs.ProviderID, 0)
			for _, p := range providers {
				if p.ID != "" {
					providerIDs = append(providerIDs, p.ID)
				}
			}

			// Copy logos if we have providers and an output path
			if len(providerIDs) > 0 {
				logging.Debug().
					Int("provider_count", len(providerIDs)).
					Str("output_path", options.OutputPath).
					Msg("Copying provider logos from models.dev")

				if logoErr := modelsdev.CopyProviderLogos(options.OutputPath, providerIDs); logoErr != nil {
					logging.Warn().
						Err(logoErr).
						Msg("Could not copy provider logos")
					// Non-fatal error - continue without logos
				}
			}
		}
	} else {
		// Save to default location
		if saveable, ok := result.(catalogs.Persistence); ok {
			if err := saveable.Save(save.WithPath(options.OutputPath)); err != nil {
				return errors.WrapIO("write", "catalog", err)
			}
		}
	}

	logging.Info().
		Int("changes_applied", changeset.Summary.TotalChanges).
		Msg("Sync completed successfully")

	// Trigger hooks for catalog changes
	s.hooks.triggerUpdate(oldCatalog, result)

	return nil
}
