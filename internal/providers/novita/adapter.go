// Package novita owns Novita's LLM inventory contract and enrichment.
package novita

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentstation/starmap/internal/providers/openai"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

type sourceModel struct {
	ID                   string `json:"id"`
	Object               string `json:"object"`
	Title                string `json:"title"`
	Description          string `json:"description"`
	ContextSize          *int64 `json:"context_size"`
	InputTokenPricePerM  *int64 `json:"input_token_price_per_m"`
	OutputTokenPricePerM *int64 `json:"output_token_price_per_m"`
}

// Options returns the Novita adapter configuration for the shared client.
func Options() []openai.Option {
	return []openai.Option{openai.WithResponseModels(responseModels), openai.WithModelEnricher(enrich)}
}

func responseModels(response openai.Response) ([]openai.Model, error) {
	if response.Data == nil {
		return nil, errors.NewParseError("json", "novita model response", "required data array is missing or null", nil)
	}
	seen := make(map[string]struct{}, len(response.Data))
	for index, model := range response.Data {
		wire, err := decode(model)
		if err != nil {
			return nil, fmt.Errorf("data[%d]: %w", index, err)
		}
		if err := validate(wire); err != nil {
			return nil, fmt.Errorf("data[%d]: %w", index, err)
		}
		if _, exists := seen[wire.ID]; exists {
			return nil, errors.NewParseError("json", "novita model", "duplicate model id", nil)
		}
		seen[wire.ID] = struct{}{}
	}
	return response.Data, nil
}

func decode(model openai.Model) (sourceModel, error) {
	var wire sourceModel
	if err := json.Unmarshal(model.RawJSON, &wire); err != nil {
		return sourceModel{}, err
	}
	return wire, nil
}

func validate(model sourceModel) error {
	if strings.TrimSpace(model.ID) == "" {
		return &errors.ValidationError{Field: "id", Message: "is required"}
	}
	if model.Object != "model" {
		return &errors.ValidationError{Field: "object", Value: model.Object, Message: "must be model"}
	}
	if strings.TrimSpace(model.Title) == "" || strings.TrimSpace(model.Description) == "" {
		return &errors.ValidationError{Field: "title_description", Message: "title and description are required"}
	}
	if model.ContextSize == nil || *model.ContextSize <= 0 {
		return &errors.ValidationError{Field: "context_size", Message: "must be positive"}
	}
	if model.InputTokenPricePerM == nil || model.OutputTokenPricePerM == nil {
		return &errors.ValidationError{Field: "token_price_per_m", Message: "input and output prices are required"}
	}
	if *model.InputTokenPricePerM < 0 || *model.OutputTokenPricePerM < 0 {
		return &errors.ValidationError{Field: "token_price_per_m", Message: "prices must not be negative"}
	}
	return nil
}

func enrich(model *catalogs.Model, source openai.Model) error {
	wire, err := decode(source)
	if err != nil {
		return err
	}
	model.Name, model.Description, model.Status = wire.Title, wire.Description, catalogs.ModelStatusActive
	model.Limits = &catalogs.ModelLimits{ContextWindow: *wire.ContextSize}
	if model.Features == nil {
		model.Features = &catalogs.ModelFeatures{}
	}
	model.Features.Streaming = true
	input, err := catalogs.NormalizeProviderTokenPrice(float64(*wire.InputTokenPricePerM), catalogs.ProviderNormalizationUnitMilliCurrencyPerMillionTokens)
	if err != nil {
		return err
	}
	output, err := catalogs.NormalizeProviderTokenPrice(float64(*wire.OutputTokenPricePerM), catalogs.ProviderNormalizationUnitMilliCurrencyPerMillionTokens)
	if err != nil {
		return err
	}
	pricing := &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: &catalogs.ModelTokenPricing{Input: &input, Output: &output}}
	batch, err := discount(pricing)
	if err != nil {
		return err
	}
	model.Pricing = pricing
	model.Modes = map[string]catalogs.ModelMode{"batch": {Pricing: batch}}
	mergeExtension(model, map[string]any{"input_token_price_per_m_raw": *wire.InputTokenPricePerM, "output_token_price_per_m_raw": *wire.OutputTokenPricePerM})
	return nil
}

func discount(pricing *catalogs.ModelPricing) (*catalogs.ModelPricing, error) {
	result := &catalogs.ModelPricing{Currency: pricing.Currency, Tokens: &catalogs.ModelTokenPricing{}}
	if pricing.Tokens.Input != nil {
		cost, err := catalogs.ScaleProviderTokenPrice(*pricing.Tokens.Input, 0.5)
		if err != nil {
			return nil, err
		}
		result.Tokens.Input = &cost
	}
	if pricing.Tokens.Output != nil {
		cost, err := catalogs.ScaleProviderTokenPrice(*pricing.Tokens.Output, 0.5)
		if err != nil {
			return nil, err
		}
		result.Tokens.Output = &cost
	}
	return result, nil
}

func mergeExtension(model *catalogs.Model, fields map[string]any) {
	if model.Extensions == nil {
		model.Extensions = catalogs.SourceExtensions{}
	}
	ext := model.Extensions[catalogs.ProviderIDNovita.String()]
	if ext.Fields == nil {
		ext.Fields = map[string]any{}
	}
	for key, value := range catalogs.NormalizeExtensionFields(fields) {
		ext.Fields[key] = value
	}
	model.Extensions[catalogs.ProviderIDNovita.String()] = ext
}
