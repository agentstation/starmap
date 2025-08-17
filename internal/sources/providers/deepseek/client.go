package deepseek

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/internal/sources/base"
	"github.com/agentstation/starmap/internal/sources/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func init() {
	// Register this provider client in the registry
	registry.RegisterClient(catalogs.ProviderIDDeepSeek, &Client{})
}

// Client implements the catalogs.Client interface for DeepSeek.
type Client struct {
	*base.OpenAIClient
}

// NewClient creates a new DeepSeek client (kept for backward compatibility).
func NewClient(apiKey string, provider *catalogs.Provider) *Client {
	provider.APIKeyValue = apiKey // Set the API key in the provider
	return &Client{
		OpenAIClient: base.NewOpenAIClient(provider, "https://api.deepseek.com"),
	}
}

// Configure sets the provider for this client (used by registry pattern).
func (c *Client) Configure(provider *catalogs.Provider) {
	c.OpenAIClient = base.NewOpenAIClient(provider, "https://api.deepseek.com")
}

// ListModels uses the base OpenAI implementation.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	return c.OpenAIClient.ListModels(ctx)
}

// GetModel uses the base OpenAI implementation.
func (c *Client) GetModel(ctx context.Context, modelID string) (*catalogs.Model, error) {
	return c.OpenAIClient.GetModel(ctx, modelID)
}

// ConvertToModel overrides the base implementation for DeepSeek-specific logic.
func (c *Client) ConvertToModel(m base.OpenAIModelData) catalogs.Model {
	model := c.OpenAIClient.ConvertToModel(m)

	// DeepSeek-specific customizations
	model.Name = c.inferDisplayName(m.ID)

	// Override author inference with DeepSeek-specific logic
	if m.OwnedBy != "" {
		authorID := catalogs.AuthorID(m.OwnedBy)
		// Map to DeepSeek author
		if m.OwnedBy == "deepseek" || strings.HasPrefix(m.OwnedBy, "deepseek") {
			authorID = catalogs.AuthorIDDeepSeek
		}
		model.Authors = []catalogs.Author{
			{ID: authorID, Name: m.OwnedBy},
		}
	} else {
		// Default to DeepSeek as the author
		model.Authors = []catalogs.Author{
			{ID: catalogs.AuthorIDDeepSeek, Name: "DeepSeek"},
		}
	}

	// Override features with DeepSeek-specific inference
	model.Features = c.inferFeatures(m.ID)

	return model
}

// inferDisplayName creates a display name from the model ID.
func (c *Client) inferDisplayName(modelID string) string {
	// Convert common DeepSeek model IDs to display names
	switch {
	case strings.Contains(modelID, "deepseek-chat"):
		return "DeepSeek Chat"
	case strings.Contains(modelID, "deepseek-coder"):
		return "DeepSeek Coder"
	case strings.Contains(modelID, "deepseek-reasoner"):
		return "DeepSeek Reasoner"
	case strings.Contains(modelID, "deepseek-r1"):
		return "DeepSeek R1"
	case strings.Contains(modelID, "deepseek-v3"):
		return "DeepSeek V3"
	default:
		// Capitalize first letter and replace dashes with spaces
		name := strings.ReplaceAll(modelID, "-", " ")
		if len(name) > 0 {
			name = strings.ToUpper(name[:1]) + name[1:]
		}
		return name
	}
}

// inferFeatures infers model features based on the model ID.
func (c *Client) inferFeatures(modelID string) *catalogs.ModelFeatures {
	features := &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
		Temperature:      true,
		TopP:             true,
		MaxTokens:        true,
		FrequencyPenalty: true,
		PresencePenalty:  true,
		Stop:             true,
		Streaming:        true,
		Tools:            true,
		ToolChoice:       true,
		FormatResponse:   true,
	}

	// Set reasoning capabilities for specific models
	if strings.Contains(modelID, "reasoner") || strings.Contains(modelID, "r1") {
		features.Reasoning = true
		features.ReasoningTokens = true
	}

	// Set coding capabilities for coder models
	if strings.Contains(modelID, "coder") {
		// Coder models typically have enhanced code generation capabilities
		// This will be further enhanced by models.dev data
	}

	return features
}
