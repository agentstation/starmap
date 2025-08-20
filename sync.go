package starmap

import (
	"context"
	"fmt"

	"github.com/agentstation/starmap/internal/catalogs/operations"
	"github.com/agentstation/starmap/internal/catalogs/persistence"
	"github.com/agentstation/starmap/internal/sources/modelsdev"
	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
)

const (
	defaultSyncOutputDir = "internal/embedded/catalog/providers"
)

// Sync synchronizes the catalog with provider APIs
func (s *starmap) Sync(opts ...catalogs.SyncOption) (*catalogs.SyncResult, error) {
	options := catalogs.NewSyncOptions(opts...)

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

	// Setup models.dev integration
	var modelsDevAPI *modelsdev.ModelsDevAPI
	var modelsDevClient *modelsdev.Client

	// Determine output directory for models.dev
	outputDir := options.OutputDir
	if outputDir == "" {
		outputDir = defaultSyncOutputDir
	}

	// Initialize models.dev client
	modelsDevClient = modelsdev.NewClient(outputDir)

	// Setup models.dev repository
	if err := modelsDevClient.EnsureRepository(); err == nil {
		if err := modelsDevClient.BuildAPI(); err == nil {
			api, err := modelsdev.ParseAPI(modelsDevClient.GetAPIPath())
			if err == nil {
				modelsDevAPI = api
			}
		}
	}

	var allChanges []operations.ProviderChangeset

	// Process each provider
	for _, providerID := range providersToSync {
		providerResult := catalogs.NewSyncProviderResult(providerID)

		// Get provider from catalog
		provider, found := catalog.Providers().Get(providerID)
		if !found {
			continue
		}

		// Load API key from environment
		provider.LoadAPIKey()

		// Get client for this provider
		clientResult, err := provider.GetClient(catalogs.WithAllowMissingAPIKey(true))
		if err != nil || clientResult.Error != nil {
			continue
		}
		client := clientResult.Client

		// Fetch models from API
		ctx := context.Background()
		if options.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, options.Timeout)
			defer cancel()
		}

		apiModels, err := client.ListModels(ctx)
		if err != nil {
			continue
		}

		providerResult.APIModelsCount = len(apiModels)

		// Enhance API models with models.dev data
		if modelsDevAPI != nil {
			var enhancedCount int
			apiModels, enhancedCount = modelsdev.EnhanceModelsWithModelsDevData(apiModels, providerID, modelsDevAPI)
			providerResult.EnhancedCount = enhancedCount
		}

		// Handle fresh sync vs normal sync
		var changeset operations.ProviderChangeset
		if options.Fresh {
			// Fresh sync: treat all API models as new additions
			changeset = operations.ProviderChangeset{
				ProviderID: providerID,
				Added:      apiModels,
				Updated:    []operations.ModelUpdate{},
				Removed:    []catalogs.Model{},
			}
		} else {
			// Normal sync: compare with existing models
			existingModels, err := persistence.GetProviderModels(catalog, providerID)
			if err != nil {
				existingModels = make(map[string]catalogs.Model)
			}

			providerResult.ExistingModelsCount = len(existingModels)
			changeset = operations.CompareProviderModels(providerID, existingModels, apiModels)
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
	}

	// Apply changes if not dry run
	if !options.DryRun && result.HasChanges() {
		for _, changeset := range allChanges {
			if changeset.HasChanges() {
				// Clean provider directory if fresh sync
				if options.Fresh {
					if err := persistence.CleanProviderDirectory(changeset.ProviderID, options.OutputDir); err != nil {
						return nil, fmt.Errorf("cleaning directory for %s: %w", changeset.ProviderID, err)
					}
				}

				if err := persistence.ApplyChangesetToOutput(catalog, changeset, options.OutputDir); err != nil {
					return nil, fmt.Errorf("applying changes for %s: %w", changeset.ProviderID, err)
				}
			}
		}

		// Copy provider logos if models.dev is available
		if modelsDevClient != nil {
			modelsdev.CopyProviderLogos(modelsDevClient, outputDir, providersToSync)
		}

		// Update internal catalog with changes
		s.mu.Lock()
		// Reload catalog to reflect file changes
		if loadable, ok := s.catalog.(interface{ Load() error }); ok {
			loadable.Load()
		}
		s.mu.Unlock()
	}

	// Cleanup models.dev repository if requested
	if options.CleanModelsDevRepo && modelsDevClient != nil {
		modelsDevClient.Cleanup()
	}

	return result, nil
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
