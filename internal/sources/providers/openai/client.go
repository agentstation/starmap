package openai

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/internal/sources/base"
	"github.com/agentstation/starmap/internal/sources/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func init() {
	// Register this provider client in the registry
	registry.RegisterClient(catalogs.ProviderIDOpenAI, &Client{})
}

// Client implements the catalogs.Client interface for OpenAI.
type Client struct {
	*base.OpenAIClient
}

// NewClient creates a new OpenAI client (kept for backward compatibility).
func NewClient(apiKey string, provider *catalogs.Provider) *Client {
	provider.APIKeyValue = apiKey // Set the API key in the provider
	return &Client{
		OpenAIClient: base.NewOpenAIClient(provider, "https://api.openai.com"),
	}
}

// Configure sets the provider for this client (used by registry pattern).
func (c *Client) Configure(provider *catalogs.Provider) {
	c.OpenAIClient = base.NewOpenAIClient(provider, "https://api.openai.com")
}

// ListModels uses the base OpenAI implementation.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	return c.OpenAIClient.ListModels(ctx)
}

// GetModel uses the base OpenAI implementation.
func (c *Client) GetModel(ctx context.Context, modelID string) (*catalogs.Model, error) {
	return c.OpenAIClient.GetModel(ctx, modelID)
}

// ConvertToModel overrides the base implementation for OpenAI-specific logic.
func (c *Client) ConvertToModel(m base.OpenAIModelData) catalogs.Model {
	model := c.OpenAIClient.ConvertToModel(m)

	// OpenAI-specific customizations
	model.Name = m.ID // OpenAI doesn't provide a separate display name

	// Override features with OpenAI-specific inference
	model.Features = c.inferFeatures(m.ID)

	return model
}

// inferFeatures infers OpenAI-specific model features based on the model ID.
func (c *Client) inferFeatures(modelID string) *catalogs.ModelFeatures {
	// Start with base features
	features := c.OpenAIClient.InferFeatures(modelID)

	// Add OpenAI-specific feature detection
	modelLower := strings.ToLower(modelID)
	switch {
	case strings.Contains(modelLower, "o1"), strings.Contains(modelLower, "o3"):
		features.Reasoning = true
		features.IncludeReasoning = true
	case strings.Contains(modelLower, "dall-e"):
		features.Modalities.Input = []catalogs.ModelModality{catalogs.ModelModalityText}
		features.Modalities.Output = []catalogs.ModelModality{catalogs.ModelModalityImage}
	case strings.Contains(modelLower, "whisper"):
		features.Modalities.Input = []catalogs.ModelModality{catalogs.ModelModalityAudio}
		features.Modalities.Output = []catalogs.ModelModality{catalogs.ModelModalityText}
	case strings.Contains(modelLower, "tts"):
		features.Modalities.Input = []catalogs.ModelModality{catalogs.ModelModalityText}
		features.Modalities.Output = []catalogs.ModelModality{catalogs.ModelModalityAudio}
	}

	return features
}
