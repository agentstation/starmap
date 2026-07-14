package differ

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/pkg/catalogs"
)

const differFieldName = "name"

// Differ detects changes between catalog resources.
//
// Differ is concrete because the package has one implementation. Its algorithm
// inputs remain the narrow catalogs.Reader interface where substitutability is
// useful.
type Differ struct {
	// Options for controlling diff behavior
	ignoreFields   map[string]bool
	deepComparison bool
	tracking       bool
}

// New creates a updated Differ with default settings.
func New(opts ...Option) *Differ {
	d := &Differ{
		ignoreFields:   make(map[string]bool),
		deepComparison: true,
		tracking:       true,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// Models compares two sets of models and returns changes.
func (diff *Differ) Models(existing, updated []*catalogs.Model) *ModelChangeset {
	changeset := &ModelChangeset{
		Added:         []catalogs.Model{},
		AddedScoped:   []ModelChange{},
		Updated:       []ModelUpdate{},
		Removed:       []catalogs.Model{},
		RemovedScoped: []ModelChange{},
	}

	// Create maps for efficient lookup
	existingMap := make(map[string]*catalogs.Model)
	for _, model := range existing {
		existingMap[model.ID] = model
	}

	newMap := make(map[string]*catalogs.Model)
	for _, model := range updated {
		newMap[model.ID] = model
	}

	// Find added and updated models
	for _, newModel := range updated {
		if existingModel, exists := existingMap[newModel.ID]; exists {
			// Check if model has been updated
			if update := diff.model(*existingModel, *newModel); update != nil {
				changeset.Updated = append(changeset.Updated, *update)
			}
		} else {
			// New model
			changeset.Added = append(changeset.Added, *newModel)
		}
	}

	// Find removed models
	for _, existingModel := range existing {
		if _, exists := newMap[existingModel.ID]; !exists {
			changeset.Removed = append(changeset.Removed, *existingModel)
		}
	}

	// Sort for consistent output
	sortModelChangeset(changeset)

	return changeset
}

// Providers compares two sets of providers and returns changes.
//
//nolint:dupl // Similar to Authors method but for different types
func (diff *Differ) Providers(existing, updated []catalogs.Provider) *ProviderChangeset {
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
	for _, provider := range updated {
		newMap[provider.ID] = provider
	}

	// Find added and updated providers
	for _, newProvider := range updated {
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

// Authors compares two sets of authors and returns changes.
//
//nolint:dupl // Similar to Providers method but for different types
func (diff *Differ) Authors(existing, updated []catalogs.Author) *AuthorChangeset {
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
	for _, author := range updated {
		newMap[author.ID] = author
	}

	// Find added and updated authors
	for _, newAuthor := range updated {
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
// Both catalogs only need to be readable.
func (diff *Differ) Catalogs(existing, updated catalogs.Reader) *Changeset {
	changeset := &Changeset{}

	// Diff providers
	existingProviders := existing.Providers().List()
	newProviders := updated.Providers().List()
	changeset.Models = diff.providerScopedModels(existingProviders, newProviders)
	changeset.Providers = diff.Providers(existingProviders, newProviders)

	// Diff authors
	existingAuthors := existing.Authors().List()
	newAuthors := updated.Authors().List()
	changeset.Authors = diff.Authors(existingAuthors, newAuthors)
	existingDefinitions, existingOfferings := canonicalRecords(existing)
	updatedDefinitions, updatedOfferings := canonicalRecords(updated)
	changeset.Definitions = diff.modelDefinitions(existingDefinitions, updatedDefinitions)
	changeset.Offerings = diff.providerOfferings(existingOfferings, updatedOfferings)

	// Calculate summary
	changeset.Summary = calculateSummary(changeset.Models, changeset.Providers, changeset.Authors, changeset.Definitions, changeset.Offerings)

	return changeset
}

func canonicalRecords(reader catalogs.Reader) ([]catalogs.ModelDefinition, []catalogs.ProviderOffering) {
	return reader.Definitions(), reader.Offerings()
}

func (diff *Differ) modelDefinitions(existing, updated []catalogs.ModelDefinition) *ModelDefinitionChangeset {
	changes := &ModelDefinitionChangeset{}
	existingByID := make(map[catalogs.ModelDefinitionID]catalogs.ModelDefinition, len(existing))
	updatedByID := make(map[catalogs.ModelDefinitionID]catalogs.ModelDefinition, len(updated))
	for _, value := range existing {
		existingByID[value.ID] = value
	}
	for _, value := range updated {
		updatedByID[value.ID] = value
	}
	for _, value := range updated {
		if previous, found := existingByID[value.ID]; !found {
			changes.Added = append(changes.Added, value)
		} else if !reflect.DeepEqual(previous, value) {
			changes.Updated = append(changes.Updated, ModelDefinitionUpdate{ID: value.ID, Existing: previous, New: value})
		}
	}
	for _, value := range existing {
		if _, found := updatedByID[value.ID]; !found {
			changes.Removed = append(changes.Removed, value)
		}
	}
	slices.SortFunc(changes.Added, func(left, right catalogs.ModelDefinition) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	slices.SortFunc(changes.Removed, func(left, right catalogs.ModelDefinition) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	slices.SortFunc(changes.Updated, func(left, right ModelDefinitionUpdate) int { return strings.Compare(string(left.ID), string(right.ID)) })
	return changes
}

func (diff *Differ) providerOfferings(existing, updated []catalogs.ProviderOffering) *ProviderOfferingChangeset {
	changes := &ProviderOfferingChangeset{}
	existingByKey := make(map[catalogs.OfferingKey]catalogs.ProviderOffering, len(existing))
	updatedByKey := make(map[catalogs.OfferingKey]catalogs.ProviderOffering, len(updated))
	for _, value := range existing {
		existingByKey[value.Key()] = value
	}
	for _, value := range updated {
		updatedByKey[value.Key()] = value
	}
	for _, value := range updated {
		if previous, found := existingByKey[value.Key()]; !found {
			changes.Added = append(changes.Added, value)
		} else if !reflect.DeepEqual(previous, value) {
			changes.Updated = append(changes.Updated, ProviderOfferingUpdate{Key: value.Key(), Existing: previous, New: value})
		}
	}
	for _, value := range existing {
		if _, found := updatedByKey[value.Key()]; !found {
			changes.Removed = append(changes.Removed, value)
		}
	}
	compareOffering := func(left, right catalogs.ProviderOffering) int {
		return strings.Compare(string(left.ProviderID)+"/"+string(left.ProviderModelID), string(right.ProviderID)+"/"+string(right.ProviderModelID))
	}
	slices.SortFunc(changes.Added, compareOffering)
	slices.SortFunc(changes.Removed, compareOffering)
	slices.SortFunc(changes.Updated, func(left, right ProviderOfferingUpdate) int {
		return strings.Compare(string(left.Key.ProviderID)+"/"+string(left.Key.ProviderModelID), string(right.Key.ProviderID)+"/"+string(right.Key.ProviderModelID))
	})
	return changes
}

func (diff *Differ) providerScopedModels(existingProviders, updatedProviders []catalogs.Provider) *ModelChangeset {
	changeset := &ModelChangeset{
		Added:         []catalogs.Model{},
		AddedScoped:   []ModelChange{},
		Updated:       []ModelUpdate{},
		Removed:       []catalogs.Model{},
		RemovedScoped: []ModelChange{},
	}

	existingByProvider := make(map[catalogs.ProviderID]catalogs.Provider, len(existingProviders))
	for _, provider := range existingProviders {
		existingByProvider[provider.ID] = provider
	}
	updatedByProvider := make(map[catalogs.ProviderID]catalogs.Provider, len(updatedProviders))
	for _, provider := range updatedProviders {
		updatedByProvider[provider.ID] = provider
	}

	for _, updatedProvider := range updatedProviders {
		existingProvider, exists := existingByProvider[updatedProvider.ID]
		if !exists {
			models := providerModels(updatedProvider)
			changeset.Added = append(changeset.Added, models...)
			changeset.AddedScoped = append(changeset.AddedScoped, providerModelChanges(updatedProvider.ID, models)...)
			continue
		}

		providerChanges := diff.Models(
			providerModelPointers(existingProvider),
			providerModelPointers(updatedProvider),
		)
		for i := range providerChanges.Updated {
			providerChanges.Updated[i].ProviderID = updatedProvider.ID
		}
		changeset.Added = append(changeset.Added, providerChanges.Added...)
		changeset.AddedScoped = append(changeset.AddedScoped, providerModelChanges(updatedProvider.ID, providerChanges.Added)...)
		changeset.Updated = append(changeset.Updated, providerChanges.Updated...)
		changeset.Removed = append(changeset.Removed, providerChanges.Removed...)
		changeset.RemovedScoped = append(changeset.RemovedScoped, providerModelChanges(updatedProvider.ID, providerChanges.Removed)...)
	}

	for _, existingProvider := range existingProviders {
		if _, exists := updatedByProvider[existingProvider.ID]; !exists {
			models := providerModels(existingProvider)
			changeset.Removed = append(changeset.Removed, models...)
			changeset.RemovedScoped = append(changeset.RemovedScoped, providerModelChanges(existingProvider.ID, models)...)
		}
	}

	sortModelChangeset(changeset)
	return changeset
}

func providerModelPointers(provider catalogs.Provider) []*catalogs.Model {
	models := make([]*catalogs.Model, 0, len(provider.Models))
	for _, model := range provider.Models {
		if model != nil {
			models = append(models, model)
		}
	}
	return models
}

func providerModels(provider catalogs.Provider) []catalogs.Model {
	models := make([]catalogs.Model, 0, len(provider.Models))
	for _, model := range provider.Models {
		if model != nil {
			models = append(models, *model)
		}
	}
	return models
}

func providerModelChanges(providerID catalogs.ProviderID, models []catalogs.Model) []ModelChange {
	changes := make([]ModelChange, 0, len(models))
	for _, model := range models {
		changes = append(changes, ModelChange{
			ProviderID: providerID,
			Model:      model,
		})
	}
	return changes
}

// model compares two models and returns an update if they differ.
func (diff *Differ) model(existing, updated catalogs.Model) *ModelUpdate {
	changes := []FieldChange{}

	// Compare basic fields
	if existing.Name != updated.Name && !diff.ignoreFields[differFieldName] {
		changes = append(changes, FieldChange{
			Path:     differFieldName,
			OldValue: existing.Name,
			NewValue: updated.Name,
			Type:     ChangeTypeUpdate,
		})
	}

	if existing.Description != updated.Description && !diff.ignoreFields["description"] {
		changes = append(changes, FieldChange{
			Path:     "description",
			OldValue: truncateString(existing.Description, 50),
			NewValue: truncateString(updated.Description, 50),
			Type:     ChangeTypeUpdate,
		})
	}

	if existing.Status != updated.Status && !diff.ignoreFields["status"] {
		changes = append(changes, FieldChange{
			Path:     "status",
			OldValue: existing.Status.String(),
			NewValue: updated.Status.String(),
			Type:     ChangeTypeUpdate,
		})
	}

	// Compare features
	if diff.deepComparison && !diff.ignoreFields["features"] {
		featureChanges := diffModelFeatures(existing.Features, updated.Features)
		changes = append(changes, featureChanges...)
	}

	if diff.deepComparison {
		if !diff.ignoreFields["attachments"] {
			changes = append(changes, diffModelPointer("attachments", existing.Attachments, updated.Attachments)...)
		}
		if !diff.ignoreFields["lineage"] {
			changes = append(changes, diffModelPointer("lineage", existing.Lineage, updated.Lineage)...)
		}
		if !diff.ignoreFields["generation"] {
			changes = append(changes, diffModelPointer("generation", existing.Generation, updated.Generation)...)
		}
		if !diff.ignoreFields["reasoning"] {
			changes = append(changes, diffModelPointer("reasoning", existing.Reasoning, updated.Reasoning)...)
		}
		if !diff.ignoreFields["reasoning_tokens"] {
			changes = append(changes, diffModelPointer("reasoning_tokens", existing.ReasoningTokens, updated.ReasoningTokens)...)
		}
		if !diff.ignoreFields["verbosity"] {
			changes = append(changes, diffModelPointer("verbosity", existing.Verbosity, updated.Verbosity)...)
		}
		if !diff.ignoreFields["tools"] {
			changes = append(changes, diffModelPointer("tools", existing.Tools, updated.Tools)...)
		}
		if !diff.ignoreFields["response"] {
			changes = append(changes, diffModelPointer("response", existing.Delivery, updated.Delivery)...)
		}
		if !diff.ignoreFields["modes"] && !reflect.DeepEqual(existing.Modes, updated.Modes) {
			changes = append(changes, FieldChange{
				Path:     "modes",
				OldValue: fmt.Sprintf("%d modes", len(existing.Modes)),
				NewValue: fmt.Sprintf("%d modes", len(updated.Modes)),
				Type:     ChangeTypeUpdate,
			})
		}
	}

	// Compare pricing
	if diff.deepComparison && !diff.ignoreFields["pricing"] {
		pricingChanges := diffModelPricing(existing.Pricing, updated.Pricing)
		changes = append(changes, pricingChanges...)
	}

	// Compare limits
	if diff.deepComparison && !diff.ignoreFields["limits"] {
		limitChanges := diffModelLimits(existing.Limits, updated.Limits)
		changes = append(changes, limitChanges...)
	}

	// Compare metadata
	if diff.deepComparison && !diff.ignoreFields["metadata"] {
		metadataChanges := diffModelMetadata(existing.Metadata, updated.Metadata)
		changes = append(changes, metadataChanges...)
	}

	if diff.deepComparison &&
		!diff.ignoreFields["extensions"] &&
		!reflect.DeepEqual(
			catalogs.NormalizeSourceExtensions(existing.Extensions),
			catalogs.NormalizeSourceExtensions(updated.Extensions),
		) {
		changes = append(changes, FieldChange{
			Path:     "extensions",
			OldValue: fmt.Sprintf("%d sources", len(existing.Extensions)),
			NewValue: fmt.Sprintf("%d sources", len(updated.Extensions)),
			Type:     ChangeTypeUpdate,
		})
	}

	// If no changes, return nil
	if len(changes) == 0 {
		return nil
	}

	return &ModelUpdate{
		ID:       existing.ID,
		Existing: existing,
		New:      updated,
		Changes:  changes,
	}
}

func diffModelPointer[T any](path string, existing, updated *T) []FieldChange {
	if reflect.DeepEqual(existing, updated) {
		return nil
	}
	return []FieldChange{{
		Path:     path,
		OldValue: formatPointerPresence(existing),
		NewValue: formatPointerPresence(updated),
		Type:     ChangeTypeUpdate,
	}}
}

func formatPointerPresence[T any](value *T) string {
	if value == nil {
		return "absent"
	}
	return "present"
}

// diffModelFeatures compares model features.
func diffModelFeatures(existing, updated *catalogs.ModelFeatures) []FieldChange {
	changes := []FieldChange{}

	if existing == nil && updated == nil {
		return changes
	}

	if existing == nil || updated == nil {
		changes = append(changes, FieldChange{
			Path:     "features",
			OldValue: fmt.Sprintf("%v", existing != nil),
			NewValue: fmt.Sprintf("%v", updated != nil),
			Type:     ChangeTypeUpdate,
		})
		return changes
	}

	// Compare boolean features
	if existing.Tools != updated.Tools {
		changes = append(changes, FieldChange{
			Path:     "features.tools",
			OldValue: fmt.Sprintf("%v", existing.Tools),
			NewValue: fmt.Sprintf("%v", updated.Tools),
			Type:     ChangeTypeUpdate,
		})
	}

	if existing.Reasoning != updated.Reasoning {
		changes = append(changes, FieldChange{
			Path:     "features.reasoning",
			OldValue: fmt.Sprintf("%v", existing.Reasoning),
			NewValue: fmt.Sprintf("%v", updated.Reasoning),
			Type:     ChangeTypeUpdate,
		})
	}

	if existing.Streaming != updated.Streaming {
		changes = append(changes, FieldChange{
			Path:     "features.streaming",
			OldValue: fmt.Sprintf("%v", existing.Streaming),
			NewValue: fmt.Sprintf("%v", updated.Streaming),
			Type:     ChangeTypeUpdate,
		})
	}

	// Compare modalities
	if !equalModalitySlices(existing.Modalities.Input, updated.Modalities.Input) {
		changes = append(changes, FieldChange{
			Path:     "features.modalities.input",
			OldValue: joinModalities(existing.Modalities.Input),
			NewValue: joinModalities(updated.Modalities.Input),
			Type:     ChangeTypeUpdate,
		})
	}

	if !equalModalitySlices(existing.Modalities.Output, updated.Modalities.Output) {
		changes = append(changes, FieldChange{
			Path:     "features.modalities.output",
			OldValue: joinModalities(existing.Modalities.Output),
			NewValue: joinModalities(updated.Modalities.Output),
			Type:     ChangeTypeUpdate,
		})
	}

	return changes
}

// diffModelPricing compares model pricing.
func diffModelPricing(existing, updated *catalogs.ModelPricing) []FieldChange {
	changes := []FieldChange{}

	if existing == nil && updated == nil {
		return changes
	}

	if existing == nil || updated == nil {
		changes = append(changes, FieldChange{
			Path:     "pricing",
			OldValue: fmt.Sprintf("%v", existing != nil),
			NewValue: fmt.Sprintf("%v", updated != nil),
			Type:     ChangeTypeUpdate,
		})
		return changes
	}

	if existing.Currency != updated.Currency {
		changes = append(changes, FieldChange{
			Path:     "pricing.currency",
			OldValue: string(existing.Currency),
			NewValue: string(updated.Currency),
			Type:     ChangeTypeUpdate,
		})
	}

	if !reflect.DeepEqual(existing.Tokens, updated.Tokens) {
		changes = append(changes, FieldChange{
			Path:     "pricing.tokens",
			OldValue: formatPresent(existing.Tokens != nil),
			NewValue: formatPresent(updated.Tokens != nil),
			Type:     ChangeTypeUpdate,
		})
	}

	if !reflect.DeepEqual(existing.Operations, updated.Operations) {
		changes = append(changes, FieldChange{
			Path:     "pricing.operations",
			OldValue: formatPresent(existing.Operations != nil),
			NewValue: formatPresent(updated.Operations != nil),
			Type:     ChangeTypeUpdate,
		})
	}

	if !reflect.DeepEqual(existing.Tiers, updated.Tiers) {
		changes = append(changes, FieldChange{
			Path:     "pricing.tiers",
			OldValue: fmt.Sprintf("%d tiers", len(existing.Tiers)),
			NewValue: fmt.Sprintf("%d tiers", len(updated.Tiers)),
			Type:     ChangeTypeUpdate,
		})
	}

	return changes
}

// diffModelLimits compares model limits.
func diffModelLimits(existing, updated *catalogs.ModelLimits) []FieldChange {
	changes := []FieldChange{}

	if existing == nil && updated == nil {
		return changes
	}

	if existing == nil || updated == nil {
		changes = append(changes, FieldChange{
			Path:     "limits",
			OldValue: fmt.Sprintf("%v", existing != nil),
			NewValue: fmt.Sprintf("%v", updated != nil),
			Type:     ChangeTypeUpdate,
		})
		return changes
	}

	if existing.ContextWindow != updated.ContextWindow {
		changes = append(changes, FieldChange{
			Path:     "limits.context_window",
			OldValue: formatTokens(existing.ContextWindow),
			NewValue: formatTokens(updated.ContextWindow),
			Type:     ChangeTypeUpdate,
		})
	}

	if existing.InputTokens != updated.InputTokens {
		changes = append(changes, FieldChange{
			Path:     "limits.input_tokens",
			OldValue: formatTokens(existing.InputTokens),
			NewValue: formatTokens(updated.InputTokens),
			Type:     ChangeTypeUpdate,
		})
	}

	if existing.OutputTokens != updated.OutputTokens {
		changes = append(changes, FieldChange{
			Path:     "limits.output_tokens",
			OldValue: formatTokens(existing.OutputTokens),
			NewValue: formatTokens(updated.OutputTokens),
			Type:     ChangeTypeUpdate,
		})
	}

	return changes
}

// diffModelMetadata compares model metadata.
func diffModelMetadata(existing, updated *catalogs.ModelMetadata) []FieldChange {
	changes := []FieldChange{}

	if existing == nil && updated == nil {
		return changes
	}

	if existing == nil || updated == nil {
		changes = append(changes, FieldChange{
			Path:     "metadata",
			OldValue: fmt.Sprintf("%v", existing != nil),
			NewValue: fmt.Sprintf("%v", updated != nil),
			Type:     ChangeTypeUpdate,
		})
		return changes
	}

	if !equalOptionalTime(existing.KnowledgeCutoff, updated.KnowledgeCutoff) {
		changes = append(changes, FieldChange{
			Path:     "metadata.knowledge_cutoff",
			OldValue: formatOptionalTime(existing.KnowledgeCutoff),
			NewValue: formatOptionalTime(updated.KnowledgeCutoff),
			Type:     ChangeTypeUpdate,
		})
	}

	if !existing.ReleaseDate.Equal(updated.ReleaseDate) {
		changes = append(changes, FieldChange{
			Path:     "metadata.release_date",
			OldValue: existing.ReleaseDate.Format("2006-01-02"),
			NewValue: updated.ReleaseDate.Format("2006-01-02"),
			Type:     ChangeTypeUpdate,
		})
	}

	if existing.OpenWeights != updated.OpenWeights {
		changes = append(changes, FieldChange{
			Path:     "metadata.open_weights",
			OldValue: fmt.Sprintf("%v", existing.OpenWeights),
			NewValue: fmt.Sprintf("%v", updated.OpenWeights),
			Type:     ChangeTypeUpdate,
		})
	}

	if !reflect.DeepEqual(existing.Tags, updated.Tags) {
		changes = append(changes, FieldChange{
			Path:     "metadata.tags",
			OldValue: fmt.Sprintf("%d tags", len(existing.Tags)),
			NewValue: fmt.Sprintf("%d tags", len(updated.Tags)),
			Type:     ChangeTypeUpdate,
		})
	}

	if !reflect.DeepEqual(existing.Architecture, updated.Architecture) {
		changes = append(changes, FieldChange{
			Path:     "metadata.architecture",
			OldValue: formatPresent(existing.Architecture != nil),
			NewValue: formatPresent(updated.Architecture != nil),
			Type:     ChangeTypeUpdate,
		})
	}

	return changes
}

// provider compares two providers.
func (diff *Differ) provider(existing, updated catalogs.Provider) *ProviderUpdate {
	changes := []FieldChange{}

	if existing.Name != updated.Name && !diff.ignoreFields[differFieldName] {
		changes = append(changes, FieldChange{
			Path:     differFieldName,
			OldValue: existing.Name,
			NewValue: updated.Name,
			Type:     ChangeTypeUpdate,
		})
	}

	if !reflect.DeepEqual(existing.Aliases, updated.Aliases) && !diff.ignoreFields["aliases"] {
		changes = append(changes, FieldChange{
			Path:     "aliases",
			OldValue: fmt.Sprintf("%d aliases", len(existing.Aliases)),
			NewValue: fmt.Sprintf("%d aliases", len(updated.Aliases)),
			Type:     ChangeTypeUpdate,
		})
	}

	if !equalOptionalString(existing.Headquarters, updated.Headquarters) && !diff.ignoreFields["headquarters"] {
		changes = append(changes, FieldChange{
			Path:     "headquarters",
			OldValue: optionalStringValue(existing.Headquarters),
			NewValue: optionalStringValue(updated.Headquarters),
			Type:     ChangeTypeUpdate,
		})
	}

	if !equalOptionalString(existing.IconURL, updated.IconURL) && !diff.ignoreFields["icon_url"] {
		changes = append(changes, FieldChange{
			Path:     "icon_url",
			OldValue: optionalStringValue(existing.IconURL),
			NewValue: optionalStringValue(updated.IconURL),
			Type:     ChangeTypeUpdate,
		})
	}

	// Check credential metadata changes.
	if !reflect.DeepEqual(existing.Credentials, updated.Credentials) && !diff.ignoreFields["credentials"] {
		changes = append(changes, FieldChange{
			Path:     "credentials",
			OldValue: "config changed",
			NewValue: "updated",
			Type:     ChangeTypeUpdate,
		})
	}

	if !reflect.DeepEqual(existing.Advisories, updated.Advisories) && !diff.ignoreFields["environment_advisories"] {
		changes = append(changes, FieldChange{
			Path:     "environment_advisories",
			OldValue: fmt.Sprintf("%d advisories", len(existing.Advisories)),
			NewValue: fmt.Sprintf("%d advisories", len(updated.Advisories)),
			Type:     ChangeTypeUpdate,
		})
	}

	// Check catalog settings changes
	if !reflect.DeepEqual(existing.Catalog, updated.Catalog) && !diff.ignoreFields["catalog"] {
		changes = append(changes, FieldChange{
			Path:     "catalog",
			OldValue: "settings changed",
			NewValue: "updated",
			Type:     ChangeTypeUpdate,
		})
	}

	if !equalOptionalString(existing.StatusPageURL, updated.StatusPageURL) && !diff.ignoreFields["status_page_url"] {
		changes = append(changes, FieldChange{
			Path:     "status_page_url",
			OldValue: optionalStringValue(existing.StatusPageURL),
			NewValue: optionalStringValue(updated.StatusPageURL),
			Type:     ChangeTypeUpdate,
		})
	}

	if !reflect.DeepEqual(existing.Invocation, updated.Invocation) && !diff.ignoreFields["invocation"] {
		changes = append(changes, FieldChange{
			Path:     "invocation",
			OldValue: formatPresent(existing.Invocation != nil),
			NewValue: formatPresent(updated.Invocation != nil),
			Type:     ChangeTypeUpdate,
		})
	}

	if !reflect.DeepEqual(existing.PrivacyPolicy, updated.PrivacyPolicy) && !diff.ignoreFields["privacy_policy"] {
		changes = append(changes, FieldChange{
			Path:     "privacy_policy",
			OldValue: formatPresent(existing.PrivacyPolicy != nil),
			NewValue: formatPresent(updated.PrivacyPolicy != nil),
			Type:     ChangeTypeUpdate,
		})
	}

	if !reflect.DeepEqual(existing.RetentionPolicy, updated.RetentionPolicy) && !diff.ignoreFields["retention_policy"] {
		changes = append(changes, FieldChange{
			Path:     "retention_policy",
			OldValue: formatPresent(existing.RetentionPolicy != nil),
			NewValue: formatPresent(updated.RetentionPolicy != nil),
			Type:     ChangeTypeUpdate,
		})
	}

	if !reflect.DeepEqual(existing.GovernancePolicy, updated.GovernancePolicy) && !diff.ignoreFields["governance_policy"] {
		changes = append(changes, FieldChange{
			Path:     "governance_policy",
			OldValue: formatPresent(existing.GovernancePolicy != nil),
			NewValue: formatPresent(updated.GovernancePolicy != nil),
			Type:     ChangeTypeUpdate,
		})
	}

	if !diff.ignoreFields["extensions"] &&
		!reflect.DeepEqual(
			catalogs.NormalizeSourceExtensions(existing.Extensions),
			catalogs.NormalizeSourceExtensions(updated.Extensions),
		) {
		changes = append(changes, FieldChange{
			Path:     "extensions",
			OldValue: fmt.Sprintf("%d sources", len(existing.Extensions)),
			NewValue: fmt.Sprintf("%d sources", len(updated.Extensions)),
			Type:     ChangeTypeUpdate,
		})
	}

	if len(changes) == 0 {
		return nil
	}

	return &ProviderUpdate{
		ID:       existing.ID,
		Existing: existing,
		New:      updated,
		Changes:  changes,
	}
}

func equalOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// author compares two authors.
func (diff *Differ) author(existing, updated catalogs.Author) *AuthorUpdate {
	changes := []FieldChange{}

	if existing.Name != updated.Name && !diff.ignoreFields[differFieldName] {
		changes = append(changes, FieldChange{
			Path:     differFieldName,
			OldValue: existing.Name,
			NewValue: updated.Name,
			Type:     ChangeTypeUpdate,
		})
	}

	var existingWebsite, newWebsite string
	if existing.Website != nil {
		existingWebsite = *existing.Website
	}
	if updated.Website != nil {
		newWebsite = *updated.Website
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
	if updated.Description != nil {
		newDesc = *updated.Description
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
		New:      updated,
		Changes:  changes,
	}
}

// sortModelChangeset sorts all slices in the changeset.
func sortModelChangeset(changeset *ModelChangeset) {
	sort.Slice(changeset.Added, func(i, j int) bool {
		return changeset.Added[i].ID < changeset.Added[j].ID
	})
	sort.Slice(changeset.AddedScoped, func(i, j int) bool {
		return modelChangeLess(changeset.AddedScoped[i], changeset.AddedScoped[j])
	})
	sort.Slice(changeset.Updated, func(i, j int) bool {
		if changeset.Updated[i].ID == changeset.Updated[j].ID {
			return changeset.Updated[i].ProviderID < changeset.Updated[j].ProviderID
		}
		return changeset.Updated[i].ID < changeset.Updated[j].ID
	})
	sort.Slice(changeset.Removed, func(i, j int) bool {
		return changeset.Removed[i].ID < changeset.Removed[j].ID
	})
	sort.Slice(changeset.RemovedScoped, func(i, j int) bool {
		return modelChangeLess(changeset.RemovedScoped[i], changeset.RemovedScoped[j])
	})
}

func modelChangeLess(left, right ModelChange) bool {
	if left.Model.ID == right.Model.ID {
		return left.ProviderID < right.ProviderID
	}
	return left.Model.ID < right.Model.ID
}

// sortProviderChangeset sorts all slices in the changeset.
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

// sortAuthorChangeset sorts all slices in the changeset.
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

// Helper functions

// formatTokens formats token counts for display.
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

func formatPresent(present bool) string {
	if present {
		return "present"
	}
	return "absent"
}

func equalOptionalTime(a, b *utc.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func formatOptionalTime(t *utc.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// truncateString truncates a string to a maximum length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// equalModalitySlices compares two ModelModality slices for equality.
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

// joinModalities joins ModelModality slices into a string.
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
