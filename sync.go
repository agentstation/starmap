package starmap

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// Sync synchronizes the catalog with provider APIs using staged source execution
func (s *starmap) Sync(ctx context.Context, opts ...SyncOption) (*SyncResult, error) {
	// Step 0: Set context
	if ctx == nil {
		ctx = context.Background()
	}

	// Step 1: Parse options
	options := NewSyncOptions(opts...)

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

	// Step 6: Setup sources with provider configurations
	if err = setup(srcs, embedded.Providers()); err != nil {
		return nil, err
	}
	defer func() {
		if cleanupErr := cleanup(srcs); cleanupErr != nil {
			logging.Warn().Err(cleanupErr).Msg("Source cleanup errors occurred")
		}
	}()

	// Step 7: Fetch catalogs from all sources
	if err = fetch(ctx, srcs, options.SourceOptions()); err != nil {
		return nil, err
	}

	// Step 8: Reconcile catalogs from all sources
	result, err := update(ctx, srcs)
	if err != nil {
		return nil, err
	}

	// Step 9: Detect changes
	changeset, err := s.diff(result.Catalog)
	if err != nil {
		return nil, err
	}

	// Step 10: Create sync result
	syncResult := convertChangesetToSyncResult(changeset, options.DryRun, options.OutputPath, result.ProviderAPICounts, result.ModelProviderMap)

	// Step 11: Apply changes if not dry run
	if !options.DryRun && changeset.HasChanges() {
		if err := s.save(result.Catalog, options, changeset); err != nil {
			return nil, err
		}
	} else if options.DryRun {
		logging.Info().Bool("dry_run", true).Msg("Dry run completed - no changes applied")
	}

	return syncResult, nil
}

// ============================================================================
// Helper Methods for Sync
// ============================================================================

// diff compares the result catalog with the existing catalog
func (s *starmap) diff(result catalogs.Catalog) (*differ.Changeset, error) {
	// Get existing catalog for comparison
	existing, err := s.Catalog()
	if err != nil {
		// If we can't get existing catalog, create an empty one for comparison
		existing, _ = catalogs.New()
	}

	// Perform change detection using differ
	differ := differ.New()
	changeset := differ.Catalogs(existing, result)

	// Log summary
	if changeset.HasChanges() {
		logging.Info().
			Int("added", len(changeset.Models.Added)).
			Int("updated", len(changeset.Models.Updated)).
			Int("removed", len(changeset.Models.Removed)).
			Msg("Changes detected")
	} else {
		logging.Info().Msg("No changes detected")
	}

	return changeset, nil
}

// save applies the catalog changes if not in dry-run mode
func (s *starmap) save(result catalogs.Catalog, options *SyncOptions, changeset *differ.Changeset) error {
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

		if saveable, ok := result.(catalogs.Persistable); ok {
			if err := saveable.SaveTo(options.OutputPath); err != nil {
				return errors.WrapIO("write", options.OutputPath, err)
			}
		}
	} else {
		// Save to default location
		if saveable, ok := result.(catalogs.Persistable); ok {
			if err := saveable.Save(); err != nil {
				return errors.WrapIO("write", "catalog", err)
			}
		}
	}

	logging.Info().
		Int("changes_applied", changeset.Summary.TotalChanges).
		Msg("Sync completed successfully")

	// Trigger hooks for catalog changes
	s.hooks.triggerCatalogUpdate(oldCatalog, result)

	return nil
}
