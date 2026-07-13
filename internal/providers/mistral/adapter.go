// Package mistral owns Mistral-specific model catalog enrichment.
package mistral

import (
	"encoding/json"
	"slices"

	"github.com/agentstation/starmap/internal/providers/openai"
	"github.com/agentstation/starmap/pkg/catalogs"
)

type capabilities struct {
	CompletionChat  bool `json:"completion_chat"`
	CompletionFIM   bool `json:"completion_fim"`
	FunctionCalling bool `json:"function_calling"`
	FineTuning      bool `json:"fine_tuning"`
	Vision          bool `json:"vision"`
	Classification  bool `json:"classification"`
}

type sourceModel struct {
	Capabilities *capabilities `json:"capabilities"`
}

// Options returns the Mistral adapter configuration for the shared client.
func Options() []openai.Option { return []openai.Option{openai.WithModelEnricher(enrich)} }

func enrich(model *catalogs.Model, source openai.Model) error {
	var wire sourceModel
	if len(source.RawJSON) > 0 {
		if err := json.Unmarshal(source.RawJSON, &wire); err != nil {
			return err
		}
	}
	if wire.Capabilities != nil {
		features := ensureFeatures(model)
		features.Tools = wire.Capabilities.FunctionCalling
		features.ToolCalls = wire.Capabilities.FunctionCalling
		features.ToolChoice = wire.Capabilities.FunctionCalling
		features.Streaming = wire.Capabilities.CompletionChat
		if wire.Capabilities.Vision && !slices.Contains(features.Modalities.Input, catalogs.ModelModalityImage) {
			features.Modalities.Input = append(features.Modalities.Input, catalogs.ModelModalityImage)
		}
		if model.Metadata == nil {
			model.Metadata = &catalogs.ModelMetadata{}
		}
		for _, tag := range []struct {
			value   catalogs.ModelTag
			enabled bool
		}{
			{catalogs.ModelTagChat, wire.Capabilities.CompletionChat},
			{catalogs.ModelTagCoding, wire.Capabilities.CompletionFIM},
			{catalogs.ModelTagFunctionCalling, wire.Capabilities.FunctionCalling},
			{catalogs.ModelTagVision, wire.Capabilities.Vision},
		} {
			if tag.enabled && !slices.Contains(model.Metadata.Tags, tag.value) {
				model.Metadata.Tags = append(model.Metadata.Tags, tag.value)
			}
		}
		mergeExtension(model, map[string]any{
			"completion_fim": wire.Capabilities.CompletionFIM,
			"fine_tuning":    wire.Capabilities.FineTuning,
			"classification": wire.Capabilities.Classification,
		})
	}
	return nil
}

func ensureFeatures(model *catalogs.Model) *catalogs.ModelFeatures {
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

func mergeExtension(model *catalogs.Model, fields map[string]any) {
	if model.Extensions == nil {
		model.Extensions = catalogs.SourceExtensions{}
	}
	ext := model.Extensions[catalogs.ProviderIDMistralAI.String()]
	if ext.Fields == nil {
		ext.Fields = map[string]any{}
	}
	for key, value := range catalogs.NormalizeExtensionFields(fields) {
		ext.Fields[key] = value
	}
	model.Extensions[catalogs.ProviderIDMistralAI.String()] = ext
}
