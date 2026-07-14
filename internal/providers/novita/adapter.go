// Package novita owns Novita's LLM inventory contract and enrichment.
package novita

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentstation/starmap/internal/connectors/openai"
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
	if _, err := decode(source); err != nil {
		return err
	}
	model.Status = catalogs.ModelStatusActive
	if model.Features == nil {
		model.Features = &catalogs.ModelFeatures{}
	}
	model.Features.Streaming = true
	return nil
}
