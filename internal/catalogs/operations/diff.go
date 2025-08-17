package operations

import (
	"fmt"
	"sort"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// ProviderChangeset represents all changes for a specific provider
type ProviderChangeset struct {
	ProviderID catalogs.ProviderID
	Added      []catalogs.Model // New models not in catalog
	Updated    []ModelUpdate    // Existing models with changes
	Removed    []catalogs.Model // Models in catalog but not in API (we don't auto-remove these)
}

// ModelUpdate represents an update to an existing model
type ModelUpdate struct {
	ModelID       string
	ExistingModel catalogs.Model
	NewModel      catalogs.Model
	Changes       []FieldChange
}

// FieldChange represents a change to a specific field
type FieldChange struct {
	Field    string
	OldValue string
	NewValue string
}

// HasChanges returns true if the changeset contains any changes
func (c ProviderChangeset) HasChanges() bool {
	return len(c.Added) > 0 || len(c.Updated) > 0 || len(c.Removed) > 0
}

// CompareProviderModels compares API models with existing catalog models and generates a changeset
func CompareProviderModels(providerID catalogs.ProviderID, existingModels map[string]catalogs.Model, apiModels []catalogs.Model) ProviderChangeset {
	changeset := ProviderChangeset{
		ProviderID: providerID,
		Added:      []catalogs.Model{},
		Updated:    []ModelUpdate{},
		Removed:    []catalogs.Model{},
	}

	// Create map of API models for easier lookup
	apiModelMap := make(map[string]catalogs.Model)
	for _, model := range apiModels {
		apiModelMap[model.ID] = model
	}

	// Find new and updated models
	for _, apiModel := range apiModels {
		if existingModel, exists := existingModels[apiModel.ID]; exists {
			// Check if model has been updated
			if update := compareModels(apiModel.ID, existingModel, apiModel); update != nil {
				changeset.Updated = append(changeset.Updated, *update)
			}
		} else {
			// New model
			changeset.Added = append(changeset.Added, apiModel)
		}
	}

	// Find removed models (in catalog but not in API)
	// Note: We don't automatically remove models as they might be manually maintained
	for modelID, existingModel := range existingModels {
		if _, exists := apiModelMap[modelID]; !exists {
			changeset.Removed = append(changeset.Removed, existingModel)
		}
	}

	// Sort slices for consistent output
	sort.Slice(changeset.Added, func(i, j int) bool {
		return changeset.Added[i].ID < changeset.Added[j].ID
	})
	sort.Slice(changeset.Updated, func(i, j int) bool {
		return changeset.Updated[i].ModelID < changeset.Updated[j].ModelID
	})
	sort.Slice(changeset.Removed, func(i, j int) bool {
		return changeset.Removed[i].ID < changeset.Removed[j].ID
	})

	return changeset
}

// compareModels compares two models and returns a ModelUpdate if they differ
func compareModels(modelID string, existing, new catalogs.Model) *ModelUpdate {
	var changes []FieldChange

	// Compare basic fields
	if existing.Name != new.Name {
		changes = append(changes, FieldChange{
			Field:    "name",
			OldValue: existing.Name,
			NewValue: new.Name,
		})
	}

	// Compare context window
	existingContext := int64(0)
	newContext := int64(0)
	if existing.Limits != nil {
		existingContext = existing.Limits.ContextWindow
	}
	if new.Limits != nil {
		newContext = new.Limits.ContextWindow
	}
	if existingContext != newContext {
		changes = append(changes, FieldChange{
			Field:    "context_window",
			OldValue: formatTokens(existingContext),
			NewValue: formatTokens(newContext),
		})
	}

	// Compare output tokens
	existingOutput := int64(0)
	newOutput := int64(0)
	if existing.Limits != nil {
		existingOutput = existing.Limits.OutputTokens
	}
	if new.Limits != nil {
		newOutput = new.Limits.OutputTokens
	}
	if existingOutput != newOutput {
		changes = append(changes, FieldChange{
			Field:    "output_tokens",
			OldValue: formatTokens(existingOutput),
			NewValue: formatTokens(newOutput),
		})
	}

	// Compare authors
	existingAuthors := formatAuthors(existing.Authors)
	newAuthors := formatAuthors(new.Authors)
	if existingAuthors != newAuthors {
		changes = append(changes, FieldChange{
			Field:    "authors",
			OldValue: existingAuthors,
			NewValue: newAuthors,
		})
	}

	// Compare basic features
	if compareFeatures(existing.Features, new.Features) {
		changes = append(changes, FieldChange{
			Field:    "features",
			OldValue: "see detailed diff",
			NewValue: "updated capabilities",
		})
	}

	// If no changes, return nil
	if len(changes) == 0 {
		return nil
	}

	return &ModelUpdate{
		ModelID:       modelID,
		ExistingModel: existing,
		NewModel:      new,
		Changes:       changes,
	}
}

// compareFeatures compares model features and returns true if they differ
func compareFeatures(existing, new *catalogs.ModelFeatures) bool {
	if existing == nil && new == nil {
		return false
	}
	if existing == nil || new == nil {
		return true
	}

	// Compare key feature flags
	return existing.Tools != new.Tools ||
		existing.ToolChoice != new.ToolChoice ||
		existing.Streaming != new.Streaming ||
		existing.Temperature != new.Temperature ||
		existing.MaxTokens != new.MaxTokens ||
		existing.Reasoning != new.Reasoning ||
		len(existing.Modalities.Input) != len(new.Modalities.Input) ||
		len(existing.Modalities.Output) != len(new.Modalities.Output)
}

// formatTokens formats token counts for display
func formatTokens(tokens int64) string {
	if tokens == 0 {
		return "0"
	}
	if tokens >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	}
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}

