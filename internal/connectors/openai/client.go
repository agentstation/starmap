// Package openai provides the shared client for OpenAI-compatible model APIs.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"path"
	"reflect"
	"slices"
	"sort"
	"strconv"
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

const (
	fieldID              = "id"
	fieldOwnedBy         = "owned_by"
	fieldMetadataTags    = "metadata.tags"
	featureReasoning     = "reasoning"
	requestFailedMessage = "request failed"
)

// Response represents the OpenAI API list models response.
type Response struct {
	Object        string                           `json:"object"`
	Data          []Model                          `json:"data"`
	UnknownFields []sourcepayload.UnknownJSONField `json:"-"`
	// RawJSON retains the source payload for provider-owned adapters whose
	// response envelope extends the OpenAI-compatible list contract.
	RawJSON json.RawMessage `json:"-"`
}

// UnmarshalJSON retains fingerprints for additive top-level fields.
func (r *Response) UnmarshalJSON(data []byte) error {
	type responseAlias Response
	var decoded responseAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "$")
	if err != nil {
		return err
	}
	*r = Response(decoded)
	r.UnknownFields = unknown
	r.RawJSON = append(r.RawJSON[:0], data...)
	return nil
}

// Model represents a model in the OpenAI API response.
type Model struct {
	ID      string  `json:"id"`
	Object  string  `json:"object"`
	OwnedBy string  `json:"owned_by"`
	Created int64   `json:"created"`
	Root    string  `json:"root,omitempty"`
	Parent  *string `json:"parent,omitempty"`
	Name    string  `json:"name,omitempty"`
	// Dynamic fields from provider-specific responses
	MaxModelLen                 *int64   `json:"max_model_len,omitempty"`
	MaxContextLength            *int64   `json:"max_context_length,omitempty"`
	ContextWindow               *int64   `json:"context_window,omitempty"`
	ContextLength               *int64   `json:"context_length,omitempty"`
	MaxCompletionTokens         *int64   `json:"max_completion_tokens,omitempty"`
	MaxOutputLength             *int64   `json:"max_output_length,omitempty"`
	InputTokenLimit             *int64   `json:"input_token_limit,omitempty"`
	OutputTokenLimit            *int64   `json:"output_token_limit,omitempty"`
	InputModalities             []string `json:"input_modalities,omitempty"`
	OutputModalities            []string `json:"output_modalities,omitempty"`
	SupportedFeatures           []string `json:"supported_features,omitempty"`
	SupportedSamplingParameters []string `json:"supported_sampling_parameters,omitempty"`
	// Common additive fields exposed by OpenAI-compatible API dialects. Their
	// canonical meaning is selected by provider configuration, not provider IDs.
	Active             *bool                            `json:"active,omitempty"`
	PublicApps         any                              `json:"public_apps,omitempty"`
	HuggingFaceID      string                           `json:"hugging_face_id,omitempty"`
	Pricing            *ModelPricing                    `json:"pricing,omitempty"`
	Kind               string                           `json:"kind,omitempty"`
	SupportsChat       *bool                            `json:"supports_chat,omitempty"`
	SupportsTools      *bool                            `json:"supports_tools,omitempty"`
	SupportsImageInput *bool                            `json:"supports_image_input,omitempty"`
	SupportsImageIn    *bool                            `json:"supports_image_in,omitempty"`
	SupportsVideoIn    *bool                            `json:"supports_video_in,omitempty"`
	SupportsReasoning  *bool                            `json:"supports_reasoning,omitempty"`
	Permission         []ModelPermission                `json:"permission,omitempty"`
	Metadata           *ModelMetadata                   `json:"metadata,omitempty"`
	Aliases            []string                         `json:"aliases,omitempty"`
	Type               string                           `json:"TYPE,omitempty"`
	Archived           *bool                            `json:"archived,omitempty"`
	Fingerprint        string                           `json:"fingerprint,omitempty"`
	Version            string                           `json:"version,omitempty"`
	UnknownFields      []sourcepayload.UnknownJSONField `json:"-"`
	// RawJSON retains one source record for provider-owned enrichment and
	// validation without teaching the shared transport provider-specific fields.
	RawJSON json.RawMessage `json:"-"`
}

// UnmarshalJSON retains fingerprints for additive model fields.
func (m *Model) UnmarshalJSON(data []byte) error {
	type modelAlias Model
	var decoded modelAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "data[]")
	if err != nil {
		return err
	}
	*m = Model(decoded)
	m.UnknownFields = unknown
	m.RawJSON = append(m.RawJSON[:0], data...)
	return nil
}

// ModelMetadata represents nested metadata returned by some OpenAI-compatible providers.
type ModelMetadata struct {
	Description       string                `json:"description,omitempty"`
	ContextLength     *int64                `json:"context_length,omitempty"`
	MaxTokens         *int64                `json:"max_tokens,omitempty"`
	Tags              []string              `json:"tags,omitempty"`
	DefaultWidth      *int64                `json:"default_width,omitempty"`
	DefaultHeight     *int64                `json:"default_height,omitempty"`
	DefaultIterations *int64                `json:"default_iterations,omitempty"`
	Pricing           *ModelMetadataPricing `json:"pricing,omitempty"`
}

// ModelPricing represents top-level provider pricing returned by OpenAI-compatible APIs.
type ModelPricing struct {
	Request        *float64 `json:"request,omitempty"`
	Prompt         *float64 `json:"prompt,omitempty"`
	Completion     *float64 `json:"completion,omitempty"`
	InputCacheRead *float64 `json:"input_cache_read,omitempty"`
	Image          *float64 `json:"image,omitempty"`
}

