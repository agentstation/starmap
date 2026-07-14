// Package anthropic provides a client for the Anthropic API.
package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

// Response structures for Anthropic API.
type modelsResponse struct {
	Data          []modelResponse                  `json:"data"`
	UnknownFields []sourcepayload.UnknownJSONField `json:"-"`
}

func (r *modelsResponse) UnmarshalJSON(data []byte) error {
	type responseAlias modelsResponse
	var decoded responseAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "$")
	if err != nil {
		return err
	}
	*r = modelsResponse(decoded)
	r.UnknownFields = unknown
	return nil
}

type modelResponse struct {
	Type           string                           `json:"type"`
	ID             string                           `json:"id"`
	DisplayName    string                           `json:"display_name"`
	CreatedAt      time.Time                        `json:"created_at"`
	MaxTokens      int64                            `json:"max_tokens,omitempty"`
	MaxInputTokens int64                            `json:"max_input_tokens,omitempty"`
	Capabilities   *modelCapabilities               `json:"capabilities,omitempty"`
	UnknownFields  []sourcepayload.UnknownJSONField `json:"-"`
}

func (m *modelResponse) UnmarshalJSON(data []byte) error {
	type modelAlias modelResponse
	var decoded modelAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "data[]")
	if err != nil {
		return err
	}
	*m = modelResponse(decoded)
	m.UnknownFields = unknown
	return nil
}

type supportedCapability struct {
	Supported bool `json:"supported"`
}

type modelCapabilities struct {
	Batch             supportedCapability         `json:"batch"`
	Citations         supportedCapability         `json:"citations"`
	CodeExecution     supportedCapability         `json:"code_execution"`
	ContextManagement contextManagementCapability `json:"context_management"`
	Effort            effortCapability            `json:"effort"`
	ImageInput        supportedCapability         `json:"image_input"`
	PDFInput          supportedCapability         `json:"pdf_input"`
	StructuredOutputs supportedCapability         `json:"structured_outputs"`
	Thinking          thinkingCapability          `json:"thinking"`
}

type contextManagementCapability struct {
	Supported             bool                `json:"supported"`
	ClearToolUses20250919 supportedCapability `json:"clear_tool_uses_20250919"`
	ClearThinking20251015 supportedCapability `json:"clear_thinking_20251015"`
	Compact20260112       supportedCapability `json:"compact_20260112"`
}

type effortCapability struct {
	Supported bool                `json:"supported"`
	Low       supportedCapability `json:"low"`
	Medium    supportedCapability `json:"medium"`
	High      supportedCapability `json:"high"`
	Max       supportedCapability `json:"max"`
}

type thinkingCapability struct {
	Supported bool                     `json:"supported"`
	Types     thinkingTypeCapabilities `json:"types"`
}

type thinkingTypeCapabilities struct {
	Adaptive supportedCapability `json:"adaptive"`
	Enabled  supportedCapability `json:"enabled"`
}

// Client implements the catalogs.Client interface for Anthropic.
type Client struct {
	provider  *catalogs.Provider
	endpoint  string
	transport *transport.Client
	mu        sync.RWMutex
}

// NewClient creates a new Anthropic client.
func NewClient(source acquisition.Source) *Client {
	provider := source.Provider()
	return &Client{
		provider:  &provider,
		endpoint:  source.EndpointURL(),
		transport: transport.New(source.Auth()),
	}
}

// ListModels retrieves all available models from Anthropic.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	endpoint := c.endpoint
	c.mu.RUnlock()

	if provider == nil {
		return nil, &errors.ConfigError{
			Component: string(catalogs.ProviderIDAnthropic),
			Message:   "provider not configured",
		}
	}

	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDAnthropic), Message: "catalog endpoint is required"}
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, errors.WrapResource("create", "request", endpoint, err)
	}

	// Add Anthropic-specific headers
	req.Header.Set("anthropic-version", "2023-06-01")

	// Use transport layer for HTTP request with authentication
	resp, err := c.transport.Do(req)
	if err != nil {
		return nil, &errors.APIError{
			Provider: string(catalogs.ProviderIDAnthropic),
			Endpoint: endpoint,
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
	return c.modelsFromResponse(result)
}

// DecodeModels validates and normalizes one already-acquired response using
// the same exact connector contract as ListModels.
func (c *Client) DecodeModels(payload []byte) ([]catalogs.Model, error) {
	if err := sourcepayload.ValidateJSON(payload); err != nil {
		return nil, err
	}
	var result modelsResponse
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, errors.WrapParse("json", "anthropic response fixture", err)
	}
	return c.modelsFromResponse(result)
}

func (c *Client) modelsFromResponse(result modelsResponse) ([]catalogs.Model, error) {
	if result.Data == nil {
		return nil, errors.NewParseError("json", "anthropic response", "required data array is missing or null", nil)
	}

	// Convert Anthropic models to starmap models
	models := make([]catalogs.Model, 0, len(result.Data))
	for _, m := range result.Data {
		m.UnknownFields = append(m.UnknownFields, result.UnknownFields...)
		model := c.convertToModel(m)
		models = append(models, *model)
	}

	return models, nil
}

