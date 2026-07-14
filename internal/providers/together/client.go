// Package together implements Together AI's native model inventories.
package together

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
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

const (
	inventoryDedicated    = "dedicated"
	inventoryServerless   = "serverless"
	invalidInventoryError = "source option inventory must be serverless or dedicated"
)

type model struct {
	ID            string                           `json:"id"`
	Object        string                           `json:"object"`
	Created       int64                            `json:"created"`
	Type          string                           `json:"type"`
	DisplayName   string                           `json:"display_name"`
	Organization  string                           `json:"organization"`
	Link          string                           `json:"link"`
	License       string                           `json:"license"`
	ContextLength int64                            `json:"context_length"`
	Pricing       pricing                          `json:"pricing"`
	UnknownFields []sourcepayload.UnknownJSONField `json:"-"`
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

type pricing struct {
	Base        *float64 `json:"base,omitempty"`
	Finetune    *float64 `json:"finetune,omitempty"`
	Hourly      *float64 `json:"hourly,omitempty"`
	Input       *float64 `json:"input,omitempty"`
	Output      *float64 `json:"output,omitempty"`
	CachedInput *float64 `json:"cached_input,omitempty"`
}

// Client retrieves Together's serverless and dedicated public inventories.
type Client struct {
	mu        sync.RWMutex
	provider  *catalogs.Provider
	endpoint  string
	transport *transport.Client
	inventory string
}

// NewClient creates a Together inventory client.
func NewClient(source acquisition.Source) *Client {
	provider := source.Provider()
	inventory, _ := source.Option("inventory")
	return &Client{provider: &provider, endpoint: source.EndpointURL(), transport: transport.New(source.Auth()), inventory: inventory}
}

// CaptureURL returns the exact configured Together inventory request URL.
func (c *Client) CaptureURL() (string, error) {
	c.mu.RLock()
	endpoint, inventory := c.endpoint, c.inventory
	c.mu.RUnlock()
	if inventory != inventoryServerless && inventory != inventoryDedicated {
		return "", &errors.ConfigError{Component: string(catalogs.ProviderIDTogetherAI), Message: invalidInventoryError}
	}
	return inventoryURL(endpoint, inventory == inventoryDedicated)
}

// ListModels retrieves exactly one configured Together inventory.
func (c *Client) ListModels(ctx context.Context) ([]catalogs.Model, error) {
	c.mu.RLock()
	provider := c.provider
	transportClient := c.transport
	endpoint := c.endpoint
	inventory := c.inventory
	c.mu.RUnlock()
	if provider == nil {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDTogetherAI), Message: "provider not configured"}
	}
	if endpoint == "" {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDTogetherAI), Message: "catalog endpoint is required"}
	}
	if inventory != inventoryServerless && inventory != inventoryDedicated {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDTogetherAI), Message: invalidInventoryError}
	}
	sourceModels, err := fetchInventory(ctx, transportClient, endpoint, inventory == inventoryDedicated)
	if err != nil {
		return nil, err
	}
	return normalizeModels(sourceModels, inventory)
}

// DecodeModels validates and normalizes one captured Together inventory using
// the same source-specific filtering and conversion path as live acquisition.
func (c *Client) DecodeModels(payload []byte) ([]catalogs.Model, error) {
	c.mu.RLock()
	inventory := c.inventory
	c.mu.RUnlock()
	if inventory != inventoryServerless && inventory != inventoryDedicated {
		return nil, &errors.ConfigError{Component: string(catalogs.ProviderIDTogetherAI), Message: invalidInventoryError}
	}
	if err := sourcepayload.ValidateJSON(payload); err != nil {
		return nil, err
	}
	var sourceModels []model
	if err := json.Unmarshal(payload, &sourceModels); err != nil {
		return nil, errors.WrapParse("json", "Together models response fixture", err)
	}
	if sourceModels == nil {
		return nil, errors.NewParseError("json", "Together models response", "required model array is null", nil)
	}
	return normalizeModels(sourceModels, inventory)
}

