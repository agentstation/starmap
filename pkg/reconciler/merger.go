package reconciler

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/utc"
)

// Merger performs the actual merging of resources
type Merger interface {
	// Models merges models from multiple sources
	Models(sources map[sources.Type][]catalogs.Model) ([]catalogs.Model, provenance.Map, error)

	// Providers merges providers from multiple sources
	Providers(sources map[sources.Type][]catalogs.Provider) ([]catalogs.Provider, provenance.Map, error)
}

// merger implements strategic three-way merge
// It's an internal implementation of the Merger interface
type merger struct {
	authorities authority.Authority
	strategy    Strategy
	tracker     provenance.Tracker
}

// newMerger creates a new strategic merger
// Returns the Merger interface to hide implementation details
func newMerger(authorities authority.Authority, strategy Strategy) Merger {
	return &merger{
		authorities: authorities,
		strategy:    strategy,
	}
}

// newMergerWithProvenance creates a new strategic merger with provenance tracking
func newMergerWithProvenance(authorities authority.Authority, strategy Strategy, tracker provenance.Tracker) Merger {
	return &merger{
		authorities: authorities,
		strategy:    strategy,
		tracker:     tracker,
	}
}

// Models merges models from multiple sources
func (merger *merger) Models(srcs map[sources.Type][]catalogs.Model) ([]catalogs.Model, provenance.Map, error) {
	// Create a map of models by ID across all sources
	modelsByID := make(map[string]map[sources.Type]catalogs.Model)

	// Collect all models
	for sourceType, models := range srcs {
		for _, model := range models {
			if modelsByID[model.ID] == nil {
				modelsByID[model.ID] = make(map[sources.Type]catalogs.Model)
			}
			modelsByID[model.ID][sourceType] = model
		}
	}

	var mergedModels []catalogs.Model
	allProvenance := make(provenance.Map)

	// Merge each model
	for modelID, sourceModels := range modelsByID {
		merged, history := merger.model(modelID, sourceModels)
		mergedModels = append(mergedModels, merged)

		// Add provenance with model prefix
		if merger.tracker != nil {
			for field, fieldProv := range history {
				key := fmt.Sprintf("models.%s.%s", modelID, field)
				// Convert FieldProvenance to []ProvenanceInfo
				provInfos := []provenance.Provenance{fieldProv.Current}
				provInfos = append(provInfos, fieldProv.History...)
				allProvenance[key] = provInfos
				merger.tracker.Track(sources.ResourceTypeModel, modelID, field, fieldProv.Current)
			}
		}
	}

	return mergedModels, allProvenance, nil
}

// Providers merges providers from multiple sources
func (merger *merger) Providers(srcs map[sources.Type][]catalogs.Provider) ([]catalogs.Provider, provenance.Map, error) {
	// Create a map of providers by ID across all sources
	providersByID := make(map[catalogs.ProviderID]map[sources.Type]catalogs.Provider)

	// Collect all providers
	for sourceType, providers := range srcs {
		for _, provider := range providers {
			if providersByID[provider.ID] == nil {
				providersByID[provider.ID] = make(map[sources.Type]catalogs.Provider)
			}
			providersByID[provider.ID][sourceType] = provider
		}
	}

	var mergedProviders []catalogs.Provider
	allProvenance := make(provenance.Map)

	// Merge each provider
	for providerID, sourceProviders := range providersByID {
		// Convert to pointer map for compatibility
		sourcePtrs := make(map[sources.Type]*catalogs.Provider)
		for source, provider := range sourceProviders {
			p := provider // Create a copy
			sourcePtrs[source] = &p
		}

		merged, history := merger.provider(providerID, sourcePtrs)
		if merged != nil {
			mergedProviders = append(mergedProviders, *merged)

			// Add provenance with provider prefix
			if merger.tracker != nil {
				for field, fieldProv := range history {
					key := fmt.Sprintf("providers.%s.%s", providerID, field)
					// Convert FieldProvenance to []ProvenanceInfo
					provInfos := []provenance.Provenance{fieldProv.Current}
					provInfos = append(provInfos, fieldProv.History...)
					allProvenance[key] = provInfos
					merger.tracker.Track(sources.ResourceTypeProvider, string(providerID), field, fieldProv.Current)
				}
			}
		}
	}

	return mergedProviders, allProvenance, nil
}

