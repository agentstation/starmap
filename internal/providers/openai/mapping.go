package openai

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	fieldID           = "id"
	fieldOwnedBy      = "owned_by"
	fieldMetadataTags = "metadata.tags"
)

func (c *Client) applyFieldMappings(model *catalogs.Model, apiModel Model) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil || provider.Catalog == nil || provider.Catalog.Endpoint.FieldMappings == nil {
		return
	}

	// Apply field mappings using direct path matching
	for _, mapping := range provider.Catalog.Endpoint.FieldMappings {
		c.setFieldByPath(model, mapping.From, mapping.To, apiModel)
	}
}

// setFieldByPath directly sets model fields based on path strings with type-safe conversion.
func (c *Client) setFieldByPath(model *catalogs.Model, fromPath, toPath string, apiModel Model) {
	sourceValue, ok := fieldMappingSourceValue(fromPath, apiModel)
	if !ok || isNilFieldMappingValue(sourceValue) {
		return
	}
	c.applyMappedField(model, toPath, sourceValue)
}

func isNilFieldMappingValue(value any) bool {
	if value == nil {
		return true
	}
	valueReflect := reflect.ValueOf(value)
	switch valueReflect.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return valueReflect.IsNil()
	default:
		return false
	}
}

func fieldMappingSourceValue(fromPath string, apiModel Model) (any, bool) {
	switch fromPath {
	case "max_model_len":
		return apiModel.MaxModelLen, true
	case "context_window":
		return apiModel.ContextWindow, true
	case "context_length":
		return apiModel.ContextLength, true
	case "max_completion_tokens":
		return apiModel.MaxCompletionTokens, true
	case "max_output_length":
		return apiModel.MaxOutputLength, true
	case "input_token_limit":
		return apiModel.InputTokenLimit, true
	case "output_token_limit":
		return apiModel.OutputTokenLimit, true
	case "name":
		return apiModel.Name, true
	case "metadata.description":
		if apiModel.Metadata != nil {
			return apiModel.Metadata.Description, true
		}
	case "metadata.context_length":
		if apiModel.Metadata != nil {
			return apiModel.Metadata.ContextLength, true
		}
	case "metadata.max_tokens":
		if apiModel.Metadata != nil {
			return apiModel.Metadata.MaxTokens, true
		}
	case fieldMetadataTags:
		if apiModel.Metadata != nil {
			return apiModel.Metadata.Tags, true
		}
	case fieldID:
		return apiModel.ID, true
	case fieldOwnedBy:
		return apiModel.OwnedBy, true
	case "created":
		return apiModel.Created, true
	}
	return nil, false
}

func (c *Client) applyMappedField(model *catalogs.Model, toPath string, sourceValue any) {
	switch toPath {
	// Limits fields
	case "limits.context_window":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.Set(catalogs.ModelLimitContextWindow, c.toInt64(sourceValue))
	case "limits.input_tokens":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.Set(catalogs.ModelLimitInputTokens, c.toInt64(sourceValue))
	case "limits.output_tokens":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.Set(catalogs.ModelLimitOutputTokens, c.toInt64(sourceValue))

	// Direct model fields for backward compatibility
	case "context_window":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.Set(catalogs.ModelLimitContextWindow, c.toInt64(sourceValue))
	case "max_completion_tokens":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.Set(catalogs.ModelLimitOutputTokens, c.toInt64(sourceValue))

	// Core model fields
	case "name":
		model.Name = c.toString(sourceValue)
	case "description":
		model.SetDescription(c.toString(sourceValue))

	case fieldMetadataTags:
		if model.Metadata == nil {
			model.Metadata = &catalogs.ModelMetadata{}
		}
		model.Metadata.Tags = c.toModelTags(sourceValue)

	// Future extensibility - add more paths as needed:
	// case "pricing.input.base":
	//     if model.Pricing == nil { model.Pricing = &catalogs.ModelPricing{} }
	//     if model.Pricing.Input == nil { model.Pricing.Input = &catalogs.ModelTokenPricing{} }
	//     model.Pricing.Input.Base = c.toFloat64(sourceValue)

	default:
		// Unknown destination path - skip silently
		return
	}
}

