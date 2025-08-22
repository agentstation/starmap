package sources

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/utc"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Provenance tracks which source provided each field
type Provenance struct {
	Source    Type        `json:"source"`
	Value     interface{} `json:"value"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// FieldMerger merges data from multiple sources using provided field authorities
type FieldMerger struct {
	modelAuthorities    []FieldAuthority
	providerAuthorities []FieldAuthority
	trackProvenance     bool
}

// NewFieldMerger creates a new field merger
func NewFieldMerger() *FieldMerger {
	return &FieldMerger{
		modelAuthorities:    DefaultModelFieldAuthorities,
		providerAuthorities: DefaultProviderFieldAuthorities,
		trackProvenance:     false,
	}
}

// WithAuthorities sets custom field authorities
func (fm *FieldMerger) WithAuthorities(modelAuth, providerAuth []FieldAuthority) *FieldMerger {
	if modelAuth != nil {
		fm.modelAuthorities = modelAuth
	}
	if providerAuth != nil {
		fm.providerAuthorities = providerAuth
	}
	return fm
}

// WithProvenance enables provenance tracking
func (fm *FieldMerger) WithProvenance(enabled bool) *FieldMerger {
	fm.trackProvenance = enabled
	return fm
}

// MergeModels merges model data from multiple sources
func (fm *FieldMerger) MergeModels(sources map[Type][]catalogs.Model) ([]catalogs.Model, map[string]Provenance) {
	// Create a map of models by ID across all sources
	modelsByID := make(map[string]map[Type]catalogs.Model)

	// Collect all models
	for sourceType, models := range sources {
		for _, model := range models {
			if modelsByID[model.ID] == nil {
				modelsByID[model.ID] = make(map[Type]catalogs.Model)
			}
			modelsByID[model.ID][sourceType] = model
		}
	}

	var mergedModels []catalogs.Model
	allProvenance := make(map[string]Provenance)

	// Merge each model
	for modelID, sourceModels := range modelsByID {
		merged, provenance := fm.mergeModel(modelID, sourceModels)
		mergedModels = append(mergedModels, merged)

		// Add provenance with model prefix
		if fm.trackProvenance {
			for field, prov := range provenance {
				allProvenance[fmt.Sprintf("models.%s.%s", modelID, field)] = prov
			}
		}
	}

	return mergedModels, allProvenance
}

// MergeProvider merges provider data from multiple sources
func (fm *FieldMerger) MergeProvider(providerSources map[Type]*catalogs.Provider, providerID catalogs.ProviderID) (*catalogs.Provider, map[string]Provenance) {
	if len(providerSources) == 0 {
		return nil, nil
	}

	// Start with a base provider
	var merged catalogs.Provider
	provenance := make(map[string]Provenance)

	// Provider fields to merge - include all important fields
	providerFields := []string{
		"name", "headquarters", "icon_url", "status_page_url",
		"authors", // Add authors field
		// API configuration
		"api_key", "env_vars", "catalog", "chat_completions",
		// Policy fields
		"privacy_policy", "retention_policy", "governance_policy",
	}

	// Merge each field
	for _, fieldPath := range providerFields {
		value, sourceType := fm.mergeProviderField(fieldPath, providerSources)
		if value != nil {
			fm.setProviderFieldValue(&merged, fieldPath, value)

			if fm.trackProvenance {
				provenance[fieldPath] = Provenance{
					Source:    sourceType,
					Value:     value,
					UpdatedAt: time.Now(),
				}
			}
		}
	}

	// Ensure ID is set
	merged.ID = providerID

	return &merged, provenance
}

// mergeModel merges a single model from multiple sources
func (fm *FieldMerger) mergeModel(modelID string, sourceModels map[Type]catalogs.Model) (catalogs.Model, map[string]Provenance) {
	merged := catalogs.Model{
		ID: modelID,
	}
	provenance := make(map[string]Provenance)

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
		value, sourceType := fm.mergeModelField(fieldPath, sourceModels)
		if value != nil {
			fm.setModelFieldValue(&merged, fieldPath, value)

			if fm.trackProvenance {
				provenance[fieldPath] = Provenance{
					Source:    sourceType,
					Value:     value,
					UpdatedAt: time.Now(),
				}
			}
		}
	}

	// Enhanced merging for complex nested structures
	// This handles cases where field-by-field merge might miss some data
	merged = fm.mergeComplexStructures(merged, sourceModels, &provenance)

	// Set timestamps
	merged.UpdatedAt = utc.Now()
	if merged.CreatedAt.IsZero() {
		merged.CreatedAt = utc.Now()
	}

	return merged, provenance
}

// mergeModelField merges a single field from multiple sources
func (fm *FieldMerger) mergeModelField(fieldPath string, sourceModels map[Type]catalogs.Model) (interface{}, Type) {
	// Get the authority for this field
	authority := GetAuthorityForField(fieldPath, fm.modelAuthorities)

	// If we have a specific authority, try that source first
	if authority != nil {
		if model, exists := sourceModels[authority.Source]; exists {
			if value := fm.getModelFieldValue(model, fieldPath); value != nil {
				return value, authority.Source
			}
		}
	}

	// Fall back to checking sources in priority order
	priorities := []Type{
		LocalCatalog,
		ModelsDevHTTP,
		ModelsDevGit,
		ProviderAPI,
		DatabaseUI,
	}

	for _, sourceType := range priorities {
		if model, exists := sourceModels[sourceType]; exists {
			if value := fm.getModelFieldValue(model, fieldPath); value != nil {
				return value, sourceType
			}
		}
	}

	return nil, ""
}

// mergeProviderField merges a single provider field from multiple sources
func (fm *FieldMerger) mergeProviderField(fieldPath string, sourceProviders map[Type]*catalogs.Provider) (interface{}, Type) {
	// Get the authority for this field
	authority := GetAuthorityForField(fieldPath, fm.providerAuthorities)

	// If we have a specific authority, try that source first
	if authority != nil {
		if provider, exists := sourceProviders[authority.Source]; exists && provider != nil {
			if value := fm.getProviderFieldValue(*provider, fieldPath); value != nil {
				return value, authority.Source
			}
		}
	}

	// Fall back to checking sources in priority order
	priorities := []Type{
		LocalCatalog,
		ModelsDevHTTP,
		ModelsDevGit,
		ProviderAPI,
		DatabaseUI,
	}

	for _, sourceType := range priorities {
		if provider, exists := sourceProviders[sourceType]; exists && provider != nil {
			if value := fm.getProviderFieldValue(*provider, fieldPath); value != nil {
				return value, sourceType
			}
		}
	}

	return nil, ""
}

// getModelFieldValue extracts a field value from a model using reflection
func (fm *FieldMerger) getModelFieldValue(model catalogs.Model, fieldPath string) interface{} {
	return fm.getFieldValue(reflect.ValueOf(model), fieldPath)
}

// getProviderFieldValue extracts a field value from a provider using reflection
func (fm *FieldMerger) getProviderFieldValue(provider catalogs.Provider, fieldPath string) interface{} {
	return fm.getFieldValue(reflect.ValueOf(provider), fieldPath)
}

// getFieldValue extracts a field value using reflection and dot notation
func (fm *FieldMerger) getFieldValue(v reflect.Value, fieldPath string) interface{} {
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
func (fm *FieldMerger) setModelFieldValue(model *catalogs.Model, fieldPath string, value interface{}) {
	fm.setFieldValue(reflect.ValueOf(model).Elem(), fieldPath, value)
}

// setProviderFieldValue sets a field value on a provider using reflection
func (fm *FieldMerger) setProviderFieldValue(provider *catalogs.Provider, fieldPath string, value interface{}) {
	fm.setFieldValue(reflect.ValueOf(provider).Elem(), fieldPath, value)
}

// setFieldValue sets a field value using reflection and dot notation
func (fm *FieldMerger) setFieldValue(v reflect.Value, fieldPath string, value interface{}) {
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
			log.Printf("Warning: cannot set field %s - not a struct at part %s", fieldPath, part)
			return
		}

		fieldName := cases.Title(language.English).String(part)
		field := current.FieldByName(fieldName)
		if !field.IsValid() {
			log.Printf("Warning: field %s not found in struct", fieldName)
			return
		}

		// If this is the last part, set the value
		if i == len(parts)-1 {
			if field.CanSet() {
				valueReflect := reflect.ValueOf(value)
				if valueReflect.Type().ConvertibleTo(field.Type()) {
					field.Set(valueReflect.Convert(field.Type()))
				} else {
					log.Printf("Warning: cannot convert value %v to type %s for field %s", value, field.Type(), fieldPath)
				}
			} else {
				log.Printf("Warning: field %s is not settable", fieldPath)
			}
			return
		}

		current = field
	}
}

// mergeComplexStructures handles merging of complex nested structures
func (fm *FieldMerger) mergeComplexStructures(merged catalogs.Model, sourceModels map[Type]catalogs.Model, provenance *map[string]Provenance) catalogs.Model {
	// Define priority order for complex structure merging
	priorities := []Type{
		LocalCatalog,
		ModelsDevHTTP,
		ModelsDevGit,
		ProviderAPI,
		DatabaseUI,
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
					if fm.trackProvenance {
						(*provenance)["limits.context_window"] = Provenance{
							Source:    sourceType,
							Value:     model.Limits.ContextWindow,
							UpdatedAt: time.Now(),
						}
					}
				}
				if model.Limits.OutputTokens > 0 {
					merged.Limits.OutputTokens = model.Limits.OutputTokens
					if fm.trackProvenance {
						(*provenance)["limits.output_tokens"] = Provenance{
							Source:    sourceType,
							Value:     model.Limits.OutputTokens,
							UpdatedAt: time.Now(),
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
				if fm.trackProvenance {
					(*provenance)["pricing"] = Provenance{
						Source:    sourceType,
						Value:     model.Pricing,
						UpdatedAt: time.Now(),
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
				// Copy open weights flag
				merged.Metadata.OpenWeights = model.Metadata.OpenWeights
				
				if fm.trackProvenance {
					(*provenance)["metadata"] = Provenance{
						Source:    sourceType,
						Value:     model.Metadata,
						UpdatedAt: time.Now(),
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
