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

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

const (
	pageSize   = 1000
	maxPages   = 20
	maxRecords = 10_000
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
	endpoint  string
	transport *transport.Client
}

// NewClient creates a Cohere client.
func NewClient(source acquisition.Source) *Client {
	provider := source.Provider()
	return &Client{provider: &provider, endpoint: source.EndpointURL(), transport: transport.New(source.Auth())}
}

// CaptureURL returns the exact first-page URL used for a governed raw
// observation. A non-empty cursor would make the capture incomplete.
func (c *Client) CaptureURL() (string, error) {
	c.mu.RLock()
	baseURL := c.endpoint
	c.mu.RUnlock()
	return coherePageURL(baseURL, "")
}

// ListModels retrieves all bounded public Cohere model pages.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	baseURL := c.endpoint
	transportClient := c.transport
	c.mu.RUnlock()
	if provider == nil {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDCohere), Message: "provider not configured"}
	}
	if baseURL == "" {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDCohere), Message: "catalog endpoint is required"}
	}
	models := make([]catalogs.Model, 0)
	seenModels := make(map[string]struct{})
	recordCount := 0
	seenCursors := make(map[string]struct{})
	cursor := ""
	for page := 1; page <= maxPages; page++ {
		response, err := c.fetchPage(ctx, transportClient, baseURL, cursor)
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
			if _, found := seenModels[converted.ID]; found {
				return nil, &errors.ConflictError{Resource: "cohere model", Actual: converted.ID, Message: "duplicate model identity"}
			}
			seenModels[converted.ID] = struct{}{}
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

// DecodeModels validates and normalizes one complete captured Cohere response
// through the same schema and conversion path as live acquisition.
func (c *Client) DecodeModels(payload []byte) ([]catalogs.Model, error) {
	if err := sourcepayload.ValidateJSON(payload); err != nil {
		return nil, err
	}
	var response modelsResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, errors.WrapParse("json", "Cohere models response fixture", err)
	}
	if response.Models == nil {
		return nil, errors.NewParseError("json", "Cohere models response", "required models array is missing or null", nil)
	}
	if response.NextPageToken != "" {
		return nil, &errors.ValidationError{Field: "cohere.next_page_token", Message: "captured response is not a complete inventory"}
	}
	result := make([]catalogs.Model, 0, len(response.Models))
	seen := make(map[string]struct{}, len(response.Models))
	for _, source := range response.Models {
		if source.Finetuned {
			continue
		}
		source.UnknownFields = append(source.UnknownFields, response.UnknownFields...)
		converted, err := convertModel(source)
		if err != nil {
			return nil, err
		}
		if _, found := seen[converted.ID]; found {
			return nil, &errors.ConflictError{Resource: "cohere model", Actual: converted.ID, Message: "duplicate model identity"}
		}
		seen[converted.ID] = struct{}{}
		result = append(result, converted)
	}
	return result, nil
}

func (c *Client) fetchPage(ctx context.Context, transportClient *transport.Client, baseURL, cursor string) (modelsResponse, error) {
	requestURL, err := coherePageURL(baseURL, cursor)
	if err != nil {
		return modelsResponse{}, err
	}
	response, err := transportClient.Get(ctx, requestURL)
	if err != nil {
		return modelsResponse{}, &errors.APIError{Provider: string(catalogs.ProviderIDCohere), Endpoint: requestURL, Message: "request failed", Err: err}
	}
	var result modelsResponse
	if err := transport.DecodeResponse(response, &result); err != nil {
		return modelsResponse{}, &errors.APIError{Provider: string(catalogs.ProviderIDCohere), Endpoint: requestURL, StatusCode: response.StatusCode, Message: "failed to decode response", Err: err}
	}
	if result.Models == nil {
		return modelsResponse{}, errors.NewParseError("json", "Cohere models response", "required models array is missing or null", nil)
	}
	return result, nil
}

func coherePageURL(baseURL, cursor string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", errors.WrapParse("url", "Cohere models endpoint", err)
	}
	query := parsed.Query()
	query.Set("page_size", fmt.Sprint(pageSize))
	if cursor != "" {
		query.Set("page_token", cursor)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
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
		Metadata: &catalogs.ModelMetadata{Architecture: &catalogs.ModelArchitecture{Tokenizer: catalogs.TokenizerCohere}},
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
	result.Extensions = catalogs.SourceExtensions{string(catalogs.ProviderIDCohere): {Fields: catalogs.NormalizeExtensionFields(fields)}}
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
