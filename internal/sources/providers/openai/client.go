// Package openai provides a unified, dynamic client for OpenAI-compatible APIs.
// This package replaces the separate openaicompat package and provides configuration-driven
// behavior based on provider YAML configuration.
package openai

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Response represents the OpenAI API list models response.
type Response struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// Model represents a model in the OpenAI API response.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
	Created int64  `json:"created"`
	// Dynamic fields from provider-specific responses
	MaxModelLen         *int64 `json:"max_model_len,omitempty"`
	ContextWindow       *int64 `json:"context_window,omitempty"`
	MaxCompletionTokens *int64 `json:"max_completion_tokens,omitempty"`
	InputTokenLimit     *int64 `json:"input_token_limit,omitempty"`
	OutputTokenLimit    *int64 `json:"output_token_limit,omitempty"`
	// Provider-specific fields
	Active     *bool `json:"active,omitempty"`      // Groq-specific
	PublicApps any   `json:"public_apps,omitempty"` // Groq-specific
}

// Client implements the catalogs.Client interface with dynamic configuration.
type Client struct {
	transport *transport.Client
	provider  *catalogs.Provider
	mu        sync.RWMutex
}

// NewClient creates a new dynamic OpenAI-compatible client.
func NewClient(provider *catalogs.Provider) *Client {
	client := &Client{
		transport: transport.New(provider),
		provider:  provider,
	}

	// Validate field mappings at startup
	if err := client.validateFieldMappings(); err != nil {
		// Log validation errors but don't fail - allow graceful degradation
		// In a production system, you might want to log these errors
		_ = err
	}

	return client
}

// Configure sets the provider for this client.
func (c *Client) Configure(provider *catalogs.Provider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.provider = provider
	c.transport = transport.New(provider)
}

// IsAPIKeyRequired returns true if the client requires an API key.
func (c *Client) IsAPIKeyRequired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider.IsAPIKeyRequired()
}

// HasAPIKey returns true if the client has an API key.
func (c *Client) HasAPIKey() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider.HasAPIKey()
}

// ListModels retrieves all available models using OpenAI-compatible API.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil {
		return nil, &errors.ValidationError{
			Field:   "provider",
			Message: "provider not configured",
		}
	}

	// Build URL from provider configuration
	url := provider.Catalog.Endpoint.URL
	if url == "" {
		return nil, &errors.ValidationError{
			Field:   "catalog.endpoint.url",
			Message: "endpoint URL not configured",
		}
	}

	// Make the request
	resp, err := c.transport.Get(ctx, url, provider)
	if err != nil {
		return nil, &errors.APIError{
			Provider:   provider.ID.String(),
			StatusCode: 0,
			Message:    "request failed",
			Err:        err,
		}
	}

	// Decode response
	var result Response
	if err := transport.DecodeResponse(resp, &result); err != nil {
		return nil, &errors.APIError{
			Provider:   provider.ID.String(),
			StatusCode: resp.StatusCode,
			Message:    "failed to decode response",
			Err:        err,
		}
	}

	// Convert to starmap models
	models := make([]catalogs.Model, 0, len(result.Data))
	for _, m := range result.Data {
		model := c.ConvertToModel(m)
		models = append(models, *model)
	}

	return models, nil
}

// ConvertToModel converts an OpenAI model response to a starmap Model using dynamic configuration.
// This method is public for testing purposes.
func (c *Client) ConvertToModel(m Model) *catalogs.Model {
	model := &catalogs.Model{
		ID:          m.ID,
		Name:        m.ID, // Default to ID, may be overridden
		Description: "",
	}

	// Apply dynamic field mappings
	c.applyFieldMappings(model, m)

	// Apply dynamic author extraction
	model.Authors = c.extractAuthors(m.ID, m.OwnedBy)

	// Apply dynamic feature rules
	model.Features = c.applyFeatureRules(m.ID)

	return model
}