// UnmarshalJSON accepts provider pricing values as either numbers or numeric strings.
func (p *ModelPricing) UnmarshalJSON(data []byte) error {
	var raw struct {
		Request        json.RawMessage `json:"request"`
		Prompt         json.RawMessage `json:"prompt"`
		Completion     json.RawMessage `json:"completion"`
		InputCacheRead json.RawMessage `json:"input_cache_read"`
		Image          json.RawMessage `json:"image"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var err error
	if p.Request, err = parseOptionalFloat(raw.Request, "request"); err != nil {
		return err
	}
	if p.Prompt, err = parseOptionalFloat(raw.Prompt, "prompt"); err != nil {
		return err
	}
	if p.Completion, err = parseOptionalFloat(raw.Completion, "completion"); err != nil {
		return err
	}
	if p.InputCacheRead, err = parseOptionalFloat(raw.InputCacheRead, "input_cache_read"); err != nil {
		return err
	}
	if p.Image, err = parseOptionalFloat(raw.Image, "image"); err != nil {
		return err
	}
	return nil
}

func parseOptionalFloat(raw json.RawMessage, field string) (*float64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var numeric float64
	if err := json.Unmarshal(raw, &numeric); err == nil {
		return &numeric, nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return nil, fmt.Errorf("parse pricing.%s: %w", field, err)
	}
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, fmt.Errorf("parse pricing.%s: %w", field, err)
	}
	return &parsed, nil
}

// ModelMetadataPricing represents nested provider pricing returned in metadata.
type ModelMetadataPricing struct {
	InputTokens     *float64 `json:"input_tokens,omitempty"`
	OutputTokens    *float64 `json:"output_tokens,omitempty"`
	CacheReadTokens *float64 `json:"cache_read_tokens,omitempty"`
	PerImageUnit    *float64 `json:"per_image_unit,omitempty"`
	InputCharacters *float64 `json:"input_characters,omitempty"`
	InputSeconds    *float64 `json:"input_seconds,omitempty"`
	OutputSeconds   *float64 `json:"output_seconds,omitempty"`
}

// UnmarshalJSON accepts nested provider pricing as numbers or numeric strings.
func (p *ModelMetadataPricing) UnmarshalJSON(data []byte) error {
	var raw struct {
		InputTokens     json.RawMessage `json:"input_tokens"`
		OutputTokens    json.RawMessage `json:"output_tokens"`
		CacheReadTokens json.RawMessage `json:"cache_read_tokens"`
		PerImageUnit    json.RawMessage `json:"per_image_unit"`
		InputCharacters json.RawMessage `json:"input_characters"`
		InputSeconds    json.RawMessage `json:"input_seconds"`
		OutputSeconds   json.RawMessage `json:"output_seconds"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var err error
	fields := []struct {
		name string
		raw  json.RawMessage
		set  func(*float64)
	}{
		{"input_tokens", raw.InputTokens, func(value *float64) { p.InputTokens = value }},
		{"output_tokens", raw.OutputTokens, func(value *float64) { p.OutputTokens = value }},
		{"cache_read_tokens", raw.CacheReadTokens, func(value *float64) { p.CacheReadTokens = value }},
		{"per_image_unit", raw.PerImageUnit, func(value *float64) { p.PerImageUnit = value }},
		{"input_characters", raw.InputCharacters, func(value *float64) { p.InputCharacters = value }},
		{"input_seconds", raw.InputSeconds, func(value *float64) { p.InputSeconds = value }},
		{"output_seconds", raw.OutputSeconds, func(value *float64) { p.OutputSeconds = value }},
	}
	for _, field := range fields {
		value, parseErr := parseOptionalFloat(field.raw, field.name)
		if parseErr != nil {
			err = parseErr
			break
		}
		field.set(value)
	}
	return err
}

// ModelPermission represents provider permission metadata.
type ModelPermission struct {
	ID           string  `json:"id,omitempty"`
	Object       string  `json:"object,omitempty"`
	Created      int64   `json:"created,omitempty"`
	Organization string  `json:"organization,omitempty"`
	Group        *string `json:"group,omitempty"`
}

// Client implements the catalogs.Client interface with dynamic configuration.
type Client struct {
	// Transport client
	transport *transport.Client

	// Provider with mutex protection
	provider *catalogs.Provider
	endpoint string
	mu       sync.RWMutex

	responseModels ResponseModelsFunc
	modelEnricher  ModelEnricher
}

// ResponseModelsFunc validates a provider response and returns its model records.
type ResponseModelsFunc func(Response) ([]Model, error)

// ModelEnricher applies provider-owned facts after the shared conversion.
type ModelEnricher func(*catalogs.Model, Model) error

// Option configures an OpenAI-compatible client.
type Option func(*Client)

// WithResponseModels installs a provider-owned response-envelope adapter.
func WithResponseModels(adapter ResponseModelsFunc) Option {
	return func(client *Client) { client.responseModels = adapter }
}

// WithModelEnricher installs a provider-owned model enrichment adapter.
func WithModelEnricher(enricher ModelEnricher) Option {
	return func(client *Client) { client.modelEnricher = enricher }
}

// NewClient creates a validated dynamic OpenAI-compatible client.
func NewClient(source acquisition.Source, opts ...Option) (*Client, error) {
	provider := source.Provider()
	client := &Client{
		provider: &provider,
		endpoint: source.EndpointURL(),
	}
	for _, opt := range opts {
		opt(client)
	}
	if err := client.validateFieldMappings(&provider); err != nil {
		return nil, err
	}
	client.transport = transport.New(source.Auth())
	return client, nil
}

// ListModels retrieves all available models using OpenAI-compatible API.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	endpoint := c.endpoint
	c.mu.RUnlock()

	if provider == nil {
		return nil, &errors.ValidationError{
			Field:   "provider",
			Message: "provider not configured",
		}
	}
	if err := c.validateFieldMappings(provider); err != nil {
		return nil, err
	}

	// Build URL from provider configuration.
	url := endpoint
	if url == "" {
		return nil, &errors.ValidationError{
			Field:   "catalog.endpoint.url",
			Message: "endpoint URL not configured",
		}
	}

	var result Response
	if err := c.fetchModelsResponse(ctx, provider, url, &result); err != nil {
		statusCode := 0
		message := "failed to decode response"
		var apiErr *errors.APIError
		if stderrors.As(err, &apiErr) {
			statusCode = apiErr.StatusCode
			if statusCode == 0 && apiErr.Message == requestFailedMessage {
				message = requestFailedMessage
			}
		}
		return nil, &errors.APIError{
			Provider: provider.ID.String(), StatusCode: statusCode,
			Message: message, Err: err,
		}
	}
	return c.modelsFromResponse(provider, result)
}

// DecodeModels validates and normalizes one already-acquired response using
// the same exact connector contract as ListModels.
func (c *Client) DecodeModels(payload []byte) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()
	if provider == nil {
		return nil, &errors.ValidationError{Field: "provider", Message: "provider not configured"}
	}
	if err := sourcepayload.ValidateJSON(payload); err != nil {
		return nil, err
	}
	var result Response
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, errors.WrapParse("json", "provider response fixture", err)
	}
	return c.modelsFromResponse(provider, result)
}

