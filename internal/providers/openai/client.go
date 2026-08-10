// Package openai provides a unified, dynamic client for OpenAI-compatible APIs.
// This package replaces the separate openaicompat package and provides configuration-driven
// behavior based on provider YAML configuration.
package openai

import (
	"context"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// Client acquires and normalizes model metadata from an OpenAI-compatible API.
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
	client.transport = transport.New()
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
	c.transport = transport.New()
	return nil
}

// ListModels retrieves all available models using OpenAI-compatible API.
func (c *Client) ListModels(
	ctx context.Context,
	material sources.ProviderCredentialMaterial,
) ([]catalogs.Model, error) {
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
	url, err := provider.BindCatalogEndpoint(material.EndpointBindings())
	if err != nil {
		return nil, err
	}

	// Make the request
	resp, err := c.transport.Get(ctx, url, provider, material)
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

	return models, result.RecordReport.Err("openai-compatible models")
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
	c.applyProviderLimits(model, apiModel)
	c.applyProviderMetadata(model, apiModel)
	c.applyProviderFeatures(model, apiModel)
	normalizeOperationalModalities(model)
	c.applyProviderPricing(model, apiModel)
	c.applyProviderExtensions(model, apiModel)
}

func normalizeOperationalModalities(model *catalogs.Model) {
	if model == nil || model.Metadata == nil {
		return
	}
	for _, tag := range model.Metadata.Tags {
		switch strings.ToLower(string(tag)) {
		case "embed", string(catalogs.ModelTagEmbedding):
			ensureModelFeatures(model).Modalities.Output = []catalogs.ModelModality{
				catalogs.ModelModalityEmbedding,
			}
		case "stt", string(catalogs.ModelTagSpeechToText):
			features := ensureModelFeatures(model)
			features.Modalities.Input = appendUniqueModality(
				features.Modalities.Input,
				catalogs.ModelModalityAudio,
			)
			features.Modalities.Output = []catalogs.ModelModality{
				catalogs.ModelModalityText,
			}
		case "tts", string(catalogs.ModelTagTextToSpeech):
			features := ensureModelFeatures(model)
			features.Modalities.Input = appendUniqueModality(
				features.Modalities.Input,
				catalogs.ModelModalityText,
			)
			features.Modalities.Output = []catalogs.ModelModality{
				catalogs.ModelModalityAudio,
			}
		case "image-gen", string(catalogs.ModelTagTextToImage):
			ensureModelFeatures(model).Modalities.Output = []catalogs.ModelModality{
				catalogs.ModelModalityImage,
			}
		case "video-gen", string(catalogs.ModelTagTextToVideo):
			ensureModelFeatures(model).Modalities.Output = []catalogs.ModelModality{
				catalogs.ModelModalityVideo,
			}
		}
	}
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
	if _, state := model.Limits.Value(catalogs.ModelLimitContextWindow); contextWindow != nil && state != catalogs.ValueKnown {
		model.Limits.Set(catalogs.ModelLimitContextWindow, *contextWindow)
	}
	if _, state := model.Limits.Value(catalogs.ModelLimitInputTokens); apiModel.InputTokenLimit != nil && state != catalogs.ValueKnown {
		model.Limits.Set(catalogs.ModelLimitInputTokens, *apiModel.InputTokenLimit)
	}
	if _, state := model.Limits.Value(catalogs.ModelLimitOutputTokens); outputTokens != nil && state != catalogs.ValueKnown {
		model.Limits.Set(catalogs.ModelLimitOutputTokens, *outputTokens)
	}
}

