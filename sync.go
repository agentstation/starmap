package starmap

import (
	"context"
	"fmt"
	"os"

	"github.com/agentstation/starmap/internal/catalogs/operations"
	"github.com/agentstation/starmap/internal/catalogs/persistence"
	_ "github.com/agentstation/starmap/internal/sources" // Auto-register sources
	sourceops "github.com/agentstation/starmap/internal/sources/operations"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/internal/syncsrc"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

const (
	defaultSyncOutputDir = "internal/embedded/catalog/providers"
)

// Sync synchronizes the catalog with provider APIs using the source pipeline system
func (s *starmap) Sync(opts ...sources.SyncOption) (*catalogs.SyncResult, error) {
	// Always use the new pipeline system now
	return s.syncWithPipeline(opts...)
}

// syncWithPipeline is the internal implementation using the new pipeline system
func (s *starmap) syncWithPipeline(opts ...sources.SyncOption) (*catalogs.SyncResult, error) {
	options := sources.NewSyncOptions(opts...)

	// Get current catalog
	catalog, err := s.Catalog()
	if err != nil {
		return nil, fmt.Errorf("getting catalog: %w", err)
	}

	// Determine which providers to sync
	var providersToSync []catalogs.ProviderID
	if options.ProviderID != nil {
		providersToSync = []catalogs.ProviderID{*options.ProviderID}
	} else {
		providersToSync = providers.ListSupportedProviders()
	}

	// Initialize result
	result := catalogs.NewSyncResult()
	result.DryRun = options.DryRun
	result.Fresh = options.Fresh
	result.OutputDir = options.OutputDir

	// Sources self-initialize - no manual setup needed

	var allChanges []operations.ProviderChangeset

	// Process each provider using the pipeline
	for _, providerID := range providersToSync {
		providerResult := catalogs.NewSyncProviderResult(providerID)

		// Get provider from catalog
		provider, found := catalog.Providers().Get(providerID)
		if !found {
			continue
		}

		// Load API key and environment variables from environment
		provider.LoadAPIKey()
		provider.LoadEnvVars()

		// Create pipeline for this provider using auto-registered sources
		sourcePipeline, err := syncsrc.Build(catalog,
			syncsrc.WithProvider(provider),
			syncsrc.WithSyncOptions(options),
		)
		if err != nil {
			// Log error but continue with next provider
			continue
		}

		// Execute pipeline
		ctx := context.Background()
		if options.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, options.Timeout)
			defer cancel()
		}

		pipelineResult, err := sourcePipeline.Execute(ctx, providerID)
		if err != nil {
			// Log error but continue with next provider
			continue
		}

		// Update provider result with pipeline statistics
		providerResult.APIModelsCount = 0
		providerResult.EnhancedCount = 0

		// Calculate statistics from pipeline
		for sourceType, stats := range pipelineResult.SourceStats {
			switch sourceType {
			case sources.ProviderAPI:
				providerResult.APIModelsCount = stats.ModelsReturned
			case sources.ModelsDevGit, sources.ModelsDevHTTP:
				providerResult.EnhancedCount = stats.ModelsReturned
			}
		}

		// Handle fresh sync vs normal sync
		var changeset operations.ProviderChangeset
		if options.Fresh {
			// Fresh sync: treat all pipeline models as new additions
			changeset = operations.ProviderChangeset{
				ProviderID: providerID,
				Added:      pipelineResult.Models,
				Updated:    []operations.ModelUpdate{},
				Removed:    []catalogs.Model{},
			}
		} else {
			// Normal sync: compare pipeline result with existing models
			existingModels, err := persistence.GetProviderModels(catalog, providerID)
			if err != nil {
				existingModels = make(map[string]catalogs.Model)
			}

			providerResult.ExistingModelsCount = len(existingModels)
			changeset = operations.CompareProviderModels(providerID, existingModels, pipelineResult.Models)
		}

		allChanges = append(allChanges, changeset)

		// Convert internal changeset to public result
		providerResult.AddedCount = len(changeset.Added)
		providerResult.UpdatedCount = len(changeset.Updated)
		providerResult.RemovedCount = len(changeset.Removed)

		// Convert models and updates to public types
		providerResult.Added = changeset.Added
		providerResult.Removed = changeset.Removed
		for _, update := range changeset.Updated {
			providerResult.Updated = append(providerResult.Updated, catalogs.ModelUpdate{
				ModelID:       update.ModelID,
				ExistingModel: update.ExistingModel,
				NewModel:      update.NewModel,
				Changes:       convertFieldChanges(update.Changes),
			})
		}

		result.ProviderResults[providerID] = providerResult
		if providerResult.HasChanges() {
			result.ProvidersChanged++
			result.TotalChanges += providerResult.AddedCount + providerResult.UpdatedCount + providerResult.RemovedCount
		}

		// Save provenance if requested
		if err := s.saveProvenance(options, pipelineResult.Provenance, providerID); err != nil {
			// Log error but don't fail the sync
		}
	}

	// Apply changes if not dry run
	if !options.DryRun && result.HasChanges() {
		for _, changeset := range allChanges {
			if changeset.HasChanges() {
				// Clean provider directory if fresh sync
				if options.Fresh {
					outputDir := options.OutputDir
					if outputDir == "" {
						outputDir = defaultSyncOutputDir
					}
					if err := persistence.CleanProviderDirectory(changeset.ProviderID, outputDir); err != nil {
						return nil, fmt.Errorf("cleaning directory for %s: %w", changeset.ProviderID, err)
					}
				}

				outputDir := options.OutputDir
				if outputDir == "" {
					outputDir = defaultSyncOutputDir
				}
				if err := persistence.ApplyChangesetToOutput(catalog, changeset, outputDir); err != nil {
					return nil, fmt.Errorf("applying changes for %s: %w", changeset.ProviderID, err)
				}
			}
		}

		// Handle post-sync operations (logo copying)
		// Only Git source can provide logos; HTTP source will show informational message
		httpOps := sourceops.GetPostSyncOperations(sources.ModelsDevHTTP)
		gitOps := sourceops.GetPostSyncOperations(sources.ModelsDevGit)
		
		if gitOps != nil {
			// Git source available - can copy logos
			if err := gitOps.CopyProviderLogos(providersToSync); err != nil {
				// Log error but don't fail the sync
			}
		} else if httpOps != nil {
			// Only HTTP source available - will inform about logo limitation
			if err := httpOps.CopyProviderLogos(providersToSync); err != nil {
				// Log error but don't fail the sync
			}
		}

		// Update internal catalog with changes and trigger hooks
		s.mu.Lock()
		// Reload catalog to reflect file changes
		if loadable, ok := s.catalog.(interface{ Load() error }); ok {
			oldCatalog := s.catalog
			loadable.Load()
			// Trigger hooks for catalog changes
			s.hooks.triggerCatalogUpdate(oldCatalog, s.catalog)
		}
		s.mu.Unlock()
	}

	// Cleanup models.dev repository/cache if requested
	if options.CleanModelsDevRepo {
		// Clean both HTTP cache and Git repo
		if postSyncOps := sourceops.GetPostSyncOperations(sources.ModelsDevHTTP); postSyncOps != nil {
			postSyncOps.Cleanup()
		}
		if postSyncOps := sourceops.GetPostSyncOperations(sources.ModelsDevGit); postSyncOps != nil {
			postSyncOps.Cleanup()
		}
	}

	return result, nil
}

// saveProvenance saves provenance information to file if requested
func (s *starmap) saveProvenance(options *sources.SyncOptions, provenance map[string]sources.Provenance, providerID catalogs.ProviderID) error {
	if !options.TrackProvenance || options.ProvenanceFile == "" {
		return nil
	}

	// Create a simple provenance report
	// This is a basic implementation - could be enhanced with YAML/JSON formatting
	file, err := os.Create(options.ProvenanceFile)
	if err != nil {
		return fmt.Errorf("creating provenance file: %w", err)
	}
	defer file.Close()

	fmt.Fprintf(file, "# Provenance Report for %s\n", providerID)
	fmt.Fprintf(file, "# Generated by Starmap\n\n")

	for field, prov := range provenance {
		fmt.Fprintf(file, "%s: %s (updated: %s)\n", field, prov.Source, prov.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}

// convertFieldChanges converts internal field changes to public types
func convertFieldChanges(changes []operations.FieldChange) []catalogs.FieldChange {
	result := make([]catalogs.FieldChange, len(changes))
	for i, change := range changes {
		result[i] = catalogs.FieldChange{
			Field:    change.Field,
			OldValue: change.OldValue,
			NewValue: change.NewValue,
		}
	}
	return result
}
