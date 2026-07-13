// Package cohere implements Cohere's native paginated model inventory.
package cohere

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

const (
	defaultModelsURL = "https://api.cohere.com/v1/models"
	pageSize         = 1000
	maxPages         = 20
	maxRecords       = 10_000
)

type modelsResponse struct {
	Models        []model                          `json:"models"`
	NextPageToken string                           `json:"next_page_token,omitempty"`
	UnknownFields []sourcepayload.UnknownJSONField `json:"-"`
}

func (r *modelsResponse) UnmarshalJSON(data []byte) error {
	type alias modelsResponse
	var decoded alias
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

type model struct {
	Name             string                           `json:"name"`
	IsDeprecated     bool                             `json:"is_deprecated"`
	Endpoints        []string                         `json:"endpoints"`
	Finetuned        bool                             `json:"finetuned"`
	ContextLength    float64                          `json:"context_length"`
	TokenizerURL     string                           `json:"tokenizer_url"`
	DefaultEndpoints []string                         `json:"default_endpoints"`
	Features         []string                         `json:"features"`
	SamplingDefaults map[string]any                   `json:"sampling_defaults"`
	UnknownFields    []sourcepayload.UnknownJSONField `json:"-"`
}

func (m *model) UnmarshalJSON(data []byte) error {
	type alias model
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	unknown, err := sourcepayload.UnknownJSONFields(data, decoded, "models[]")
	if err != nil {
		return err
	}
	*m = model(decoded)
	m.UnknownFields = unknown
	return nil
}

// Client retrieves Cohere model inventory.
type Client struct {
	mu        sync.RWMutex
	provider  *catalogs.Provider
	transport *transport.Client
}

// NewClient creates a Cohere client.
func NewClient(provider *catalogs.Provider) *Client {
	return &Client{provider: provider, transport: transport.New(provider)}
}

// IsAPIKeyRequired reports whether the configured inventory requires a key.
func (c *Client) IsAPIKeyRequired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider != nil && c.provider.IsAPIKeyRequired()
}

// HasAPIKey reports whether the configured provider has a resolved key.
func (c *Client) HasAPIKey() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider != nil && c.provider.HasAPIKey()
}

// ListModels retrieves all bounded public Cohere model pages.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	transportClient := c.transport
	c.mu.RUnlock()
	if provider == nil {
		return nil, &errors.ConfigError{Component: "cohere", Message: "provider not configured"}
	}
	baseURL := provider.CatalogEndpointURL()
	if baseURL == "" {
		baseURL = defaultModelsURL
	}
	models := make([]catalogs.Model, 0)
	recordCount := 0
	seenCursors := make(map[string]struct{})
	cursor := ""
	for page := 1; page <= maxPages; page++ {
		response, err := c.fetchPage(ctx, transportClient, provider, baseURL, cursor)
		if err != nil {
			return nil, err
		}
		recordCount += len(response.Models)
		if recordCount > maxRecords {
			return nil, &errors.ValidationError{Field: "cohere.models", Value: recordCount, Message: "exceeds maximum record count"}
		}
		for _, source := range response.Models {
			if source.Finetuned {
				continue
			}
			source.UnknownFields = append(source.UnknownFields, response.UnknownFields...)
			converted, err := convertModel(source)
			if err != nil {
				return nil, err
			}
			models = append(models, converted)
		}
		if response.NextPageToken == "" {
			return models, nil
		}
		if response.NextPageToken == cursor {
			return nil, &errors.ConflictError{Resource: "cohere pagination cursor", Expected: cursor, Actual: response.NextPageToken, Message: "cursor did not advance"}
		}
		if _, exists := seenCursors[response.NextPageToken]; exists {
			return nil, &errors.ConflictError{Resource: "cohere pagination cursor", Actual: response.NextPageToken, Message: "cursor repeated"}
		}
		seenCursors[response.NextPageToken] = struct{}{}
		cursor = response.NextPageToken
	}
	return nil, &errors.ValidationError{Field: "cohere.pages", Value: maxPages, Message: "source did not terminate within maximum pages"}
}

func (c *Client) fetchPage(ctx context.Context, transportClient *transport.Client, provider *catalogs.Provider, baseURL, cursor string) (modelsResponse, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return modelsResponse{}, errors.WrapParse("url", "Cohere models endpoint", err)
	}
	query := parsed.Query()
	query.Set("page_size", fmt.Sprint(pageSize))
	if cursor != "" {
		query.Set("page_token", cursor)
	}
	parsed.RawQuery = query.Encode()
	response, err := transportClient.Get(ctx, parsed.String(), provider)
	if err != nil {
		return modelsResponse{}, &errors.APIError{Provider: "cohere", Endpoint: parsed.String(), Message: "request failed", Err: err}
	}
	var result modelsResponse
	if err := transport.DecodeResponse(response, &result); err != nil {
		return modelsResponse{}, &errors.APIError{Provider: "cohere", Endpoint: parsed.String(), StatusCode: response.StatusCode, Message: "failed to decode response", Err: err}
	}
	if result.Models == nil {
		return modelsResponse{}, errors.NewParseError("json", "Cohere models response", "required models array is missing or null", nil)
	}
	return result, nil
}

