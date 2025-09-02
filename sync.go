package starmap

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/reconcile"
	"github.com/agentstation/starmap/pkg/sources"
)

// Sync synchronizes the catalog with provider APIs using staged source execution
func (s *starmap) Sync(ctx context.Context, opts ...SyncOption) (*SyncResult, error) {
	// Build new sync options
	options := NewSyncOptions(opts...)
	if ctx == nil {
		ctx = context.Background()
	}

	// Create the result in memory catalog
	result, err := catalogs.New()
	if err != nil {
		return nil, errors.WrapResource("create", "catalog", "result", err)
	}

	// Create a context with a timeout if specified
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	// Get sources to use
	sourcesToUse := s.sourcesWithOptions(options)

	// SETUP PHASE: Initialize sources with dependencies
	// First, get the embedded catalog for provider configs
	embeddedCat, err := catalogs.New(catalogs.WithEmbedded())
	if err != nil {
		return nil, errors.WrapResource("load", "catalog", "embedded", err)
	}

	// Setup all sources with provider configs
	for _, source := range sourcesToUse {
		if err := source.Setup(embeddedCat.Providers()); err != nil {
			return nil, errors.WrapResource("setup", "source", string(source.Name()), err)
		}
	}

	// FETCH PHASE: Fetch catalogs from each source
	// Convert sync options to source options
	var sourceOpts []sources.SourceOption
	if options.ProviderID != nil {
		sourceOpts = append(sourceOpts, sources.WithProviderFilter(*options.ProviderID))
	}
	// Extract source-specific flags from context
	if options.Context != nil {
		if fresh, ok := options.Context["fresh"].(bool); ok && fresh {
			sourceOpts = append(sourceOpts, sources.WithFresh(true))
		}
		if safeMode, ok := options.Context["safeMode"].(bool); ok && safeMode {
			sourceOpts = append(sourceOpts, sources.WithSafeMode(true))
		}
		// Pass through all context for source-specific needs
		for k, v := range options.Context {
			sourceOpts = append(sourceOpts, sources.WithSourceContext(k, v))
		}
	}

	// Collect catalogs from all sources
	sourceCatalogs := make(map[reconcile.SourceName]catalogs.Catalog)
	logger := logging.FromContext(ctx)
	
	// Track model counts from Provider APIs for accurate reporting
	providerAPICounts := make(map[catalogs.ProviderID]int)
	
	// IMPORTANT: Include the embedded catalog as "Local Catalog" source to preserve all provider fields
	// This ensures fields like APIKey, StatusPageURL, PrivacyPolicy, etc. are not lost
	// The authorities are configured to use LocalCatalog as the source for provider configuration fields
	sourceCatalogs[reconcile.LocalCatalog] = embeddedCat
	
	for _, source := range sourcesToUse {
		logger.Info().Str("source", string(source.Name())).Msg("Fetching")

		// Fetch catalog from source
		catalog, err := source.Fetch(ctx, sourceOpts...)
		if err != nil {
			if options.FailFast {
				return nil, &errors.SyncError{
					Provider: string(source.Name()),
					Err:      err,
				}
			}
			logger.Warn().Err(err).Str("source", string(source.Name())).Msg("Source fetch had errors")
			// Don't skip if we still got a catalog (partial success)
			if catalog == nil {
				logger.Warn().Str("source", string(source.Name())).Msg("No catalog returned, skipping source")
				continue
			}
		}

		sourceCatalogs[reconcile.SourceName(source.Name())] = catalog
		
		// Track model counts from Provider APIs
		if source.Name() == sources.ProviderAPI {
			// Count models per provider from the API fetch
			// We need to use the provider.Models map to properly track association
			for _, provider := range catalog.Providers().List() {
				if provider.Models != nil {
					providerAPICounts[provider.ID] = len(provider.Models)
				}
			}
		}
		
		// Debug: log provider count from this source
		providerCount := len(catalog.Providers().List())
		modelCount := len(catalog.Models().List())
		logger.Debug().
			Str("source", string(source.Name())).
			Int("providers", providerCount).
			Int("models", modelCount).
			Msg("Added source catalog to reconciliation")
	}

	// RECONCILE PHASE: Use the new reconciler to merge catalogs
	reconciler, err := reconcile.New(
		reconcile.WithStrategy(reconcile.NewAuthorityBasedStrategy(reconcile.NewDefaultAuthorityProvider())),
		reconcile.WithProvenance(true),
	)
	if err != nil {
		return nil, errors.WrapResource("create", "reconciler", "", err)
	}

	reconcileResult, err := reconciler.ReconcileCatalogs(ctx, reconcile.SourceName(sources.ProviderAPI), sourceCatalogs)
	if err != nil {
		return nil, &errors.SyncError{
			Provider: "all",
			Err:      err,
		}
	}

	result = reconcileResult.Catalog
	
	// Build model-to-provider mapping from the reconciled catalog
	// This ensures we properly track which provider owns each model
	modelProviderMap := make(map[string]catalogs.ProviderID)
	for _, provider := range result.Providers().List() {
		if provider.Models != nil {
			for modelID := range provider.Models {
				modelProviderMap[modelID] = provider.ID
			}
		}
	}

	// CLEANUP PHASE: Clean up any resources
	for _, source := range sourcesToUse {
		if err := source.Cleanup(); err != nil {
			logger.Warn().
				Err(err).
				Str("source", string(source.Name())).
				Msg("Cleanup failed")
		}
	}

	// Get existing catalog for comparison
	existing, err := s.Catalog()
	if err != nil {
		// If we can't get existing catalog, create an empty one for comparison
		existing, _ = catalogs.New()
	}

	// Perform change detection using new differ
	differ := reconcile.NewDiffer()
	changeset := differ.DiffCatalogs(existing, result)

	// Create sync result from new changeset
	syncResult := convertChangesetToSyncResult(changeset, options.DryRun, options.OutputPath, providerAPICounts, modelProviderMap)

	// Log summary
	if changeset.HasChanges() {
		logger.Info().
			Int("added", len(changeset.Models.Added)).
			Int("updated", len(changeset.Models.Updated)).
			Int("removed", len(changeset.Models.Removed)).
			Msg("Changes detected")
	} else {
		logger.Info().Msg("No changes detected")
	}

	// Apply changes if not dry run
	if !options.DryRun && changeset.HasChanges() {
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
				logger.Info().
					Str("provider", string(p.ID)).
					Int("models", modelCount).
					Msg("Provider model count before save")
			}
			
			if saveable, ok := result.(catalogs.Persistable); ok {
				if err := saveable.SaveTo(options.OutputPath); err != nil {
					return nil, errors.WrapIO("write", options.OutputPath, err)
				}
			}
		} else {
			// Save to default location
			if saveable, ok := result.(catalogs.Persistable); ok {
				if err := saveable.Save(); err != nil {
					return nil, errors.WrapIO("write", "catalog", err)
				}
			}
		}

		logger.Info().
			Int("changes_applied", changeset.Summary.TotalChanges).
			Msg("Sync completed successfully")

		// Trigger hooks for catalog changes
		s.hooks.triggerCatalogUpdate(oldCatalog, result)
	} else if options.DryRun {
		logger.Info().Bool("dry_run", true).Msg("Dry run completed - no changes applied")
	}

	return syncResult, nil
}

