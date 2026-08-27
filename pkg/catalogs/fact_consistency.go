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
		}
	}
	return validateMediaOperationFacts(model)
}

// validateMediaOperationFacts holds a model whose tag names a dedicated media
// operation to the facts that operation requires. It reads the same table the
// derivation reads, so a model that carries the tag and fails the shape is one
// the derivation would have published as a chat model instead.
//
// The output set is exact on purpose. A model that also writes text answers
// through chat completions, and a tag that says otherwise contradicts the
// modalities beside it.
func validateMediaOperationFacts(model Model) error {
	for _, tag := range model.Metadata.Tags {
		required, output, found := mediaFactsForTag(tag)
		if !found {
			continue
		}
		if !sameModalitySet(model.Features.Modalities.Output, output) {
			return modelFactError(
				"features.modalities.output",
				model.Features.Modalities.Output,
				string(tag)+" models must declare exactly "+modalityList(output)+" output",
			)
		}
		for _, modality := range required {
			if !slices.Contains(model.Features.Modalities.Input, modality) {
				return modelFactError(
					"features.modalities.input",
					model.Features.Modalities.Input,
					string(tag)+" models must declare "+string(modality)+" input",
				)
			}
		}
	}
	return nil
}

// mediaFactsForTag collects what every operation sharing one tag agrees on: the
// exact output set they all name, and the input modalities every one of them
// requires. Image generation and image editing share a tag and an output, and
// they differ only in that editing also reads an image, so the shared input is
// the intersection rather than either list.
func mediaFactsForTag(tag ModelTag) (input, output []ModelModality, found bool) {
	for _, facts := range mediaOperationFacts {
		if !slices.Contains(facts.Tags, tag) {
			continue
		}
		if !found {
			input = slices.Clone(facts.Input)
			output = slices.Clone(facts.Output)
			found = true
			continue
		}
		input = slices.DeleteFunc(input, func(modality ModelModality) bool {
			return !slices.Contains(facts.Input, modality)
		})
	}
	return input, output, found
}

func modalityList(modalities []ModelModality) string {
	names := make([]string, 0, len(modalities))
	for _, modality := range modalities {
		names = append(names, string(modality))
	}
	return strings.Join(names, ", ")
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