// model merges a single model from multiple sources
func (merger *merger) model(modelID string, sourceModels map[sources.Type]catalogs.Model) (catalogs.Model, map[string]provenance.Field) {
	merged := catalogs.Model{
		ID: modelID,
	}
	history := make(map[string]provenance.Field)

	// Model fields to merge - includes all fields with defined authorities
	// Using actual Go struct field names (capitalized)
	modelFields := []string{
		// Basic identity fields
		"Name", "Description", "Authors",

		// Pricing fields (models.dev is authoritative)
		"Pricing", // Will be handled by mergeComplexModelStructures

		// Limits fields (models.dev is authoritative)
		"Limits", // Will be handled by mergeComplexModelStructures

		// Metadata fields (models.dev is authoritative)
		"Metadata", // Will be handled by mergeComplexModelStructures

		// Core features (models.dev and provider API both contribute)
		"Features", // Will be handled by mergeComplexModelStructures

		// Generation parameters (Provider API is authoritative)
		"Generation",
	}

	// Merge each field according to authorities
	for _, fieldPath := range modelFields {
		value, sourceType := merger.modelField(fieldPath, sourceModels)
		if value != nil {
			merger.setModelFieldValue(&merged, fieldPath, value)

			history[fieldPath] = provenance.Field{
				Current: provenance.Provenance{
					Source:    sourceType,
					Field:     fieldPath,
					Value:     value,
					Timestamp: time.Now(),
				},
			}
		}
	}

	// Enhanced merging for complex nested structures
	merged = merger.complexModelStructures(merged, sourceModels, &history)

	// Set timestamps
	merged.UpdatedAt = utc.Now()
	if merged.CreatedAt.IsZero() {
		merged.CreatedAt = utc.Now()
	}

	return merged, history
}

// provider merges a single provider from multiple sources
func (merger *merger) provider(providerID catalogs.ProviderID, sourceProviders map[sources.Type]*catalogs.Provider) (*catalogs.Provider, map[string]provenance.Field) {
	if len(sourceProviders) == 0 {
		return nil, nil
	}

	// Start with a base provider
	var merged catalogs.Provider
	history := make(map[string]provenance.Field)

	// Provider fields to merge - using Go struct field names
	providerFields := []string{
		"Name", "Headquarters", "IconURL", "StatusPageURL",
		"Authors", "Models", "Aliases",
		// API configuration
		"APIKey", "EnvVars", "Catalog", "ChatCompletions",
		// Policy fields
		"PrivacyPolicy", "RetentionPolicy", "GovernancePolicy",
	}

	// Merge each field
	for _, fieldPath := range providerFields {
		value, sourceType := merger.providerField(fieldPath, sourceProviders)
		if value != nil {
			merger.setProviderFieldValue(&merged, fieldPath, value)

			history[fieldPath] = provenance.Field{
				Current: provenance.Provenance{
					Source:    sourceType,
					Field:     fieldPath,
					Value:     value,
					Timestamp: time.Now(),
				},
			}
		}
	}

	// Ensure ID is set
	merged.ID = providerID

	return &merged, history
}

// modelField merges a single field from multiple model sources
func (merger *merger) modelField(fieldPath string, sourceModels map[sources.Type]catalogs.Model) (any, sources.Type) {
	// Collect all values from sources
	values := make(map[sources.Type]any)
	for source, model := range sourceModels {
		if value := merger.modelFieldValue(model, fieldPath); value != nil {
			values[source] = value
		}
	}

	if len(values) > 0 {
		// Let the strategy decide - it will use authorities if it's AuthorityStrategy
		// or source priority if it's SourcePriorityStrategy
		value, source, _ := merger.strategy.ResolveConflict(fieldPath, values)
		return value, source
	}

	return nil, ""
}

// providerField merges a single provider field from multiple sources
func (merger *merger) providerField(fieldPath string, sourceProviders map[sources.Type]*catalogs.Provider) (any, sources.Type) {
	// Collect all values from sources
	values := make(map[sources.Type]any)
	for source, provider := range sourceProviders {
		if provider != nil {
			if value := merger.providerFieldValue(*provider, fieldPath); value != nil {
				values[source] = value
			}
		}
	}

	if len(values) > 0 {
		// Let the strategy decide - it will use authorities if it's AuthorityStrategy
		// or source priority if it's SourcePriorityStrategy
		value, source, _ := merger.strategy.ResolveConflict(fieldPath, values)
		return value, source
	}

	return nil, ""
}

