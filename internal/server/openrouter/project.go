package openrouter

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

// ProjectModel resolves and projects one canonical model.
func ProjectModel(
	catalog *catalogs.Catalog,
	authorID catalogs.AuthorID,
	slug string,
	pathPrefix string,
) (Model, error) {
	resolved, err := resolve(catalog, authorID, slug)
	if err != nil {
		return Model{}, err
	}
	definition := resolved.definition
	architecture := projectArchitecture(definition)
	parameters := supportedParameters(definition.Capabilities.Features)
	preferred := preferredOffering(resolved.offerings)
	canonicalID := string(definition.ID)
	responseID := variantID(canonicalID, resolved.variant)
	name := variantName(definition.Name, resolved.variant)

	model := Model{
		Architecture:        architecture,
		CanonicalSlug:       canonicalID,
		ContextLength:       maximumContextLength(resolved.offerings),
		Created:             definitionCreated(definition),
		DefaultParameters:   defaultParameters(definition.Capabilities.Generation),
		Description:         definition.Description,
		ID:                  responseID,
		KnowledgeCutoff:     knowledgeCutoff(definition),
		Links:               ModelLinks{Details: endpointDetailsPath(pathPrefix, responseID)},
		Name:                name,
		SupportedParameters: parameters,
		Reasoning:           projectReasoning(definition),
	}
	if preferred != nil {
		model.Pricing = projectPricing(preferred.Pricing)
		model.TopProvider = projectTopProvider(catalog, *preferred)
	}
	return model, nil
}

// ProjectEndpoints resolves and projects every eligible provider offering for
// one canonical model. Catalog ordering is preserved.
func ProjectEndpoints(
	catalog *catalogs.Catalog,
	authorID catalogs.AuthorID,
	slug string,
) (Endpoints, error) {
	resolved, err := resolve(catalog, authorID, slug)
	if err != nil {
		return Endpoints{}, err
	}
	definition := resolved.definition
	responseID := variantID(string(definition.ID), resolved.variant)
	result := Endpoints{
		Architecture: projectArchitecture(definition),
		Created:      definitionCreated(definition),
		Description:  definition.Description,
		Endpoints:    make([]Endpoint, 0, len(resolved.offerings)),
		ID:           responseID,
		Name:         variantName(definition.Name, resolved.variant),
	}
	parameters := supportedParameters(definition.Capabilities.Features)
	for _, offering := range resolved.offerings {
		provider, providerErr := catalog.Provider(offering.ProviderID)
		if providerErr != nil {
			return Endpoints{}, &pkgerrors.ValidationError{
				Field:   "openrouter.provider",
				Value:   offering.ProviderID,
				Message: "referenced provider is unavailable",
			}
		}
		result.Endpoints = append(result.Endpoints, projectEndpoint(
			definition,
			provider,
			offering,
			parameters,
		))
	}
	return result, nil
}

func projectArchitecture(definition catalogs.ModelDefinition) Architecture {
	input := make([]string, 0)
	output := make([]string, 0)
	if features := definition.Capabilities.Features; features != nil {
		input = projectModalities(features.Modalities.Input)
		output = projectModalities(features.Modalities.Output)
	}
	architecture := Architecture{
		InputModalities:  input,
		Modality:         modality(input, output),
		OutputModalities: output,
		Tokenizer:        "Other",
	}
	if weights := definition.Weights.Architecture; weights != nil {
		architecture.Tokenizer = tokenizerName(weights.Tokenizer)
	}
	return architecture
}

func projectModalities(modalities []catalogs.ModelModality) []string {
	result := make([]string, 0, len(modalities))
	for _, item := range modalities {
		value := string(item)
		switch item {
		case catalogs.ModelModalityPDF:
			value = "file"
		case catalogs.ModelModalityEmbedding:
			value = "embeddings"
		}
		result = append(result, value)
	}
	slices.SortFunc(result, func(left, right string) int {
		leftRank := modalityRank(left)
		rightRank := modalityRank(right)
		if leftRank != rightRank {
			return leftRank - rightRank
		}
		return strings.Compare(left, right)
	})
	return result
}

func modalityRank(modality string) int {
	switch modality {
	case "text":
		return 0
	case "image":
		return 1
	case "file":
		return 2
	case "audio":
		return 3
	case "video":
		return 4
	case "embeddings":
		return 5
	default:
		return 6
	}
}

func modality(input, output []string) string {
	if len(input) == 0 && len(output) == 0 {
		return ""
	}
	return strings.Join(input, "+") + "->" + strings.Join(output, "+")
}