func (c *Client) modelsFromResponse(provider *catalogs.Provider, result Response) ([]catalogs.Model, error) {
	var sourceModels []Model
	var err error
	if c.responseModels != nil {
		sourceModels, err = c.responseModels(result)
	} else {
		sourceModels, err = configuredResponseModels(provider, result)
	}
	if err != nil {
		return nil, &errors.APIError{
			Provider: provider.ID.String(),
			Message:  "models response schema drift",
			Err:      err,
		}
	}

	// Convert to starmap models
	models := make([]catalogs.Model, 0, len(sourceModels))
	for _, m := range sourceModels {
		m.UnknownFields = append(m.UnknownFields, result.UnknownFields...)
		model, err := c.convertToModel(m)
		if err != nil {
			return nil, &errors.APIError{Provider: provider.ID.String(), Message: "model normalization failed", Err: err}
		}
		models = append(models, *model)
	}

	return models, nil
}

func (c *Client) fetchModelsResponse(ctx context.Context, provider *catalogs.Provider, url string, result *Response) error {
	resp, err := c.transport.Get(ctx, url)
	if err != nil {
		return &errors.APIError{Provider: provider.ID.String(), Message: requestFailedMessage, Err: err}
	}
	return transport.DecodeResponse(resp, result)
}

// ConvertToModel converts an OpenAI model response to a starmap Model using dynamic configuration.
// This method is public for testing purposes.
func (c *Client) ConvertToModel(m Model) *catalogs.Model {
	model, _ := c.convertToModel(m)
	return model
}

func (c *Client) convertToModel(m Model) (*catalogs.Model, error) {
	model := &catalogs.Model{
		ID:          m.ID,
		Name:        m.ID, // Default to ID, may be overridden
		Description: "",
		Features:    c.applyFeatureRules(m),
	}

	// Apply dynamic field mappings
	if err := c.applyFieldMappings(model, m); err != nil {
		return nil, err
	}

	// Apply dynamic author extraction
	model.Authors = c.extractAuthors(m.ID, m.OwnedBy)

	if err := c.applyProviderDefaults(model, m); err != nil {
		return nil, err
	}
	if c.modelEnricher != nil {
		if err := c.modelEnricher(model, m); err != nil {
			return nil, err
		}
	}

	if m.Root != "" || m.Parent != nil {
		model.Lineage = &catalogs.ModelLineage{}
		if m.Root != "" {
			root := m.Root
			model.Lineage.Root = &root
		}
		if m.Parent != nil && *m.Parent != "" {
			parent := *m.Parent
			model.Lineage.Parent = &parent
		}
	}

	return model, nil
}

func (c *Client) applyProviderDefaults(model *catalogs.Model, apiModel Model) error {
	if apiModel.Name != "" && model.Name == model.ID {
		model.Name = apiModel.Name
	}
	if apiModel.Created > 0 {
		created := utc.New(time.Unix(apiModel.Created, 0))
		model.CreatedAt = created
		if model.UpdatedAt.IsZero() {
			model.UpdatedAt = created
		}
	}
	if apiModel.Active != nil {
		if *apiModel.Active {
			model.Status = catalogs.ModelStatusActive
		} else {
			model.Status = catalogs.ModelStatusUnknown
		}
	}
	if apiModel.Archived != nil {
		if *apiModel.Archived {
			model.Status = catalogs.ModelStatusDeprecated
		} else if model.Status == "" {
			model.Status = catalogs.ModelStatusActive
		}
	}
	c.applyProviderLimits(model, apiModel)
	c.applyProviderMetadata(model, apiModel)
	c.applyProviderFeatures(model, apiModel)
	if err := c.applyProviderPricing(model, apiModel); err != nil {
		return err
	}
	c.applyProviderExtensions(model, apiModel)
	return nil
}

func (c *Client) applyProviderLimits(model *catalogs.Model, apiModel Model) {
	contextWindow := firstInt64(apiModel.ContextWindow, apiModel.ContextLength, apiModel.MaxModelLen, apiModel.MaxContextLength)
	if apiModel.Metadata != nil && contextWindow == nil {
		contextWindow = apiModel.Metadata.ContextLength
	}
	outputTokens := firstInt64(apiModel.MaxCompletionTokens, apiModel.OutputTokenLimit, apiModel.MaxOutputLength)
	if apiModel.Metadata != nil && outputTokens == nil {
		outputTokens = apiModel.Metadata.MaxTokens
	}
	if contextWindow == nil && apiModel.InputTokenLimit == nil && outputTokens == nil {
		return
	}
	if model.Limits == nil {
		model.Limits = &catalogs.ModelLimits{}
	}
	if contextWindow != nil && model.Limits.ContextWindow == 0 {
		model.Limits.ContextWindow = *contextWindow
	}
	if apiModel.InputTokenLimit != nil && model.Limits.InputTokens == 0 {
		model.Limits.InputTokens = *apiModel.InputTokenLimit
	}
	if outputTokens != nil && model.Limits.OutputTokens == 0 {
		model.Limits.OutputTokens = *outputTokens
	}
}

func (c *Client) applyProviderMetadata(model *catalogs.Model, apiModel Model) {
	if apiModel.Metadata == nil {
		return
	}
	if model.Description == "" {
		model.Description = apiModel.Metadata.Description
	}
	if len(apiModel.Metadata.Tags) > 0 {
		if model.Metadata == nil {
			model.Metadata = &catalogs.ModelMetadata{}
		}
		if len(model.Metadata.Tags) == 0 {
			model.Metadata.Tags = c.toModelTags(apiModel.Metadata.Tags)
		}
	}
}