// getModelFieldValue extracts a field value from a model using reflection
func (merger *merger) modelFieldValue(model catalogs.Model, fieldPath string) any {
	return merger.fieldValue(reflect.ValueOf(model), fieldPath)
}

// providerFieldValue extracts a field value from a provider using reflection
func (merger *merger) providerFieldValue(provider catalogs.Provider, fieldPath string) any {
	return merger.fieldValue(reflect.ValueOf(provider), fieldPath)
}

// fieldValue extracts a field value using reflection and dot notation
func (merger *merger) fieldValue(v reflect.Value, fieldPath string) any {
	parts := strings.Split(fieldPath, ".")

	current := v
	for _, part := range parts {
		if current.Kind() == reflect.Ptr {
			if current.IsNil() {
				return nil
			}
			current = current.Elem()
		}

		if current.Kind() != reflect.Struct {
			return nil
		}

		// Use the field name directly (already properly capitalized)
		field := current.FieldByName(part)
		if !field.IsValid() {
			return nil
		}

		current = field
	}

	if !current.IsValid() || current.IsZero() {
		return nil
	}

	return current.Interface()
}

// setModelFieldValue sets a field value on a model using reflection
func (merger *merger) setModelFieldValue(model *catalogs.Model, fieldPath string, value any) {
	merger.setFieldValue(reflect.ValueOf(model).Elem(), fieldPath, value)
}

// setProviderFieldValue sets a field value on a provider using reflection
func (merger *merger) setProviderFieldValue(provider *catalogs.Provider, fieldPath string, value any) {
	merger.setFieldValue(reflect.ValueOf(provider).Elem(), fieldPath, value)
}

// setFieldValue sets a field value using reflection and dot notation
func (merger *merger) setFieldValue(v reflect.Value, fieldPath string, value any) {
	parts := strings.Split(fieldPath, ".")

	current := v
	for i, part := range parts {
		if current.Kind() == reflect.Ptr {
			if current.IsNil() {
				// Create new struct for pointer field
				newStruct := reflect.New(current.Type().Elem())
				current.Set(newStruct)
			}
			current = current.Elem()
		}

		if current.Kind() != reflect.Struct {
			logging.Warn().
				Str("field_path", fieldPath).
				Str("part", part).
				Msg("Cannot set field - not a struct")
			return
		}

		// Use the field name directly (already properly capitalized)
		field := current.FieldByName(part)
		if !field.IsValid() {
			logging.Warn().
				Str("field_name", part).
				Msg("Field not found in struct")
			return
		}

		// If this is the last part, set the value
		if i == len(parts)-1 {
			if field.CanSet() {
				valueReflect := reflect.ValueOf(value)
				if valueReflect.Type().ConvertibleTo(field.Type()) {
					field.Set(valueReflect.Convert(field.Type()))
				} else {
					logging.Warn().
						Interface("value", value).
						Str("target_type", field.Type().String()).
						Str("field_path", fieldPath).
						Msg("Cannot convert value to target type")
				}
			} else {
				logging.Warn().
					Str("field_path", fieldPath).
					Msg("Field is not settable")
			}
			return
		}

		current = field
	}
}

