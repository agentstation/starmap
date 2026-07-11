// Package openai provides a unified, dynamic client for OpenAI-compatible APIs.
// This package replaces the separate openaicompat package and provides configuration-driven
// behavior based on provider YAML configuration.
package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

const (
	fieldID           = "id"
	fieldOwnedBy      = "owned_by"
	fieldMetadataTags = "metadata.tags"
)

// Response represents the OpenAI API list models response.
type Response struct {
	Object        string                           `json:"object"`
	Data          []Model                          `json:"data"`
	UnknownFields []sourcepayload.UnknownJSONField `json:"-"`
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
	// Provider-specific fields
	Active             *bool                            `json:"active,omitempty"`               // Groq-specific
	PublicApps         any                              `json:"public_apps,omitempty"`          // Groq-specific
	HuggingFaceID      string                           `json:"hugging_face_id,omitempty"`      // Groq/aggregator-specific
	Pricing            *ModelPricing                    `json:"pricing,omitempty"`              // Provider-specific pricing
	Kind               string                           `json:"kind,omitempty"`                 // Fireworks-specific
	SupportsChat       *bool                            `json:"supports_chat,omitempty"`        // Fireworks-specific
	SupportsTools      *bool                            `json:"supports_tools,omitempty"`       // Fireworks-specific
	SupportsImageInput *bool                            `json:"supports_image_input,omitempty"` // Fireworks-specific
	SupportsImageIn    *bool                            `json:"supports_image_in,omitempty"`    // Moonshot-specific
	SupportsVideoIn    *bool                            `json:"supports_video_in,omitempty"`    // Moonshot-specific
	SupportsReasoning  *bool                            `json:"supports_reasoning,omitempty"`   // Moonshot-specific
	Permission         []ModelPermission                `json:"permission,omitempty"`           // Moonshot/OpenAI permission metadata
	Metadata           *ModelMetadata                   `json:"metadata,omitempty"`
	UnknownFields      []sourcepayload.UnknownJSONField `json:"-"`
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
	mu       sync.RWMutex
}

// NewClient creates a validated dynamic OpenAI-compatible client.
func NewClient(provider *catalogs.Provider) (*Client, error) {
	client := &Client{
		provider: provider,
	}
	if err := client.validateFieldMappings(provider); err != nil {
		return nil, err
	}
	client.transport = transport.New(provider)
	return client, nil
}

// Configure sets the provider for this client.
func (c *Client) Configure(provider *catalogs.Provider) error {
	if err := c.validateFieldMappings(provider); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.provider = provider
	c.transport = transport.New(provider)
	return nil
}

// IsAPIKeyRequired returns true if the client requires an API key.
func (c *Client) IsAPIKeyRequired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider.IsAPIKeyRequired()
}

// HasAPIKey returns true if the client has an API key.
func (c *Client) HasAPIKey() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider.HasAPIKey()
}

// ListModels retrieves all available models using OpenAI-compatible API.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
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
	url := provider.CatalogEndpointURL()
	if url == "" {
		return nil, &errors.ValidationError{
			Field:   "catalog.endpoint.url",
			Message: "endpoint URL not configured",
		}
	}

	// Make the request
	resp, err := c.transport.Get(ctx, url, provider)
	if err != nil {
		return nil, &errors.APIError{
			Provider:   provider.ID.String(),
			StatusCode: 0,
			Message:    "request failed",
			Err:        err,
		}
	}

	// Decode response
	var result Response
	if err := transport.DecodeResponse(resp, &result); err != nil {
		return nil, &errors.APIError{
			Provider:   provider.ID.String(),
			StatusCode: resp.StatusCode,
			Message:    "failed to decode response",
			Err:        err,
		}
	}
	if result.Data == nil {
		return nil, &errors.APIError{
			Provider: provider.ID.String(), StatusCode: resp.StatusCode,
			Message: "models response schema drift",
			Err:     errors.NewParseError("json", "openai-compatible response", "required data array is missing or null", nil),
		}
	}

	// Convert to starmap models
	models := make([]catalogs.Model, 0, len(result.Data))
	for _, m := range result.Data {
		m.UnknownFields = append(m.UnknownFields, result.UnknownFields...)
		model := c.ConvertToModel(m)
		models = append(models, *model)
	}

	return models, nil
}

