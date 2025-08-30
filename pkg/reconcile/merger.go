package reconcile

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/utc"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Merger performs the actual merging of resources
type Merger interface {
	// MergeModels merges models from multiple sources
	MergeModels(sources map[SourceName][]catalogs.Model) ([]catalogs.Model, ProvenanceMap, error)

	// MergeProviders merges providers from multiple sources
	MergeProviders(sources map[SourceName][]catalogs.Provider) ([]catalogs.Provider, ProvenanceMap, error)

	// MergeField merges a single field using authorities
	MergeField(fieldPath string, values map[SourceName]interface{}) (interface{}, SourceName)
	
	// WithProvenance sets the provenance tracker
	WithProvenance(tracker ProvenanceTracker)
}

// StrategicMerger implements strategic three-way merge
type StrategicMerger struct {
	authorities AuthorityProvider
	strategy    Strategy
	tracker     ProvenanceTracker
}

// NewStrategicMerger creates a new StrategicMerger
func NewStrategicMerger(authorities AuthorityProvider, strategy Strategy) *StrategicMerger {
	return &StrategicMerger{
		authorities: authorities,
		strategy:    strategy,
	}
}

// WithProvenance sets the provenance tracker
func (sm *StrategicMerger) WithProvenance(tracker ProvenanceTracker) {
	sm.tracker = tracker
}

// MergeModels merges models from multiple sources
func (sm *StrategicMerger) MergeModels(sources map[SourceName][]catalogs.Model) ([]catalogs.Model, ProvenanceMap, error) {
	// Create a map of models by ID across all sources
	modelsByID := make(map[string]map[SourceName]catalogs.Model)

	// Collect all models
	for sourceType, models := range sources {
		for _, model := range models {
			if modelsByID[model.ID] == nil {
				modelsByID[model.ID] = make(map[SourceName]catalogs.Model)
			}
			modelsByID[model.ID][sourceType] = model
		}
	}

	var mergedModels []catalogs.Model
	allProvenance := make(ProvenanceMap)

	// Merge each model
	for modelID, sourceModels := range modelsByID {
		merged, provenance := sm.mergeModel(modelID, sourceModels)
		mergedModels = append(mergedModels, merged)

		// Add provenance with model prefix
		if sm.tracker != nil {
			for field, fieldProv := range provenance {
				key := fmt.Sprintf("models.%s.%s", modelID, field)
				// Convert FieldProvenance to []ProvenanceInfo
				provInfos := []ProvenanceInfo{fieldProv.Current}
				provInfos = append(provInfos, fieldProv.History...)
				allProvenance[key] = provInfos
				sm.tracker.Track(ResourceTypeModel, modelID, field, fieldProv.Current)
			}
		}
	}

	return mergedModels, allProvenance, nil
}

// MergeProviders merges providers from multiple sources
func (sm *StrategicMerger) MergeProviders(sources map[SourceName][]catalogs.Provider) ([]catalogs.Provider, ProvenanceMap, error) {
	// Create a map of providers by ID across all sources
	providersByID := make(map[catalogs.ProviderID]map[SourceName]catalogs.Provider)

	// Collect all providers
	for sourceType, providers := range sources {
		for _, provider := range providers {
			if providersByID[provider.ID] == nil {
				providersByID[provider.ID] = make(map[SourceName]catalogs.Provider)
			}
			providersByID[provider.ID][sourceType] = provider
		}
	}

	var mergedProviders []catalogs.Provider
	allProvenance := make(ProvenanceMap)

	// Merge each provider
	for providerID, sourceProviders := range providersByID {
		// Convert to pointer map for compatibility
		sourcePtrs := make(map[SourceName]*catalogs.Provider)
		for source, provider := range sourceProviders {
			p := provider // Create a copy
			sourcePtrs[source] = &p
		}
		
		merged, provenance := sm.mergeProvider(providerID, sourcePtrs)
		if merged != nil {
			mergedProviders = append(mergedProviders, *merged)

			// Add provenance with provider prefix
			if sm.tracker != nil {
				for field, fieldProv := range provenance {
					key := fmt.Sprintf("providers.%s.%s", providerID, field)
					// Convert FieldProvenance to []ProvenanceInfo
					provInfos := []ProvenanceInfo{fieldProv.Current}
					provInfos = append(provInfos, fieldProv.History...)
					allProvenance[key] = provInfos
					sm.tracker.Track(ResourceTypeProvider, string(providerID), field, fieldProv.Current)
				}
			}
		}
	}

	return mergedProviders, allProvenance, nil
}

// MergeField merges a single field using authorities
func (sm *StrategicMerger) MergeField(fieldPath string, values map[SourceName]interface{}) (interface{}, SourceName) {
	// Use the strategy to determine the value
	value, source, _ := sm.strategy.ResolveConflict(fieldPath, values)
	return value, source
}