func tokenizerName(tokenizer catalogs.Tokenizer) string {
	switch tokenizer {
	case catalogs.TokenizerGPT:
		return "GPT"
	case catalogs.TokenizerClaude:
		return "Claude"
	case catalogs.TokenizerCohere:
		return "Cohere"
	case catalogs.TokenizerDeepSeek:
		return "DeepSeek"
	case catalogs.TokenizerGemini:
		return "Gemini"
	case catalogs.TokenizerGrok:
		return "Grok"
	case catalogs.TokenizerLlama2:
		return "Llama2"
	case catalogs.TokenizerLlama3:
		return "Llama3"
	case catalogs.TokenizerLlama4:
		return "Llama4"
	case catalogs.TokenizerMistral:
		return "Mistral"
	case catalogs.TokenizerNova:
		return "Nova"
	case catalogs.TokenizerQwen:
		return "Qwen"
	case catalogs.TokenizerQwen3:
		return "Qwen3"
	case catalogs.TokenizerRouter:
		return "Router"
	case catalogs.TokenizerYi:
		return "Yi"
	default:
		return "Other"
	}
}

func projectEndpoint(
	definition catalogs.ModelDefinition,
	provider catalogs.Provider,
	offering catalogs.ProviderOffering,
	parameters []string,
) Endpoint {
	result := Endpoint{
		ModelID:             string(offering.ProviderModelID),
		ModelName:           definition.Name,
		Name:                provider.Name + " | " + string(offering.ProviderModelID),
		Pricing:             projectPricing(offering.Pricing),
		ProviderName:        provider.Name,
		Quantization:        definitionQuantization(definition),
		Status:              0,
		SupportedParameters: append([]string(nil), parameters...),
		// Cache pricing does not prove that caching is implicit.
		SupportsImplicitCaching: false,
		Tag:                     string(provider.ID),
	}
	if offering.Limits != nil {
		result.ContextLength = positiveInt64(offering.Limits.ContextWindow)
		result.MaxPromptTokens = positiveInt64(offering.Limits.InputTokens)
		result.MaxCompletionTokens = positiveInt64(offering.Limits.OutputTokens)
	}
	return result
}

func definitionQuantization(definition catalogs.ModelDefinition) string {
	if architecture := definition.Weights.Architecture; architecture != nil &&
		architecture.Quantization != "" {
		return string(architecture.Quantization)
	}
	return "unknown"
}

func projectTopProvider(
	catalog *catalogs.Catalog,
	offering catalogs.ProviderOffering,
) *TopProvider {
	result := &TopProvider{}
	if offering.Limits != nil {
		result.ContextLength = positiveInt64(offering.Limits.ContextWindow)
		result.MaxCompletionTokens = positiveInt64(offering.Limits.OutputTokens)
	}
	if provider, err := catalog.Provider(offering.ProviderID); err == nil &&
		provider.GovernancePolicy != nil {
		result.IsModerated = provider.GovernancePolicy.Moderated
	}
	return result
}

func maximumContextLength(offerings []catalogs.ProviderOffering) int64 {
	var maximum int64
	for _, offering := range offerings {
		if offering.Limits != nil && offering.Limits.ContextWindow > maximum {
			maximum = offering.Limits.ContextWindow
		}
	}
	return maximum
}

func positiveInt64(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func definitionCreated(definition catalogs.ModelDefinition) int64 {
	if !definition.Metadata.ReleaseDate.IsZero() {
		return definition.Metadata.ReleaseDate.Unix()
	}
	if !definition.CreatedAt.IsZero() {
		return definition.CreatedAt.Unix()
	}
	return 0
}

func knowledgeCutoff(definition catalogs.ModelDefinition) *string {
	if definition.Metadata.KnowledgeCutoff == nil ||
		definition.Metadata.KnowledgeCutoff.IsZero() {
		return nil
	}
	value := definition.Metadata.KnowledgeCutoff.DateOnly()
	return &value
}

func projectReasoning(definition catalogs.ModelDefinition) *Reasoning {
	features := definition.Capabilities.Features
	levels := definition.Capabilities.Reasoning
	if (features == nil || !features.Reasoning) && levels == nil {
		return nil
	}
	result := &Reasoning{
		DefaultEnabled: features != nil && features.Reasoning,
		Mandatory:      false,
	}
	if levels == nil {
		result.SupportedEfforts = []string{}
		return result
	}
	result.SupportedEfforts = make([]string, 0, len(levels.Levels))
	for _, level := range levels.Levels {
		result.SupportedEfforts = append(
			result.SupportedEfforts,
			openRouterEffort(level),
		)
	}
	if levels.Default != nil {
		value := openRouterEffort(*levels.Default)
		result.DefaultEffort = &value
	}
	return result
}

func openRouterEffort(level catalogs.ModelControlLevel) string {
	if level == catalogs.ModelControlLevelMinimum {
		return "minimal"
	}
	return string(level)
}

func endpointDetailsPath(pathPrefix, modelID string) string {
	author, slug, found := strings.Cut(modelID, "/")
	if !found {
		return ""
	}
	return fmt.Sprintf(
		"%s/models/%s/%s/endpoints",
		strings.TrimSuffix(pathPrefix, "/"),
		url.PathEscape(author),
		url.PathEscape(slug),
	)
}

func variantID(canonicalID, variant string) string {
	if variant == "" {
		return canonicalID
	}
	return canonicalID + ":" + variant
}

func variantName(name, variant string) string {
	if variant == "" {
		return name
	}
	return name + " (" + variant + ")"
}