func (c *Client) applyProviderFeatures(model *catalogs.Model, apiModel Model) {
	features := ensureModelFeatures(model)
	if len(apiModel.InputModalities) > 0 {
		features.Modalities.Input = convertProviderModalities(apiModel.InputModalities)
	}
	if len(apiModel.OutputModalities) > 0 {
		features.Modalities.Output = convertProviderModalities(apiModel.OutputModalities)
	}
	if boolValue(apiModel.SupportsImageInput) || boolValue(apiModel.SupportsImageIn) {
		features.Modalities.Input = appendUniqueModality(features.Modalities.Input, catalogs.ModelModalityImage)
	}
	if boolValue(apiModel.SupportsVideoIn) {
		features.Modalities.Input = appendUniqueModality(features.Modalities.Input, catalogs.ModelModalityVideo)
	}
	if boolValue(apiModel.SupportsTools) {
		features.Tools = true
		features.ToolCalls = true
		features.ToolChoice = true
	}
	if boolValue(apiModel.SupportsReasoning) {
		features.Reasoning = true
	}
	for _, feature := range apiModel.SupportedFeatures {
		switch strings.ToLower(feature) {
		case "tools", "tool_use", "tool_calls":
			features.Tools = true
			features.ToolCalls = true
			features.ToolChoice = true
		case "json_mode", "json_object":
			features.FormatResponse = true
		case "structured_outputs", "structured_output", "json_schema":
			features.StructuredOutputs = true
		case featureReasoning, "thinking":
			features.Reasoning = true
		}
	}
	for _, parameter := range apiModel.SupportedSamplingParameters {
		switch strings.ToLower(parameter) {
		case "temperature":
			features.Temperature = true
		case "top_p":
			features.TopP = true
		case "top_k":
			features.TopK = true
		case "stop":
			features.Stop = true
		case "frequency_penalty":
			features.FrequencyPenalty = true
		case "presence_penalty":
			features.PresencePenalty = true
		case "seed":
			features.Seed = true
		case "logprobs":
			features.Logprobs = true
		case "top_logprobs":
			features.TopLogprobs = true
		}
	}
}

func (c *Client) applyProviderPricing(model *catalogs.Model, apiModel Model) error {
	if c.hasConfiguredPricingMappings() {
		return nil
	}
	if apiModel.Pricing == nil && (apiModel.Metadata == nil || apiModel.Metadata.Pricing == nil) {
		return nil
	}
	ensureModelPricing(model)
	if err := applyOpenAICompatiblePricing(model.Pricing, apiModel.Pricing); err != nil {
		return err
	}
	if apiModel.Metadata != nil {
		if err := applyOpenAICompatibleMetadataPricing(model.Pricing, apiModel.Metadata.Pricing); err != nil {
			return err
		}
	}
	if model.Pricing.Tokens.Input == nil && model.Pricing.Tokens.Output == nil && model.Pricing.Tokens.Cache == nil {
		model.Pricing.Tokens = nil
	}
	if model.Pricing.Tokens == nil && model.Pricing.Operations == nil {
		model.Pricing = nil
		return nil
	}
	return model.Pricing.Validate()
}

func (c *Client) hasConfiguredPricingMappings() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.provider == nil || c.provider.Catalog == nil {
		return false
	}
	for _, mapping := range c.provider.Catalog.Sources[0].Endpoint.FieldMappings {
		if strings.HasPrefix(mapping.To, "pricing.") {
			return true
		}
	}
	return false
}

func ensureModelPricing(model *catalogs.Model) {
	if model.Pricing == nil {
		model.Pricing = &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD}
	}
	if model.Pricing.Currency == "" {
		model.Pricing.Currency = catalogs.ModelPricingCurrencyUSD
	}
	if model.Pricing.Tokens == nil {
		model.Pricing.Tokens = &catalogs.ModelTokenPricing{}
	}
}

func applyOpenAICompatiblePricing(pricing *catalogs.ModelPricing, source *ModelPricing) error {
	if source == nil {
		return nil
	}
	// This optional compatible-dialect block reports token prices in currency
	// per 1M tokens, matching catalogs.ModelTokenCost.Per1M. Provider contracts with different units
	// need an explicit provider-specific conversion before this mapping.
	if source.Prompt != nil && pricing.Tokens.Input == nil {
		cost, err := catalogs.NormalizeProviderTokenPrice(*source.Prompt, catalogs.ProviderNormalizationUnitPerMillionTokens)
		if err != nil {
			return err
		}
		pricing.Tokens.Input = &cost
	}
	if source.Completion != nil && pricing.Tokens.Output == nil {
		cost, err := catalogs.NormalizeProviderTokenPrice(*source.Completion, catalogs.ProviderNormalizationUnitPerMillionTokens)
		if err != nil {
			return err
		}
		pricing.Tokens.Output = &cost
	}
	if source.InputCacheRead != nil {
		ensureTokenCachePricing(pricing.Tokens)
		if pricing.Tokens.Cache.Read == nil {
			cost, err := catalogs.NormalizeProviderTokenPrice(*source.InputCacheRead, catalogs.ProviderNormalizationUnitPerMillionTokens)
			if err != nil {
				return err
			}
			pricing.Tokens.Cache.Read = &cost
		}
	}
	if source.Request != nil || source.Image != nil {
		ensureOperationPricing(pricing)
		if source.Request != nil && pricing.Operations.Request == nil {
			pricing.Operations.Request = source.Request
		}
		if source.Image != nil && pricing.Operations.ImageGen == nil {
			pricing.Operations.ImageGen = source.Image
		}
	}
	return nil
}

func applyOpenAICompatibleMetadataPricing(pricing *catalogs.ModelPricing, source *ModelMetadataPricing) error {
	if source == nil {
		return nil
	}
	// This optional compatible-dialect metadata block reports token fields in
	// currency per 1M tokens, matching Per1M.
	if source.InputTokens != nil && pricing.Tokens.Input == nil {
		cost, err := catalogs.NormalizeProviderTokenPrice(*source.InputTokens, catalogs.ProviderNormalizationUnitPerMillionTokens)
		if err != nil {
			return err
		}
		pricing.Tokens.Input = &cost
	}
	if source.OutputTokens != nil && pricing.Tokens.Output == nil {
		cost, err := catalogs.NormalizeProviderTokenPrice(*source.OutputTokens, catalogs.ProviderNormalizationUnitPerMillionTokens)
		if err != nil {
			return err
		}
		pricing.Tokens.Output = &cost
	}
	if source.CacheReadTokens != nil {
		ensureTokenCachePricing(pricing.Tokens)
		if pricing.Tokens.Cache.Read == nil {
			cost, err := catalogs.NormalizeProviderTokenPrice(*source.CacheReadTokens, catalogs.ProviderNormalizationUnitPerMillionTokens)
			if err != nil {
				return err
			}
			pricing.Tokens.Cache.Read = &cost
		}
	}
	if source.PerImageUnit != nil || source.InputSeconds != nil || source.OutputSeconds != nil {
		ensureOperationPricing(pricing)
		if source.PerImageUnit != nil && pricing.Operations.ImageGen == nil {
			pricing.Operations.ImageGen = source.PerImageUnit
		}
		if source.InputSeconds != nil && pricing.Operations.AudioInput == nil {
			pricing.Operations.AudioInput = source.InputSeconds
		}
		if source.OutputSeconds != nil && pricing.Operations.AudioGen == nil {
			pricing.Operations.AudioGen = source.OutputSeconds
		}
	}
	return nil
}