// mergeModel merges a single model from multiple sources
func (sm *StrategicMerger) mergeModel(modelID string, sourceModels map[SourceName]catalogs.Model) (catalogs.Model, map[string]FieldProvenance) {
	merged := catalogs.Model{
		ID: modelID,
	}
	provenance := make(map[string]FieldProvenance)

	// Model fields to merge - includes all fields with defined authorities
	modelFields := []string{
		// Basic identity fields
		"name", "description", "authors",

		// Pricing fields (models.dev is authoritative)
		"pricing.input", "pricing.output", "pricing.caching", "pricing.batch", "pricing.image",

		// Limits fields (models.dev is authoritative)
		"limits.context_window", "limits.output_tokens",

		// Metadata fields (models.dev is authoritative)
		"metadata.knowledge_cutoff", "metadata.release_date", "metadata.open_weights",
		"metadata.tags", "metadata.architecture",

		// Core features (models.dev and provider API both contribute)
		"features.tool_calls", "features.tools", "features.tool_choice", "features.web_search",
		"features.reasoning", "features.streaming", "features.temperature", "features.top_p", "features.max_tokens",

		// Generation parameters (Provider API is authoritative)
		"generation",
	}

	// Merge each field according to authorities
	for _, fieldPath := range modelFields {
		value, sourceType := sm.mergeModelField(fieldPath, sourceModels)
		if value != nil {
			sm.setModelFieldValue(&merged, fieldPath, value)

			provenance[fieldPath] = FieldProvenance{
				Current: ProvenanceInfo{
					Source:    sourceType,
					Field:     fieldPath,
					Value:     value,
					Timestamp: time.Now(),
				},
			}
		}
	}

	// Enhanced merging for complex nested structures
	merged = sm.mergeComplexModelStructures(merged, sourceModels, &provenance)

	// Set timestamps
	merged.UpdatedAt = utc.Now()
	if merged.CreatedAt.IsZero() {
		merged.CreatedAt = utc.Now()
	}

	return merged, provenance
}

