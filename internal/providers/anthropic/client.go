// Package anthropic provides a client for the Anthropic API.
package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/sourcepayload"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// Response structures for Anthropic API.
type modelsResponse struct {
	Data          []modelResponse                  `json:"data"`
	UnknownFields []sourcepayload.UnknownJSONField `json:"-"`
	RecordReport  sourcepayload.RecordReport       `json:"-"`
}

func (r *modelsResponse) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "$")
	if err != nil {
		return err
	}
	var records []modelResponse
	var report sourcepayload.RecordReport
	if len(decoded.Data) != 0 && string(decoded.Data) != "null" {
		records, report, err = sourcepayload.DecodeJSONArray[modelResponse](
			decoded.Data,
			"data",
			constants.MaxCatalogModels,
		)
		if err != nil {
			return err
		}
	}
	*r = modelsResponse{Data: records, UnknownFields: unknown, RecordReport: report}
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
	transport *transport.Client
	mu        sync.RWMutex
}

// NewClient creates an Anthropic transport client.
func NewClient(provider *catalogs.Provider) *Client {
	return &Client{
		provider:  provider,
		transport: transport.New(),
	}
}

// Configure sets the provider for this client (used by registry pattern).
func (c *Client) Configure(provider *catalogs.Provider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.provider = provider
	c.transport = transport.New()
}

// ListModels retrieves all available models from Anthropic.
func (c *Client) ListModels(
	ctx context.Context,
	material sources.ProviderCredentialMaterial,
) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil {
		return nil, &errors.ConfigError{
			Component: "provider",
			Message:   "provider not configured",
		}
	}

	url, err := provider.BindCatalogEndpoint(material.EndpointBindings())
	if err != nil {
		return nil, err
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.WrapResource("create", "request", url, err)
	}

	// Use transport layer for HTTP request with authentication
	resp, err := c.transport.Do(req, provider, material)
	if err != nil {
		return nil, &errors.APIError{
			Provider: string(provider.ID),
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

	return models, result.RecordReport.Err("anthropic models")
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
	canonicalCapabilities := response.Capabilities.ImageInput.Supported ||
		response.Capabilities.PDFInput.Supported ||
		response.Capabilities.StructuredOutputs.Supported ||
		response.Capabilities.Thinking.Supported ||
		response.Capabilities.Effort.Supported
	var features *catalogs.ModelFeatures
	if canonicalCapabilities {
		features = &catalogs.ModelFeatures{}
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