func (c *Client) applyProviderExtensions(model *catalogs.Model, apiModel Model) {
	fields := make(map[string]any)
	if apiModel.Object != "" && apiModel.Object != "model" {
		fields["object"] = apiModel.Object
	}
	if apiModel.HuggingFaceID != "" {
		fields["hugging_face_id"] = apiModel.HuggingFaceID
	}
	if apiModel.Kind != "" {
		fields["kind"] = apiModel.Kind
	}
	if len(apiModel.Aliases) > 0 {
		fields["aliases"] = append([]string(nil), apiModel.Aliases...)
	}
	if apiModel.Type != "" {
		fields["type"] = apiModel.Type
	}
	if apiModel.Archived != nil {
		fields["archived"] = *apiModel.Archived
	}
	if apiModel.Fingerprint != "" {
		fields["fingerprint"] = apiModel.Fingerprint
	}
	if apiModel.Version != "" {
		fields["version"] = apiModel.Version
	}
	if apiModel.SupportsChat != nil {
		fields["supports_chat"] = *apiModel.SupportsChat
	}
	if apiModel.PublicApps != nil {
		fields["public_apps"] = apiModel.PublicApps
	}
	if len(apiModel.Permission) > 0 {
		fields["permission"] = permissionExtensions(apiModel.Permission)
	}
	if apiModel.Metadata != nil {
		metadataFields := make(map[string]any)
		if apiModel.Metadata.DefaultWidth != nil {
			metadataFields["default_width"] = *apiModel.Metadata.DefaultWidth
		}
		if apiModel.Metadata.DefaultHeight != nil {
			metadataFields["default_height"] = *apiModel.Metadata.DefaultHeight
		}
		if apiModel.Metadata.DefaultIterations != nil {
			metadataFields["default_iterations"] = *apiModel.Metadata.DefaultIterations
		}
		if apiModel.Metadata.Pricing != nil && apiModel.Metadata.Pricing.InputCharacters != nil {
			metadataFields["pricing"] = map[string]any{
				"input_characters": *apiModel.Metadata.Pricing.InputCharacters,
			}
		}
		if len(metadataFields) > 0 {
			fields["metadata"] = metadataFields
		}
	}
	if len(apiModel.UnknownFields) > 0 {
		fields["unknown_fields"] = apiModel.UnknownFields
	}
	if len(fields) == 0 {
		return
	}
	source := c.extensionSource()
	if model.Extensions == nil {
		model.Extensions = catalogs.SourceExtensions{}
	}
	extension := model.Extensions[source]
	if extension.Fields == nil {
		extension.Fields = make(map[string]any)
	}
	for key, value := range catalogs.NormalizeExtensionFields(fields) {
		extension.Fields[key] = value
	}
	model.Extensions[source] = extension
}

