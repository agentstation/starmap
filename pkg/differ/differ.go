package differ

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Differ handles change detection between resources
type Differ interface {
	// Models compares two sets of models and returns changes
	Models(existing, new []catalogs.Model) *ModelChangeset

	// Providers compares two sets of providers and returns changes
	Providers(existing, new []catalogs.Provider) *ProviderChangeset

	// Authors compares two sets of authors and returns changes
	Authors(existing, new []catalogs.Author) *AuthorChangeset

	// Catalogs compares two complete catalogs
	// Both catalogs only need to be readable
	Catalogs(existing, new catalogs.Reader) *Changeset
}

// differ is the default implementation of Differ
type differ struct {
	// Options for controlling diff behavior
	ignoreFields   map[string]bool
	deepComparison bool
	tracking       bool
}

// New creates a new Differ with default settings
func New(opts ...DifferOption) Differ {
	d := &differ{
		ignoreFields:   make(map[string]bool),
		deepComparison: true,
		tracking:       true,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// Models compares two sets of models and returns changes
func (diff *differ) Models(existing, new []catalogs.Model) *ModelChangeset {
	changeset := &ModelChangeset{
		Added:   []catalogs.Model{},
		Updated: []ModelUpdate{},
		Removed: []catalogs.Model{},
	}

	// Create maps for efficient lookup
	existingMap := make(map[string]catalogs.Model)
	for _, model := range existing {
		existingMap[model.ID] = model
	}

	newMap := make(map[string]catalogs.Model)
	for _, model := range new {
		newMap[model.ID] = model
	}

	// Find added and updated models
	for _, newModel := range new {
		if existingModel, exists := existingMap[newModel.ID]; exists {
			// Check if model has been updated
			if update := diff.model(existingModel, newModel); update != nil {
				changeset.Updated = append(changeset.Updated, *update)
			}
		} else {
			// New model
			changeset.Added = append(changeset.Added, newModel)
		}
	}

	// Find removed models
	for _, existingModel := range existing {
		if _, exists := newMap[existingModel.ID]; !exists {
			changeset.Removed = append(changeset.Removed, existingModel)
		}
	}

	// Sort for consistent output
	sortModelChangeset(changeset)

	return changeset
}

// Providers compares two sets of providers and returns changes
func (diff *differ) Providers(existing, new []catalogs.Provider) *ProviderChangeset {
	changeset := &ProviderChangeset{
		Added:   []catalogs.Provider{},
		Updated: []ProviderUpdate{},
		Removed: []catalogs.Provider{},
	}

	// Create maps for efficient lookup
	existingMap := make(map[catalogs.ProviderID]catalogs.Provider)
	for _, provider := range existing {
		existingMap[provider.ID] = provider
	}

	newMap := make(map[catalogs.ProviderID]catalogs.Provider)
	for _, provider := range new {
		newMap[provider.ID] = provider
	}

	// Find added and updated providers
	for _, newProvider := range new {
		if existingProvider, exists := existingMap[newProvider.ID]; exists {
			// Check if provider has been updated
			if update := diff.provider(existingProvider, newProvider); update != nil {
				changeset.Updated = append(changeset.Updated, *update)
			}
		} else {
			// New provider
			changeset.Added = append(changeset.Added, newProvider)
		}
	}

	// Find removed providers
	for _, existingProvider := range existing {
		if _, exists := newMap[existingProvider.ID]; !exists {
			changeset.Removed = append(changeset.Removed, existingProvider)
		}
	}

	// Sort for consistent output
	sortProviderChangeset(changeset)

	return changeset
}

// Authors compares two sets of authors and returns changes
func (diff *differ) Authors(existing, new []catalogs.Author) *AuthorChangeset {
	changeset := &AuthorChangeset{
		Added:   []catalogs.Author{},
		Updated: []AuthorUpdate{},
		Removed: []catalogs.Author{},
	}

	// Create maps for efficient lookup
	existingMap := make(map[catalogs.AuthorID]catalogs.Author)
	for _, author := range existing {
		existingMap[author.ID] = author
	}

	newMap := make(map[catalogs.AuthorID]catalogs.Author)
	for _, author := range new {
		newMap[author.ID] = author
	}

	// Find added and updated authors
	for _, newAuthor := range new {
		if existingAuthor, exists := existingMap[newAuthor.ID]; exists {
			// Check if author has been updated
			if update := diff.author(existingAuthor, newAuthor); update != nil {
				changeset.Updated = append(changeset.Updated, *update)
			}
		} else {
			// New author
			changeset.Added = append(changeset.Added, newAuthor)
		}
	}

	// Find removed authors
	for _, existingAuthor := range existing {
		if _, exists := newMap[existingAuthor.ID]; !exists {
			changeset.Removed = append(changeset.Removed, existingAuthor)
		}
	}

	// Sort for consistent output
	sortAuthorChangeset(changeset)

	return changeset
}

// Catalogs compares two complete catalogs
// Both catalogs only need to be readable
func (diff *differ) Catalogs(existing, new catalogs.Reader) *Changeset {
	changeset := &Changeset{}

	// Diff models (from providers and authors)
	existingModels := existing.GetAllModels()
	newModels := new.GetAllModels()
	changeset.Models = diff.Models(existingModels, newModels)

	// Diff providers
	existingProviders := []catalogs.Provider{}
	for _, p := range existing.Providers().List() {
		existingProviders = append(existingProviders, *p)
	}
	newProviders := []catalogs.Provider{}
	for _, p := range new.Providers().List() {
		newProviders = append(newProviders, *p)
	}
	changeset.Providers = diff.Providers(existingProviders, newProviders)

	// Diff authors
	existingAuthors := []catalogs.Author{}
	for _, a := range existing.Authors().List() {
		existingAuthors = append(existingAuthors, *a)
	}
	newAuthors := []catalogs.Author{}
	for _, a := range new.Authors().List() {
		newAuthors = append(newAuthors, *a)
	}
	changeset.Authors = diff.Authors(existingAuthors, newAuthors)

	// Calculate summary
	changeset.Summary = calculateSummary(changeset)

	return changeset
}

// model compares two models and returns an update if they differ
func (diff *differ) model(existing, new catalogs.Model) *ModelUpdate {
	changes := []FieldChange{}

	// Compare basic fields
	if existing.Name != new.Name && !diff.ignoreFields["name"] {
		changes = append(changes, FieldChange{
			Path:     "name",
			OldValue: existing.Name,
			NewValue: new.Name,
			Type:     ChangeTypeUpdate,
		})
	}

	if existing.Description != new.Description && !diff.ignoreFields["description"] {
		changes = append(changes, FieldChange{
			Path:     "description",
			OldValue: truncateString(existing.Description, 50),
			NewValue: truncateString(new.Description, 50),
			Type:     ChangeTypeUpdate,
		})
	}

	// Compare features
	if diff.deepComparison && !diff.ignoreFields["features"] {
		featureChanges := diffModelFeatures(existing.Features, new.Features)
		changes = append(changes, featureChanges...)
	}

	// Compare pricing
	if diff.deepComparison && !diff.ignoreFields["pricing"] {
		pricingChanges := diffModelPricing(existing.Pricing, new.Pricing)
		changes = append(changes, pricingChanges...)
	}

	// Compare limits
	if diff.deepComparison && !diff.ignoreFields["limits"] {
		limitChanges := diffModelLimits(existing.Limits, new.Limits)
		changes = append(changes, limitChanges...)
	}

	// Compare metadata
	if diff.deepComparison && !diff.ignoreFields["metadata"] {
		metadataChanges := diffModelMetadata(existing.Metadata, new.Metadata)
		changes = append(changes, metadataChanges...)
	}

	// If no changes, return nil
	if len(changes) == 0 {
		return nil
	}

	return &ModelUpdate{
		ID:       existing.ID,
		Existing: existing,
		New:      new,
		Changes:  changes,
	}
}

// diffModelFeatures compares model features
func diffModelFeatures(existing, new *catalogs.ModelFeatures) []FieldChange {
	changes := []FieldChange{}

	if existing == nil && new == nil {
		return changes
	}

	if existing == nil || new == nil {
		changes = append(changes, FieldChange{
			Path:     "features",
			OldValue: fmt.Sprintf("%v", existing != nil),
			NewValue: fmt.Sprintf("%v", new != nil),
			Type:     ChangeTypeUpdate,
		})
		return changes
	}

	// Compare boolean features
	if existing.Tools != new.Tools {
		changes = append(changes, FieldChange{
			Path:     "features.tools",
			OldValue: fmt.Sprintf("%v", existing.Tools),
			NewValue: fmt.Sprintf("%v", new.Tools),
			Type:     ChangeTypeUpdate,
		})
	}

	if existing.Reasoning != new.Reasoning {
		changes = append(changes, FieldChange{
			Path:     "features.reasoning",
			OldValue: fmt.Sprintf("%v", existing.Reasoning),
			NewValue: fmt.Sprintf("%v", new.Reasoning),
			Type:     ChangeTypeUpdate,
		})
	}

	if existing.Streaming != new.Streaming {
		changes = append(changes, FieldChange{
			Path:     "features.streaming",
			OldValue: fmt.Sprintf("%v", existing.Streaming),
			NewValue: fmt.Sprintf("%v", new.Streaming),
			Type:     ChangeTypeUpdate,
		})
	}

	// Compare modalities
	if !equalModalitySlices(existing.Modalities.Input, new.Modalities.Input) {
		changes = append(changes, FieldChange{
			Path:     "features.modalities.input",
			OldValue: joinModalities(existing.Modalities.Input),
			NewValue: joinModalities(new.Modalities.Input),
			Type:     ChangeTypeUpdate,
		})
	}

	if !equalModalitySlices(existing.Modalities.Output, new.Modalities.Output) {
		changes = append(changes, FieldChange{
			Path:     "features.modalities.output",
			OldValue: joinModalities(existing.Modalities.Output),
			NewValue: joinModalities(new.Modalities.Output),
			Type:     ChangeTypeUpdate,
		})
	}

	return changes
}

// diffModelPricing compares model pricing
func diffModelPricing(existing, new *catalogs.ModelPricing) []FieldChange {
	changes := []FieldChange{}

	if existing == nil && new == nil {
		return changes
	}

	if existing == nil || new == nil {
		changes = append(changes, FieldChange{
			Path:     "pricing",
			OldValue: fmt.Sprintf("%v", existing != nil),
			NewValue: fmt.Sprintf("%v", new != nil),
			Type:     ChangeTypeUpdate,
		})
		return changes
	}

	// Compare token pricing
	if existing.Tokens != nil && new.Tokens != nil {
		if existing.Tokens.Input != nil && new.Tokens.Input != nil {
			if existing.Tokens.Input.Per1M != new.Tokens.Input.Per1M {
				changes = append(changes, FieldChange{
					Path:     "pricing.tokens.input",
					OldValue: fmt.Sprintf("%.6f", existing.Tokens.Input.Per1M),
					NewValue: fmt.Sprintf("%.6f", new.Tokens.Input.Per1M),
					Type:     ChangeTypeUpdate,
				})
			}
		}

		if existing.Tokens.Output != nil && new.Tokens.Output != nil {
			if existing.Tokens.Output.Per1M != new.Tokens.Output.Per1M {
				changes = append(changes, FieldChange{
					Path:     "pricing.tokens.output",
					OldValue: fmt.Sprintf("%.6f", existing.Tokens.Output.Per1M),
					NewValue: fmt.Sprintf("%.6f", new.Tokens.Output.Per1M),
					Type:     ChangeTypeUpdate,
				})
			}
		}
	}

	return changes
}

// diffModelLimits compares model limits
func diffModelLimits(existing, new *catalogs.ModelLimits) []FieldChange {
	changes := []FieldChange{}

	if existing == nil && new == nil {
		return changes
	}

	if existing == nil || new == nil {
		changes = append(changes, FieldChange{
			Path:     "limits",
			OldValue: fmt.Sprintf("%v", existing != nil),
			NewValue: fmt.Sprintf("%v", new != nil),
			Type:     ChangeTypeUpdate,
		})
		return changes
	}

	if existing.ContextWindow != new.ContextWindow {
		changes = append(changes, FieldChange{
			Path:     "limits.context_window",
			OldValue: formatTokens(existing.ContextWindow),
			NewValue: formatTokens(new.ContextWindow),
			Type:     ChangeTypeUpdate,
		})
	}

	if existing.OutputTokens != new.OutputTokens {
		changes = append(changes, FieldChange{
			Path:     "limits.output_tokens",
			OldValue: formatTokens(existing.OutputTokens),
			NewValue: formatTokens(new.OutputTokens),
			Type:     ChangeTypeUpdate,
		})
	}

	return changes
}

// diffModelMetadata compares model metadata
func diffModelMetadata(existing, new *catalogs.ModelMetadata) []FieldChange {
	changes := []FieldChange{}

	if existing == nil && new == nil {
		return changes
	}

	if existing == nil || new == nil {
		changes = append(changes, FieldChange{
			Path:     "metadata",
			OldValue: fmt.Sprintf("%v", existing != nil),
			NewValue: fmt.Sprintf("%v", new != nil),
			Type:     ChangeTypeUpdate,
		})
		return changes
	}

	if existing.KnowledgeCutoff != nil && new.KnowledgeCutoff != nil && !existing.KnowledgeCutoff.Equal(*new.KnowledgeCutoff) {
		changes = append(changes, FieldChange{
			Path:     "metadata.knowledge_cutoff",
			OldValue: existing.KnowledgeCutoff.Format("2006-01-02"),
			NewValue: new.KnowledgeCutoff.Format("2006-01-02"),
			Type:     ChangeTypeUpdate,
		})
	}

	if !existing.ReleaseDate.Equal(new.ReleaseDate) {
		changes = append(changes, FieldChange{
			Path:     "metadata.release_date",
			OldValue: existing.ReleaseDate.Format("2006-01-02"),
			NewValue: new.ReleaseDate.Format("2006-01-02"),
			Type:     ChangeTypeUpdate,
		})
	}

	if existing.OpenWeights != new.OpenWeights {
		changes = append(changes, FieldChange{
			Path:     "metadata.open_weights",
			OldValue: fmt.Sprintf("%v", existing.OpenWeights),
			NewValue: fmt.Sprintf("%v", new.OpenWeights),
			Type:     ChangeTypeUpdate,
		})
	}

	return changes
}

// provider compares two providers
func (diff *differ) provider(existing, new catalogs.Provider) *ProviderUpdate {
	changes := []FieldChange{}

	if existing.Name != new.Name && !diff.ignoreFields["name"] {
		changes = append(changes, FieldChange{
			Path:     "name",
			OldValue: existing.Name,
			NewValue: new.Name,
			Type:     ChangeTypeUpdate,
		})
	}

	// Check API configuration changes
	if !reflect.DeepEqual(existing.APIKey, new.APIKey) && !diff.ignoreFields["api_key"] {
		changes = append(changes, FieldChange{
			Path:     "api_key",
			OldValue: "config changed",
			NewValue: "updated",
			Type:     ChangeTypeUpdate,
		})
	}

	// Check catalog settings changes
	if !reflect.DeepEqual(existing.Catalog, new.Catalog) && !diff.ignoreFields["catalog"] {
		changes = append(changes, FieldChange{
			Path:     "catalog",
			OldValue: "settings changed",
			NewValue: "updated",
			Type:     ChangeTypeUpdate,
		})
	}

	if len(changes) == 0 {
		return nil
	}

	return &ProviderUpdate{
		ID:       existing.ID,
		Existing: existing,
		New:      new,
		Changes:  changes,
	}
}

// author compares two authors
func (diff *differ) author(existing, new catalogs.Author) *AuthorUpdate {
	changes := []FieldChange{}

	if existing.Name != new.Name && !diff.ignoreFields["name"] {
		changes = append(changes, FieldChange{
			Path:     "name",
			OldValue: existing.Name,
			NewValue: new.Name,
			Type:     ChangeTypeUpdate,
		})
	}

	var existingWebsite, newWebsite string
	if existing.Website != nil {
		existingWebsite = *existing.Website
	}
	if new.Website != nil {
		newWebsite = *new.Website
	}
	if existingWebsite != newWebsite && !diff.ignoreFields["website"] {
		changes = append(changes, FieldChange{
			Path:     "website",
			OldValue: existingWebsite,
			NewValue: newWebsite,
			Type:     ChangeTypeUpdate,
		})
	}

	var existingDesc, newDesc string
	if existing.Description != nil {
		existingDesc = *existing.Description
	}
	if new.Description != nil {
		newDesc = *new.Description
	}
	if existingDesc != newDesc && !diff.ignoreFields["description"] {
		changes = append(changes, FieldChange{
			Path:     "description",
			OldValue: truncateString(existingDesc, 50),
			NewValue: truncateString(newDesc, 50),
			Type:     ChangeTypeUpdate,
		})
	}

	if len(changes) == 0 {
		return nil
	}

	return &AuthorUpdate{
		ID:       existing.ID,
		Existing: existing,
		New:      new,
		Changes:  changes,
	}
}

// sortModelChangeset sorts all slices in the changeset
func sortModelChangeset(changeset *ModelChangeset) {
	sort.Slice(changeset.Added, func(i, j int) bool {
		return changeset.Added[i].ID < changeset.Added[j].ID
	})
	sort.Slice(changeset.Updated, func(i, j int) bool {
		return changeset.Updated[i].ID < changeset.Updated[j].ID
	})
	sort.Slice(changeset.Removed, func(i, j int) bool {
		return changeset.Removed[i].ID < changeset.Removed[j].ID
	})
}

// sortProviderChangeset sorts all slices in the changeset
func sortProviderChangeset(changeset *ProviderChangeset) {
	sort.Slice(changeset.Added, func(i, j int) bool {
		return changeset.Added[i].ID < changeset.Added[j].ID
	})
	sort.Slice(changeset.Updated, func(i, j int) bool {
		return changeset.Updated[i].ID < changeset.Updated[j].ID
	})
	sort.Slice(changeset.Removed, func(i, j int) bool {
		return changeset.Removed[i].ID < changeset.Removed[j].ID
	})
}

// sortAuthorChangeset sorts all slices in the changeset
func sortAuthorChangeset(changeset *AuthorChangeset) {
	sort.Slice(changeset.Added, func(i, j int) bool {
		return changeset.Added[i].ID < changeset.Added[j].ID
	})
	sort.Slice(changeset.Updated, func(i, j int) bool {
		return changeset.Updated[i].ID < changeset.Updated[j].ID
	})
	sort.Slice(changeset.Removed, func(i, j int) bool {
		return changeset.Removed[i].ID < changeset.Removed[j].ID
	})
}

// calculateSummary calculates changeset summary statistics
func calculateSummary(changeset *Changeset) ChangesetSummary {
	return ChangesetSummary{
		ModelsAdded:      len(changeset.Models.Added),
		ModelsUpdated:    len(changeset.Models.Updated),
		ModelsRemoved:    len(changeset.Models.Removed),
		ProvidersAdded:   len(changeset.Providers.Added),
		ProvidersUpdated: len(changeset.Providers.Updated),
		ProvidersRemoved: len(changeset.Providers.Removed),
		AuthorsAdded:     len(changeset.Authors.Added),
		AuthorsUpdated:   len(changeset.Authors.Updated),
		AuthorsRemoved:   len(changeset.Authors.Removed),
		TotalChanges: len(changeset.Models.Added) + len(changeset.Models.Updated) + len(changeset.Models.Removed) +
			len(changeset.Providers.Added) + len(changeset.Providers.Updated) + len(changeset.Providers.Removed) +
			len(changeset.Authors.Added) + len(changeset.Authors.Updated) + len(changeset.Authors.Removed),
	}
}

// Helper functions

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

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// equalStringSlices compares two string slices for equality
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// equalModalitySlices compares two ModelModality slices for equality
func equalModalitySlices(a, b []catalogs.ModelModality) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// joinModalities joins ModelModality slices into a string
func joinModalities(modalities []catalogs.ModelModality) string {
	if len(modalities) == 0 {
		return ""
	}
	strs := make([]string, len(modalities))
	for i, m := range modalities {
		strs[i] = string(m)
	}
	return strings.Join(strs, ",")
}
