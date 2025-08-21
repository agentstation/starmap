package groq

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentstation/starmap/internal/sources/providers/baseclient"
	"github.com/agentstation/starmap/internal/sources/providers/registry"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func init() {
	// Register this provider client in the registry
	registry.RegisterClient(catalogs.ProviderIDGroq, &Client{})
}

// GroqResponse represents the Groq API response structure.
type GroqResponse struct {
	Object string          `json:"object"`
	Data   []GroqModelData `json:"data"`
}

// GroqModelData extends OpenAI model data with Groq-specific fields.
// Note: Groq's API documentation at https://console.groq.com/docs/api-reference#models-retrieve
// is missing the max_completion_tokens field, but it is actually returned by both the
// models list endpoint and individual model endpoints as of 2025.
type GroqModelData struct {
	baseclient.OpenAIModelData
	Active              bool `json:"active"`
	ContextWindow       int  `json:"context_window"`
	MaxCompletionTokens int  `json:"max_completion_tokens"` // Not documented but present in API response
	PublicApps          any  `json:"public_apps"`
}

// Client implements the catalogs.Client interface for Groq.
type Client struct {
	*baseclient.OpenAIClient
}

// NewClient creates a new Groq client (kept for backward compatibility).
func NewClient(apiKey string, provider *catalogs.Provider) *Client {
	provider.APIKeyValue = apiKey // Set the API key in the provider
	return &Client{
		OpenAIClient: baseclient.NewOpenAIClient(provider, "https://api.groq.com/openai"),
	}
}

// Configure sets the provider for this client (used by registry pattern).
func (c *Client) Configure(provider *catalogs.Provider) {
	c.OpenAIClient = baseclient.NewOpenAIClient(provider, "https://api.groq.com/openai")
}

// ListModels implements single-step fetching to get complete Groq model data.
// As of 2025, the Groq API now includes all necessary fields (context_window, max_completion_tokens)
// in the models list response, eliminating the need for individual model fetches.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	provider := c.OpenAIClient.GetProvider()
	if provider == nil {
		return nil, fmt.Errorf("provider not configured")
	}

	// Single API call to get the complete list of models with all fields
	listURL := "https://api.groq.com/openai/v1/models"
	if rb := transport.NewRequestBuilder(provider); rb.GetBaseURL() != "" {
		listURL = rb.GetBaseURL()
	}

	resp, err := c.OpenAIClient.GetTransport().Get(ctx, listURL, provider)
	if err != nil {
		return nil, fmt.Errorf("groq: list request failed: %w", err)
	}

	var listResult GroqResponse
	if err := transport.DecodeResponse(resp, &listResult); err != nil {
		return nil, fmt.Errorf("groq: list decode failed: %w", err)
	}

	// Convert each model directly - no need for additional fetches
	models := make([]catalogs.Model, 0, len(listResult.Data))
	for _, groqModel := range listResult.Data {
		model := c.ConvertToGroqModel(groqModel)
		models = append(models, model)
	}

	return models, nil
}

// GetModel overrides the base implementation to parse Groq-specific fields from individual model endpoint.
func (c *Client) GetModel(ctx context.Context, modelID string) (*catalogs.Model, error) {
	// Get the provider from the base client
	provider := c.OpenAIClient.GetProvider()
	if provider == nil {
		return nil, fmt.Errorf("provider not configured")
	}

	// Build URL for individual model endpoint
	url := "https://api.groq.com/openai/v1/models/" + modelID
	if rb := transport.NewRequestBuilder(provider); rb.GetBaseURL() != "" {
		url = rb.GetBaseURL() + "/" + modelID
	}

	// Make the request using the base client's transport
	resp, err := c.OpenAIClient.GetTransport().Get(ctx, url, provider)
	if err != nil {
		return nil, fmt.Errorf("groq: request failed: %w", err)
	}

	// Decode response with Groq-specific structure
	var result GroqModelData
	if err := transport.DecodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("groq: %w", err)
	}

	// Convert to starmap model
	model := c.ConvertToGroqModel(result)
	return &model, nil
}

// ConvertToGroqModel converts a Groq model response to a starmap Model with Groq-specific fields.
func (c *Client) ConvertToGroqModel(m GroqModelData) catalogs.Model {
	// Start with base conversion
	model := c.OpenAIClient.ConvertToModel(m.OpenAIModelData)

	// Apply Groq-specific customizations
	model.Name = c.formatModelName(m.ID)
	model.Features = c.inferFeatures(m.ID)

	// Set Groq-specific limits from API response
	if model.Limits == nil {
		model.Limits = &catalogs.ModelLimits{}
	}

	if m.ContextWindow > 0 {
		model.Limits.ContextWindow = int64(m.ContextWindow)
	}

	if m.MaxCompletionTokens > 0 {
		model.Limits.OutputTokens = int64(m.MaxCompletionTokens)
	}

	// Set active status if available
	if !m.Active {
		if model.Features == nil {
			model.Features = &catalogs.ModelFeatures{}
		}
		// We could add a deprecation field to ModelFeatures if needed
		// For now, we'll include this information in the model name or description
	}

	return model
}

// ConvertToModel overrides the base implementation for backward compatibility.
func (c *Client) ConvertToModel(m baseclient.OpenAIModelData) catalogs.Model {
	// This method is kept for compatibility but shouldn't be used with the new ListModels
	model := c.OpenAIClient.ConvertToModel(m)
	model.Name = c.formatModelName(m.ID)
	model.Features = c.inferFeatures(m.ID)
	return model
}

// formatModelName creates a display name from the model ID.
func (c *Client) formatModelName(modelID string) string {
	// Remove organization prefix if present
	if idx := strings.Index(modelID, "/"); idx != -1 {
		return modelID[idx+1:]
	}
	return modelID
}

// inferFeatures infers Groq-specific model features based on the model ID.
func (c *Client) inferFeatures(modelID string) *catalogs.ModelFeatures {
	// Start with base features
	features := c.OpenAIClient.InferFeatures(modelID)

	// Add Groq-specific capabilities
	features.Seed = true // Groq supports seed parameter

	modelLower := strings.ToLower(modelID)

	// Groq primarily hosts open models with standard chat capabilities
	// Most models support function calling
	if strings.Contains(modelLower, "llama") || strings.Contains(modelLower, "mixtral") || strings.Contains(modelLower, "gemma") {
		features.Tools = true
		features.ToolChoice = true
		features.StructuredOutputs = true
	}

	// Vision models
	if strings.Contains(modelLower, "vision") || strings.Contains(modelLower, "llava") {
		features.Modalities.Input = []catalogs.ModelModality{
			catalogs.ModelModalityText,
			catalogs.ModelModalityImage,
		}
	}

	// Guard models for safety
	if strings.Contains(modelLower, "guard") {
		features.Tools = false
		features.ToolChoice = false
	}

	// Whisper for speech-to-text
	if strings.Contains(modelLower, "whisper") {
		features.Modalities.Input = []catalogs.ModelModality{catalogs.ModelModalityAudio}
		features.Modalities.Output = []catalogs.ModelModality{catalogs.ModelModalityText}
	}

	return features
}
