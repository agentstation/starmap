package openrouter

import (
	"github.com/agentstation/starmap/pkg/catalogs"
)

func supportedParameters(features *catalogs.ModelFeatures) []string {
	if features == nil {
		return []string{}
	}
	parameters := make([]string, 0, 32)
	add := func(supported bool, name string) {
		if supported {
			parameters = append(parameters, name)
		}
	}
	add(features.Tools, "tools")
	add(features.ToolChoice, "tool_choice")
	add(features.WebSearch, "web_search_options")
	add(features.Reasoning, "reasoning")
	add(features.IncludeReasoning, "include_reasoning")
	add(features.ReasoningEffort, "reasoning_effort")
	add(features.Verbosity, "verbosity")
	add(features.Temperature, "temperature")
	add(features.TopP, "top_p")
	add(features.TopK, "top_k")
	add(features.TopA, "top_a")
	add(features.MinP, "min_p")
	add(features.TypicalP, "typical_p")
	add(features.TFS, "tfs")
	add(features.MaxTokens, "max_tokens")
	add(features.MaxOutputTokens, "max_completion_tokens")
	add(features.Stop, "stop")
	add(features.StopTokenIDs, "stop_token_ids")
	add(features.FrequencyPenalty, "frequency_penalty")
	add(features.PresencePenalty, "presence_penalty")
	add(features.RepetitionPenalty, "repetition_penalty")
	add(features.LogitBias, "logit_bias")
	add(features.Seed, "seed")
	add(features.Logprobs, "logprobs")
	add(features.TopLogprobs, "top_logprobs")
	add(features.N, "n")
	add(features.BestOf, "best_of")
	add(features.FormatResponse, "response_format")
	add(features.StructuredOutputs, "structured_outputs")
	return parameters
}

func defaultParameters(generation *catalogs.ModelGeneration) map[string]any {
	if generation == nil {
		return nil
	}
	defaults := make(map[string]any)
	addFloat := func(name string, value *catalogs.FloatRange) {
		if value != nil {
			defaults[name] = value.Default
		}
	}
	addInt := func(name string, value *catalogs.IntRange) {
		if value != nil {
			defaults[name] = value.Default
		}
	}
	addFloat("temperature", generation.Temperature)
	addFloat("top_p", generation.TopP)
	addInt("top_k", generation.TopK)
	addFloat("top_a", generation.TopA)
	addFloat("min_p", generation.MinP)
	addFloat("typical_p", generation.TypicalP)
	addFloat("tfs", generation.TFS)
	if generation.MaxTokens != nil {
		defaults["max_tokens"] = *generation.MaxTokens
	}
	if generation.MaxOutputTokens != nil {
		defaults["max_completion_tokens"] = *generation.MaxOutputTokens
	}
	addFloat("frequency_penalty", generation.FrequencyPenalty)
	addFloat("presence_penalty", generation.PresencePenalty)
	addFloat("repetition_penalty", generation.RepetitionPenalty)
	addInt("n", generation.N)
	addInt("best_of", generation.BestOf)
	if len(defaults) == 0 {
		return nil
	}
	return defaults
}