func firstInt64(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func ensureModelFeatures(model *catalogs.Model) *catalogs.ModelFeatures {
	if model.Features == nil {
		model.Features = &catalogs.ModelFeatures{}
	}
	if len(model.Features.Modalities.Input) == 0 {
		model.Features.Modalities.Input = []catalogs.ModelModality{catalogs.ModelModalityText}
	}
	if len(model.Features.Modalities.Output) == 0 {
		model.Features.Modalities.Output = []catalogs.ModelModality{catalogs.ModelModalityText}
	}
	return model.Features
}

func convertProviderModalities(modalities []string) []catalogs.ModelModality {
	converted := make([]catalogs.ModelModality, 0, len(modalities))
	for _, modality := range modalities {
		switch strings.ToLower(strings.TrimSpace(modality)) {
		case "text":
			converted = appendUniqueModality(converted, catalogs.ModelModalityText)
		case "image":
			converted = appendUniqueModality(converted, catalogs.ModelModalityImage)
		case "audio":
			converted = appendUniqueModality(converted, catalogs.ModelModalityAudio)
		case "video":
			converted = appendUniqueModality(converted, catalogs.ModelModalityVideo)
		case "pdf":
			converted = appendUniqueModality(converted, catalogs.ModelModalityPDF)
		case "embedding", "embeddings":
			converted = appendUniqueModality(converted, catalogs.ModelModalityEmbedding)
		}
	}
	return converted
}

func appendUniqueModality(modalities []catalogs.ModelModality, modality catalogs.ModelModality) []catalogs.ModelModality {
	if slices.Contains(modalities, modality) {
		return modalities
	}
	return append(modalities, modality)
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func ensureTokenCachePricing(pricing *catalogs.ModelTokenPricing) {
	if pricing.Cache == nil {
		pricing.Cache = &catalogs.ModelTokenCachePricing{}
	}
}

func ensureOperationPricing(pricing *catalogs.ModelPricing) {
	if pricing.Operations == nil {
		pricing.Operations = &catalogs.ModelOperationPricing{}
	}
}

func permissionExtensions(permissions []ModelPermission) []any {
	extensions := make([]any, 0, len(permissions))
	for _, permission := range permissions {
		fields := make(map[string]any)
		if permission.ID != "" {
			fields["id"] = permission.ID
		}
		if permission.Object != "" {
			fields["object"] = permission.Object
		}
		if permission.Created > 0 {
			fields["created"] = permission.Created
		}
		if permission.Organization != "" {
			fields["organization"] = permission.Organization
		}
		if permission.Group != nil {
			fields["group"] = *permission.Group
		}
		extensions = append(extensions, fields)
	}
	return extensions
}

func (c *Client) extensionSource() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.provider != nil && c.provider.ID != "" {
		return c.provider.ID.String()
	}
	return "provider_api"
}

func dataModels(response Response) ([]Model, error) {
	if response.Data == nil {
		return nil, errors.NewParseError("json", "openai-compatible response", "required data array is missing or null", nil)
	}
	return response.Data, nil
}

func configuredResponseModels(provider *catalogs.Provider, response Response) ([]Model, error) {
	collection := "data"
	if provider != nil && provider.Catalog != nil && provider.Catalog.Sources[0].Endpoint.ResponseCollection != "" {
		collection = provider.Catalog.Sources[0].Endpoint.ResponseCollection
	}
	if collection == "data" {
		models, err := dataModels(response)
		if err != nil {
			return nil, err
		}
		return validateModelRecords(response.Object, models)
	}

	current := response.RawJSON
	for _, segment := range strings.Split(collection, ".") {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(current, &object); err != nil {
			return nil, errors.NewParseError("json", "openai-compatible response collection "+collection, "collection parent must be an object", err)
		}
		next, found := object[segment]
		if !found || len(next) == 0 || string(next) == "null" {
			return nil, errors.NewParseError("json", "openai-compatible response collection "+collection, "required model array is missing or null", nil)
		}
		current = next
	}
	var models []Model
	if err := json.Unmarshal(current, &models); err != nil {
		return nil, errors.NewParseError("json", "openai-compatible response collection "+collection, "collection must be a model array", err)
	}
	if models == nil {
		return nil, errors.NewParseError("json", "openai-compatible response collection "+collection, "required model array is missing or null", nil)
	}
	return validateModelRecords(response.Object, models)
}

func validateModelRecords(object string, models []Model) ([]Model, error) {
	if object != "" && object != "list" {
		return nil, errors.NewParseError("json", "openai-compatible response", "object must be list when present", nil)
	}
	seen := make(map[string]int, len(models))
	validated := make([]Model, 0, len(models))
	for index, model := range models {
		if strings.TrimSpace(model.ID) == "" {
			return nil, errors.NewParseError("json", fmt.Sprintf("openai-compatible response data[%d]", index), "model id is required", nil)
		}
		if model.Object != "" && model.Object != "model" {
			return nil, errors.NewParseError("json", fmt.Sprintf("openai-compatible response data[%d]", index), "object must be model when present", nil)
		}
		if priorIndex, found := seen[model.ID]; found {
			// Some compatible APIs emit the same record more than once. Treat only
			// byte-identical records as one observation; a repeated identity with
			// different source facts remains ambiguous and fails closed.
			if !bytes.Equal(validated[priorIndex].RawJSON, model.RawJSON) {
				return nil, errors.NewParseError("json", "openai-compatible response", "conflicting duplicate model id", nil)
			}
			continue
		}
		seen[model.ID] = len(validated)
		validated = append(validated, model)
	}
	return validated, nil
}

// applyFieldMappings applies configured field mappings using direct path matching.
func (c *Client) applyFieldMappings(model *catalogs.Model, apiModel Model) error {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil || provider.Catalog == nil || provider.Catalog.Sources[0].Endpoint.FieldMappings == nil {
		return nil
	}

	// Apply field mappings using direct path matching
	for _, mapping := range provider.Catalog.Sources[0].Endpoint.FieldMappings {
		if err := c.setFieldByPath(model, mapping, apiModel); err != nil {
			return err
		}
	}
	return nil
}

// setFieldByPath directly sets model fields based on path strings with type-safe conversion.
func (c *Client) setFieldByPath(model *catalogs.Model, mapping catalogs.FieldMapping, apiModel Model) error {
	sourceValue, ok, err := fieldMappingSourceValue(mapping.From, apiModel)
	if err != nil {
		return err
	}
	if !ok || isNilFieldMappingValue(sourceValue) {
		return nil
	}
	return c.applyMappedField(model, mapping, sourceValue)
}

func isNilFieldMappingValue(value any) bool {
	if value == nil {
		return true
	}
	valueReflect := reflect.ValueOf(value)
	switch valueReflect.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return valueReflect.IsNil()
	default:
		return false
	}
}

func fieldMappingSourceValue(fromPath string, apiModel Model) (any, bool, error) { //nolint:gocyclo // Typed fallbacks preserve direct unit-test construction while raw JSON is authoritative for provider extensions.
	if len(apiModel.RawJSON) > 0 {
		value, found, err := rawJSONFieldValue(apiModel.RawJSON, fromPath)
		if err != nil || found {
			return value, found, err
		}
	}
	switch fromPath {
	case "max_model_len":
		return apiModel.MaxModelLen, true, nil
	case "context_window":
		return apiModel.ContextWindow, true, nil
	case "context_length":
		return apiModel.ContextLength, true, nil
	case "max_completion_tokens":
		return apiModel.MaxCompletionTokens, true, nil
	case "max_output_length":
		return apiModel.MaxOutputLength, true, nil
	case "input_token_limit":
		return apiModel.InputTokenLimit, true, nil
	case "output_token_limit":
		return apiModel.OutputTokenLimit, true, nil
	case "name":
		return apiModel.Name, true, nil
	case "metadata.description":
		if apiModel.Metadata != nil {
			return apiModel.Metadata.Description, true, nil
		}
	case "metadata.context_length":
		if apiModel.Metadata != nil {
			return apiModel.Metadata.ContextLength, true, nil
		}
	case "metadata.max_tokens":
		if apiModel.Metadata != nil {
			return apiModel.Metadata.MaxTokens, true, nil
		}
	case fieldMetadataTags:
		if apiModel.Metadata != nil {
			return apiModel.Metadata.Tags, true, nil
		}
	case fieldID:
		return apiModel.ID, true, nil
	case fieldOwnedBy:
		return apiModel.OwnedBy, true, nil
	case "created":
		return apiModel.Created, true, nil
	case "archived":
		return apiModel.Archived, true, nil
	case "supports_tools":
		return apiModel.SupportsTools, true, nil
	case "supports_reasoning":
		return apiModel.SupportsReasoning, true, nil
	case "pricing.request":
		if apiModel.Pricing != nil {
			return apiModel.Pricing.Request, true, nil
		}
	case "pricing.prompt":
		if apiModel.Pricing != nil {
			return apiModel.Pricing.Prompt, true, nil
		}
	case "pricing.completion":
		if apiModel.Pricing != nil {
			return apiModel.Pricing.Completion, true, nil
		}
	case "pricing.input_cache_read":
		if apiModel.Pricing != nil {
			return apiModel.Pricing.InputCacheRead, true, nil
		}
	case "pricing.image":
		if apiModel.Pricing != nil {
			return apiModel.Pricing.Image, true, nil
		}
	case "metadata.pricing.input_tokens":
		if apiModel.Metadata != nil && apiModel.Metadata.Pricing != nil {
			return apiModel.Metadata.Pricing.InputTokens, true, nil
		}
	case "metadata.pricing.output_tokens":
		if apiModel.Metadata != nil && apiModel.Metadata.Pricing != nil {
			return apiModel.Metadata.Pricing.OutputTokens, true, nil
		}
	case "metadata.pricing.cache_read_tokens":
		if apiModel.Metadata != nil && apiModel.Metadata.Pricing != nil {
			return apiModel.Metadata.Pricing.CacheReadTokens, true, nil
		}
	case "metadata.pricing.per_image_unit":
		if apiModel.Metadata != nil && apiModel.Metadata.Pricing != nil {
			return apiModel.Metadata.Pricing.PerImageUnit, true, nil
		}
	}
	return nil, false, nil
}