// mergeProvider merges a single provider from multiple sources
func (sm *StrategicMerger) mergeProvider(providerID catalogs.ProviderID, sourceProviders map[SourceName]*catalogs.Provider) (*catalogs.Provider, map[string]FieldProvenance) {
	if len(sourceProviders) == 0 {
		return nil, nil
	}

	// Start with a base provider
	var merged catalogs.Provider
	provenance := make(map[string]FieldProvenance)

	// Provider fields to merge
	providerFields := []string{
		"name", "headquarters", "icon_url", "status_page_url",
		"authors",
		// API configuration
		"api_key", "env_vars", "catalog", "chat_completions",
		// Policy fields
		"privacy_policy", "retention_policy", "governance_policy",
	}

	// Merge each field
	for _, fieldPath := range providerFields {
		value, sourceType := sm.mergeProviderField(fieldPath, sourceProviders)
		if value != nil {
			sm.setProviderFieldValue(&merged, fieldPath, value)

			provenance[fieldPath] = FieldProvenance{
				Current: ProvenanceInfo{
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

	return &merged, provenance
}

// mergeModelField merges a single field from multiple model sources
func (sm *StrategicMerger) mergeModelField(fieldPath string, sourceModels map[SourceName]catalogs.Model) (interface{}, SourceName) {
	// Get the authority for this field
	authority := sm.authorities.GetAuthority(fieldPath, ResourceTypeModel)

	// If we have a specific authority, try that source first
	if authority != nil {
		if model, exists := sourceModels[authority.Source]; exists {
			if value := sm.getModelFieldValue(model, fieldPath); value != nil {
				return value, authority.Source
			}
		}
	}

	// Fall back to checking sources using the strategy
	values := make(map[SourceName]interface{})
	for source, model := range sourceModels {
		if value := sm.getModelFieldValue(model, fieldPath); value != nil {
			values[source] = value
		}
	}

	if len(values) > 0 {
		value, source, _ := sm.strategy.ResolveConflict(fieldPath, values)
	return value, source
	}

	return nil, ""
}

// mergeProviderField merges a single provider field from multiple sources
func (sm *StrategicMerger) mergeProviderField(fieldPath string, sourceProviders map[SourceName]*catalogs.Provider) (interface{}, SourceName) {
	// Get the authority for this field
	authority := sm.authorities.GetAuthority(fieldPath, ResourceTypeProvider)

	// If we have a specific authority, try that source first
	if authority != nil {
		if provider, exists := sourceProviders[authority.Source]; exists && provider != nil {
			if value := sm.getProviderFieldValue(*provider, fieldPath); value != nil {
				return value, authority.Source
			}
		}
	}

	// Fall back to checking sources using the strategy
	values := make(map[SourceName]interface{})
	for source, provider := range sourceProviders {
		if provider != nil {
			if value := sm.getProviderFieldValue(*provider, fieldPath); value != nil {
				values[source] = value
			}
		}
	}

	if len(values) > 0 {
		value, source, _ := sm.strategy.ResolveConflict(fieldPath, values)
	return value, source
	}

	return nil, ""
}

// getModelFieldValue extracts a field value from a model using reflection
func (sm *StrategicMerger) getModelFieldValue(model catalogs.Model, fieldPath string) interface{} {
	return sm.getFieldValue(reflect.ValueOf(model), fieldPath)
}

// getProviderFieldValue extracts a field value from a provider using reflection
func (sm *StrategicMerger) getProviderFieldValue(provider catalogs.Provider, fieldPath string) interface{} {
	return sm.getFieldValue(reflect.ValueOf(provider), fieldPath)
}

// getFieldValue extracts a field value using reflection and dot notation
func (sm *StrategicMerger) getFieldValue(v reflect.Value, fieldPath string) interface{} {
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

		field := current.FieldByName(cases.Title(language.English).String(part))
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
func (sm *StrategicMerger) setModelFieldValue(model *catalogs.Model, fieldPath string, value interface{}) {
	sm.setFieldValue(reflect.ValueOf(model).Elem(), fieldPath, value)
}

// setProviderFieldValue sets a field value on a provider using reflection
func (sm *StrategicMerger) setProviderFieldValue(provider *catalogs.Provider, fieldPath string, value interface{}) {
	sm.setFieldValue(reflect.ValueOf(provider).Elem(), fieldPath, value)
}

// setFieldValue sets a field value using reflection and dot notation
func (sm *StrategicMerger) setFieldValue(v reflect.Value, fieldPath string, value interface{}) {
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

		fieldName := cases.Title(language.English).String(part)
		field := current.FieldByName(fieldName)
		if !field.IsValid() {
			logging.Warn().
				Str("field_name", fieldName).
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

// mergeComplexModelStructures handles merging of complex nested structures
func (sm *StrategicMerger) mergeComplexModelStructures(merged catalogs.Model, sourceModels map[SourceName]catalogs.Model, provenance *map[string]FieldProvenance) catalogs.Model {
	// Define priority order for complex structure merging
	priorities := []SourceName{
		LocalCatalog,
		ModelsDevHTTP,
		ModelsDevGit,
		ProviderAPI,
	}

	// Merge Limits structure (models.dev is authoritative)
	for _, sourceType := range priorities {
		if model, exists := sourceModels[sourceType]; exists && model.Limits != nil {
			if merged.Limits == nil {
				merged.Limits = &catalogs.ModelLimits{}
			}

			// Merge specific limit fields if they're not already set or source has higher authority
			if sourceType == ModelsDevHTTP || sourceType == ModelsDevGit {
				if model.Limits.ContextWindow > 0 {
					merged.Limits.ContextWindow = model.Limits.ContextWindow
					if provenance != nil {
						(*provenance)["limits.context_window"] = FieldProvenance{
							Current: ProvenanceInfo{
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
					if provenance != nil {
						(*provenance)["limits.output_tokens"] = FieldProvenance{
							Current: ProvenanceInfo{
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
			if sourceType == ModelsDevHTTP || sourceType == ModelsDevGit {
				merged.Pricing = model.Pricing
				if provenance != nil {
					(*provenance)["pricing"] = FieldProvenance{
						Current: ProvenanceInfo{
							Source:    sourceType,
							Field:     "pricing",
							Value:     model.Pricing,
							Timestamp: time.Now(),
						},
					}
				}
				break
			}
		}
	}

	// Merge Metadata structure (models.dev is authoritative)
	for _, sourceType := range priorities {
		if model, exists := sourceModels[sourceType]; exists && model.Metadata != nil {
			if sourceType == ModelsDevHTTP || sourceType == ModelsDevGit {
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

				if provenance != nil {
					(*provenance)["metadata"] = FieldProvenance{
						Current: ProvenanceInfo{
							Source:    sourceType,
							Field:     "metadata",
							Value:     model.Metadata,
							Timestamp: time.Now(),
						},
					}
				}
				break
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
			if sourceType == ProviderAPI {
				// Provider API is authoritative for current capabilities
				merged.Features.Modalities = model.Features.Modalities
				merged.Features.Streaming = model.Features.Streaming
				// Copy core generation features
				merged.Features.Temperature = model.Features.Temperature
				merged.Features.TopP = model.Features.TopP
				merged.Features.MaxTokens = model.Features.MaxTokens
			} else if sourceType == ModelsDevHTTP || sourceType == ModelsDevGit {
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