// formatAuthors formats author list for display
func formatAuthors(authors []catalogs.Author) string {
	if len(authors) == 0 {
		return "none"
	}
	var names []string
	for _, author := range authors {
		if author.Name != "" {
			names = append(names, author.Name)
		} else {
			names = append(names, string(author.ID))
		}
	}
	return strings.Join(names, ", ")
}

// PrintChangeset prints a human-readable summary of changes
func PrintChangeset(changeset ProviderChangeset) {
	fmt.Printf("Provider: %s\n", changeset.ProviderID)
	fmt.Printf("─────────────────────────\n")

	// Added models
	if len(changeset.Added) > 0 {
		fmt.Printf("\n➕ Added Models (%d):\n", len(changeset.Added))
		for _, model := range changeset.Added {
			fmt.Printf("  • %s", model.ID)
			if model.Name != "" && model.Name != model.ID {
				fmt.Printf(" (%s)", model.Name)
			}
			if model.Limits != nil && model.Limits.ContextWindow > 0 {
				fmt.Printf(" - %s context", formatTokens(model.Limits.ContextWindow))
			}
			fmt.Println()
		}
	}

	// Updated models
	if len(changeset.Updated) > 0 {
		fmt.Printf("\n🔄 Updated Models (%d):\n", len(changeset.Updated))
		for _, update := range changeset.Updated {
			fmt.Printf("  • %s:\n", update.ModelID)
			for _, change := range update.Changes {
				fmt.Printf("    - %s: %s → %s\n", change.Field, change.OldValue, change.NewValue)
			}
		}
	}

	// Removed models (informational only)
	if len(changeset.Removed) > 0 {
		fmt.Printf("\n⚠️  Models in catalog but not in API (%d):\n", len(changeset.Removed))
		fmt.Printf("   (These will NOT be automatically removed)\n")
		for _, model := range changeset.Removed {
			fmt.Printf("  • %s", model.ID)
			if model.Name != "" && model.Name != model.ID {
				fmt.Printf(" (%s)", model.Name)
			}
			fmt.Println()
		}
	}

	fmt.Println()
}
