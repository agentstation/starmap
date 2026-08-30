package catalogs

import (
	"encoding/json"
	"slices"
	"strings"

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
		Endpoints:       append([]ProviderOfferingEndpoint(nil), candidate.endpoints...),
		Lifecycle:       lifecycle,
		DeprecatedAt:    copyPtr(model.DeprecatedAt),
		RetiresAt:       copyPtr(model.RetiresAt),
	}
	offering.Service = deriveOfferingCapabilities(model)
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

func deriveProviderOfferingEndpoints(
	provider Provider,
	definitionID ModelDefinitionID,
	providerModelID string,
	capabilities ProviderOfferingServiceCapabilities,
) []ProviderOfferingEndpoint {
	if provider.Inference == nil {
		return nil
	}
	endpoints := make([]ProviderOfferingEndpoint, 0, len(capabilities.Operations))
	for _, operation := range capabilities.Operations {
		inferenceEndpoint, found := provider.Inference.Endpoint(operation)
		if !found {
			continue
		}
		endpointType := inferenceEndpoint.Type
		authorID, _, parseErr := ParseModelDefinitionID(definitionID)
		if parseErr == nil {
			if authorType, exists := inferenceEndpoint.ProtocolsByAuthor[authorID]; exists {
				endpointType = authorType
			}
		}
		endpointPath := inferenceEndpoint.Path
		streamPath := inferenceEndpoint.StreamPath
		if authorPath, exists := inferenceEndpoint.PathsByAuthor[authorID]; exists {
			endpointPath = authorPath
		}
		if authorStreamPath, exists := inferenceEndpoint.StreamPathsByAuthor[authorID]; exists {
			streamPath = authorStreamPath
		}
		resolvedEndpoint := inferenceEndpoint
		resolvedEndpoint.Path = endpointPath
		endpointURL := provider.Inference.EndpointURL(resolvedEndpoint, "")
		endpointURL = strings.ReplaceAll(endpointURL, "{provider_model_id}", providerModelID)
		endpointURL = strings.ReplaceAll(endpointURL, "{publisher}", string(authorID))
		streamURL := ""
		if streamPath != "" {
			resolvedEndpoint.Path = streamPath
			streamURL = provider.Inference.EndpointURL(resolvedEndpoint, "")
			streamURL = strings.ReplaceAll(streamURL, "{provider_model_id}", providerModelID)
			streamURL = strings.ReplaceAll(streamURL, "{publisher}", string(authorID))
		}
		endpoints = append(endpoints, ProviderOfferingEndpoint{
			Operation: operation,
			Type:      endpointType,
			URL:       endpointURL,
			StreamURL: streamURL,
		})
	}
	return endpoints
}

func deriveOfferingCapabilities(model Model) ProviderOfferingServiceCapabilities {
	capabilities := ProviderOfferingServiceCapabilities{}
	if isEmbeddingModel(model) {
		capabilities.Operations = []ProviderOperation{ProviderOperationEmbeddings}
	}
	if isRerankModel(model) {
		capabilities.Operations = append(capabilities.Operations, ProviderOperationRerank)
	}
	if isModerationModel(model) {
		capabilities.Operations = append(capabilities.Operations, ProviderOperationModerations)
	}
	if isChatCompletionModel(model) {
		capabilities.Operations = append(capabilities.Operations, ProviderOperationChatCompletions)
	}
	capabilities.Operations = append(capabilities.Operations, mediaOperations(model)...)
	if model.Pricing != nil && model.Pricing.Tokens != nil &&
		(model.Pricing.Tokens.CacheRead != nil || model.Pricing.Tokens.CacheWrite != nil) {
		supported := true
		capabilities.PromptCache = &supported
	}
	return capabilities
}

func isEmbeddingModel(model Model) bool {
	if model.Features != nil &&
		slices.Contains(model.Features.Modalities.Output, ModelModalityEmbedding) {
		return true
	}
	if model.Metadata == nil {
		return false
	}
	return slices.Contains(model.Metadata.Tags, ModelTagEmbedding) ||
		slices.Contains(model.Metadata.Tags, ModelTag("embed"))
}

// isRerankModel reports whether the model orders documents by relevance. A
// reranker reads text and writes text, which is also the shape of a chat
// model, so the tag is the only fact that separates the two. The media
// operation table keys on modalities and therefore cannot name this one.
func isRerankModel(model Model) bool {
	if model.Metadata == nil {
		return false
	}
	return slices.Contains(model.Metadata.Tags, ModelTagRerank)
}

// isModerationModel reports whether the model classifies text against harm
// categories. A moderation model reads text like a chat model does, so the
// tag is the only fact that separates the two, exactly as with rerank.
func isModerationModel(model Model) bool {
	if model.Metadata == nil {
		return false
	}
	return slices.Contains(model.Metadata.Tags, ModelTagModeration)
}

func isChatCompletionModel(model Model) bool {
	if isEmbeddingModel(model) {
		return false
	}
	if chargesForGeneratedMedia(model) {
		return false
	}
	if model.Metadata != nil {
		for _, tag := range model.Metadata.Tags {
			switch tag {
			case "embed", ModelTagEmbedding, "image-gen", "video-gen", "tts", "stt",
				ModelTagRerank, ModelTagModeration, ModelTagTextToImage, ModelTagTextToSpeech,
				ModelTagSpeechToText, ModelTagTextToVideo:
				return false
			}
		}
	}
	return model.Features != nil &&
		slices.Contains(model.Features.Modalities.Input, ModelModalityText) &&
		slices.Contains(model.Features.Modalities.Output, ModelModalityText)
}

// chargesForGeneratedMedia reports whether the model prices the production of
// image, audio, or video output. Only the generation prices describe what a
// model serves. A flat request fee, or an input surcharge such as audio_input,
// describes what the model accepts and what it costs, so neither disqualifies
// a chat model. A declared price of zero states that the provider charges
// nothing, not that the model generates media.
func chargesForGeneratedMedia(model Model) bool {
	if model.Pricing == nil || model.Pricing.Operations == nil {
		return false
	}
	operations := model.Pricing.Operations
	return pricesOperation(operations.ImageGen) ||
		pricesOperation(operations.AudioGen) ||
		pricesOperation(operations.VideoGen)
}

func pricesOperation(price *float64) bool {
	return price != nil && *price > 0
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