func (c *Client) applyProviderMetadata(model *catalogs.Model, apiModel Model) {
	if apiModel.Metadata == nil {
		return
	}
	if _, state := model.DescriptionValue(); state != catalogs.ValueKnown && apiModel.Metadata.Description != "" {
		model.SetDescription(apiModel.Metadata.Description)
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
	if !hasProviderFeatureClaims(apiModel) {
		return
	}
	features := ensureModelFeatures(model)
	applyProviderModalities(features, apiModel)
	applyProviderCapabilityFlags(features, apiModel)
	applySupportedFeatures(features, apiModel.SupportedFeatures)
	applySupportedSamplingParameters(features, apiModel.SupportedSamplingParameters)
}

func hasProviderFeatureClaims(apiModel Model) bool {
	return len(apiModel.InputModalities) > 0 ||
		len(apiModel.OutputModalities) > 0 ||
		apiModel.SupportsImageInput != nil ||
		apiModel.SupportsImageIn != nil ||
		apiModel.SupportsVideoIn != nil ||
		apiModel.SupportsTools != nil ||
		apiModel.SupportsReasoning != nil ||
		len(apiModel.SupportedFeatures) > 0 ||
		len(apiModel.SupportedSamplingParameters) > 0
}

func applyProviderModalities(features *catalogs.ModelFeatures, apiModel Model) {
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
}

func applyProviderCapabilityFlags(features *catalogs.ModelFeatures, apiModel Model) {
	if apiModel.SupportsTools != nil {
		features.SetSupport(catalogs.ModelFeatureTools, *apiModel.SupportsTools)
		features.SetSupport(catalogs.ModelFeatureToolCalls, *apiModel.SupportsTools)
		features.SetSupport(catalogs.ModelFeatureToolChoice, *apiModel.SupportsTools)
	}
	if apiModel.SupportsReasoning != nil {
		features.SetSupport(catalogs.ModelFeatureReasoning, *apiModel.SupportsReasoning)
	}
}

func applySupportedFeatures(features *catalogs.ModelFeatures, supported []string) {
	for _, feature := range supported {
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
}

func applySupportedSamplingParameters(features *catalogs.ModelFeatures, supported []string) {
	for _, parameter := range supported {
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
	applyOpenAICompatiblePricing(model.Pricing, apiModel.Pricing, c.topLevelTokenPriceScale())
	if apiModel.Metadata != nil {
		applyOpenAICompatibleMetadataPricing(model.Pricing, apiModel.Metadata.Pricing)
	}
	if model.Pricing.Tokens.Input == nil && model.Pricing.Tokens.Output == nil &&
		model.Pricing.Tokens.CacheRead == nil && model.Pricing.Tokens.CacheWrite == nil {
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

func applyOpenAICompatiblePricing(pricing *catalogs.ModelPricing, source *ModelPricing, tokenPriceScale float64) {
	if source == nil {
		return
	}
	// OpenAI-compatible pricing blocks do not share a unit contract. Apply the
	// provider-specific scale before storing the canonical USD-per-million
	// value; never infer units from the price magnitude.
	if source.Prompt != nil && pricing.Tokens.Input == nil {
		pricing.Tokens.Input = &catalogs.ModelTokenCost{
			Per1M: normalizeProviderTokenPrice(*source.Prompt * tokenPriceScale),
		}
	}
	if source.Completion != nil && pricing.Tokens.Output == nil {
		pricing.Tokens.Output = &catalogs.ModelTokenCost{
			Per1M: normalizeProviderTokenPrice(*source.Completion * tokenPriceScale),
		}
	}
	if source.InputCacheRead != nil {
		if pricing.Tokens.CacheRead == nil {
			pricing.Tokens.CacheRead = &catalogs.ModelTokenCost{
				Per1M: normalizeProviderTokenPrice(*source.InputCacheRead * tokenPriceScale),
			}
		}
	}
	if source.Request != nil || source.Image != nil {
		ensureOperationPricing(pricing)
		if source.Request != nil && pricing.Operations.Request == nil {
			pricing.Operations.Request = normalizeProviderOperationPrice(source.Request)
		}
		if source.Image != nil && pricing.Operations.ImageGen == nil {
			pricing.Operations.ImageGen = normalizeProviderOperationPrice(source.Image)
		}
	}
}

func (c *Client) topLevelTokenPriceScale() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.provider != nil && c.provider.Catalog != nil &&
		c.provider.Catalog.Endpoint.ProtocolOptions.OpenAI != nil &&
		c.provider.Catalog.Endpoint.ProtocolOptions.OpenAI.TokenPriceUnit == catalogs.ProviderTokenPriceUnitPerToken {
		return 1_000_000
	}
	return 1
}

func applyOpenAICompatibleMetadataPricing(pricing *catalogs.ModelPricing, source *ModelMetadataPricing) {
	if source == nil {
		return
	}
	// DeepInfra's metadata.pricing token fields are reported in USD per 1M
	// tokens by its public /v1/openai/models payload, matching Per1M.
	if source.InputTokens != nil && pricing.Tokens.Input == nil {
		pricing.Tokens.Input = &catalogs.ModelTokenCost{Per1M: normalizeProviderTokenPrice(*source.InputTokens)}
	}
	if source.OutputTokens != nil && pricing.Tokens.Output == nil {
		pricing.Tokens.Output = &catalogs.ModelTokenCost{Per1M: normalizeProviderTokenPrice(*source.OutputTokens)}
	}
	if source.CacheReadTokens != nil {
		if pricing.Tokens.CacheRead == nil {
			pricing.Tokens.CacheRead = &catalogs.ModelTokenCost{Per1M: normalizeProviderTokenPrice(*source.CacheReadTokens)}
		}
	}
	if source.PerImageUnit != nil || source.InputSeconds != nil || source.OutputSeconds != nil {
		ensureOperationPricing(pricing)
		if source.PerImageUnit != nil && pricing.Operations.ImageGen == nil {
			pricing.Operations.ImageGen = normalizeProviderOperationPrice(source.PerImageUnit)
		}
		if source.InputSeconds != nil && pricing.Operations.AudioInput == nil {
			pricing.Operations.AudioInput = normalizeProviderOperationPrice(source.InputSeconds)
		}
		if source.OutputSeconds != nil && pricing.Operations.AudioGen == nil {
			pricing.Operations.AudioGen = normalizeProviderOperationPrice(source.OutputSeconds)
		}
	}
}

// Provider APIs sometimes emit binary-float artifacts around human decimal
// list prices. Normalize only representational noise; no unit conversion or
// source precedence decision happens here.
func normalizeProviderTokenPrice(price float64) float64 {
	const decimals = 1_000_000
	return snapProviderPrice(price, decimals)
}

func normalizeProviderOperationPrice(price *float64) *float64 {
	if price == nil {
		return nil
	}
	const decimals = 1_000_000_000
	normalized := snapProviderPrice(*price, decimals)
	return &normalized
}

func snapProviderPrice(price, decimalScale float64) float64 {
	candidate := math.Round(price*decimalScale) / decimalScale
	tolerance := math.Max(math.Abs(price), math.Abs(candidate)) * 1e-7
	if math.Abs(price-candidate) <= tolerance {
		return candidate
	}
	return price
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
	if slices.Contains(modalities, modality) {
		return modalities
	}
	return append(modalities, modality)
}

func boolValue(value *bool) bool {
	return value != nil && *value
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