func rawJSONFieldValue(payload json.RawMessage, sourcePath string) (any, bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false, errors.NewParseError("json", "openai-compatible model field "+sourcePath, "model record must be valid JSON", err)
	}
	for _, segment := range strings.Split(sourcePath, ".") {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		value, ok = object[segment]
		if !ok || value == nil {
			return nil, false, nil
		}
	}
	switch value.(type) {
	case string, bool, json.Number, []any:
		return value, true, nil
	default:
		return nil, false, &errors.ValidationError{Field: sourcePath, Value: value, Message: "mapped source field must be a scalar or scalar list"}
	}
}

func (c *Client) applyMappedField(model *catalogs.Model, mapping catalogs.FieldMapping, sourceValue any) error {
	if strings.HasPrefix(mapping.To, "pricing.") {
		return applyMappedPricing(model, mapping, sourceValue)
	}
	if strings.HasPrefix(mapping.To, "limits.") {
		c.applyMappedLimit(model, mapping.To, sourceValue)
		return nil
	}
	if strings.HasPrefix(mapping.To, "features.") {
		return c.applyMappedFeature(model, mapping, sourceValue)
	}
	if strings.HasPrefix(mapping.To, "extensions.") {
		applyMappedExtension(model, mapping.To, sourceValue)
		return nil
	}
	switch mapping.To {
	case "name":
		model.Name = c.toString(sourceValue)
	case "description":
		model.Description = c.toString(sourceValue)

	case fieldMetadataTags:
		if model.Metadata == nil {
			model.Metadata = &catalogs.ModelMetadata{}
		}
		model.Metadata.Tags = c.toModelTags(sourceValue)
	case "lifecycle":
		value := c.toString(sourceValue)
		if mapped, found := mapping.Values[value]; found {
			value = mapped
		}
		model.Status = catalogs.ModelStatus(value)
	}
	return nil
}

func (c *Client) applyMappedLimit(model *catalogs.Model, target string, sourceValue any) {
	if model.Limits == nil {
		model.Limits = &catalogs.ModelLimits{}
	}
	value := c.toInt64(sourceValue)
	switch target {
	case "limits.context_window":
		model.Limits.ContextWindow = value
	case "limits.input_tokens":
		model.Limits.InputTokens = value
	case "limits.output_tokens":
		model.Limits.OutputTokens = value
	}
}

func (c *Client) applyMappedFeature(model *catalogs.Model, mapping catalogs.FieldMapping, sourceValue any) error {
	if mapping.To == "features.modalities.input" || mapping.To == "features.modalities.output" {
		return applyMappedModalities(model, mapping, sourceValue)
	}
	value, ok := mappedBool(sourceValue)
	if !ok {
		return &errors.ValidationError{Field: mapping.From, Value: sourceValue, Message: "must contain a boolean capability value"}
	}
	features := ensureModelFeatures(model)
	switch mapping.To {
	case "features.tools":
		features.Tools, features.ToolCalls, features.ToolChoice = value, value, value
	case "features.tool_choice":
		features.ToolChoice = value
	case "features.reasoning":
		features.Reasoning = value
	case "features.structured_outputs":
		features.StructuredOutputs = value
	case "features.format_response":
		features.FormatResponse = value
	case "features.streaming":
		features.Streaming = value
	case "features.modalities.image_input":
		if value {
			features.Modalities.Input = appendUniqueModality(features.Modalities.Input, catalogs.ModelModalityImage)
		}
	}
	return nil
}

func applyMappedModalities(model *catalogs.Model, mapping catalogs.FieldMapping, sourceValue any) error {
	modalities, err := mappedModalities(sourceValue)
	if err != nil {
		return &errors.ValidationError{Field: mapping.From, Value: sourceValue, Message: err.Error()}
	}
	features := ensureModelFeatures(model)
	if mapping.To == "features.modalities.input" {
		features.Modalities.Input = modalities
	} else {
		features.Modalities.Output = modalities
	}
	return nil
}

func applyMappedExtension(model *catalogs.Model, target string, sourceValue any) {
	parts := strings.Split(target, ".")
	if model.Extensions == nil {
		model.Extensions = make(catalogs.SourceExtensions)
	}
	extension := model.Extensions[parts[1]]
	if extension.Fields == nil {
		extension.Fields = make(map[string]any)
	}
	extension.Fields[parts[2]] = catalogs.NormalizeExtensionFields(map[string]any{"value": sourceValue})["value"]
	model.Extensions[parts[1]] = extension
}

// toInt64 converts various types to int64 with nil-safe handling.
func (c *Client) toInt64(v any) int64 {
	switch val := v.(type) {
	case *int64:
		if val != nil {
			return *val
		}
	case int64:
		return val
	case *int:
		if val != nil {
			return int64(*val)
		}
	case int:
		return int64(val)
	case *int32:
		if val != nil {
			return int64(*val)
		}
	case int32:
		return int64(val)
	case *float64:
		if val != nil {
			return int64(*val)
		}
	case float64:
		return int64(val)
	case string:
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			return parsed
		}
	case json.Number:
		if parsed, err := val.Int64(); err == nil {
			return parsed
		}
	}
	return 0
}

// toString converts various types to string with nil-safe handling.
func (c *Client) toString(v any) string {
	switch val := v.(type) {
	case *string:
		if val != nil {
			return *val
		}
	case string:
		return val
	case *int64:
		if val != nil {
			return strconv.FormatInt(*val, 10)
		}
	case int64:
		return strconv.FormatInt(val, 10)
	case int:
		return strconv.Itoa(val)
	case *bool:
		if val != nil {
			return strconv.FormatBool(*val)
		}
	case bool:
		return strconv.FormatBool(val)
	}
	return ""
}

