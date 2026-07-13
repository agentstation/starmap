// Package huggingface implements Hugging Face Inference Providers inventory.
package huggingface

import (
	"context"
	"encoding/json"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

const defaultModelsURL = "https://router.huggingface.co/v1/models"

type response struct {
	Object string  `json:"object"`
	Data   []model `json:"data"`
}

type model struct {
	ID            string                           `json:"id"`
	Object        string                           `json:"object"`
	Created       int64                            `json:"created"`
	OwnedBy       string                           `json:"owned_by"`
	Architecture  architecture                     `json:"architecture"`
	Providers     []provider                       `json:"providers"`
	UnknownFields []sourcepayload.UnknownJSONField `json:"-"`
}

func (m *model) UnmarshalJSON(data []byte) error {
	type alias model
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "data[]")
	if err != nil {
		return err
	}
	*m = model(decoded)
	m.UnknownFields = unknown
	return nil
}

type architecture struct {
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

type provider struct {
	Provider                 string   `json:"provider"`
	Status                   string   `json:"status"`
	ContextLength            *int64   `json:"context_length,omitempty"`
	Pricing                  *pricing `json:"pricing,omitempty"`
	IsFree                   *bool    `json:"is_free,omitempty"`
	SupportsTools            *bool    `json:"supports_tools,omitempty"`
	SupportsStructuredOutput *bool    `json:"supports_structured_output,omitempty"`
	FirstTokenLatencyMS      *float64 `json:"first_token_latency_ms,omitempty"`
	Throughput               *float64 `json:"throughput,omitempty"`
	IsModelAuthor            bool     `json:"is_model_author"`
}

type pricing struct {
	Input  *float64 `json:"input,omitempty"`
	Output *float64 `json:"output,omitempty"`
}

// Client retrieves the public Hugging Face router inventory.
type Client struct {
	mu        sync.RWMutex
	provider  *catalogs.Provider
	transport *transport.Client
}

// NewClient creates a Hugging Face inventory client.
func NewClient(provider *catalogs.Provider) *Client {
	return &Client{provider: provider, transport: transport.New(provider)}
}

// IsAPIKeyRequired reports whether public inventory authentication is required.
func (c *Client) IsAPIKeyRequired() bool { return false }

// HasAPIKey reports whether an invocation token is resolved.
func (c *Client) HasAPIKey() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider != nil && c.provider.HasAPIKey()
}

// ListModels expands every model/provider pair into an independently routable offering.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	configured := c.provider
	client := c.transport
	c.mu.RUnlock()
	if configured == nil {
		return nil, &errors.ConfigError{Component: "huggingface", Message: "provider not configured"}
	}
	endpoint := configured.CatalogEndpointURL()
	if endpoint == "" {
		endpoint = defaultModelsURL
	}
	httpResponse, err := client.Get(ctx, endpoint, configured)
	if err != nil {
		return nil, &errors.APIError{Provider: "huggingface", Endpoint: endpoint, Message: "request failed", Err: err}
	}
	var envelope response
	if err := transport.DecodeResponse(httpResponse, &envelope); err != nil {
		return nil, &errors.APIError{Provider: "huggingface", Endpoint: endpoint, StatusCode: httpResponse.StatusCode, Message: "failed to decode response", Err: err}
	}
	if envelope.Object != "list" || envelope.Data == nil {
		return nil, &errors.ValidationError{Field: "huggingface.models", Value: envelope.Object, Message: "requires object=list and a non-null data array"}
	}
	observedAt := httpResponse.Header.Get("Date")
	if parsed, parseErr := time.Parse(time.RFC1123, observedAt); parseErr == nil {
		observedAt = parsed.UTC().Format(time.RFC3339)
	} else {
		observedAt = ""
	}
	result := make([]catalogs.Model, 0)
	for _, source := range envelope.Data {
		converted, convertErr := convertModel(source, observedAt)
		if convertErr != nil {
			return nil, convertErr
		}
		result = append(result, converted...)
	}
	slices.SortFunc(result, func(left, right catalogs.Model) int { return strings.Compare(left.ID, right.ID) })
	return result, nil
}

func convertModel(source model, observedAt string) ([]catalogs.Model, error) {
	if strings.TrimSpace(source.ID) == "" || strings.TrimSpace(source.OwnedBy) == "" || source.Providers == nil {
		return nil, &errors.ValidationError{Field: "huggingface.model", Value: source.ID, Message: "id, owned_by, and providers are required"}
	}
	result := make([]catalogs.Model, 0, len(source.Providers))
	for _, upstream := range source.Providers {
		converted, err := convertProvider(source, upstream, observedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
}

func convertProvider(source model, upstream provider, observedAt string) (catalogs.Model, error) {
	providerID := strings.TrimSpace(upstream.Provider)
	if err := validateProvider(upstream, providerID); err != nil {
		return catalogs.Model{}, err
	}
	routeID := source.ID + ":" + providerID
	features := &catalogs.ModelFeatures{Modalities: catalogs.ModelModalities{
		Input: modalities(source.Architecture.InputModalities), Output: modalities(source.Architecture.OutputModalities),
	}}
	availability := catalogs.OfferingAvailabilityAvailable
	apis := []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}
	if upstream.Status == "error" {
		availability = catalogs.OfferingAvailabilityUnavailable
		apis = []catalogs.InvocationAPI{}
	}
	result := catalogs.Model{
		ID: routeID, DefinitionID: catalogs.ModelDefinitionID(source.ID), Name: source.ID,
		Authors: []catalogs.Author{{ID: authorID(source.OwnedBy), Name: source.OwnedBy}},
		Status:  catalogs.ModelStatusActive, Features: features, InvocationAPIs: apis,
		OfferingEndpoint:     catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeOpenAI, BaseURL: "https://router.huggingface.co/v1"},
		OfferingDeployment:   catalogs.ProviderDeployment{Type: "serverless", Tier: "inference-provider"},
		OfferingAvailability: availability,
		AggregatorUpstream:   &catalogs.AggregatorUpstream{ProviderID: catalogs.ProviderID(providerID), ProviderModelID: catalogs.ProviderModelID(source.ID)},
	}
	if source.Created > 0 {
		created := utc.New(time.Unix(source.Created, 0))
		result.CreatedAt, result.UpdatedAt = created, created
	}
	if upstream.ContextLength != nil {
		result.Limits = &catalogs.ModelLimits{ContextWindow: *upstream.ContextLength}
	}
	pricing, err := catalogPricing(upstream.Pricing)
	if err != nil {
		return catalogs.Model{}, err
	}
	result.Pricing = pricing
	result.Extensions = catalogs.SourceExtensions{"huggingface": {Fields: catalogs.NormalizeExtensionFields(providerEvidence(source, upstream, observedAt))}}
	return result, nil
}

