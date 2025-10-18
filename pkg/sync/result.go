package sync

import (
	"fmt"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/differ"
)

// Result represents the complete result of a sync operation.
type Result struct {
	// Overall statistics
	TotalChanges     int                                     // Total number of changes across all providers
	ProvidersChanged int                                     // Number of providers with changes
	ProviderResults  map[catalogs.ProviderID]*ProviderResult // Results per provider

	// Operation metadata
	DryRun    bool   // Whether this was a dry run
	Fresh     bool   // Whether this was a fresh sync
	OutputDir string // Where files were written (empty means default)
}

// ProviderResult represents sync results for a single provider.
type ProviderResult struct {
	ProviderID catalogs.ProviderID  // The provider that was synced
	Added      []catalogs.Model     // New models not in catalog
	Updated    []differ.ModelUpdate // Existing models with changes
	Removed    []catalogs.Model     // Models in catalog but not in API (informational only)

	// Summary counts
	AddedCount   int // Number of models added
	UpdatedCount int // Number of models updated
	RemovedCount int // Number of models removed from API (not deleted from catalog)

	// Metadata
	APIModelsCount      int // Total models fetched from API
	ExistingModelsCount int // Total models that existed in catalog
	EnhancedCount       int // Number of models enhanced with models.dev data
}

// HasChanges returns true if the sync result contains any changes.
func (sr *Result) HasChanges() bool {
	return sr.TotalChanges > 0
}

// HasChanges returns true if the provider result contains any changes.
func (spr *ProviderResult) HasChanges() bool {
	return spr.AddedCount > 0 || spr.UpdatedCount > 0 || spr.RemovedCount > 0
}

// Summary returns a human-readable summary of the sync result.
func (sr *Result) Summary() string {
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

// Summary returns a human-readable summary of the provider result.
func (spr *ProviderResult) Summary() string {
	if !spr.HasChanges() {
		return fmt.Sprintf("%s: No changes", spr.ProviderID)
	}

	return fmt.Sprintf("%s: %d added, %d updated, %d removed",
		spr.ProviderID, spr.AddedCount, spr.UpdatedCount, spr.RemovedCount)
}

// ChangesetToResult converts a reconcile.Changeset to a SyncResult.
func ChangesetToResult(changeset *differ.Changeset, dryRun bool, outputDir string, providerAPICounts map[catalogs.ProviderID]int, modelProviderMap map[string]catalogs.ProviderID) *Result {
	result := &Result{
		TotalChanges:    changeset.Summary.TotalChanges,
		DryRun:          dryRun,
		OutputDir:       outputDir,
		ProviderResults: make(map[catalogs.ProviderID]*ProviderResult),
	}

	// Group models by provider for the provider results
	providerAdded := make(map[catalogs.ProviderID][]catalogs.Model)
	providerUpdated := make(map[catalogs.ProviderID][]differ.ModelUpdate)
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
		providerResult := &ProviderResult{
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

// getModelProvider extracts the provider ID from a model using the provider map.
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
	case strings.Contains(modelID, "gemini") || strings.Contains(modelID, "gemma") || strings.Contains(modelID, "imagen"):
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