// toModelTags converts provider tag strings to catalog model tags.
func (c *Client) toModelTags(v any) []catalogs.ModelTag {
	switch val := v.(type) {
	case []string:
		tags := make([]catalogs.ModelTag, 0, len(val))
		for _, tag := range val {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			tags = append(tags, catalogs.ModelTag(tag))
		}
		return tags
	case []catalogs.ModelTag:
		return append([]catalogs.ModelTag(nil), val...)
	case []any:
		tags := make([]catalogs.ModelTag, 0, len(val))
		for _, item := range val {
			text, ok := item.(string)
			if !ok || strings.TrimSpace(text) == "" {
				continue
			}
			tags = append(tags, catalogs.ModelTag(text))
		}
		return tags
	}
	return nil
}

func mappedModalities(value any) ([]catalogs.ModelModality, error) {
	var raw []string
	switch typed := value.(type) {
	case []string:
		raw = typed
	case []any:
		raw = make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("modality list must contain only strings")
			}
			raw = append(raw, text)
		}
	default:
		return nil, fmt.Errorf("modalities must be a string list")
	}
	return convertProviderModalities(raw), nil
}

// extractAuthors extracts authors using configured author mappings.
func (c *Client) extractAuthors(modelID, ownedBy string) []catalogs.Author {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil {
		return []catalogs.Author{{ID: catalogs.AuthorIDOpenAI, Name: "OpenAI"}}
	}

	// Use configured author mapping if available
	if provider.Catalog != nil && provider.Catalog.Sources[0].Endpoint.AuthorMapping != nil {
		mapping := provider.Catalog.Sources[0].Endpoint.AuthorMapping

		// Get the field value to map from
		var fieldValue string
		switch mapping.Field {
		case fieldOwnedBy:
			fieldValue = ownedBy
		case fieldID:
			fieldValue = modelID
		default:
			fieldValue = ownedBy // default to owned_by
		}

		// Apply normalization if configured
		if authorID, exists := resolveMappedAuthor(fieldValue, mapping.Normalized); exists {
			return []catalogs.Author{{ID: authorID, Name: authorID.String()}}
		}

		// Explicit mappings are an allowlist. Unmatched model IDs are unknown,
		// while owned_by often identifies the serving aggregator rather than the
		// model author; neither may invent an author cross-reference.
		return nil
	}

	// Fallback to provider's configured authors or infer from owned_by
	if provider.Catalog != nil && len(provider.Catalog.Sources[0].Authors) > 0 {
		authors := make([]catalogs.Author, len(provider.Catalog.Sources[0].Authors))
		for i, authorID := range provider.Catalog.Sources[0].Authors {
			authors[i] = catalogs.Author{ID: authorID, Name: authorID.String()}
		}
		return authors
	}

	// Final fallback - infer from owned_by
	if authorID := catalogs.ParseAuthorID(ownedBy); authorID != catalogs.AuthorIDUnknown {
		return []catalogs.Author{{ID: authorID, Name: authorID.String()}}
	}

	return []catalogs.Author{{ID: catalogs.AuthorIDOpenAI, Name: "OpenAI"}}
}

func resolveMappedAuthor(value string, normalized map[string]catalogs.AuthorID) (catalogs.AuthorID, bool) {
	if value == "" || len(normalized) == 0 {
		return catalogs.AuthorIDUnknown, false
	}

	if authorID, exists := normalized[value]; exists {
		return authorID, true
	}

	valueLower := strings.ToLower(value)
	patterns := make([]string, 0, len(normalized))
	for key := range normalized {
		if strings.ToLower(key) == valueLower {
			return normalized[key], true
		}
		if strings.ContainsAny(key, "*?[") {
			patterns = append(patterns, key)
		}
	}

	sort.Slice(patterns, func(i, j int) bool {
		if len(patterns[i]) == len(patterns[j]) {
			return patterns[i] < patterns[j]
		}
		return len(patterns[i]) > len(patterns[j])
	})

	for _, pattern := range patterns {
		matched, err := path.Match(strings.ToLower(pattern), valueLower)
		if err == nil && matched {
			return normalized[pattern], true
		}
	}

	return catalogs.AuthorIDUnknown, false
}

// applyFeatureRules applies configured feature rules to infer model features.
func (c *Client) applyFeatureRules(apiModel Model) *catalogs.ModelFeatures {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	// Start with base OpenAI features
	features := &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
		Temperature: true,
		TopP:        true,
		MaxTokens:   true,
		Stop:        true,
		Streaming:   true,
	}

	if provider == nil || provider.Catalog == nil || provider.Catalog.Sources[0].Endpoint.FeatureRules == nil {
		return features
	}

	// Apply configured feature rules
	for _, rule := range provider.Catalog.Sources[0].Endpoint.FeatureRules {
		c.applyFeatureRule(features, apiModel, rule)
	}

	return features
}

// applyFeatureRule applies a single feature rule to the model features.
func (c *Client) applyFeatureRule(features *catalogs.ModelFeatures, apiModel Model, rule catalogs.FeatureRule) {
	value, found, err := fieldMappingSourceValue(rule.Field, apiModel)
	if err != nil || !found {
		return
	}
	fieldValues := mappedStrings(value)

	// Check if any of the "contains" values match
	matches := false
	for _, fieldValue := range fieldValues {
		fieldLower := strings.ToLower(fieldValue)
		for _, contains := range rule.Contains {
			if strings.Contains(fieldLower, strings.ToLower(contains)) {
				matches = true
				break
			}
		}
		if matches {
			break
		}
	}

	if !matches {
		return
	}

	// Apply the feature value
	switch rule.Feature {
	case "tools":
		features.Tools = rule.Value
		features.ToolCalls = rule.Value
		features.ToolChoice = rule.Value
	case "tool_choice":
		features.ToolChoice = rule.Value
	case "structured_outputs":
		features.StructuredOutputs = rule.Value
	case featureReasoning:
		features.Reasoning = rule.Value
	case "top_k":
		features.TopK = rule.Value
	case "format_response":
		features.FormatResponse = rule.Value
	}
}

func mappedStrings(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []string:
		return typed
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
		return values
	default:
		return nil
	}
}

// validateFieldMappings validates that all configured field mappings use valid paths.
func (c *Client) validateFieldMappings(provider *catalogs.Provider) error {
	if provider == nil || provider.Catalog == nil || provider.Catalog.Sources[0].Endpoint.FieldMappings == nil {
		return nil
	}

	if err := catalogs.ValidateProviderFieldMappings(provider.Catalog.Sources[0].Endpoint.FieldMappings); err != nil {
		return err
	}
	return nil
}
