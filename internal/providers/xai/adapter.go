// Package xai owns xAI's language-model inventory contract and enrichment.
package xai

import (
	"encoding/json"
	"fmt"

	"github.com/agentstation/starmap/internal/providers/openai"
	"github.com/agentstation/starmap/pkg/catalogs"
)

type sourceModel struct {
	PromptTextTokenPrice                  *int64 `json:"prompt_text_token_price"`
	CachedPromptTextTokenPrice            *int64 `json:"cached_prompt_text_token_price"`
	PromptImageTokenPrice                 *int64 `json:"prompt_image_token_price"`
	CompletionTextTokenPrice              *int64 `json:"completion_text_token_price"`
	SearchPrice                           *int64 `json:"search_price"`
	PromptTextTokenPriceLongContext       *int64 `json:"prompt_text_token_price_long_context"`
	CachedPromptTextTokenPriceLongContext *int64 `json:"cached_prompt_text_token_price_long_context"`
	CompletionTextTokenPriceLongContext   *int64 `json:"completion_text_token_price_long_context"`
	LongContextThreshold                  *int64 `json:"long_context_threshold"`
}

// Options returns the xAI adapter configuration for the shared client.
func Options() []openai.Option {
	return []openai.Option{openai.WithModelEnricher(enrich)}
}

func decode(model openai.Model) (sourceModel, error) {
	var wire sourceModel
	if err := json.Unmarshal(model.RawJSON, &wire); err != nil {
		return sourceModel{}, err
	}
	return wire, nil
}

func validate(model sourceModel) error {
	for _, price := range []struct {
		name  string
		value *int64
	}{
		{"prompt_text_token_price", model.PromptTextTokenPrice}, {"cached_prompt_text_token_price", model.CachedPromptTextTokenPrice},
		{"prompt_image_token_price", model.PromptImageTokenPrice}, {"completion_text_token_price", model.CompletionTextTokenPrice},
		{"search_price", model.SearchPrice}, {"prompt_text_token_price_long_context", model.PromptTextTokenPriceLongContext},
		{"cached_prompt_text_token_price_long_context", model.CachedPromptTextTokenPriceLongContext},
		{"completion_text_token_price_long_context", model.CompletionTextTokenPriceLongContext},
	} {
		if price.value != nil && *price.value < 0 {
			return fmt.Errorf("%s must not be negative", price.name)
		}
	}
	if model.LongContextThreshold != nil && *model.LongContextThreshold < 0 {
		return fmt.Errorf("long_context_threshold must not be negative")
	}
	return nil
}

func enrich(model *catalogs.Model, source openai.Model) error {
	model.Status = catalogs.ModelStatusActive
	wire, err := decode(source)
	if err != nil {
		return err
	}
	if err := validate(wire); err != nil {
		return err
	}
	if err := applyPricing(model, wire); err != nil {
		return err
	}
	if wire.PromptImageTokenPrice != nil {
		mergeExtension(model, map[string]any{"prompt_image_token_price_cents_per_100m": *wire.PromptImageTokenPrice})
	}
	return nil
}

func applyPricing(model *catalogs.Model, source sourceModel) error {
	if source.PromptTextTokenPrice == nil && source.CachedPromptTextTokenPrice == nil && source.CompletionTextTokenPrice == nil && source.SearchPrice == nil {
		return nil
	}
	pricing := &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: &catalogs.ModelTokenPricing{}}
	if source.PromptTextTokenPrice != nil {
		cost, err := tokenCost(*source.PromptTextTokenPrice)
		if err != nil {
			return err
		}
		pricing.Tokens.Input = &cost
	}
	if source.CompletionTextTokenPrice != nil {
		cost, err := tokenCost(*source.CompletionTextTokenPrice)
		if err != nil {
			return err
		}
		pricing.Tokens.Output = &cost
	}
	if source.CachedPromptTextTokenPrice != nil && *source.CachedPromptTextTokenPrice > 0 {
		cost, err := tokenCost(*source.CachedPromptTextTokenPrice)
		if err != nil {
			return err
		}
		pricing.Tokens.Cache = &catalogs.ModelTokenCachePricing{Read: &cost}
	}
	if source.SearchPrice != nil && *source.SearchPrice > 0 {
		value := float64(*source.SearchPrice) / 10_000_000_000
		pricing.Operations = &catalogs.ModelOperationPricing{WebSearch: &value}
	}
	if err := applyLongContext(pricing, source); err != nil {
		return err
	}
	model.Pricing = pricing
	return nil
}

func applyLongContext(pricing *catalogs.ModelPricing, source sourceModel) error {
	if source.LongContextThreshold == nil || *source.LongContextThreshold <= 0 {
		return nil
	}
	tokens := &catalogs.ModelTokenPricing{}
	if source.PromptTextTokenPriceLongContext != nil && *source.PromptTextTokenPriceLongContext > 0 {
		cost, err := tokenCost(*source.PromptTextTokenPriceLongContext)
		if err != nil {
			return err
		}
		tokens.Input = &cost
	} else if source.PromptTextTokenPrice != nil {
		cost, err := tokenCost(*source.PromptTextTokenPrice)
		if err != nil {
			return err
		}
		tokens.Input = &cost
	}
	if source.CompletionTextTokenPriceLongContext != nil && *source.CompletionTextTokenPriceLongContext > 0 {
		cost, err := tokenCost(*source.CompletionTextTokenPriceLongContext)
		if err != nil {
			return err
		}
		tokens.Output = &cost
	} else if source.CompletionTextTokenPrice != nil {
		cost, err := tokenCost(*source.CompletionTextTokenPrice)
		if err != nil {
			return err
		}
		tokens.Output = &cost
	}
	cached := source.CachedPromptTextTokenPriceLongContext
	if cached == nil || *cached == 0 {
		cached = source.CachedPromptTextTokenPrice
	}
	if cached != nil && *cached > 0 {
		cost, err := tokenCost(*cached)
		if err != nil {
			return err
		}
		tokens.Cache = &catalogs.ModelTokenCachePricing{Read: &cost}
	}
	pricing.Tiers = append(pricing.Tiers, catalogs.ModelPricingTier{Name: "long_context", Type: catalogs.ModelPricingTierTypeContext, Size: *source.LongContextThreshold, Tokens: tokens})
	return nil
}

func tokenCost(value int64) (catalogs.ModelTokenCost, error) {
	return catalogs.NormalizeProviderTokenPrice(float64(value), catalogs.ProviderNormalizationUnitCentsPer100MillionTokens)
}

func mergeExtension(model *catalogs.Model, fields map[string]any) {
	if model.Extensions == nil {
		model.Extensions = catalogs.SourceExtensions{}
	}
	ext := model.Extensions[catalogs.ProviderIDXAI.String()]
	if ext.Fields == nil {
		ext.Fields = map[string]any{}
	}
	for key, value := range catalogs.NormalizeExtensionFields(fields) {
		ext.Fields[key] = value
	}
	model.Extensions[catalogs.ProviderIDXAI.String()] = ext
}
