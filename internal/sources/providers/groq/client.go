package groq

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/internal/sources/providers/baseclient"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

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
func NewClient(provider *catalogs.Provider) *Client {
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
		return nil, &errors.ConfigError{
			Component: "groq",
			Message:   "provider not configured",
		}
	}

	// Single API call to get the complete list of models with all fields
	listURL := "https://api.groq.com/openai/v1/models"
	if rb := transport.NewRequestBuilder(provider); rb.GetBaseURL() != "" {
		listURL = rb.GetBaseURL()
	}

	resp, err := c.OpenAIClient.GetTransport().Get(ctx, listURL, provider)
	if err != nil {
		return nil, &errors.APIError{
			Provider: "groq",
			Endpoint: listURL,
			Message:  "list request failed",
			Err:      err,
		}
	}

	var listResult GroqResponse
	if err := transport.DecodeResponse(resp, &listResult); err != nil {
		return nil, errors.WrapParse("json", "groq response", err)
	}

	// Extract authors from API response and update provider configuration
	c.extractAndUpdateAuthors(listResult.Data)

	// Convert each model directly - no need for additional fetches
	models := make([]catalogs.Model, 0, len(listResult.Data))
	for _, groqModel := range listResult.Data {
		model := c.ConvertToGroqModel(groqModel)
		models = append(models, model)
	}

	return models, nil
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

// extractAndUpdateAuthors extracts authors from API response and updates provider configuration
func (c *Client) extractAndUpdateAuthors(models []GroqModelData) {
	provider := c.OpenAIClient.GetProvider()
	if provider == nil {
		return
	}

	// Collect unique authors from API response
	authorSet := make(map[string]bool)
	for _, model := range models {
		if model.OwnedBy != "" {
			// Normalize and split composite authors (e.g., "DeepSeek / Meta")
			authors := c.normalizeAuthor(model.OwnedBy)
			for _, author := range authors {
				authorSet[author] = true
			}
		}
	}

	// Convert to sorted slice
	var discoveredAuthors []string
	for author := range authorSet {
		discoveredAuthors = append(discoveredAuthors, author)
	}

	// Merge with existing configured authors
	provider.Authors = c.mergeAuthors(provider.Authors, discoveredAuthors)
}

// normalizeAuthor normalizes author names and handles composite authors
func (c *Client) normalizeAuthor(ownedBy string) []string {
	// Handle composite authors like "DeepSeek / Meta"
	if strings.Contains(ownedBy, "/") {
		parts := strings.Split(ownedBy, "/")
		var normalized []string
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				normalized = append(normalized, c.normalizeAuthorName(part))
			}
		}
		return normalized
	}

	return []string{c.normalizeAuthorName(ownedBy)}
}

// normalizeAuthorName normalizes a single author name to lowercase kebab-case
func (c *Client) normalizeAuthorName(name string) string {
	// Convert to lowercase and replace spaces with hyphens
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, " ", "-")
	normalized = strings.ReplaceAll(normalized, "_", "-")

	// Handle special cases
	switch normalized {
	case "alibaba-cloud":
		return "alibaba"
	case "moonshot-ai":
		return "moonshot"
	case "playai":
		return "playai"
	case "hugging-face":
		return "huggingfaceh4"
	}

	return normalized
}

// mergeAuthors merges existing and discovered authors (additive-only, preserves manual config)
func (c *Client) mergeAuthors(existing []catalogs.AuthorID, discovered []string) []catalogs.AuthorID {
	authorSet := make(map[string]bool)

	// ALWAYS preserve existing authors (manual configuration)
	for _, author := range existing {
		if string(author) != "" {
			authorSet[string(author)] = true
		}
	}

	// Add newly discovered authors (from API)
	for _, author := range discovered {
		if author != "" {
			authorSet[author] = true
		}
	}

	// Convert back to slice and sort
	var merged []catalogs.AuthorID
	for author := range authorSet {
		merged = append(merged, catalogs.AuthorID(author))
	}

	// Sort for consistent output
	for i := 0; i < len(merged); i++ {
		for j := i + 1; j < len(merged); j++ {
			if merged[i] > merged[j] {
				merged[i], merged[j] = merged[j], merged[i]
			}
		}
	}

	return merged
}
