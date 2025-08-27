package baseclient

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// OpenAIResponse represents the standard OpenAI API response structure.
type OpenAIResponse struct {
	Object string            `json:"object"`
	Data   []OpenAIModelData `json:"data"`
}

// OpenAIModelData represents a model in the OpenAI API response.
type OpenAIModelData struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// OpenAIClient provides a base implementation for OpenAI-compatible APIs.
type OpenAIClient struct {
	transport *transport.Client
	provider  *catalogs.Provider
	baseURL   string
	mu        sync.RWMutex
}

// NewOpenAIClient creates a new OpenAI-compatible client.
func NewOpenAIClient(provider *catalogs.Provider, baseURL string) *OpenAIClient {
	return &OpenAIClient{
		transport: transport.NewForProvider(provider),
		provider:  provider,
		baseURL:   baseURL,
	}
}

// IsAPIKeyRequired returns true if the client requires an API key.
func (c *OpenAIClient) IsAPIKeyRequired() bool {
	return c.provider.IsAPIKeyRequired()
}

// HasAPIKey returns true if the client has an API key.
func (c *OpenAIClient) HasAPIKey() bool {
	return c.provider.HasAPIKey()
}

// Configure sets the provider for this client.
func (c *OpenAIClient) Configure(provider *catalogs.Provider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.provider = provider
	c.transport = transport.NewForProvider(provider)
}

// GetProvider returns the current provider (thread-safe).
func (c *OpenAIClient) GetProvider() *catalogs.Provider {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider
}

// GetTransport returns the current transport client (thread-safe).
func (c *OpenAIClient) GetTransport() *transport.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.transport
}

// ListModels retrieves all available models using OpenAI-compatible API.
func (c *OpenAIClient) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil {
		return nil, fmt.Errorf("provider not configured")
	}

	// Build URL - use provider's URL if available, otherwise use base URL
	url := c.baseURL + "/v1/models"
	if rb := transport.NewRequestBuilder(provider); rb.GetBaseURL() != "" {
		url = rb.GetBaseURL()
	}

	// Make the request
	resp, err := c.transport.Get(ctx, url, provider)
	if err != nil {
		return nil, fmt.Errorf("openai-compatible: request failed: %w", err)
	}

	// Decode response
	var result OpenAIResponse
	if err := transport.DecodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("openai-compatible: %w", err)
	}

	// Convert to starmap models
	models := make([]catalogs.Model, 0, len(result.Data))
	for _, m := range result.Data {
		model := c.ConvertToModel(m)
		models = append(models, model)
	}

	return models, nil
}

// ConvertToModel converts an OpenAI model response to a starmap Model.
// This method can be overridden by specific providers for customization.
func (c *OpenAIClient) ConvertToModel(m OpenAIModelData) catalogs.Model {
	model := catalogs.Model{
		ID:   m.ID,
		Name: m.ID, // Default to ID, providers can override for display names
	}

	// Set created time
	if m.Created > 0 {
		// TODO: Import UTC time package when available
		// model.CreatedAt = utc.Time{Time: time.Unix(m.Created, 0)}
		// model.UpdatedAt = model.CreatedAt
	}

	// Map owner to author
	if m.OwnedBy != "" {
		authorID := c.normalizeAuthorID(m.OwnedBy)
		authorName := c.normalizeAuthorName(authorID, m.OwnedBy)
		model.Authors = []catalogs.Author{
			{ID: authorID, Name: authorName},
		}
	}

	// Set basic features - providers can override for specific capabilities
	model.Features = c.InferFeatures(m.ID)

	return model
}

// InferFeatures infers model features based on the model ID.
// This method can be overridden by specific providers.
func (c *OpenAIClient) InferFeatures(modelID string) *catalogs.ModelFeatures {
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
	}

	// Basic pattern matching for common model types
	modelLower := strings.ToLower(modelID)
	switch {
	case strings.Contains(modelLower, "gpt-4"), strings.Contains(modelLower, "gpt-3.5-turbo"):
		features.Tools = true
		features.ToolChoice = true
		features.Logprobs = true
		features.StructuredOutputs = true
		features.FormatResponse = true
	case strings.Contains(modelLower, "embedding"):
		// Embedding models output vectors, not text
		features.Modalities.Output = []catalogs.ModelModality{}
	}

	return features
}

// normalizeAuthorID normalizes the author ID to match our catalog.
func (c *OpenAIClient) normalizeAuthorID(ownedBy string) catalogs.AuthorID {
	switch strings.ToLower(ownedBy) {
	case "openai", "openai-internal", "system":
		// OpenAI API uses various owner values for their models:
		// - "openai": for some models
		// - "openai-internal": for older/internal models
		// - "system": for most current models
		// All should be attributed to OpenAI
		return catalogs.AuthorIDOpenAI
	case "meta":
		return catalogs.AuthorIDMeta
	case "google":
		return catalogs.AuthorIDGoogle
	case "mistralai", "mistral ai", "mistral":
		return catalogs.AuthorIDMistralAI
	case "microsoft":
		return catalogs.AuthorIDMicrosoft
	case "deepseek":
		return catalogs.AuthorIDDeepSeek
	default:
		return catalogs.AuthorID(strings.ToLower(ownedBy))
	}
}

// normalizeAuthorName normalizes the author name based on the normalized ID.
// For OpenAI API responses, we want to use canonical names instead of raw API values.
func (c *OpenAIClient) normalizeAuthorName(authorID catalogs.AuthorID, rawOwnedBy string) string {
	switch authorID {
	case catalogs.AuthorIDOpenAI:
		return "OpenAI"
	case catalogs.AuthorIDMeta:
		return "Meta"
	case catalogs.AuthorIDGoogle:
		return "Google"
	case catalogs.AuthorIDMistralAI:
		return "Mistral AI"
	case catalogs.AuthorIDMicrosoft:
		return "Microsoft"
	case catalogs.AuthorIDDeepSeek:
		return "DeepSeek"
	default:
		// For unknown authors, still return the raw value
		return rawOwnedBy
	}
}