func validateProvider(upstream provider, providerID string) error {
	if providerID == "" || (upstream.Status != "live" && upstream.Status != "error") {
		return &errors.ValidationError{Field: "huggingface.provider", Value: providerID, Message: "provider and live/error status are required"}
	}
	if slices.Contains([]string{"auto", "fastest", "cheapest", "preferred"}, providerID) {
		return &errors.ValidationError{Field: "huggingface.provider", Value: providerID, Message: "routing policies are not provider identities"}
	}
	if upstream.ContextLength != nil && *upstream.ContextLength < 0 {
		return &errors.ValidationError{Field: "huggingface.provider.context_length", Value: *upstream.ContextLength, Message: "must not be negative"}
	}
	values := map[string]*float64{"first_token_latency_ms": upstream.FirstTokenLatencyMS, "throughput": upstream.Throughput}
	if upstream.Pricing != nil {
		values["pricing.input"] = upstream.Pricing.Input
		values["pricing.output"] = upstream.Pricing.Output
	}
	for name, value := range values {
		if value != nil && (*value < 0 || math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return &errors.ValidationError{Field: "huggingface.provider." + name, Value: *value, Message: "must be finite and non-negative"}
		}
	}
	return nil
}

func catalogPricing(source *pricing) (*catalogs.ModelPricing, error) {
	if source == nil {
		return nil, nil
	}
	tokens := &catalogs.ModelTokenPricing{}
	if source.Input != nil {
		cost, err := catalogs.NormalizeProviderTokenPrice(*source.Input, catalogs.ProviderNormalizationUnitPerMillionTokens)
		if err != nil {
			return nil, err
		}
		tokens.Input = &cost
	}
	if source.Output != nil {
		cost, err := catalogs.NormalizeProviderTokenPrice(*source.Output, catalogs.ProviderNormalizationUnitPerMillionTokens)
		if err != nil {
			return nil, err
		}
		tokens.Output = &cost
	}
	return &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: tokens}, nil
}

func providerEvidence(source model, upstream provider, observedAt string) map[string]any {
	fields := map[string]any{"status": upstream.Status, "is_model_author": upstream.IsModelAuthor}
	if upstream.IsFree != nil {
		fields["is_free"] = *upstream.IsFree
	}
	if upstream.SupportsTools != nil {
		fields["supports_tools"] = *upstream.SupportsTools
	}
	if upstream.SupportsStructuredOutput != nil {
		fields["supports_structured_output"] = *upstream.SupportsStructuredOutput
	}
	if upstream.FirstTokenLatencyMS != nil {
		fields["first_token_latency_ms"] = *upstream.FirstTokenLatencyMS
	}
	if upstream.Throughput != nil {
		fields["throughput_tokens_per_second"] = *upstream.Throughput
	}
	if observedAt != "" && (upstream.FirstTokenLatencyMS != nil || upstream.Throughput != nil) {
		fields["metrics_observed_at"] = observedAt
	}
	if len(source.UnknownFields) > 0 {
		fields["unknown_fields"] = source.UnknownFields
	}
	return fields
}

func modalities(values []string) []catalogs.ModelModality {
	result := make([]catalogs.ModelModality, 0, len(values))
	for _, value := range values {
		switch value {
		case "text":
			result = append(result, catalogs.ModelModalityText)
		case "image":
			result = append(result, catalogs.ModelModalityImage)
		case "audio":
			result = append(result, catalogs.ModelModalityAudio)
		case "video":
			result = append(result, catalogs.ModelModalityVideo)
		}
	}
	return result
}

func authorID(value string) catalogs.AuthorID {
	switch strings.ToLower(value) {
	case "coherelabs":
		return catalogs.AuthorIDCohere
	case "minimaxai":
		return catalogs.AuthorIDMiniMax
	case "qwen":
		return catalogs.AuthorIDAlibabaQwen
	case "deepseek-ai":
		return catalogs.AuthorIDDeepSeek
	case "meta-llama":
		return catalogs.AuthorIDMeta
	case "moonshotai":
		return catalogs.AuthorID("moonshot-ai")
	case "zai-org":
		return catalogs.AuthorIDZhipuAI
	default:
		return catalogs.AuthorID(strings.ToLower(value))
	}
}
