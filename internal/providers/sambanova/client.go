// Package sambanova implements SambaNova Cloud model discovery.
package sambanova

import (
	"context"
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

const defaultModelsURL = "https://api.sambanova.ai/v1/models"

type response struct {
	Object string  `json:"object"`
	Data   []model `json:"data"`
}

type model struct {
	ID                  string                           `json:"id"`
	Object              string                           `json:"object"`
	OwnedBy             string                           `json:"owned_by"`
	ContextLength       int64                            `json:"context_length"`
	MaxCompletionTokens int64                            `json:"max_completion_tokens"`
	Pricing             pricing                          `json:"pricing"`
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

// Client retrieves SambaNova's priced OpenAI-compatible inventory.
type Client struct {
	mu        sync.RWMutex
	provider  *catalogs.Provider
	transport *transport.Client
}

// NewClient creates a SambaNova Cloud client.
func NewClient(provider *catalogs.Provider) *Client {
	return &Client{provider: provider, transport: transport.New(provider)}
}

// IsAPIKeyRequired reports that inventory requires bearer authentication.
func (c *Client) IsAPIKeyRequired() bool { return true }

// HasAPIKey reports whether the SambaNova API key is resolved.
func (c *Client) HasAPIKey() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.provider != nil && c.provider.HasAPIKey()
}

// ListModels retrieves the current one-page SambaNova inventory.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider, client := c.provider, c.transport
	c.mu.RUnlock()
	if provider == nil {
		return nil, &errors.ConfigError{Component: "sambanova", Message: "provider not configured"}
	}
	endpoint := provider.CatalogEndpointURL()
	if endpoint == "" {
		endpoint = defaultModelsURL
	}
	httpResponse, err := client.Get(ctx, endpoint, provider)
	if err != nil {
		return nil, &errors.APIError{Provider: "sambanova", Endpoint: endpoint, Message: "request failed", Err: err}
	}
	var envelope response
	if err := transport.DecodeResponse(httpResponse, &envelope); err != nil {
		return nil, &errors.APIError{Provider: "sambanova", Endpoint: endpoint, StatusCode: httpResponse.StatusCode, Message: "failed to decode response", Err: err}
	}
	if envelope.Data == nil || envelope.Object != "list" {
		return nil, &errors.ValidationError{Field: "sambanova.models", Value: envelope.Object, Message: "requires object=list and non-null data"}
	}
	result := make([]catalogs.Model, 0, len(envelope.Data))
	seen := make(map[string]struct{}, len(envelope.Data))
	for _, source := range envelope.Data {
		converted, err := convertModel(source)
		if err != nil {
			return nil, err
		}
		if _, found := seen[converted.ID]; found {
			return nil, &errors.ConflictError{Resource: "SambaNova model", Actual: converted.ID, Message: "duplicate model ID"}
		}
		seen[converted.ID] = struct{}{}
		result = append(result, converted)
	}
	slices.SortFunc(result, func(left, right catalogs.Model) int { return strings.Compare(left.ID, right.ID) })
	return result, nil
}

func convertModel(source model) (catalogs.Model, error) {
	if strings.TrimSpace(source.ID) == "" || source.Object != "model" || source.ContextLength < 0 || source.MaxCompletionTokens < 0 {
		return catalogs.Model{}, &errors.ValidationError{Field: "sambanova.model", Value: source.ID, Message: "requires id, object=model, and non-negative limits"}
	}
	pricing, err := catalogPricing(source.Pricing)
	if err != nil {
		return catalogs.Model{}, err
	}
	author := authorID(source.ID, source.OwnedBy)
	return catalogs.Model{
		ID: source.ID, Name: source.ID, Authors: []catalogs.Author{{ID: author, Name: author.String()}}, Status: catalogs.ModelStatusActive,
		Limits: &catalogs.ModelLimits{ContextWindow: source.ContextLength, OutputTokens: source.MaxCompletionTokens}, Pricing: pricing,
		Features:       &catalogs.ModelFeatures{Modalities: catalogs.ModelModalities{Input: []catalogs.ModelModality{catalogs.ModelModalityText}, Output: []catalogs.ModelModality{catalogs.ModelModalityText}}},
		InvocationAPIs: []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}, OfferingEndpoint: catalogs.ProviderOfferingEndpoint{Type: catalogs.EndpointTypeOpenAI, BaseURL: "https://api.sambanova.ai/v1"},
		OfferingDeployment: catalogs.ProviderDeployment{Type: "serverless", Tier: "on-demand"},
		Extensions:         catalogs.SourceExtensions{"sambanova": {Fields: catalogs.NormalizeExtensionFields(map[string]any{"owned_by": source.OwnedBy, "unknown_fields": source.UnknownFields})}},
	}, nil
}

func catalogPricing(source pricing) (*catalogs.ModelPricing, error) {
	prompt, err := price(source.Prompt, "prompt")
	if err != nil {
		return nil, err
	}
	completion, err := price(source.Completion, "completion")
	if err != nil {
		return nil, err
	}
	if prompt == nil && completion == nil {
		return nil, nil
	}
	tokens := &catalogs.ModelTokenPricing{}
	if prompt != nil {
		cost, err := catalogs.NormalizeProviderTokenPrice(*prompt, catalogs.ProviderNormalizationUnitPerToken)
		if err != nil {
			return nil, err
		}
		tokens.Input = &cost
	}
	if completion != nil {
		cost, err := catalogs.NormalizeProviderTokenPrice(*completion, catalogs.ProviderNormalizationUnitPerToken)
		if err != nil {
			return nil, err
		}
		tokens.Output = &cost
	}
	return &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: tokens}, nil
}

func price(value, field string) (*float64, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return nil, &errors.ValidationError{Field: "sambanova.model.pricing." + field, Value: value, Message: "must be a finite non-negative decimal string"}
	}
	return &parsed, nil
}

func authorID(modelID, ownedBy string) catalogs.AuthorID {
	value := strings.ToLower(modelID + " " + ownedBy)
	switch {
	case strings.Contains(value, "deepseek"):
		return catalogs.AuthorIDDeepSeek
	case strings.Contains(value, "llama"), strings.Contains(value, "meta"):
		return catalogs.AuthorIDMeta
	case strings.Contains(value, "qwen"):
		return catalogs.AuthorIDAlibabaQwen
	case strings.Contains(value, "mistral"):
		return catalogs.AuthorIDMistralAI
	case strings.Contains(value, "openai"), strings.Contains(value, "gpt-oss"):
		return catalogs.AuthorIDOpenAI
	default:
		return catalogs.AuthorID("sambanova")
	}
}