// complexModelStructures handles merging of complex nested structures
func (merger *merger) complexModelStructures(merged catalogs.Model, sourceModels map[sources.Type]catalogs.Model, history *map[string]provenance.Field) catalogs.Model {
	// Define priority order for complex structure merging
	priorities := []sources.Type{
		sources.LocalCatalog,
		sources.ModelsDevHTTP,
		sources.ModelsDevGit,
		sources.ProviderAPI,
	}

	// Merge Limits structure (models.dev is authoritative)
	for _, sourceType := range priorities {
		if model, exists := sourceModels[sourceType]; exists && model.Limits != nil {
			if merged.Limits == nil {
				merged.Limits = &catalogs.ModelLimits{}
			}

			// Merge specific limit fields if they're not already set or source has higher authority
			switch sourceType {
			case sources.ModelsDevHTTP, sources.ModelsDevGit:
				if model.Limits.ContextWindow > 0 {
					merged.Limits.ContextWindow = model.Limits.ContextWindow
					if history != nil {
						(*history)["limits.context_window"] = provenance.Field{
							Current: provenance.Provenance{
								Source:    sourceType,
								Field:     "limits.context_window",
								Value:     model.Limits.ContextWindow,
								Timestamp: time.Now(),
							},
						}
					}
				}
				if model.Limits.OutputTokens > 0 {
					merged.Limits.OutputTokens = model.Limits.OutputTokens
					if history != nil {
						(*history)["limits.output_tokens"] = provenance.Field{
							Current: provenance.Provenance{
								Source:    sourceType,
								Field:     "limits.output_tokens",
								Value:     model.Limits.OutputTokens,
								Timestamp: time.Now(),
							},
						}
					}
				}
			}
			break // Use first available source in priority order
		}
	}

	// Merge Pricing structure (models.dev is authoritative)
	for _, sourceType := range priorities {
		if model, exists := sourceModels[sourceType]; exists && model.Pricing != nil {
			switch sourceType {
			case sources.ModelsDevHTTP, sources.ModelsDevGit:
				merged.Pricing = model.Pricing
				if history != nil {
					(*history)["pricing"] = provenance.Field{
						Current: provenance.Provenance{
							Source:    sourceType,
							Field:     "pricing",
							Value:     model.Pricing,
							Timestamp: time.Now(),
						},
					}
				}
			}
		}
	}

	// Merge Metadata structure (models.dev is authoritative)
	for _, sourceType := range priorities {
		if model, exists := sourceModels[sourceType]; exists && model.Metadata != nil {
			switch sourceType {
			case sources.ModelsDevHTTP, sources.ModelsDevGit:
				if merged.Metadata == nil {
					merged.Metadata = &catalogs.ModelMetadata{}
				}

				// Copy metadata fields from models.dev
				if !model.Metadata.ReleaseDate.IsZero() {
					merged.Metadata.ReleaseDate = model.Metadata.ReleaseDate
				}
				if model.Metadata.KnowledgeCutoff != nil && !model.Metadata.KnowledgeCutoff.IsZero() {
					merged.Metadata.KnowledgeCutoff = model.Metadata.KnowledgeCutoff
				}
				merged.Metadata.OpenWeights = model.Metadata.OpenWeights

				if history != nil {
					(*history)["metadata"] = provenance.Field{
						Current: provenance.Provenance{
							Source:    sourceType,
							Field:     "metadata",
							Value:     model.Metadata,
							Timestamp: time.Now(),
						},
					}
				}
			}
		}
	}

	// Merge Features structure (combination of sources)
	for _, sourceType := range priorities {
		if model, exists := sourceModels[sourceType]; exists && model.Features != nil {
			if merged.Features == nil {
				merged.Features = &catalogs.ModelFeatures{}
			}

			// For features, we merge from all sources, with provider API getting priority for capabilities
			switch sourceType {
			case sources.ProviderAPI:
				// Provider API is authoritative for current capabilities
				merged.Features.Modalities = model.Features.Modalities
				merged.Features.Streaming = model.Features.Streaming
				// Copy core generation features
				merged.Features.Temperature = model.Features.Temperature
				merged.Features.TopP = model.Features.TopP
				merged.Features.MaxTokens = model.Features.MaxTokens
			case sources.ModelsDevHTTP, sources.ModelsDevGit:
				// models.dev might have additional feature information that's not in API
				if model.Features.ToolCalls && !merged.Features.ToolCalls {
					merged.Features.ToolCalls = model.Features.ToolCalls
				}
				if model.Features.Tools && !merged.Features.Tools {
					merged.Features.Tools = model.Features.Tools
				}
				if model.Features.ToolChoice && !merged.Features.ToolChoice {
					merged.Features.ToolChoice = model.Features.ToolChoice
				}
				if model.Features.WebSearch && !merged.Features.WebSearch {
					merged.Features.WebSearch = model.Features.WebSearch
				}
				if model.Features.Reasoning && !merged.Features.Reasoning {
					merged.Features.Reasoning = model.Features.Reasoning
				}
				// Merge modalities if not already set
				if len(merged.Features.Modalities.Input) == 0 && len(model.Features.Modalities.Input) > 0 {
					merged.Features.Modalities.Input = model.Features.Modalities.Input
				}
				if len(merged.Features.Modalities.Output) == 0 && len(model.Features.Modalities.Output) > 0 {
					merged.Features.Modalities.Output = model.Features.Modalities.Output
				}
			}
		}
	}

	return merged
}