// SyncResult represents the complete result of a sync operation
type SyncResult struct {
	// Overall statistics
	TotalChanges     int                                         // Total number of changes across all providers
	ProvidersChanged int                                         // Number of providers with changes
	ProviderResults  map[catalogs.ProviderID]*SyncProviderResult // Results per provider

	// Operation metadata
	DryRun    bool   // Whether this was a dry run
	Fresh     bool   // Whether this was a fresh sync
	OutputDir string // Where files were written (empty means default)
}

// SyncProviderResult represents sync results for a single provider
type SyncProviderResult struct {
	ProviderID catalogs.ProviderID     // The provider that was synced
	Added      []catalogs.Model        // New models not in catalog
	Updated    []reconcile.ModelUpdate // Existing models with changes
	Removed    []catalogs.Model        // Models in catalog but not in API (informational only)

	// Summary counts
	AddedCount   int // Number of models added
	UpdatedCount int // Number of models updated
	RemovedCount int // Number of models removed from API (not deleted from catalog)

	// Metadata
	APIModelsCount      int // Total models fetched from API
	ExistingModelsCount int // Total models that existed in catalog
	EnhancedCount       int // Number of models enhanced with models.dev data
}

// HasChanges returns true if the sync result contains any changes
func (sr *SyncResult) HasChanges() bool {
	return sr.TotalChanges > 0
}

// HasChanges returns true if the provider result contains any changes
func (spr *SyncProviderResult) HasChanges() bool {
	return spr.AddedCount > 0 || spr.UpdatedCount > 0 || spr.RemovedCount > 0
}

// Summary returns a human-readable summary of the sync result
func (sr *SyncResult) Summary() string {
	if !sr.HasChanges() {
		return "No changes detected"
	}

	var parts []string
	if sr.DryRun {
		parts = append(parts, "(Dry run)")
	}
	if sr.Fresh {
		parts = append(parts, "(Fresh sync)")
	}

	summary := fmt.Sprintf("%d total changes across %d providers", sr.TotalChanges, sr.ProvidersChanged)
	if len(parts) > 0 {
		summary += " " + strings.Join(parts, " ")
	}

	return summary
}

