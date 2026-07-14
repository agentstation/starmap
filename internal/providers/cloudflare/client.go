// Package cloudflare implements Cloudflare Workers AI model discovery.
package cloudflare

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

const perPage = 50
const maxPages = 32

type response struct {
	Data []model `json:"data"`
}

type model struct {
	ID                  string                           `json:"id"`
	Name                string                           `json:"name"`
	Description         string                           `json:"description"`
	ContextLength       int64                            `json:"context_length"`
	Pricing             pricing                          `json:"pricing"`
	Architecture        architecture                     `json:"architecture"`
	SupportedParameters []string                         `json:"supported_parameters"`
	UnknownFields       []sourcepayload.UnknownJSONField `json:"-"`
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

type pricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
}

type architecture struct {
	Modality         string   `json:"modality"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

// Client retrieves the account-authenticated Workers AI marketplace catalog.
type Client struct {
	mu        sync.RWMutex
	provider  *catalogs.Provider
	endpoint  string
	accountID string
	transport *transport.Client
}

// NewClient creates a Cloudflare Workers AI client.
func NewClient(source acquisition.Source) *Client {
	provider := source.Provider()
	accountID, _ := source.Binding("account")
	return &Client{provider: &provider, endpoint: source.EndpointURL(), accountID: accountID, transport: transport.New(source.Auth())}
}

// ListModels traverses the bounded OpenRouter-format model search response.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider, client, baseURL, accountID := c.provider, c.transport, c.endpoint, c.accountID
	c.mu.RUnlock()
	if provider == nil || accountID == "" {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDCloudflare), Message: "account ID is required"}
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDCloudflare), Message: "catalog endpoint is required"}
	}
	result := make([]catalogs.Model, 0)
	for page := 1; page <= maxPages; page++ {
		endpoint, err := modelsURL(baseURL, accountID, page)
		if err != nil {
			return nil, err
		}
		httpResponse, err := client.Get(ctx, endpoint)
		if err != nil {
			return nil, &errors.APIError{Provider: string(catalogs.ProviderIDCloudflare), Endpoint: endpoint, Message: "request failed", Err: err}
		}
		var envelope response
		if err := transport.DecodeResponse(httpResponse, &envelope); err != nil {
			return nil, &errors.APIError{Provider: string(catalogs.ProviderIDCloudflare), Endpoint: endpoint, StatusCode: httpResponse.StatusCode, Message: "failed to decode response", Err: err}
		}
		if envelope.Data == nil {
			return nil, &errors.ValidationError{Field: "cloudflare.models.data", Message: "required data array is null"}
		}
		for _, source := range envelope.Data {
			converted, err := convertModel(source)
			if err != nil {
				return nil, err
			}
			result = append(result, converted)
		}
		if len(envelope.Data) < perPage {
			slices.SortFunc(result, func(left, right catalogs.Model) int { return strings.Compare(left.ID, right.ID) })
			return result, nil
		}
	}
	return nil, &errors.ValidationError{Field: "cloudflare.models.pages", Value: maxPages, Message: "page limit exceeded"}
}

func modelsURL(baseURL, accountID string, page int) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", &errors.ValidationError{Field: "cloudflare.api_base_url", Value: baseURL, Message: "absolute URL is required"}
	}
	parsed.Path += "/accounts/" + url.PathEscape(accountID) + "/ai/models/search"
	query := parsed.Query()
	query.Set("format", "openrouter")
	query.Set("hide_experimental", "true")
	query.Set("include_deprecated", "true")
	query.Set("page", strconv.Itoa(page))
	query.Set("per_page", strconv.Itoa(perPage))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func convertModel(source model) (catalogs.Model, error) {
	if strings.TrimSpace(source.ID) == "" || source.ContextLength < 0 {
		return catalogs.Model{}, &errors.ValidationError{Field: "cloudflare.model", Value: source.ID, Message: "id is required and context length must not be negative"}
	}
	pricing, err := catalogPricing(source.Pricing)
	if err != nil {
		return catalogs.Model{}, err
	}
	input, output := source.Architecture.InputModalities, source.Architecture.OutputModalities
	if len(input) == 0 && len(output) == 0 {
		parts := strings.Split(source.Architecture.Modality, "->")
		if len(parts) == 2 {
			input, output = []string{parts[0]}, []string{parts[1]}
		}
	}
	apis := invocationAPIs(output)
	name := source.Name
	if name == "" {
		name = source.ID
	}
	result := catalogs.Model{
		ID: source.ID, Name: name, Description: source.Description, Authors: []catalogs.Author{{ID: authorID(source.ID), Name: authorID(source.ID).String()}},
		Status: catalogs.ModelStatusActive, Limits: &catalogs.ModelLimits{ContextWindow: source.ContextLength}, Pricing: pricing,
		Features: &catalogs.ModelFeatures{Modalities: catalogs.ModelModalities{Input: modalities(input), Output: modalities(output)}}, InvocationAPIs: apis,
		Extensions: catalogs.SourceExtensions{"cloudflare": {Fields: catalogs.NormalizeExtensionFields(map[string]any{"supported_parameters": source.SupportedParameters, "unknown_fields": source.UnknownFields})}},
	}
	if len(apis) == 0 {
		result.OfferingAccess = &catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityDiscoverable, APIs: []catalogs.InvocationAPI{}}
	}
	return result, nil
}

func catalogPricing(source pricing) (*catalogs.ModelPricing, error) {
	input, err := parsePrice(source.Prompt, "prompt")
	if err != nil {
		return nil, err
	}
	output, err := parsePrice(source.Completion, "completion")
	if err != nil {
		return nil, err
	}
	if input == nil && output == nil {
		return nil, nil
	}
	tokens := &catalogs.ModelTokenPricing{}
	if input != nil {
		cost, err := catalogs.NormalizeProviderTokenPrice(*input, catalogs.ProviderNormalizationUnitPerToken)
		if err != nil {
			return nil, err
		}
		tokens.Input = &cost
	}
	if output != nil {
		cost, err := catalogs.NormalizeProviderTokenPrice(*output, catalogs.ProviderNormalizationUnitPerToken)
		if err != nil {
			return nil, err
		}
		tokens.Output = &cost
	}
	return &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: tokens}, nil
}

func parsePrice(value, field string) (*float64, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return nil, &errors.ValidationError{Field: "cloudflare.model.pricing." + field, Value: value, Message: "must be a finite non-negative decimal string"}
	}
	return &parsed, nil
}

func invocationAPIs(output []string) []catalogs.InvocationAPI {
	for _, value := range output {
		switch strings.ToLower(value) {
		case "text":
			return []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}
		case "embedding", "embeddings":
			return []catalogs.InvocationAPI{catalogs.InvocationAPIEmbeddings}
		case "image":
			return []catalogs.InvocationAPI{catalogs.InvocationAPIImageGeneration}
		case "audio":
			return []catalogs.InvocationAPI{catalogs.InvocationAPIAudio}
		}
	}
	return []catalogs.InvocationAPI{}
}

func modalities(values []string) []catalogs.ModelModality {
	result := make([]catalogs.ModelModality, 0, len(values))
	for _, value := range values {
		result = append(result, catalogs.ModelModality(strings.ToLower(value)))
	}
	return result
}

func authorID(modelID string) catalogs.AuthorID {
	parts := strings.Split(strings.TrimPrefix(modelID, "@cf/"), "/")
	if len(parts) == 0 {
		return catalogs.AuthorID("cloudflare")
	}
	switch strings.ToLower(parts[0]) {
	case "meta":
		return catalogs.AuthorIDMeta
	case "openai":
		return catalogs.AuthorIDOpenAI
	case "mistral", "mistralai":
		return catalogs.AuthorIDMistralAI
	case "google":
		return catalogs.AuthorIDGoogle
	case "deepseek-ai":
		return catalogs.AuthorIDDeepSeek
	default:
		return catalogs.AuthorID(strings.ToLower(parts[0]))
	}
}