// applyFieldMappings applies configured field mappings using direct path matching.
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
	// Get source value from API model
	var sourceValue any
	switch fromPath {
	case "max_model_len":
		sourceValue = apiModel.MaxModelLen
	case "context_window":
		sourceValue = apiModel.ContextWindow
	case "max_completion_tokens":
		sourceValue = apiModel.MaxCompletionTokens
	case "input_token_limit":
		sourceValue = apiModel.InputTokenLimit
	case "output_token_limit":
		sourceValue = apiModel.OutputTokenLimit
	case "id":
		sourceValue = apiModel.ID
	case "owned_by":
		sourceValue = apiModel.OwnedBy
	case "created":
		sourceValue = apiModel.Created
	default:
		// Unknown source field - skip silently
		return
	}

	// Skip if source value is nil pointer
	if sourceValue == nil {
		return
	}

	// Set destination field with automatic type conversion and struct initialization
	switch toPath {
	// Limits fields
	case "limits.context_window":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.ContextWindow = c.toInt64(sourceValue)
	case "limits.output_tokens":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.OutputTokens = c.toInt64(sourceValue)

	// Direct model fields for backward compatibility
	case "context_window":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.ContextWindow = c.toInt64(sourceValue)
	case "max_completion_tokens":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.OutputTokens = c.toInt64(sourceValue)

	// Core model fields
	case "name":
		model.Name = c.toString(sourceValue)
	case "description":
		model.Description = c.toString(sourceValue)

	// Future extensibility - add more paths as needed:
	// case "metadata.release_date":
	//     if model.Metadata == nil { model.Metadata = &catalogs.ModelMetadata{} }
	//     model.Metadata.ReleaseDate = c.toTime(sourceValue)
	// case "features.tools":
	//     if model.Features == nil { model.Features = &catalogs.ModelFeatures{} }
	//     model.Features.Tools = c.toBool(sourceValue)
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

// extractAuthors extracts authors using configured author mappings.
func (c *Client) extractAuthors(modelID, ownedBy string) []catalogs.Author {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil {
		return []catalogs.Author{{ID: catalogs.AuthorIDOpenAI, Name: "OpenAI"}}
	}

	// Use configured author mapping if available
	if provider.Catalog != nil && provider.Catalog.Endpoint.AuthorMapping != nil {
		mapping := provider.Catalog.Endpoint.AuthorMapping

		// Get the field value to map from
		var fieldValue string
		switch mapping.Field {
		case "owned_by":
			fieldValue = ownedBy
		case "id":
			fieldValue = modelID
		default:
			fieldValue = ownedBy // default to owned_by
		}

		// Apply normalization if configured
		if normalizedID, exists := mapping.Normalized[fieldValue]; exists {
			fieldValue = string(normalizedID)
		}

		// Convert to AuthorID and return
		if authorID := catalogs.ParseAuthorID(fieldValue); authorID != catalogs.AuthorIDUnknown {
			return []catalogs.Author{{ID: authorID, Name: authorID.String()}}
		}
	}

	// Fallback to provider's configured authors or infer from owned_by
	if len(provider.Catalog.Authors) > 0 {
		authors := make([]catalogs.Author, len(provider.Catalog.Authors))
		for i, authorID := range provider.Catalog.Authors {
			authors[i] = catalogs.Author{ID: authorID, Name: authorID.String()}
		}
		return authors
	}

	// Final fallback - infer from owned_by
	if authorID := catalogs.ParseAuthorID(ownedBy); authorID != catalogs.AuthorIDUnknown {
		return []catalogs.Author{{ID: authorID, Name: authorID.String()}}
	}

	return []catalogs.Author{{ID: catalogs.AuthorIDOpenAI, Name: "OpenAI"}}
}

// applyFeatureRules applies configured feature rules to infer model features.
func (c *Client) applyFeatureRules(modelID string) *catalogs.ModelFeatures {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	// Start with base OpenAI features
	features := &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
		Temperature: true,
		TopP:        true,
		MaxTokens:   true,
		Stop:        true,
		Streaming:   true,
	}

	if provider == nil || provider.Catalog == nil || provider.Catalog.Endpoint.FeatureRules == nil {
		return features
	}

	// Apply configured feature rules
	for _, rule := range provider.Catalog.Endpoint.FeatureRules {
		c.applyFeatureRule(features, modelID, rule)
	}

	return features
}

// applyFeatureRule applies a single feature rule to the model features.
func (c *Client) applyFeatureRule(features *catalogs.ModelFeatures, modelID string, rule catalogs.FeatureRule) {
	// Get field value to check
	var fieldValue string
	switch rule.Field {
	case "id":
		fieldValue = modelID
	default:
		return // Unknown field
	}

	// Check if any of the "contains" values match
	fieldLower := strings.ToLower(fieldValue)
	matches := false
	for _, contains := range rule.Contains {
		if strings.Contains(fieldLower, strings.ToLower(contains)) {
			matches = true
			break
		}
	}

	if !matches {
		return
	}

	// Apply the feature value
	switch rule.Feature {
	case "tools":
		features.Tools = rule.Value
	case "tool_choice":
		features.ToolChoice = rule.Value
	case "structured_outputs":
		features.StructuredOutputs = rule.Value
	case "reasoning":
		features.Reasoning = rule.Value
	case "top_k":
		features.TopK = rule.Value
	case "format_response":
		features.FormatResponse = rule.Value
	}
}

// validateFieldMappings validates that all configured field mappings use valid paths.
func (c *Client) validateFieldMappings() error {
	if c.provider == nil || c.provider.Catalog == nil || c.provider.Catalog.Endpoint.FieldMappings == nil {
		return nil
	}

	for _, mapping := range c.provider.Catalog.Endpoint.FieldMappings {
		if !c.isValidSourceField(mapping.From) {
			return &errors.ValidationError{
				Field:   "field_mappings.from",
				Message: "invalid source field: " + mapping.From,
			}
		}
		if !c.isValidDestinationPath(mapping.To) {
			return &errors.ValidationError{
				Field:   "field_mappings.to",
				Message: "invalid destination path: " + mapping.To,
			}
		}
	}
	return nil
}

// isValidSourceField checks if a source field exists in the API model.
func (c *Client) isValidSourceField(field string) bool {
	validFields := map[string]bool{
		"max_model_len":         true,
		"context_window":        true,
		"max_completion_tokens": true,
		"input_token_limit":     true,
		"output_token_limit":    true,
		"id":                    true,
		"owned_by":              true,
		"created":               true,
	}
	return validFields[field]
}

// isValidDestinationPath checks if a destination path is valid in the Model struct.
func (c *Client) isValidDestinationPath(path string) bool {
	validPaths := map[string]bool{
		// Limits fields
		"limits.context_window": true,
		"limits.output_tokens":  true,

		// Direct model fields for backward compatibility
		"context_window":        true,
		"max_completion_tokens": true,

		// Core model fields
		"name":        true,
		"description": true,

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