// ConvertToModel converts an OpenAI model response to a starmap Model using dynamic configuration.
// This method is public for testing purposes.
func (c *Client) ConvertToModel(m Model) *catalogs.Model {
	model := &catalogs.Model{
		ID:          m.ID,
		Name:        m.ID, // Default to ID, may be overridden
		Description: "",
	}

	// Apply dynamic field mappings
	c.applyFieldMappings(model, m)

	// Apply dynamic author extraction
	model.Authors = c.extractAuthors(m.ID, m.OwnedBy)

	// Apply dynamic feature rules
	model.Features = c.applyFeatureRules(m)

	c.applyProviderDefaults(model, m)

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

	return model
}

func (c *Client) applyProviderDefaults(model *catalogs.Model, apiModel Model) {
	if apiModel.Name != "" && model.Name == model.ID {
		model.Name = apiModel.Name
	}
	if apiModel.Created > 0 {
		created := utc.Time{Time: time.Unix(apiModel.Created, 0).UTC()}
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
	c.applyProviderLimits(model, apiModel)
	c.applyProviderMetadata(model, apiModel)
	c.applyProviderFeatures(model, apiModel)
	c.applyProviderPricing(model, apiModel)
	c.applyProviderExtensions(model, apiModel)
}

func (c *Client) applyProviderLimits(model *catalogs.Model, apiModel Model) {
	contextWindow := firstInt64(apiModel.ContextWindow, apiModel.ContextLength, apiModel.MaxModelLen)
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
		case "reasoning", "thinking":
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

func (c *Client) applyProviderPricing(model *catalogs.Model, apiModel Model) {
	if apiModel.Pricing == nil && (apiModel.Metadata == nil || apiModel.Metadata.Pricing == nil) {
		return
	}
	ensureModelPricing(model)
	applyOpenAICompatiblePricing(model.Pricing, apiModel.Pricing)
	if apiModel.Metadata != nil {
		applyOpenAICompatibleMetadataPricing(model.Pricing, apiModel.Metadata.Pricing)
	}
	if model.Pricing.Tokens.Input == nil && model.Pricing.Tokens.Output == nil && model.Pricing.Tokens.Cache == nil {
		model.Pricing.Tokens = nil
	}
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

func applyOpenAICompatiblePricing(pricing *catalogs.ModelPricing, source *ModelPricing) {
	if source == nil {
		return
	}
	// Current OpenAI-compatible providers that expose this top-level block
	// (notably Groq) report token prices in USD per 1M tokens, matching
	// catalogs.ModelTokenCost.Per1M. Provider families with different units
	// need an explicit provider-specific conversion before this mapping.
	if source.Prompt != nil && pricing.Tokens.Input == nil {
		pricing.Tokens.Input = &catalogs.ModelTokenCost{Per1M: *source.Prompt}
	}
	if source.Completion != nil && pricing.Tokens.Output == nil {
		pricing.Tokens.Output = &catalogs.ModelTokenCost{Per1M: *source.Completion}
	}
	if source.InputCacheRead != nil {
		ensureTokenCachePricing(pricing.Tokens)
		if pricing.Tokens.Cache.Read == nil {
			pricing.Tokens.Cache.Read = &catalogs.ModelTokenCost{Per1M: *source.InputCacheRead}
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
}

func applyOpenAICompatibleMetadataPricing(pricing *catalogs.ModelPricing, source *ModelMetadataPricing) {
	if source == nil {
		return
	}
	// DeepInfra's metadata.pricing token fields are reported in USD per 1M
	// tokens by its public /v1/openai/models payload, matching Per1M.
	if source.InputTokens != nil && pricing.Tokens.Input == nil {
		pricing.Tokens.Input = &catalogs.ModelTokenCost{Per1M: *source.InputTokens}
	}
	if source.OutputTokens != nil && pricing.Tokens.Output == nil {
		pricing.Tokens.Output = &catalogs.ModelTokenCost{Per1M: *source.OutputTokens}
	}
	if source.CacheReadTokens != nil {
		ensureTokenCachePricing(pricing.Tokens)
		if pricing.Tokens.Cache.Read == nil {
			pricing.Tokens.Cache.Read = &catalogs.ModelTokenCost{Per1M: *source.CacheReadTokens}
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
	model.Extensions[source] = catalogs.SourceExtension{
		Fields: catalogs.NormalizeExtensionFields(fields),
	}
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
	for _, existing := range modalities {
		if existing == modality {
			return modalities
		}
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

// applyFieldMappings applies configured field mappings using direct path matching.
func (c *Client) applyFieldMappings(model *catalogs.Model, apiModel Model) {
	c.mu.RLock()
	provider := c.provider
	c.mu.RUnlock()

	if provider == nil || provider.Catalog == nil || provider.Catalog.Endpoint.FieldMappings == nil {
		return
	}

	// Apply field mappings using direct path matching
	for _, mapping := range provider.Catalog.Endpoint.FieldMappings {
		c.setFieldByPath(model, mapping.From, mapping.To, apiModel)
	}
}

// setFieldByPath directly sets model fields based on path strings with type-safe conversion.
func (c *Client) setFieldByPath(model *catalogs.Model, fromPath, toPath string, apiModel Model) {
	sourceValue, ok := fieldMappingSourceValue(fromPath, apiModel)
	if !ok || isNilFieldMappingValue(sourceValue) {
		return
	}
	c.applyMappedField(model, toPath, sourceValue)
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

func fieldMappingSourceValue(fromPath string, apiModel Model) (any, bool) {
	switch fromPath {
	case "max_model_len":
		return apiModel.MaxModelLen, true
	case "context_window":
		return apiModel.ContextWindow, true
	case "context_length":
		return apiModel.ContextLength, true
	case "max_completion_tokens":
		return apiModel.MaxCompletionTokens, true
	case "max_output_length":
		return apiModel.MaxOutputLength, true
	case "input_token_limit":
		return apiModel.InputTokenLimit, true
	case "output_token_limit":
		return apiModel.OutputTokenLimit, true
	case "name":
		return apiModel.Name, true
	case "metadata.description":
		if apiModel.Metadata != nil {
			return apiModel.Metadata.Description, true
		}
	case "metadata.context_length":
		if apiModel.Metadata != nil {
			return apiModel.Metadata.ContextLength, true
		}
	case "metadata.max_tokens":
		if apiModel.Metadata != nil {
			return apiModel.Metadata.MaxTokens, true
		}
	case fieldMetadataTags:
		if apiModel.Metadata != nil {
			return apiModel.Metadata.Tags, true
		}
	case fieldID:
		return apiModel.ID, true
	case fieldOwnedBy:
		return apiModel.OwnedBy, true
	case "created":
		return apiModel.Created, true
	}
	return nil, false
}

func (c *Client) applyMappedField(model *catalogs.Model, toPath string, sourceValue any) {
	switch toPath {
	// Limits fields
	case "limits.context_window":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.ContextWindow = c.toInt64(sourceValue)
	case "limits.input_tokens":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.InputTokens = c.toInt64(sourceValue)
	case "limits.output_tokens":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.OutputTokens = c.toInt64(sourceValue)

	// Direct model fields for backward compatibility
	case "context_window":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.ContextWindow = c.toInt64(sourceValue)
	case "max_completion_tokens":
		if model.Limits == nil {
			model.Limits = &catalogs.ModelLimits{}
		}
		model.Limits.OutputTokens = c.toInt64(sourceValue)

	// Core model fields
	case "name":
		model.Name = c.toString(sourceValue)
	case "description":
		model.Description = c.toString(sourceValue)

	case fieldMetadataTags:
		if model.Metadata == nil {
			model.Metadata = &catalogs.ModelMetadata{}
		}
		model.Metadata.Tags = c.toModelTags(sourceValue)

	// Future extensibility - add more paths as needed:
	// case "pricing.input.base":
	//     if model.Pricing == nil { model.Pricing = &catalogs.ModelPricing{} }
	//     if model.Pricing.Input == nil { model.Pricing.Input = &catalogs.ModelTokenPricing{} }
	//     model.Pricing.Input.Base = c.toFloat64(sourceValue)

	default:
		// Unknown destination path - skip silently
		return
	}
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
	}
	return nil
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
	if provider.Catalog != nil && provider.Catalog.Endpoint.AuthorMapping != nil {
		mapping := provider.Catalog.Endpoint.AuthorMapping

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
	if provider.Catalog != nil && len(provider.Catalog.Authors) > 0 {
		authors := make([]catalogs.Author, len(provider.Catalog.Authors))
		for i, authorID := range provider.Catalog.Authors {
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

	if provider == nil || provider.Catalog == nil || provider.Catalog.Endpoint.FeatureRules == nil {
		return features
	}

	// Apply configured feature rules
	for _, rule := range provider.Catalog.Endpoint.FeatureRules {
		c.applyFeatureRule(features, apiModel, rule)
	}

	return features
}

// applyFeatureRule applies a single feature rule to the model features.
func (c *Client) applyFeatureRule(features *catalogs.ModelFeatures, apiModel Model, rule catalogs.FeatureRule) {
	// Get field value to check
	var fieldValues []string
	switch rule.Field {
	case fieldID:
		fieldValues = []string{apiModel.ID}
	case fieldOwnedBy:
		fieldValues = []string{apiModel.OwnedBy}
	case fieldMetadataTags:
		if apiModel.Metadata != nil {
			fieldValues = apiModel.Metadata.Tags
		}
	default:
		return // Unknown field
	}

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
	case "tool_choice":
		features.ToolChoice = rule.Value
	case "structured_outputs":
		features.StructuredOutputs = rule.Value
	case "reasoning":
		features.Reasoning = rule.Value
	case "top_k":
		features.TopK = rule.Value
	case "format_response":
		features.FormatResponse = rule.Value
	}
}

// validateFieldMappings validates that all configured field mappings use valid paths.
func (c *Client) validateFieldMappings(provider *catalogs.Provider) error {
	if provider == nil || provider.Catalog == nil || provider.Catalog.Endpoint.FieldMappings == nil {
		return nil
	}

	for _, mapping := range provider.Catalog.Endpoint.FieldMappings {
		if !c.isValidSourceField(mapping.From) {
			return &errors.ValidationError{
				Field: "field_mappings.from", Value: mapping.From,
				Message: "invalid source field: " + mapping.From,
			}
		}
		if !c.isValidDestinationPath(mapping.To) {
			return &errors.ValidationError{
				Field: "field_mappings.to", Value: mapping.To,
				Message: "invalid destination path: " + mapping.To,
			}
		}
	}
	return nil
}

// isValidSourceField checks if a source field exists in the API model.
func (c *Client) isValidSourceField(field string) bool {
	validFields := map[string]bool{
		"max_model_len":           true,
		"context_window":          true,
		"context_length":          true,
		"max_completion_tokens":   true,
		"max_output_length":       true,
		"input_token_limit":       true,
		"output_token_limit":      true,
		"name":                    true,
		"metadata.description":    true,
		"metadata.context_length": true,
		"metadata.max_tokens":     true,
		fieldMetadataTags:         true,
		fieldID:                   true,
		fieldOwnedBy:              true,
		"created":                 true,
	}
	return validFields[field]
}

// isValidDestinationPath checks if a destination path is valid in the Model struct.
func (c *Client) isValidDestinationPath(path string) bool {
	validPaths := map[string]bool{
		// Limits fields
		"limits.context_window": true,
		"limits.input_tokens":   true,
		"limits.output_tokens":  true,

		// Direct model fields for backward compatibility
		"context_window":        true,
		"max_completion_tokens": true,

		// Core model fields
		"name":            true,
		"description":     true,
		fieldMetadataTags: true,

		// Future paths can be added here as needed:
		// "metadata.release_date":     true,
		// "metadata.open_weights":     true,
		// "features.tools":            true,
		// "features.reasoning":        true,
		// "pricing.input.base":        true,
		// "pricing.output.base":       true,
	}
	return validPaths[path]
}
