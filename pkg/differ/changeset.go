// Package differ provides functionality for comparing catalogs and detecting changes.
package differ

import (
	"fmt"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// ChangeType represents the type of change.
type ChangeType string

const (
	// ChangeTypeAdd indicates an item was added.
	ChangeTypeAdd ChangeType = "add"
	// ChangeTypeUpdate indicates an item was updated.
	ChangeTypeUpdate ChangeType = "update"
	// ChangeTypeRemove indicates an item was removed.
	ChangeTypeRemove ChangeType = "remove"
)

// FieldChange represents a change to a specific field.
type FieldChange struct {
	Path     string     // Field path (e.g., "pricing.input")
	OldValue string     // Previous value (string representation)
	NewValue string     // New value (string representation)
	Type     ChangeType // Type of change
	Source   sources.ID // Source that caused the change (for provenance)
}

// ModelUpdate represents an update to an existing model.
type ModelUpdate struct {
	ID       string         // ID of the model being updated
	Existing catalogs.Model // Current model
	New      catalogs.Model // New model
	Changes  []FieldChange  // Detailed list of field changes
}

// ProviderUpdate represents an update to an existing provider.
type ProviderUpdate struct {
	ID       catalogs.ProviderID // ID of the provider being updated
	Existing catalogs.Provider   // Current provider
	New      catalogs.Provider   // New provider
	Changes  []FieldChange       // Detailed list of field changes
}

// AuthorUpdate represents an update to an existing author.
type AuthorUpdate struct {
	ID       catalogs.AuthorID // ID of the author being updated
	Existing catalogs.Author   // Current author
	New      catalogs.Author   // New author
	Changes  []FieldChange     // Detailed list of field changes
}

// ModelChangeset represents changes to models.
type ModelChangeset struct {
	Added   []catalogs.Model // New models
	Updated []ModelUpdate    // Updated models
	Removed []catalogs.Model // Removed models
}

// ProviderChangeset represents changes to providers.
type ProviderChangeset struct {
	Added   []catalogs.Provider // New providers
	Updated []ProviderUpdate    // Updated providers
	Removed []catalogs.Provider // Removed providers
}

// AuthorChangeset represents changes to authors.
type AuthorChangeset struct {
	Added   []catalogs.Author // New authors
	Updated []AuthorUpdate    // Updated authors
	Removed []catalogs.Author // Removed authors
}

// Changeset represents all changes between two catalogs.
type Changeset struct {
	Models    *ModelChangeset    // Model changes
	Providers *ProviderChangeset // Provider changes
	Authors   *AuthorChangeset   // Author changes
	Summary   ChangesetSummary   // Summary statistics
}

// ChangesetSummary provides summary statistics for a changeset.
type ChangesetSummary struct {
	ModelsAdded      int
	ModelsUpdated    int
	ModelsRemoved    int
	ProvidersAdded   int
	ProvidersUpdated int
	ProvidersRemoved int
	AuthorsAdded     int
	AuthorsUpdated   int
	AuthorsRemoved   int
	TotalChanges     int
}

// HasChanges returns true if the changeset contains any changes.
func (c *Changeset) HasChanges() bool {
	return c.Summary.TotalChanges > 0
}

// calculateSummary computes the summary for a changeset.
func calculateSummary(models *ModelChangeset, providers *ProviderChangeset, authors *AuthorChangeset) ChangesetSummary {
	modelsAdded := len(models.Added)
	modelsUpdated := len(models.Updated)
	modelsRemoved := len(models.Removed)
	providersAdded := len(providers.Added)
	providersUpdated := len(providers.Updated)
	providersRemoved := len(providers.Removed)
	authorsAdded := len(authors.Added)
	authorsUpdated := len(authors.Updated)
	authorsRemoved := len(authors.Removed)

	return ChangesetSummary{
		ModelsAdded:      modelsAdded,
		ModelsUpdated:    modelsUpdated,
		ModelsRemoved:    modelsRemoved,
		ProvidersAdded:   providersAdded,
		ProvidersUpdated: providersUpdated,
		ProvidersRemoved: providersRemoved,
		AuthorsAdded:     authorsAdded,
		AuthorsUpdated:   authorsUpdated,
		AuthorsRemoved:   authorsRemoved,
		TotalChanges: modelsAdded + modelsUpdated + modelsRemoved +
			providersAdded + providersUpdated + providersRemoved +
			authorsAdded + authorsUpdated + authorsRemoved,
	}
}

// IsEmpty returns true if the changeset contains no changes.
func (c *Changeset) IsEmpty() bool {
	return c.Summary.TotalChanges == 0
}

// HasChanges returns true if the model changeset contains any changes.
func (m *ModelChangeset) HasChanges() bool {
	return len(m.Added) > 0 || len(m.Updated) > 0 || len(m.Removed) > 0
}

// HasChanges returns true if the provider changeset contains any changes.
func (p *ProviderChangeset) HasChanges() bool {
	return len(p.Added) > 0 || len(p.Updated) > 0 || len(p.Removed) > 0
}

// HasChanges returns true if the author changeset contains any changes.
func (a *AuthorChangeset) HasChanges() bool {
	return len(a.Added) > 0 || len(a.Updated) > 0 || len(a.Removed) > 0
}

// String returns a human-readable summary of the changeset.
func (c *Changeset) String() string {
	if c.IsEmpty() {
		return "No changes detected"
	}

	var parts []string

	// Models summary
	if c.Models.HasChanges() {
		modelParts := []string{}
		if len(c.Models.Added) > 0 {
			modelParts = append(modelParts, fmt.Sprintf("%d added", len(c.Models.Added)))
		}
		if len(c.Models.Updated) > 0 {
			modelParts = append(modelParts, fmt.Sprintf("%d updated", len(c.Models.Updated)))
		}
		if len(c.Models.Removed) > 0 {
			modelParts = append(modelParts, fmt.Sprintf("%d removed", len(c.Models.Removed)))
		}
		parts = append(parts, fmt.Sprintf("Models: %s", strings.Join(modelParts, ", ")))
	}

	// Providers summary
	if c.Providers.HasChanges() {
		providerParts := []string{}
		if len(c.Providers.Added) > 0 {
			providerParts = append(providerParts, fmt.Sprintf("%d added", len(c.Providers.Added)))
		}
		if len(c.Providers.Updated) > 0 {
			providerParts = append(providerParts, fmt.Sprintf("%d updated", len(c.Providers.Updated)))
		}
		if len(c.Providers.Removed) > 0 {
			providerParts = append(providerParts, fmt.Sprintf("%d removed", len(c.Providers.Removed)))
		}
		parts = append(parts, fmt.Sprintf("Providers: %s", strings.Join(providerParts, ", ")))
	}

	// Authors summary
	if c.Authors.HasChanges() {
		authorParts := []string{}
		if len(c.Authors.Added) > 0 {
			authorParts = append(authorParts, fmt.Sprintf("%d added", len(c.Authors.Added)))
		}
		if len(c.Authors.Updated) > 0 {
			authorParts = append(authorParts, fmt.Sprintf("%d updated", len(c.Authors.Updated)))
		}
		if len(c.Authors.Removed) > 0 {
			authorParts = append(authorParts, fmt.Sprintf("%d removed", len(c.Authors.Removed)))
		}
		parts = append(parts, fmt.Sprintf("Authors: %s", strings.Join(authorParts, ", ")))
	}

	return fmt.Sprintf("Changeset: %s (Total: %d changes)", strings.Join(parts, "; "), c.Summary.TotalChanges)
}

// Print outputs a detailed, human-readable view of the changeset.
func (c *Changeset) Print() {
	fmt.Println(c.String())
	fmt.Println(strings.Repeat("─", 80))

	// Print model changes
	if c.Models.HasChanges() {
		c.Models.Print()
	}

	// Print provider changes
	if c.Providers.HasChanges() {
		c.Providers.Print()
	}

	// Print author changes
	if c.Authors.HasChanges() {
		c.Authors.Print()
	}
}

// Print outputs model changes in a human-readable format.
func (m *ModelChangeset) Print() {
	if len(m.Added) > 0 {
		fmt.Printf("\n➕ Added Models (%d):\n", len(m.Added))
		for _, model := range m.Added {
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

	if len(m.Updated) > 0 {
		fmt.Printf("\n🔄 Updated Models (%d):\n", len(m.Updated))
		for _, update := range m.Updated {
			fmt.Printf("  • %s:\n", update.ID)
			for _, change := range update.Changes {
				fmt.Printf("    - %s: %s → %s\n", change.Path, change.OldValue, change.NewValue)
			}
		}
	}

	if len(m.Removed) > 0 {
		fmt.Printf("\n⚠️  Removed Models (%d):\n", len(m.Removed))
		for _, model := range m.Removed {
			fmt.Printf("  • %s", model.ID)
			if model.Name != "" && model.Name != model.ID {
				fmt.Printf(" (%s)", model.Name)
			}
			fmt.Println()
		}
	}
}

// Print outputs provider changes in a human-readable format.
//
//nolint:dupl // Similar to AuthorChangeset.Print but for different types
func (p *ProviderChangeset) Print() {
	if len(p.Added) > 0 {
		fmt.Printf("\n➕ Added Providers (%d):\n", len(p.Added))
		for _, provider := range p.Added {
			fmt.Printf("  • %s", provider.ID)
			if provider.Name != "" {
				fmt.Printf(" (%s)", provider.Name)
			}
			fmt.Println()
		}
	}

	if len(p.Updated) > 0 {
		fmt.Printf("\n🔄 Updated Providers (%d):\n", len(p.Updated))
		for _, update := range p.Updated {
			fmt.Printf("  • %s:\n", update.ID)
			for _, change := range update.Changes {
				fmt.Printf("    - %s: %s → %s\n", change.Path, change.OldValue, change.NewValue)
			}
		}
	}

	if len(p.Removed) > 0 {
		fmt.Printf("\n⚠️  Removed Providers (%d):\n", len(p.Removed))
		for _, provider := range p.Removed {
			fmt.Printf("  • %s", provider.ID)
			if provider.Name != "" {
				fmt.Printf(" (%s)", provider.Name)
			}
			fmt.Println()
		}
	}
}

// Print outputs author changes in a human-readable format.
//
//nolint:dupl // Similar to ProviderChangeset.Print but for different types
func (a *AuthorChangeset) Print() {
	if len(a.Added) > 0 {
		fmt.Printf("\n➕ Added Authors (%d):\n", len(a.Added))
		for _, author := range a.Added {
			fmt.Printf("  • %s", author.ID)
			if author.Name != "" {
				fmt.Printf(" (%s)", author.Name)
			}
			fmt.Println()
		}
	}

	if len(a.Updated) > 0 {
		fmt.Printf("\n🔄 Updated Authors (%d):\n", len(a.Updated))
		for _, update := range a.Updated {
			fmt.Printf("  • %s:\n", update.ID)
			for _, change := range update.Changes {
				fmt.Printf("    - %s: %s → %s\n", change.Path, change.OldValue, change.NewValue)
			}
		}
	}

	if len(a.Removed) > 0 {
		fmt.Printf("\n⚠️  Removed Authors (%d):\n", len(a.Removed))
		for _, author := range a.Removed {
			fmt.Printf("  • %s", author.ID)
			if author.Name != "" {
				fmt.Printf(" (%s)", author.Name)
			}
			fmt.Println()
		}
	}
}

// ApplyStrategy represents how to apply changes.
type ApplyStrategy string

const (
	// ApplyAll applies all changes including removals.
	ApplyAll ApplyStrategy = "all"

	// ApplyAdditive only applies additions and updates, never removes.
	ApplyAdditive ApplyStrategy = "additive"

	// ApplyUpdatesOnly only applies updates to existing items.
	ApplyUpdatesOnly ApplyStrategy = "updates-only"

	// ApplyAdditionsOnly only applies new additions.
	ApplyAdditionsOnly ApplyStrategy = "additions-only"
)

// Filter filters the changeset based on the apply strategy.
func (c *Changeset) Filter(strategy ApplyStrategy) *Changeset {
	filtered := &Changeset{
		Models:    &ModelChangeset{},
		Providers: &ProviderChangeset{},
		Authors:   &AuthorChangeset{},
	}

	switch strategy {
	case ApplyAll:
		// Return everything
		return c

	case ApplyAdditive:
		// Include additions and updates, exclude removals
		filtered.Models.Added = c.Models.Added
		filtered.Models.Updated = c.Models.Updated
		filtered.Providers.Added = c.Providers.Added
		filtered.Providers.Updated = c.Providers.Updated
		filtered.Authors.Added = c.Authors.Added
		filtered.Authors.Updated = c.Authors.Updated

	case ApplyUpdatesOnly:
		// Only include updates
		filtered.Models.Updated = c.Models.Updated
		filtered.Providers.Updated = c.Providers.Updated
		filtered.Authors.Updated = c.Authors.Updated

	case ApplyAdditionsOnly:
		// Only include additions
		filtered.Models.Added = c.Models.Added
		filtered.Providers.Added = c.Providers.Added
		filtered.Authors.Added = c.Authors.Added
	}

	// Recalculate summary
	filtered.Summary = calculateSummary(filtered.Models, filtered.Providers, filtered.Authors)

	return filtered
}
