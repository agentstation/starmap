package catalogs

import (
	"encoding/json"
	"slices"

	"github.com/agentstation/starmap/pkg/errors"
)

func deriveProviderOffering(candidate providerModelCandidate) (ProviderOffering, error) {
	model := candidate.model
	lifecycle, err := offeringLifecycle(model.Status)
	if err != nil {
		return ProviderOffering{}, errors.WrapResource(
			"derive",
			"provider offering",
			string(candidate.providerID)+"/"+model.ID,
			err,
		)
	}
	offering := ProviderOffering{
		ProviderID:      candidate.providerID,
		ProviderModelID: ProviderModelID(model.ID),
		DefinitionID:    candidate.definitionID,
		Pricing:         deepCopyModelPricing(model.Pricing),
		Availability:    OfferingAvailabilityUnknown,
		Endpoint:        candidate.endpoint,
		Lifecycle:       lifecycle,
	}
	if model.Limits != nil {
		limits := *model.Limits
		offering.Limits = &limits
	}
	if len(model.Modes) > 0 {
		offering.Modes = make(map[string]ProviderOfferingMode, len(model.Modes))
	}
	for modeName, modelMode := range model.Modes {
		mode := ProviderOfferingMode{Pricing: deepCopyModelPricing(modelMode.Pricing)}
		if modelMode.Provider != nil {
			mode.Request.Headers = make(OfferingRequestHeaders, len(modelMode.Provider.Headers))
			for header, value := range modelMode.Provider.Headers {
				mode.Request.Headers[header] = value
			}
			mode.Request.Body = make(OfferingRequestBody, len(modelMode.Provider.Body))
			for field, value := range modelMode.Provider.Body {
				encoded, err := json.Marshal(value)
				if err != nil {
					return ProviderOffering{}, errors.WrapParse(
						"json",
						"provider offering mode "+modeName+" body field "+field,
						err,
					)
				}
				mode.Request.Body[field] = encoded
			}
		}
		offering.Modes[modeName] = mode
	}
	if err := offering.Validate(); err != nil {
		return ProviderOffering{}, errors.WrapResource(
			"derive",
			"provider offering",
			string(candidate.providerID)+"/"+model.ID,
			err,
		)
	}
	return offering, nil
}

func deriveProviderOfferingEndpoint(provider Provider, model Model) ProviderOfferingEndpoint {
	endpoint := ProviderOfferingEndpoint{}
	if provider.ChatCompletions != nil && provider.ChatCompletions.URL != nil &&
		isChatCompletionModel(model) {
		if provider.Catalog != nil {
			endpoint.Type = provider.Catalog.Endpoint.Type
		}
		endpoint.URL = *provider.ChatCompletions.URL
	}
	return endpoint
}

func isChatCompletionModel(model Model) bool {
	if model.Pricing != nil && model.Pricing.Operations != nil {
		return false
	}
	if model.Metadata != nil {
		for _, tag := range model.Metadata.Tags {
			switch tag {
			case "embed", ModelTagEmbedding, "image-gen", "video-gen", "tts", "stt",
				"rerank", ModelTagTextToImage, ModelTagTextToSpeech, ModelTagSpeechToText:
				return false
			}
		}
	}
	return slices.Contains(model.Features.Modalities.Input, ModelModalityText) &&
		slices.Contains(model.Features.Modalities.Output, ModelModalityText)
}

func offeringLifecycle(status ModelStatus) (OfferingLifecycle, error) {
	switch status {
	case ModelStatusActive:
		return OfferingLifecycleActive, nil
	case ModelStatusBeta, ModelStatusPreview:
		return OfferingLifecyclePreview, nil
	case ModelStatusDeprecated:
		return OfferingLifecycleDeprecated, nil
	case "", ModelStatusUnknown:
		return OfferingLifecycleUnknown, nil
	default:
		return "", &errors.ValidationError{
			Field:   "model.status",
			Value:   status,
			Message: "must be active, beta, preview, deprecated, unknown, or empty",
		}
	}
}
