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

	// Basic provider fields to merge
	providerFields := []string{
		"name", "headquarters", "icon_url", "status_page_url",
		"requires_moderation",
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

	// Basic model fields to merge
	modelFields := []string{
		"name", "description",
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