func normalizeModels(sourceModels []model, inventory string) ([]catalogs.Model, error) {
	models := make(map[string]catalogs.Model)
	for _, source := range sourceModels {
		converted, ok, err := convertModel(source, inventory)
		if err != nil {
			return nil, err
		}
		if ok {
			if _, found := models[converted.ID]; found {
				return nil, &errors.ConflictError{Resource: "together model", Actual: converted.ID, Message: "duplicate model identity"}
			}
			models[converted.ID] = converted
		}
	}
	ids := make([]string, 0, len(models))
	for id := range models {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	result := make([]catalogs.Model, 0, len(ids))
	for _, id := range ids {
		result = append(result, models[id])
	}
	return result, nil
}

func fetchInventory(ctx context.Context, client *transport.Client, endpoint string, dedicated bool) ([]model, error) {
	requestURL, err := inventoryURL(endpoint, dedicated)
	if err != nil {
		return nil, err
	}
	response, err := client.Get(ctx, requestURL)
	if err != nil {
		return nil, &errors.APIError{Provider: string(catalogs.ProviderIDTogetherAI), Endpoint: requestURL, Message: "request failed", Err: err}
	}
	var result []model
	if err := transport.DecodeResponse(response, &result); err != nil {
		return nil, &errors.APIError{Provider: string(catalogs.ProviderIDTogetherAI), Endpoint: requestURL, StatusCode: response.StatusCode, Message: "failed to decode response", Err: err}
	}
	if result == nil {
		return nil, errors.NewParseError("json", "Together models response", "required model array is null", nil)
	}
	return result, nil
}

func inventoryURL(endpoint string, dedicated bool) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", errors.WrapParse("url", "Together models endpoint", err)
	}
	query := parsed.Query()
	if dedicated {
		query.Set(inventoryDedicated, "true")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func convertModel(source model, deploymentType string) (catalogs.Model, bool, error) {
	if strings.TrimSpace(source.ID) == "" || strings.TrimSpace(source.Type) == "" {
		return catalogs.Model{}, false, &errors.ValidationError{Field: "together.model", Value: source.ID, Message: "id and type are required"}
	}
	if source.ContextLength < 0 {
		return catalogs.Model{}, false, &errors.ValidationError{Field: "together.model.context_length", Value: source.ContextLength, Message: "must not be negative"}
	}
	if err := source.Pricing.validate(); err != nil {
		return catalogs.Model{}, false, err
	}
	authorID, found := togetherAuthor(source.Organization, source.ID)
	if !found {
		return catalogs.Model{}, false, nil
	}
	name := source.DisplayName
	if name == "" {
		name = source.ID
	}
	result := catalogs.Model{
		ID: source.ID, Name: name, Authors: []catalogs.Author{{ID: authorID, Name: authorID.String()}},
		Status: catalogs.ModelStatusActive, Limits: &catalogs.ModelLimits{ContextWindow: source.ContextLength},
		Features:           &catalogs.ModelFeatures{Modalities: catalogs.ModelModalities{Input: []catalogs.ModelModality{catalogs.ModelModalityText}, Output: []catalogs.ModelModality{catalogs.ModelModalityText}}},
		OfferingDeployment: catalogs.ProviderDeployment{Type: deploymentType},
	}
	result.InvocationAPIs = invocationAPIs(source.Type)
	if slices.Contains(result.InvocationAPIs, catalogs.InvocationAPIEmbeddings) {
		result.Features.Modalities.Output = []catalogs.ModelModality{catalogs.ModelModalityEmbedding}
	}
	if slices.Contains(result.InvocationAPIs, catalogs.InvocationAPIImageGeneration) {
		result.Features.Modalities.Output = []catalogs.ModelModality{catalogs.ModelModalityImage}
	}
	if source.Created > 0 {
		created := utc.New(time.Unix(source.Created, 0))
		result.CreatedAt = created
		result.UpdatedAt = created
	}
	pricing, err := source.Pricing.catalogPricing()
	if err != nil {
		return catalogs.Model{}, false, err
	}
	result.Pricing = pricing
	deployment := catalogs.ProviderDeployment{Type: deploymentType}
	result.Modes = map[string]catalogs.ModelMode{deploymentType: {Pricing: result.Pricing, Deployment: &deployment}}
	fields := map[string]any{"organization": source.Organization, "type": source.Type, "license": source.License, "link": source.Link}
	if source.Pricing.Base != nil {
		fields["base_price"] = *source.Pricing.Base
	}
	if source.Pricing.Finetune != nil {
		fields["finetune_price"] = *source.Pricing.Finetune
	}
	if source.Pricing.Hourly != nil {
		fields["hourly_price"] = *source.Pricing.Hourly
	}
	if len(source.UnknownFields) > 0 {
		fields["unknown_fields"] = source.UnknownFields
	}
	result.Extensions = catalogs.SourceExtensions{string(catalogs.ProviderIDTogetherAI): {Fields: catalogs.NormalizeExtensionFields(fields)}}
	return result, true, nil
}

func (p pricing) validate() error {
	for name, value := range map[string]*float64{"base": p.Base, "finetune": p.Finetune, "hourly": p.Hourly, "input": p.Input, "output": p.Output, "cached_input": p.CachedInput} {
		if value != nil && (*value < 0 || math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return &errors.ValidationError{Field: "together.model.pricing." + name, Value: *value, Message: "must be finite and non-negative"}
		}
	}
	return nil
}

func (p pricing) catalogPricing() (*catalogs.ModelPricing, error) {
	if p.Input == nil && p.Output == nil && p.CachedInput == nil {
		return nil, nil
	}
	tokens := &catalogs.ModelTokenPricing{}
	if p.Input != nil {
		cost, err := catalogs.NormalizeProviderTokenPrice(*p.Input, catalogs.ProviderNormalizationUnitPerMillionTokens)
		if err != nil {
			return nil, err
		}
		tokens.Input = &cost
	}
	if p.Output != nil {
		cost, err := catalogs.NormalizeProviderTokenPrice(*p.Output, catalogs.ProviderNormalizationUnitPerMillionTokens)
		if err != nil {
			return nil, err
		}
		tokens.Output = &cost
	}
	if p.CachedInput != nil {
		cost, err := catalogs.NormalizeProviderTokenPrice(*p.CachedInput, catalogs.ProviderNormalizationUnitPerMillionTokens)
		if err != nil {
			return nil, err
		}
		tokens.Cache = &catalogs.ModelTokenCachePricing{Read: &cost}
	}
	return &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: tokens}, nil
}