// toInt64 converts various types to int64 with nil-safe handling.
func (c *Client) toInt64(v any) int64 {
	switch val := v.(type) {
	case *int64:
		if val != nil {
			return *val
		}
	case int64:
		return val
	case *int:
		if val != nil {
			return int64(*val)
		}
	case int:
		return int64(val)
	case *int32:
		if val != nil {
			return int64(*val)
		}
	case int32:
		return int64(val)
	case *float64:
		if val != nil {
			return int64(*val)
		}
	case float64:
		return int64(val)
	case string:
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			return parsed
		}
	}
	return 0
}

// toString converts various types to string with nil-safe handling.
func (c *Client) toString(v any) string {
	switch val := v.(type) {
	case *string:
		if val != nil {
			return *val
		}
	case string:
		return val
	case *int64:
		if val != nil {
			return strconv.FormatInt(*val, 10)
		}
	case int64:
		return strconv.FormatInt(val, 10)
	case int:
		return strconv.Itoa(val)
	}
	return ""
}

// toModelTags converts provider tag strings to catalog model tags.
func (c *Client) toModelTags(v any) []catalogs.ModelTag {
	switch val := v.(type) {
	case []string:
		tags := make([]catalogs.ModelTag, 0, len(val))
		for _, tag := range val {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			tags = append(tags, catalogs.ModelTag(tag))
		}
		return tags
	case []catalogs.ModelTag:
		return append([]catalogs.ModelTag(nil), val...)
	}
	return nil
}

// extractAuthors extracts authors using configured author mappings.
func (c *Client) extractAuthors(modelID, ownedBy string) []catalogs.Author {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil || provider.Catalog == nil || provider.Catalog.Endpoint.AuthorMapping == nil {
		return nil
	}
	mapping := provider.Catalog.Endpoint.AuthorMapping
	var fieldValue string
	switch mapping.Field {
	case fieldOwnedBy:
		fieldValue = ownedBy
	case fieldID:
		fieldValue = modelID
	default:
		return nil
	}
	if authorID, found := mapping.Resolve(fieldValue); found {
		return []catalogs.Author{{ID: authorID, Name: authorID.String()}}
	}
	return nil
}

// validateFieldMappings validates that all configured field mappings use valid paths.
func (c *Client) validateFieldMappings(provider *catalogs.Provider) error {
	if provider == nil || provider.Catalog == nil || provider.Catalog.Endpoint.FieldMappings == nil {
		return nil
	}

	for _, mapping := range provider.Catalog.Endpoint.FieldMappings {
		if !c.isValidSourceField(mapping.From) {
			return &errors.ValidationError{
				Field: "field_mappings.from", Value: mapping.From,
				Message: "invalid source field: " + mapping.From,
			}
		}
		if !c.isValidDestinationPath(mapping.To) {
			return &errors.ValidationError{
				Field: "field_mappings.to", Value: mapping.To,
				Message: "invalid destination path: " + mapping.To,
			}
		}
	}
	return nil
}

// isValidSourceField checks if a source field exists in the API model.
func (c *Client) isValidSourceField(field string) bool {
	validFields := map[string]bool{
		"max_model_len":           true,
		"context_window":          true,
		"context_length":          true,
		"max_completion_tokens":   true,
		"max_output_length":       true,
		"input_token_limit":       true,
		"output_token_limit":      true,
		"name":                    true,
		"metadata.description":    true,
		"metadata.context_length": true,
		"metadata.max_tokens":     true,
		fieldMetadataTags:         true,
		fieldID:                   true,
		fieldOwnedBy:              true,
		"created":                 true,
	}
	return validFields[field]
}

// isValidDestinationPath checks if a destination path is valid in the Model struct.
func (c *Client) isValidDestinationPath(path string) bool {
	validPaths := map[string]bool{
		// Limits fields
		"limits.context_window": true,
		"limits.input_tokens":   true,
		"limits.output_tokens":  true,

		// Direct model fields for backward compatibility
		"context_window":        true,
		"max_completion_tokens": true,

		// Core model fields
		"name":            true,
		"description":     true,
		fieldMetadataTags: true,

		// Future paths can be added here as needed:
		// "metadata.release_date":     true,
		// "metadata.open_weights":     true,
		// "features.tools":            true,
		// "features.reasoning":        true,
		// "pricing.input.base":        true,
		// "pricing.output.base":       true,
	}
	return validPaths[path]
}
