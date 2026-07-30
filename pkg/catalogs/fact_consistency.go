package catalogs

import (
	"fmt"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

func validateModelFactConsistency(model Model) error {
	if !model.CreatedAt.IsZero() && !model.UpdatedAt.IsZero() &&
		model.CreatedAt.After(model.UpdatedAt) {
		return modelFactError(
			"timestamps",
			fmt.Sprintf("%s > %s", model.CreatedAt, model.UpdatedAt),
			"created_at must not be later than updated_at",
		)
	}
	if err := validateReasoningFacts(
		model.Features,
		model.Reasoning,
		model.ReasoningTokens,
		"model.features.reasoning",
	); err != nil {
		return err
	}
	if model.Metadata == nil || model.Features == nil {
		return nil
	}
	for _, tag := range model.Metadata.Tags {
		switch strings.ToLower(string(tag)) {
		case "embed", string(ModelTagEmbedding):
			if !slices.Contains(
				model.Features.Modalities.Output,
				ModelModalityEmbedding,
			) {
				return modelFactError(
					"features.modalities.output",
					model.Features.Modalities.Output,
					"embedding models must declare embedding output",
				)
			}
		case "stt", string(ModelTagSpeechToText):
			if !slices.Contains(model.Features.Modalities.Input, ModelModalityAudio) ||
				!slices.Contains(model.Features.Modalities.Output, ModelModalityText) {
				return modelFactError(
					"features.modalities",
					model.Features.Modalities,
					"speech-to-text models must declare audio input and text output",
				)
			}
		case "tts", string(ModelTagTextToSpeech):
			if !slices.Contains(model.Features.Modalities.Input, ModelModalityText) ||
				!slices.Contains(model.Features.Modalities.Output, ModelModalityAudio) {
				return modelFactError(
					"features.modalities",
					model.Features.Modalities,
					"text-to-speech models must declare text input and audio output",
				)
			}
		case "image-gen", string(ModelTagTextToImage):
			if !slices.Contains(model.Features.Modalities.Output, ModelModalityImage) {
				return modelFactError(
					"features.modalities.output",
					model.Features.Modalities.Output,
					"image-generation models must declare image output",
				)
			}
		case "video-gen":
			if !slices.Contains(model.Features.Modalities.Output, ModelModalityVideo) {
				return modelFactError(
					"features.modalities.output",
					model.Features.Modalities.Output,
					"video-generation models must declare video output",
				)
			}
		}
	}
	return nil
}

func validateDefinitionFactConsistency(definition ModelDefinition) error {
	if !definition.CreatedAt.IsZero() && !definition.UpdatedAt.IsZero() &&
		definition.CreatedAt.After(definition.UpdatedAt) {
		return definitionValidationError(
			"timestamps",
			fmt.Sprintf("%s > %s", definition.CreatedAt, definition.UpdatedAt),
			"created_at must not be later than updated_at",
		)
	}
	return validateReasoningFacts(
		definition.Capabilities.Features,
		definition.Capabilities.Reasoning,
		definition.Capabilities.ReasoningTokens,
		"capabilities.features.reasoning",
	)
}

func validateReasoningFacts(
	features *ModelFeatures,
	levels *ModelControlLevels,
	tokens *IntRange,
	field string,
) error {
	hasReasoningControl := levels != nil || tokens != nil
	hasReasoningSubfeature := features != nil &&
		(features.ReasoningEffort || features.ReasoningTokens || features.IncludeReasoning)
	if !hasReasoningControl && !hasReasoningSubfeature {
		return nil
	}
	if features != nil && features.Reasoning {
		return nil
	}
	return &errors.ValidationError{
		Field:   field,
		Value:   false,
		Message: "reasoning controls and subordinate capabilities require reasoning support",
	}
}

func modelFactError(field string, value any, message string) error {
	return &errors.ValidationError{
		Field:   "model." + field,
		Value:   value,
		Message: message,
	}
}
