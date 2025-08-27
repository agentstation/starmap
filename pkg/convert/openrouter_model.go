package convert

import (
	"fmt"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// OpenRouterModel represents a model in OpenRouter API format.
// Field order matches the OpenRouter API response schema.
type OpenRouterModel struct {
	ID                  string                 `json:"id"`
	Name                string                 `json:"name"`
	Created             int64                  `json:"created"`
	Description         string                 `json:"description"`
	Architecture        OpenRouterArchitecture `json:"architecture"`
	TopProvider         OpenRouterTopProvider  `json:"top_provider"`
	Pricing             OpenRouterPricing      `json:"pricing"`
	CanonicalSlug       string                 `json:"canonical_slug"`
	ContextLength       int64                  `json:"context_length"`
	HuggingFaceID       string                 `json:"hugging_face_id,omitempty"`
	PerRequestLimits    any                    `json:"per_request_limits"`
	SupportedParameters []string               `json:"supported_parameters"`
}

// OpenRouterArchitecture represents the architecture object in OpenRouter format.
type OpenRouterArchitecture struct {
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	Tokenizer        string   `json:"tokenizer"`
	InstructType     *string  `json:"instruct_type"`
}

// OpenRouterPricing represents the pricing object in OpenRouter format.
type OpenRouterPricing struct {
	Prompt            string `json:"prompt"`
	Completion        string `json:"completion"`
	Request           string `json:"request"`
	Image             string `json:"image"`
	WebSearch         string `json:"web_search"`
	InternalReasoning string `json:"internal_reasoning"`
	InputCacheRead    string `json:"input_cache_read"`
	InputCacheWrite   string `json:"input_cache_write"`
}

// OpenRouterTopProvider represents the top provider object in OpenRouter format.
type OpenRouterTopProvider struct {
	ContextLength       int64 `json:"context_length"`
	MaxCompletionTokens int64 `json:"max_completion_tokens"`
	IsModerated         bool  `json:"is_moderated"`
}

// OpenRouterModelsResponse represents the root response object for OpenRouter models API.
type OpenRouterModelsResponse struct {
	Data []OpenRouterModel `json:"data"`
}

// ToOpenRouterModel converts a Model to OpenRouter format.
func ToOpenRouterModel(m *catalogs.Model) OpenRouterModel {
	openRouterModel := OpenRouterModel{
		ID:                  m.ID,
		CanonicalSlug:       m.ID, // Use ID as canonical slug if not specified
		Name:                m.Name,
		Created:             m.CreatedAt.Unix(),
		Description:         m.Description,
		ContextLength:       0,
		PerRequestLimits:    nil,
		SupportedParameters: OpenRouterSupportedParameters(m),
	}

	// Set context length from limits
	if m.Limits != nil {
		openRouterModel.ContextLength = m.Limits.ContextWindow
		openRouterModel.TopProvider.ContextLength = m.Limits.ContextWindow
		openRouterModel.TopProvider.MaxCompletionTokens = m.Limits.OutputTokens
	}

	// Convert architecture
	if m.Features != nil {
		openRouterModel.Architecture = OpenRouterArchitecture{
			InputModalities:  convertModalities(m.Features.Modalities.Input),
			OutputModalities: convertModalities(m.Features.Modalities.Output),
			Tokenizer:        convertTokenizer(getTokenizer(m)),
			InstructType:     nil, // Would need additional field in our model
		}
	}

	// Convert pricing
	if m.Pricing != nil {
		openRouterModel.Pricing = OpenRouterPricing{
			Prompt:            convertTokenCost(getTokenCost(m.Pricing, "input")),
			Completion:        convertTokenCost(getTokenCost(m.Pricing, "output")),
			Request:           convertOperationCost(getOperationCost(m.Pricing, "request")),
			Image:             convertOperationCost(getOperationCost(m.Pricing, "image")),
			WebSearch:         convertOperationCost(getOperationCost(m.Pricing, "web_search")),
			InternalReasoning: convertTokenCost(getTokenCost(m.Pricing, "reasoning")),
			InputCacheRead:    convertCacheCost(getCacheCost(m.Pricing), "read"),
			InputCacheWrite:   convertCacheCost(getCacheCost(m.Pricing), "write"),
		}
	} else {
		// Default to all free if no pricing info
		openRouterModel.Pricing = OpenRouterPricing{
			Prompt:            "0",
			Completion:        "0",
			Request:           "0",
			Image:             "0",
			WebSearch:         "0",
			InternalReasoning: "0",
			InputCacheRead:    "0",
			InputCacheWrite:   "0",
		}
	}

	return openRouterModel
}

// Helper functions for conversion

// OpenRouterSupportedParameters returns a list of parameters that this model supports.
// This matches the OpenRouter API format for supported_parameters.
func OpenRouterSupportedParameters(m *catalogs.Model) []string {
	var params []string

	if m.Features == nil {
		return params
	}

	// Generation control parameters
	if m.Features.MaxTokens {
		params = append(params, "max_tokens")
	}
	if m.Features.Temperature {
		params = append(params, "temperature")
	}
	if m.Features.TopP {
		params = append(params, "top_p")
	}
	if m.Features.TopK {
		params = append(params, "top_k")
	}
	if m.Features.TopA {
		params = append(params, "top_a")
	}
	if m.Features.MinP {
		params = append(params, "min_p")
	}
	if m.Features.FrequencyPenalty {
		params = append(params, "frequency_penalty")
	}
	if m.Features.PresencePenalty {
		params = append(params, "presence_penalty")
	}
	if m.Features.RepetitionPenalty {
		params = append(params, "repetition_penalty")
	}
	if m.Features.LogitBias {
		params = append(params, "logit_bias")
	}
	if m.Features.Seed {
		params = append(params, "seed")
	}
	if m.Features.Stop {
		params = append(params, "stop")
	}
	if m.Features.Logprobs {
		params = append(params, "logprobs")
	}

	// Advanced generation parameters
	if m.Generation != nil && m.Generation.TopLogprobs != nil {
		params = append(params, "top_logprobs")
	}

	// Reasoning parameters
	if m.Features.Reasoning {
		params = append(params, "reasoning")
	}
	if m.Features.IncludeReasoning {
		params = append(params, "include_reasoning")
	}

	// Response format parameters
	if m.Features.FormatResponse {
		params = append(params, "response_format")
	}
	if m.Features.StructuredOutputs {
		params = append(params, "structured_outputs")
	}

	// Tool parameters
	if m.Features.Tools {
		params = append(params, "tools")
	}
	if m.Features.ToolChoice {
		params = append(params, "tool_choice")
	}

	// Web search parameters
	if m.Features.WebSearch {
		params = append(params, "web_search_options")
	}

	return params
}

func getTokenizer(m *catalogs.Model) catalogs.Tokenizer {
	if m.Metadata != nil && m.Metadata.Architecture != nil {
		return m.Metadata.Architecture.Tokenizer
	}
	return catalogs.TokenizerUnknown
}

func getTokenCost(pricing *catalogs.ModelPricing, costType string) *catalogs.TokenCost {
	if pricing == nil || pricing.Tokens == nil {
		return nil
	}

	switch costType {
	case "input":
		return pricing.Tokens.Input
	case "output":
		return pricing.Tokens.Output
	case "reasoning":
		return pricing.Tokens.Reasoning
	default:
		return nil
	}
}

func getOperationCost(pricing *catalogs.ModelPricing, costType string) *float64 {
	if pricing == nil || pricing.Operations == nil {
		return nil
	}

	switch costType {
	case "request":
		return pricing.Operations.Request
	case "image":
		return pricing.Operations.ImageInput
	case "web_search":
		return pricing.Operations.WebSearch
	default:
		return nil
	}
}

func getCacheCost(pricing *catalogs.ModelPricing) *catalogs.TokenCacheCost {
	if pricing == nil || pricing.Tokens == nil {
		return nil
	}
	return pricing.Tokens.Cache
}

func convertModalities(modalities []catalogs.ModelModality) []string {
	if modalities == nil {
		return []string{}
	}

	result := make([]string, len(modalities))
	for i, modality := range modalities {
		result[i] = string(modality)
	}
	return result
}

func convertTokenizer(tokenizer catalogs.Tokenizer) string {
	// Map our tokenizer types to OpenRouter format
	switch tokenizer {
	case catalogs.TokenizerGPT:
		return "GPT"
	case catalogs.TokenizerClaude:
		return "Claude"
	case catalogs.TokenizerLlama2:
		return "Llama2"
	case catalogs.TokenizerLlama3:
		return "Llama3"
	case catalogs.TokenizerGemini:
		return "Gemini"
	case catalogs.TokenizerMistral:
		return "Mistral"
	default:
		return "Other"
	}
}

func convertTokenCost(cost *catalogs.TokenCost) string {
	if cost == nil {
		return "0"
	}
	return fmt.Sprintf("%.10f", cost.PerToken)
}

func convertOperationCost(cost *float64) string {
	if cost == nil {
		return "0"
	}
	return fmt.Sprintf("%.10f", *cost)
}

func convertCacheCost(cache *catalogs.TokenCacheCost, operation string) string {
	if cache == nil {
		return "0"
	}

	switch operation {
	case "read":
		return convertTokenCost(cache.Read)
	case "write":
		return convertTokenCost(cache.Write)
	default:
		return "0"
	}
}
