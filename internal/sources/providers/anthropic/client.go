// Package anthropic provides a client for the Anthropic API.
package anthropic

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Response structures for Anthropic API.
type modelsResponse struct {
	Data []modelResponse `json:"data"`
}

type modelResponse struct {
	Type        string    `json:"type"`
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// Client implements the catalogs.Client interface for Anthropic.
type Client struct {
	provider  *catalogs.Provider
	transport *transport.Client
	mu        sync.RWMutex
}

// NewClient creates a new Anthropic client (kept for backward compatibility).
func NewClient(provider *catalogs.Provider) *Client {
	return &Client{
		provider:  provider,
		transport: transport.NewForProvider(provider),
	}
}

// IsAPIKeyRequired returns true if the client requires an API key.
func (c *Client) IsAPIKeyRequired() bool {
	return c.provider.IsAPIKeyRequired()
}

// HasAPIKey returns true if the client has an API key.
func (c *Client) HasAPIKey() bool {
	return c.provider.HasAPIKey()
}

// Configure sets the provider for this client (used by registry pattern).
func (c *Client) Configure(provider *catalogs.Provider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.provider = provider
	c.transport = transport.NewForProvider(provider)
}

// ListModels retrieves all available models from Anthropic.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil {
		return nil, &errors.ConfigError{
			Component: "anthropic",
			Message:   "provider not configured",
		}
	}

	// Build URL - use provider's URL if available, otherwise use default
	url := "https://api.anthropic.com/v1/models"
	if rb := transport.NewRequestBuilder(provider); rb.GetBaseURL() != "" {
		url = rb.GetBaseURL()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.WrapResource("create", "request", url, err)
	}

	// Add Anthropic-specific headers
	req.Header.Set("anthropic-version", "2023-06-01")

	// Use transport layer for HTTP request with authentication
	resp, err := c.transport.Do(req, provider)
	if err != nil {
		return nil, &errors.APIError{
			Provider: "anthropic",
			Endpoint: url,
			Message:  "request failed",
			Err:      err,
		}
	}
	defer func() { _ = resp.Body.Close() }()

	// Decode response using transport utility
	var result modelsResponse
	if err := transport.DecodeResponse(resp, &result); err != nil {
		return nil, errors.WrapParse("json", "anthropic response", err)
	}

	// Convert Anthropic models to starmap models
	models := make([]catalogs.Model, 0, len(result.Data))
	for _, m := range result.Data {
		model := c.convertToModel(m)
		models = append(models, model)
	}

	return models, nil
}

// convertToModel converts an Anthropic model response to a starmap Model.
func (c *Client) convertToModel(m modelResponse) catalogs.Model {
	model := catalogs.Model{
		ID:   m.ID,
		Name: m.DisplayName,
	}

	// Set created time
	if !m.CreatedAt.IsZero() {
		model.CreatedAt = utc.Time{Time: m.CreatedAt}
		model.UpdatedAt = model.CreatedAt
	}

	// Set Anthropic as the author
	model.Authors = []catalogs.Author{
		{ID: catalogs.AuthorIDAnthropic, Name: "Anthropic"},
	}

	// Set basic features based on model ID patterns
	// Note: Detailed limits and pricing will be enhanced by models.dev integration
	model.Features = c.inferFeatures(m.ID)

	// Don't set limits - let models.dev provide accurate data
	// Anthropic API doesn't return token limits, so we rely on models.dev

	return model
}

// inferFeatures infers model features based on the model ID.
func (c *Client) inferFeatures(modelID string) *catalogs.ModelFeatures {
	features := &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
		Temperature:    true,
		TopP:           true,
		TopK:           true,
		MaxTokens:      true,
		Stop:           true,
		Streaming:      true,
		Tools:          true,
		ToolChoice:     true,
		FormatResponse: true,
	}

	// Check for specific Claude model capabilities
	switch {
	case contains(modelID, "claude-3"):
		features.Modalities.Input = []catalogs.ModelModality{
			catalogs.ModelModalityText,
			catalogs.ModelModalityImage,
		}
		features.StructuredOutputs = true
		features.WebSearch = false
	case contains(modelID, "claude-opus-4"):
		features.Modalities.Input = []catalogs.ModelModality{
			catalogs.ModelModalityText,
			catalogs.ModelModalityImage,
		}
		features.StructuredOutputs = true
		features.Reasoning = true
		features.IncludeReasoning = true
	case contains(modelID, "claude-2"):
		features.StructuredOutputs = false
		features.WebSearch = false
	}

	return features
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
