package cerebras

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/internal/sources/base"
	"github.com/agentstation/starmap/internal/sources/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func init() {
	// Register this provider client in the registry
	registry.RegisterClient(catalogs.ProviderIDCerebras, &Client{})
}

// Client implements the catalogs.Client interface for Cerebras.
type Client struct {
	*base.OpenAIClient
}

// NewClient creates a new Cerebras client (kept for backward compatibility).
func NewClient(apiKey string, provider *catalogs.Provider) *Client {
	provider.APIKeyValue = apiKey // Set the API key in the provider
	return &Client{
		OpenAIClient: base.NewOpenAIClient(provider, "https://api.cerebras.ai"),
	}
}

// Configure sets the provider for this client (used by registry pattern).
func (c *Client) Configure(provider *catalogs.Provider) {
	c.OpenAIClient = base.NewOpenAIClient(provider, "https://api.cerebras.ai")
}

// ListModels uses the base OpenAI implementation.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	return c.OpenAIClient.ListModels(ctx)
}

// GetModel uses the base OpenAI implementation.
func (c *Client) GetModel(ctx context.Context, modelID string) (*catalogs.Model, error) {
	return c.OpenAIClient.GetModel(ctx, modelID)
}

// ConvertToModel overrides the base implementation for Cerebras-specific logic.
func (c *Client) ConvertToModel(m base.OpenAIModelData) catalogs.Model {
	model := c.OpenAIClient.ConvertToModel(m)

	// Cerebras-specific customizations
	model.Name = c.inferDisplayName(m.ID)

	// Override author inference with Cerebras-specific logic
	if m.OwnedBy != "" {
		authorID := catalogs.AuthorID(m.OwnedBy)
		// Map common owners to known authors
		switch strings.ToLower(m.OwnedBy) {
		case "cerebras":
			authorID = catalogs.AuthorIDCerebras
		case "meta", "meta-llama":
			authorID = catalogs.AuthorIDMeta
		case "openai":
			authorID = catalogs.AuthorIDOpenAI
		case "deepseek":
			authorID = catalogs.AuthorIDDeepSeek
		case "qwen", "alibaba":
			authorID = catalogs.AuthorIDAlibabaQwen
		}
		model.Authors = []catalogs.Author{
			{ID: authorID, Name: m.OwnedBy},
		}
	} else {
		// Default to inferring author from model ID
		model.Authors = c.inferAuthors(m.ID)
	}

	// Override features with Cerebras-specific inference
	model.Features = c.inferFeatures(m.ID)

	return model
}

// inferDisplayName creates a display name from the model ID.
func (c *Client) inferDisplayName(modelID string) string {
	// Convert common Cerebras-hosted model IDs to display names
	switch {
	case strings.Contains(modelID, "llama-3.1"):
		return "Llama 3.1"
	case strings.Contains(modelID, "llama-3.3"):
		return "Llama 3.3"
	case strings.Contains(modelID, "llama-4-scout"):
		return "Llama 4 Scout"
	case strings.Contains(modelID, "llama-4-maverick"):
		return "Llama 4 Maverick"
	case strings.Contains(modelID, "deepseek-r1"):
		return "DeepSeek R1"
	case strings.Contains(modelID, "qwen-3"):
		return "Qwen 3"
	case strings.Contains(modelID, "gpt-oss"):
		return "GPT OSS"
	default:
		// Capitalize first letter and replace dashes with spaces
		name := strings.ReplaceAll(modelID, "-", " ")
		if len(name) > 0 {
			name = strings.ToUpper(name[:1]) + name[1:]
		}
		return name
	}
}

// inferAuthors infers model authors based on the model ID.
func (c *Client) inferAuthors(modelID string) []catalogs.Author {
	switch {
	case strings.Contains(modelID, "llama"):
		return []catalogs.Author{{ID: catalogs.AuthorIDMeta, Name: "Meta"}}
	case strings.Contains(modelID, "deepseek"):
		return []catalogs.Author{{ID: catalogs.AuthorIDDeepSeek, Name: "DeepSeek"}}
	case strings.Contains(modelID, "qwen"):
		return []catalogs.Author{{ID: catalogs.AuthorIDAlibabaQwen, Name: "Alibaba"}}
	case strings.Contains(modelID, "gpt"):
		return []catalogs.Author{{ID: catalogs.AuthorIDOpenAI, Name: "OpenAI"}}
	default:
		// Default to Cerebras as the author
		return []catalogs.Author{{ID: catalogs.AuthorIDCerebras, Name: "Cerebras"}}
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
	if strings.Contains(modelID, "thinking") || strings.Contains(modelID, "r1") {
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
