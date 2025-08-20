package catalogs

import (
	"fmt"
	"strings"
)

// SyncResult represents the complete result of a sync operation
type SyncResult struct {
	// Overall statistics
	TotalChanges     int                                // Total number of changes across all providers
	ProvidersChanged int                                // Number of providers with changes
	ProviderResults  map[ProviderID]*SyncProviderResult // Results per provider

	// Operation metadata
	DryRun    bool   // Whether this was a dry run
	Fresh     bool   // Whether this was a fresh sync
	OutputDir string // Where files were written (empty means default)
}

// SyncProviderResult represents sync results for a single provider
type SyncProviderResult struct {
	ProviderID ProviderID    // The provider that was synced
	Added      []Model       // New models not in catalog
	Updated    []ModelUpdate // Existing models with changes
	Removed    []Model       // Models in catalog but not in API (informational only)

	// Summary counts
	AddedCount   int // Number of models added
	UpdatedCount int // Number of models updated
	RemovedCount int // Number of models removed from API (not deleted from catalog)

	// Metadata
	APIModelsCount      int // Total models fetched from API
	ExistingModelsCount int // Total models that existed in catalog
	EnhancedCount       int // Number of models enhanced with models.dev data
}

// ModelUpdate represents an update to an existing model
type ModelUpdate struct {
	ModelID       string        // ID of the model being updated
	ExistingModel Model         // Current model in catalog
	NewModel      Model         // New model from API
	Changes       []FieldChange // Detailed list of field changes
}

// FieldChange represents a change to a specific field
type FieldChange struct {
	Field    string // Name of the field that changed
	OldValue string // Previous value (string representation)
	NewValue string // New value (string representation)
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
		ProviderResults: make(map[ProviderID]*SyncProviderResult),
	}
}

// NewProviderResult creates a new ProviderResult
func NewSyncProviderResult(providerID ProviderID) *SyncProviderResult {
	return &SyncProviderResult{
		ProviderID: providerID,
		Added:      []Model{},
		Updated:    []ModelUpdate{},
		Removed:    []Model{},
	}
}