func invocationAPIs(modelType string) []catalogs.InvocationAPI {
	switch strings.ToLower(modelType) {
	case "chat", "language", "code":
		return []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}
	case "embedding":
		return []catalogs.InvocationAPI{catalogs.InvocationAPIEmbeddings}
	case "rerank":
		return []catalogs.InvocationAPI{catalogs.InvocationAPIRerank}
	case "image":
		return []catalogs.InvocationAPI{catalogs.InvocationAPIImageGeneration}
	default:
		return []catalogs.InvocationAPI{}
	}
}

func togetherAuthor(organization, modelID string) (catalogs.AuthorID, bool) {
	value := strings.ToLower(strings.TrimSpace(organization))
	if value == "" {
		value = strings.ToLower(strings.SplitN(modelID, "/", 2)[0])
	}
	authors := map[string]catalogs.AuthorID{
		"alibaba": catalogs.AuthorIDAlibabaQwen, "qwen": catalogs.AuthorIDAlibabaQwen,
		"deepseek": catalogs.AuthorIDDeepSeek, "deepseek-ai": catalogs.AuthorIDDeepSeek,
		"google": catalogs.AuthorIDGoogle, "meta": catalogs.AuthorIDMeta, "meta-llama": catalogs.AuthorIDMeta,
		"mistral": catalogs.AuthorIDMistralAI, "mistralai": catalogs.AuthorIDMistralAI,
		"moonshotai": catalogs.AuthorIDMoonshot, "nvidia": catalogs.AuthorIDNVIDIA,
		"minimax": catalogs.AuthorIDMiniMax, "minimaxai": catalogs.AuthorIDMiniMax,
		"openai": catalogs.AuthorIDOpenAI, string(catalogs.ProviderIDTogetherAI): catalogs.AuthorIDTogether,
		"zai-org": catalogs.AuthorIDZhipuAI, "zhipu ai": catalogs.AuthorIDZhipuAI,
	}
	author, found := authors[value]
	return author, found
}