// Summary returns a human-readable summary of the provider result
func (spr *SyncProviderResult) Summary() string {
	if !spr.HasChanges() {
		return fmt.Sprintf("%s: No changes", spr.ProviderID)
	}

	return fmt.Sprintf("%s: %d added, %d updated, %d removed",
		spr.ProviderID, spr.AddedCount, spr.UpdatedCount, spr.RemovedCount)
}

// NewResult creates a new Result with initialized maps
func NewSyncResult() *SyncResult {
	return &SyncResult{
		ProviderResults: make(map[catalogs.ProviderID]*SyncProviderResult),
	}
}

// NewProviderResult creates a new ProviderResult
func NewSyncProviderResult(providerID catalogs.ProviderID) *SyncProviderResult {
	return &SyncProviderResult{
		ProviderID: providerID,
		Added:      []catalogs.Model{},
		Updated:    []reconcile.ModelUpdate{},
		Removed:    []catalogs.Model{},
	}
}

// convertChangesetToSyncResult converts a reconcile.Changeset to a SyncResult
func convertChangesetToSyncResult(changeset *reconcile.Changeset, dryRun bool, outputDir string, providerAPICounts map[catalogs.ProviderID]int, modelProviderMap map[string]catalogs.ProviderID) *SyncResult {
	result := &SyncResult{
		TotalChanges:    changeset.Summary.TotalChanges,
		DryRun:          dryRun,
		OutputDir:       outputDir,
		ProviderResults: make(map[catalogs.ProviderID]*SyncProviderResult),
	}

	// Group models by provider for the provider results
	providerAdded := make(map[catalogs.ProviderID][]catalogs.Model)
	providerUpdated := make(map[catalogs.ProviderID][]reconcile.ModelUpdate)
	providerRemoved := make(map[catalogs.ProviderID][]catalogs.Model)

	for _, model := range changeset.Models.Added {
		providerID := getModelProvider(model, modelProviderMap)
		providerAdded[providerID] = append(providerAdded[providerID], model)
	}

	for _, update := range changeset.Models.Updated {
		providerID := getModelProvider(update.New, modelProviderMap)
		providerUpdated[providerID] = append(providerUpdated[providerID], update)
	}

	for _, model := range changeset.Models.Removed {
		providerID := getModelProvider(model, modelProviderMap)
		providerRemoved[providerID] = append(providerRemoved[providerID], model)
	}

	// Collect all providers that have changes
	allProviders := make(map[catalogs.ProviderID]bool)
	for providerID := range providerAdded {
		allProviders[providerID] = true
	}
	for providerID := range providerUpdated {
		allProviders[providerID] = true
	}
	for providerID := range providerRemoved {
		allProviders[providerID] = true
	}

	// Create provider results
	for providerID := range allProviders {
		providerResult := &SyncProviderResult{
			ProviderID:     providerID,
			Added:          providerAdded[providerID],
			Updated:        providerUpdated[providerID],
			Removed:        providerRemoved[providerID],
			AddedCount:     len(providerAdded[providerID]),
			UpdatedCount:   len(providerUpdated[providerID]),
			RemovedCount:   len(providerRemoved[providerID]),
			APIModelsCount: providerAPICounts[providerID], // Now properly set from actual API fetch
		}
		result.ProviderResults[providerID] = providerResult
		result.ProvidersChanged++
	}

	return result
}

// getModelProvider extracts the provider ID from a model using the provider map
func getModelProvider(model catalogs.Model, modelProviderMap map[string]catalogs.ProviderID) catalogs.ProviderID {
	// Use the model-to-provider map if available
	if providerID, ok := modelProviderMap[model.ID]; ok {
		return providerID
	}
	
	// Fallback: Try to infer from model ID patterns (for models not in the map)
	// This should rarely happen in practice
	modelID := strings.ToLower(model.ID)
	switch {
	case strings.Contains(modelID, "gpt") || strings.Contains(modelID, "dall") || strings.Contains(modelID, "whisper") || strings.Contains(modelID, "o1") || strings.Contains(modelID, "o3"):
		return "openai"
	case strings.Contains(modelID, "claude"):
		return "anthropic"
	case strings.Contains(modelID, "gemini") || strings.Contains(modelID, "gemma"):
		return "google-ai-studio"
	case strings.Contains(modelID, "llama") || strings.Contains(modelID, "mistral"):
		return "groq"
	case strings.Contains(modelID, "deepseek"):
		return "deepseek"
	default:
		// If we really can't determine, return unknown
		// This should be very rare with the provider map
		return "unknown"
	}
}