// convertToModel converts an Anthropic model response to a starmap Model.
func (c *Client) convertToModel(m modelResponse) *catalogs.Model {
	model := catalogs.Model{
		ID:   m.ID,
		Name: m.DisplayName,
	}

	// Set created time
	if !m.CreatedAt.IsZero() {
		model.CreatedAt = utc.New(m.CreatedAt)
		model.UpdatedAt = model.CreatedAt
	}

	// Set Anthropic as the author
	model.Authors = []catalogs.Author{
		{ID: catalogs.AuthorIDAnthropic, Name: "Anthropic"},
	}

	// Set basic features based on model ID patterns
	// Note: Detailed limits and pricing will be enhanced by models.dev integration
	model.Features = c.inferFeatures(m.ID)
	c.applyResponseFields(&model, m)
	if len(m.UnknownFields) > 0 {
		if model.Extensions == nil {
			model.Extensions = catalogs.SourceExtensions{}
		}
		extension := model.Extensions[c.extensionSource()]
		if extension.Fields == nil {
			extension.Fields = make(map[string]any)
		}
		extension.Fields["unknown_fields"] = m.UnknownFields
		model.Extensions[c.extensionSource()] = extension
	}

	// Don't set limits - let models.dev provide accurate data
	// Anthropic API doesn't return token limits, so we rely on models.dev

	return &model
}

func (c *Client) applyResponseFields(model *catalogs.Model, response modelResponse) {
	if response.MaxInputTokens > 0 || response.MaxTokens > 0 {
		model.Limits = &catalogs.ModelLimits{
			ContextWindow: response.MaxInputTokens,
			InputTokens:   response.MaxInputTokens,
			OutputTokens:  response.MaxTokens,
		}
	}
	if response.Capabilities == nil {
		return
	}
	features := model.Features
	if features == nil {
		features = &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
				Output: []catalogs.ModelModality{catalogs.ModelModalityText},
			},
		}
		model.Features = features
	}
	if response.Capabilities.ImageInput.Supported {
		features.Modalities.Input = appendAnthropicModality(features.Modalities.Input, catalogs.ModelModalityImage)
		features.Attachments = true
	}
	if response.Capabilities.PDFInput.Supported {
		features.Modalities.Input = appendAnthropicModality(features.Modalities.Input, catalogs.ModelModalityPDF)
		features.Attachments = true
	}
	if response.Capabilities.StructuredOutputs.Supported {
		features.StructuredOutputs = true
		features.FormatResponse = true
	}
	if response.Capabilities.Thinking.Supported {
		features.Reasoning = true
		features.IncludeReasoning = true
	}
	if response.Capabilities.Effort.Supported {
		features.ReasoningEffort = true
		model.Reasoning = &catalogs.ModelControlLevels{
			Levels: anthropicEffortLevels(response.Capabilities.Effort),
		}
	}
	if extensionFields := anthropicCapabilityExtensions(*response.Capabilities); len(extensionFields) > 0 {
		if model.Extensions == nil {
			model.Extensions = catalogs.SourceExtensions{}
		}
		model.Extensions[c.extensionSource()] = catalogs.SourceExtension{Fields: extensionFields}
	}
}

func appendAnthropicModality(modalities []catalogs.ModelModality, modality catalogs.ModelModality) []catalogs.ModelModality {
	if slices.Contains(modalities, modality) {
		return modalities
	}
	return append(modalities, modality)
}

func anthropicEffortLevels(effort effortCapability) []catalogs.ModelControlLevel {
	levels := make([]catalogs.ModelControlLevel, 0, 4)
	if effort.Low.Supported {
		levels = append(levels, catalogs.ModelControlLevelLow)
	}
	if effort.Medium.Supported {
		levels = append(levels, catalogs.ModelControlLevelMedium)
	}
	if effort.High.Supported {
		levels = append(levels, catalogs.ModelControlLevelHigh)
	}
	if effort.Max.Supported {
		levels = append(levels, catalogs.ModelControlLevelMaximum)
	}
	return levels
}

func anthropicCapabilityExtensions(capabilities modelCapabilities) map[string]any {
	fields := make(map[string]any)
	addSupportedExtension(fields, "batch", capabilities.Batch)
	addSupportedExtension(fields, "citations", capabilities.Citations)
	addSupportedExtension(fields, "code_execution", capabilities.CodeExecution)
	if capabilities.ContextManagement.Supported {
		fields["context_management"] = map[string]any{
			"supported":                true,
			"clear_tool_uses_20250919": capabilities.ContextManagement.ClearToolUses20250919.Supported,
			"clear_thinking_20251015":  capabilities.ContextManagement.ClearThinking20251015.Supported,
			"compact_20260112":         capabilities.ContextManagement.Compact20260112.Supported,
		}
	}
	if capabilities.Thinking.Types.Adaptive.Supported || capabilities.Thinking.Types.Enabled.Supported {
		fields["thinking_types"] = map[string]any{
			"adaptive": capabilities.Thinking.Types.Adaptive.Supported,
			"enabled":  capabilities.Thinking.Types.Enabled.Supported,
		}
	}
	return fields
}

func addSupportedExtension(fields map[string]any, name string, capability supportedCapability) {
	if capability.Supported {
		fields[name] = true
	}
}

func (c *Client) extensionSource() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.provider != nil && c.provider.ID != "" {
		return c.provider.ID.String()
	}
	return catalogs.ProviderIDAnthropic.String()
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