func convertModel(source model) (catalogs.Model, error) {
	if strings.TrimSpace(source.Name) == "" {
		return catalogs.Model{}, &errors.ValidationError{Field: "cohere.model.name", Message: "is required"}
	}
	if source.ContextLength < 0 || math.Trunc(source.ContextLength) != source.ContextLength || source.ContextLength > math.MaxInt64 {
		return catalogs.Model{}, &errors.ValidationError{Field: "cohere.model.context_length", Value: source.ContextLength, Message: "must be a non-negative integer"}
	}
	result := catalogs.Model{
		ID: source.Name, Name: source.Name,
		Authors: []catalogs.Author{{ID: catalogs.AuthorIDCohere, Name: "Cohere"}},
		Status:  catalogs.ModelStatusActive,
		Limits:  &catalogs.ModelLimits{ContextWindow: int64(source.ContextLength)},
		Features: &catalogs.ModelFeatures{Modalities: catalogs.ModelModalities{
			Input: []catalogs.ModelModality{catalogs.ModelModalityText}, Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		}},
		Metadata:         &catalogs.ModelMetadata{Architecture: &catalogs.ModelArchitecture{Tokenizer: catalogs.TokenizerCohere}},
		OfferingEndpoint: catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeCohere, BaseURL: "https://api.cohere.com"},
	}
	if source.IsDeprecated {
		result.Status = catalogs.ModelStatusDeprecated
	}
	result.InvocationAPIs = cohereInvocationAPIs(source.Endpoints)
	if slices.Contains(result.InvocationAPIs, catalogs.InvocationAPIEmbeddings) {
		result.Features.Modalities.Output = []catalogs.ModelModality{catalogs.ModelModalityEmbedding}
		result.Metadata.Tags = append(result.Metadata.Tags, catalogs.ModelTagEmbedding)
	}
	applyCohereFeatures(&result, source)
	fields := map[string]any{
		"endpoints": source.Endpoints, "default_endpoints": source.DefaultEndpoints,
		"features": source.Features, "tokenizer_url": source.TokenizerURL,
	}
	if len(source.SamplingDefaults) > 0 {
		fields["sampling_defaults"] = source.SamplingDefaults
	}
	if len(source.UnknownFields) > 0 {
		fields["unknown_fields"] = source.UnknownFields
	}
	result.Extensions = catalogs.SourceExtensions{"cohere": {Fields: catalogs.NormalizeExtensionFields(fields)}}
	return result, nil
}

func cohereInvocationAPIs(endpoints []string) []catalogs.InvocationAPI {
	apis := make([]catalogs.InvocationAPI, 0, len(endpoints))
	for _, endpoint := range endpoints {
		var api catalogs.InvocationAPI
		switch strings.ToLower(endpoint) {
		case "chat":
			api = catalogs.InvocationAPIChatCompletions
		case "embed":
			api = catalogs.InvocationAPIEmbeddings
		case "rerank":
			api = catalogs.InvocationAPIRerank
		default:
			continue
		}
		if !slices.Contains(apis, api) {
			apis = append(apis, api)
		}
	}
	slices.Sort(apis)
	return apis
}

func applyCohereFeatures(result *catalogs.Model, source model) {
	for _, feature := range source.Features {
		switch strings.ToLower(feature) {
		case "chat-completions", "text-generation", "instruction-following":
			if !slices.Contains(result.Metadata.Tags, catalogs.ModelTagInstruct) {
				result.Metadata.Tags = append(result.Metadata.Tags, catalogs.ModelTagInstruct)
			}
		case "tool-use", "tool-calling":
			result.Features.Tools = true
			result.Features.ToolCalls = true
			result.Features.ToolChoice = true
		}
	}
	if _, ok := source.SamplingDefaults["temperature"]; ok {
		result.Features.Temperature = true
	}
	if _, ok := source.SamplingDefaults["p"]; ok {
		result.Features.TopP = true
	}
	if _, ok := source.SamplingDefaults["k"]; ok {
		result.Features.TopK = true
	}
	if _, ok := source.SamplingDefaults["frequency_penalty"]; ok {
		result.Features.FrequencyPenalty = true
	}
	if _, ok := source.SamplingDefaults["presence_penalty"]; ok {
		result.Features.PresencePenalty = true
	}
}
