package baseclient

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// GoogleResponse represents the standard Google API response structure.
type GoogleResponse struct {
	Models []GoogleModelData `json:"models"`
}

// GoogleModelData represents a model in the Google API response.
type GoogleModelData struct {
	Name                       string   `json:"name"`
	BaseModelID                string   `json:"baseModelId,omitempty"`
	Version                    string   `json:"version,omitempty"`
	DisplayName                string   `json:"displayName,omitempty"`
	Description                string   `json:"description,omitempty"`
	InputTokenLimit            int64    `json:"inputTokenLimit,omitempty"`
	OutputTokenLimit           int64    `json:"outputTokenLimit,omitempty"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods,omitempty"`
	Temperature                float64  `json:"temperature,omitempty"`
	TopP                       float64  `json:"topP,omitempty"`
	TopK                       int      `json:"topK,omitempty"`
}

// GoogleClient provides a base implementation for Google APIs.
type GoogleClient struct {
	transport *transport.Client
	provider  *catalogs.Provider
	baseURL   string
	mu        sync.RWMutex
}

// NewGoogleClient creates a new Google-compatible client.
func NewGoogleClient(provider *catalogs.Provider, baseURL string) *GoogleClient {
	return &GoogleClient{
		transport: transport.NewForProvider(provider),
		provider:  provider,
		baseURL:   baseURL,
	}
}

// IsAPIKeyRequired returns true if the client requires an API key.
func (c *GoogleClient) IsAPIKeyRequired() bool {
	return c.provider.IsAPIKeyRequired()
}

// HasAPIKey returns true if the client has an API key.
func (c *GoogleClient) HasAPIKey() bool {
	return c.provider.HasAPIKey()
}

// Configure sets the provider for this client.
func (c *GoogleClient) Configure(provider *catalogs.Provider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.provider = provider
	c.transport = transport.NewForProvider(provider)
}

// ListModels retrieves all available models using Google-compatible API.
func (c *GoogleClient) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil {
		return nil, fmt.Errorf("provider not configured")
	}

	// Build URL - use provider's URL if available, otherwise use base URL
	url := c.baseURL
	if rb := transport.NewRequestBuilder(provider); rb.GetBaseURL() != "" {
		url = rb.GetBaseURL()
	}

	// Make the request
	resp, err := c.transport.Get(ctx, url, provider)
	if err != nil {
		return nil, fmt.Errorf("google-compatible: request failed: %w", err)
	}

	// Decode response
	var result GoogleResponse
	if err := transport.DecodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("google-compatible: %w", err)
	}

	// Convert to starmap models
	models := make([]catalogs.Model, 0, len(result.Models))
	for _, m := range result.Models {
		model := c.ConvertToModel(m)
		models = append(models, model)
	}

	return models, nil
}

// ConvertToModel converts a Google model response to a starmap Model.
// This method can be overridden by specific providers for customization.
func (c *GoogleClient) ConvertToModel(m GoogleModelData) catalogs.Model {
	// Extract model ID from name (format may vary by provider)
	modelID := c.ExtractModelID(m.Name)

	model := catalogs.Model{
		ID:          modelID,
		Name:        m.DisplayName,
		Description: m.Description,
	}

	// Set Google as the author
	model.Authors = []catalogs.Author{
		{ID: catalogs.AuthorIDGoogle, Name: "Google"},
	}

	// Set limits if available
	if m.InputTokenLimit > 0 || m.OutputTokenLimit > 0 {
		model.Limits = &catalogs.ModelLimits{
			ContextWindow: m.InputTokenLimit,
			OutputTokens:  m.OutputTokenLimit,
		}
	}

	// Set generation parameters if provided
	if m.Temperature > 0 || m.TopP > 0 || m.TopK > 0 {
		model.Generation = &catalogs.ModelGeneration{}

		if m.Temperature > 0 {
			model.Generation.Temperature = &catalogs.FloatRange{
				Min:     0.0,
				Max:     2.0,
				Default: m.Temperature,
			}
		}

		if m.TopP > 0 {
			model.Generation.TopP = &catalogs.FloatRange{
				Min:     0.0,
				Max:     1.0,
				Default: m.TopP,
			}
		}

		if m.TopK > 0 {
			model.Generation.TopK = &catalogs.IntRange{
				Min:     1,
				Max:     100,
				Default: m.TopK,
			}
		}
	}

	// Set features based on supported methods and model ID
	model.Features = c.InferFeatures(modelID, m.SupportedGenerationMethods)

	return model
}

// ExtractModelID extracts the model ID from the full name.
// This method can be overridden by specific providers.
func (c *GoogleClient) ExtractModelID(name string) string {
	// Default implementation - extract everything after the last '/'
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

// InferFeatures infers model features based on the model ID and supported methods.
// This method can be overridden by specific providers.
func (c *GoogleClient) InferFeatures(modelID string, supportedMethods []string) *catalogs.ModelFeatures {
	features := &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
		Temperature: true,
		TopP:        true,
		TopK:        true,
		MaxTokens:   true,
		Stop:        true,
		Streaming:   true,
	}

	// Check for specific model capabilities
	modelLower := strings.ToLower(modelID)
	switch {
	case strings.Contains(modelLower, "gemini"):
		features.Tools = true
		features.ToolChoice = true
		features.StructuredOutputs = true
		features.FormatResponse = true

		// Gemini Pro Vision and newer models support images
		if strings.Contains(modelLower, "vision") ||
			strings.Contains(modelLower, "gemini-1.5") ||
			strings.Contains(modelLower, "gemini-2") {
			features.Modalities.Input = []catalogs.ModelModality{
				catalogs.ModelModalityText,
				catalogs.ModelModalityImage,
			}
		}
	case strings.Contains(modelLower, "palm"):
		features.Tools = false
		features.StructuredOutputs = false
	case strings.Contains(modelLower, "bison"):
		features.Tools = false
		features.StructuredOutputs = false
	}

	// Check supported generation methods
	for _, method := range supportedMethods {
		switch strings.ToLower(method) {
		case "generatecontent":
			// Standard text generation
		case "streamgeneratecontent":
			features.Streaming = true
		case "counttokens":
			// Token counting capability
		case "embedcontent":
			// Embedding capability
			features.Modalities.Output = []catalogs.ModelModality{} // Embeddings don't have standard text output
		}
	}

	return features
}
