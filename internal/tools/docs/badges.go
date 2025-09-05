package docs

import (
	"fmt"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// featureBadges generates HTML badges for ALL model features (50+ technical specifications).
func featureBadges(model *catalogs.Model) string {
	if model.Features == nil {
		return ""
	}

	var badges []string

	// Core Modality Badges - Show all supported modalities
	badges = append(badges, modalityBadges(model.Features)...)

	// Tool & Function Badges - Show complete tool capabilities
	badges = append(badges, toolBadges(model.Features)...)

	// Reasoning Badges - Show reasoning capabilities
	badges = append(badges, reasoningBadges(model.Features)...)

	// Generation Control Badges - Show ALL 50+ generation parameters
	badges = append(badges, generationBadges(model.Features)...)

	// Response Delivery Badges
	badges = append(badges, deliveryBadges(model.Features)...)

	return strings.Join(badges, " ")
}

// modalityBadges generates badges for input/output modalities.
func modalityBadges(features *catalogs.ModelFeatures) []string {
	var badges []string

	// Core modalities with detailed tooltips
	if hasText(features) {
		badges = append(badges, createBadge("text", "✓", "blue", "Supports text generation and processing"))
	}
	if hasVision(features) {
		badges = append(badges, createBadge("vision", "✓", "purple", "Can analyze and understand images"))
	}
	if hasAudio(features) {
		badges = append(badges, createBadge("audio", "✓", "green", "Processes and generates audio content"))
	}
	if hasVideo(features) {
		badges = append(badges, createBadge("video", "✓", "red", "Can process video input"))
	}
	if hasPDF(features) {
		badges = append(badges, createBadge("PDF", "✓", "orange", "Native PDF document processing"))
	}

	// Show detailed modality information if available
	if len(features.Modalities.Input) > 0 {
		inputStr := strings.Join(modalitySliceToStrings(features.Modalities.Input), ",")
		badges = append(badges, createBadge("input", inputStr, "teal", "Supported input modalities"))
	}
	if len(features.Modalities.Output) > 0 {
		outputStr := strings.Join(modalitySliceToStrings(features.Modalities.Output), ",")
		badges = append(badges, createBadge("output", outputStr, "cyan", "Supported output modalities"))
	}

	return badges
}

// toolBadges generates badges for tool and function capabilities.
func toolBadges(features *catalogs.ModelFeatures) []string {
	var badges []string

	if features.ToolCalls {
		badges = append(badges, createBadge("tool_calls", "✓", "yellow", "Can invoke and call tools in responses"))
	}
	if features.Tools {
		badges = append(badges, createBadge("tools", "✓", "yellow", "Accepts tool definitions in requests"))
	}
	if features.ToolChoice {
		badges = append(badges, createBadge("tool_choice", "✓", "yellow", "Supports tool choice strategies (auto/none/required)"))
	}
	if features.WebSearch {
		badges = append(badges, createBadge("web_search", "✓", "indigo", "Can perform web searches"))
	}
	if features.Attachments {
		badges = append(badges, createBadge("attachments", "✓", "pink", "Supports file attachments"))
	}

	return badges
}

// reasoningBadges generates badges for reasoning capabilities.
func reasoningBadges(features *catalogs.ModelFeatures) []string {
	var badges []string

	if features.Reasoning {
		badges = append(badges, createBadge("reasoning", "✓", "lime", "Supports basic reasoning"))
	}
	if features.ReasoningEffort {
		badges = append(badges, createBadge("reasoning_effort", "configurable", "lime", "Configurable reasoning intensity levels"))
	}
	if features.ReasoningTokens {
		badges = append(badges, createBadge("reasoning_tokens", "✓", "lime", "Specific reasoning token allocation"))
	}
	if features.IncludeReasoning {
		badges = append(badges, createBadge("include_reasoning", "✓", "lime", "Can include reasoning traces in response"))
	}
	if features.Verbosity {
		badges = append(badges, createBadge("verbosity", "✓", "gray", "Supports verbosity control (GPT-5+)"))
	}

	return badges
}

// generationBadges generates badges for ALL generation control parameters.
//
//nolint:gocyclo // Many feature flags to check
func generationBadges(features *catalogs.ModelFeatures) []string {
	var badges []string

	// Core Sampling Controls (most important)
	if features.Temperature {
		badges = append(badges, createBadge("temperature", "core", "red", "Temperature sampling control"))
	}
	if features.TopP {
		badges = append(badges, createBadge("top_p", "core", "red", "Nucleus sampling (top-p)"))
	}
	if features.TopK {
		badges = append(badges, createBadge("top_k", "advanced", "orange", "Top-k sampling"))
	}
	if features.TopA {
		badges = append(badges, createBadge("top_a", "advanced", "orange", "Top-a sampling threshold"))
	}
	if features.MinP {
		badges = append(badges, createBadge("min_p", "advanced", "orange", "Minimum probability threshold"))
	}
	if features.TypicalP {
		badges = append(badges, createBadge("typical_p", "advanced", "orange", "Typical sampling"))
	}
	if features.TFS {
		badges = append(badges, createBadge("tfs", "advanced", "orange", "Tail free sampling"))
	}

	// Length and Termination Controls
	if features.MaxTokens {
		badges = append(badges, createBadge("max_tokens", "core", "blue", "Maximum token limit"))
	}
	if features.MaxOutputTokens {
		badges = append(badges, createBadge("max_output_tokens", "core", "blue", "Output token limit"))
	}
	if features.Stop {
		badges = append(badges, createBadge("stop", "core", "blue", "Stop sequences"))
	}
	if features.StopTokenIDs {
		badges = append(badges, createBadge("stop_token_ids", "advanced", "cyan", "Numeric stop token IDs"))
	}

	// Repetition Controls
	if features.FrequencyPenalty {
		badges = append(badges, createBadge("frequency_penalty", "core", "purple", "Frequency penalty"))
	}
	if features.PresencePenalty {
		badges = append(badges, createBadge("presence_penalty", "core", "purple", "Presence penalty"))
	}
	if features.RepetitionPenalty {
		badges = append(badges, createBadge("repetition_penalty", "advanced", "purple", "Repetition penalty"))
	}
	if features.NoRepeatNgramSize {
		badges = append(badges, createBadge("no_repeat_ngram", "niche", "indigo", "N-gram repetition blocking"))
	}
	if features.LengthPenalty {
		badges = append(badges, createBadge("length_penalty", "niche", "indigo", "Length penalty"))
	}

	// Token Biasing
	if features.LogitBias {
		badges = append(badges, createBadge("logit_bias", "core", "yellow", "Token-level bias adjustment"))
	}
	if features.BadWords {
		badges = append(badges, createBadge("bad_words", "advanced", "yellow", "Disallowed tokens"))
	}
	if features.AllowedTokens {
		badges = append(badges, createBadge("allowed_tokens", "niche", "yellow", "Token whitelist"))
	}

	// Determinism
	if features.Seed {
		badges = append(badges, createBadge("seed", "advanced", "green", "Deterministic seeding"))
	}

	// Observability
	if features.Logprobs {
		badges = append(badges, createBadge("logprobs", "core", "teal", "Log probabilities"))
	}
	if features.TopLogprobs {
		badges = append(badges, createBadge("top_logprobs", "core", "teal", "Top N log probabilities"))
	}
	if features.Echo {
		badges = append(badges, createBadge("echo", "advanced", "teal", "Echo prompt with completion"))
	}

	// Multiplicity and Reranking
	if features.N {
		badges = append(badges, createBadge("n", "advanced", "pink", "Multiple candidates"))
	}
	if features.BestOf {
		badges = append(badges, createBadge("best_of", "advanced", "pink", "Server-side sampling"))
	}

	// Alternative Sampling Strategies (Niche)
	if features.Mirostat {
		badges = append(badges, createBadge("mirostat", "niche", "gray", "Mirostat sampling"))
	}
	if features.MirostatTau {
		badges = append(badges, createBadge("mirostat_tau", "niche", "gray", "Mirostat tau parameter"))
	}
	if features.MirostatEta {
		badges = append(badges, createBadge("mirostat_eta", "niche", "gray", "Mirostat eta parameter"))
	}
	if features.ContrastiveSearchPenaltyAlpha {
		badges = append(badges, createBadge("contrastive_search", "niche", "gray", "Contrastive decoding"))
	}

	// Beam Search (Niche)
	if features.NumBeams {
		badges = append(badges, createBadge("num_beams", "niche", "brown", "Beam search"))
	}
	if features.EarlyStopping {
		badges = append(badges, createBadge("early_stopping", "niche", "brown", "Early stopping in beam search"))
	}
	if features.DiversityPenalty {
		badges = append(badges, createBadge("diversity_penalty", "niche", "brown", "Diversity penalty in beam search"))
	}

	return badges
}

// deliveryBadges generates badges for response delivery options.
func deliveryBadges(features *catalogs.ModelFeatures) []string {
	var badges []string

	if features.FormatResponse {
		badges = append(badges, createBadge("format_response", "✓", "cyan", "Alternative response formats"))
	}
	if features.StructuredOutputs {
		badges = append(badges, createBadge("structured_outputs", "✓", "cyan", "JSON schema validation"))
	}
	if features.Streaming {
		badges = append(badges, createBadge("streaming", "✓", "cyan", "Response streaming"))
	}

	return badges
}

// createBadge creates an HTML badge with tooltip.
func createBadge(label, value, color, tooltip string) string {
	// Create shield.io style markdown badge
	// Encode special characters for URL safety
	encodedLabel := strings.ReplaceAll(label, "_", "__")
	encodedLabel = strings.ReplaceAll(encodedLabel, " ", "_")
	encodedLabel = strings.ReplaceAll(encodedLabel, "-", "--")

	encodedValue := strings.ReplaceAll(value, "_", "__")
	encodedValue = strings.ReplaceAll(encodedValue, " ", "_")
	encodedValue = strings.ReplaceAll(encodedValue, "-", "--")

	// Return pure markdown badge (no HTML wrapper for better Hugo compatibility)
	return fmt.Sprintf("![%s](https://img.shields.io/badge/%s-%s-%s)",
		tooltip, encodedLabel, encodedValue, color)
}

// modalitySliceToStrings converts ModelModality slice to string slice.
func modalitySliceToStrings(modalities []catalogs.ModelModality) []string {
	result := make([]string, len(modalities))
	for i, m := range modalities {
		result[i] = string(m)
	}
	return result
}

// technicalSpecBadges generates detailed technical specification badges.
func technicalSpecBadges(model *catalogs.Model) string {
	if model.Features == nil {
		return ""
	}

	sections := []string{
		samplingBadgeSection(model.Features),
		repetitionBadgeSection(model.Features),
		observabilityBadgeSection(model.Features),
		advancedBadgeSection(model.Features),
	}

	// Filter empty sections
	var nonEmpty []string
	for _, section := range sections {
		if section != "" {
			nonEmpty = append(nonEmpty, section)
		}
	}

	return strings.Join(nonEmpty, "\n\n")
}

// samplingBadgeSection creates a section for sampling control badges.
func samplingBadgeSection(features *catalogs.ModelFeatures) string {
	var badges []string

	if features.Temperature {
		badges = append(badges, "![Temperature](https://img.shields.io/badge/temperature-supported-red)")
	}
	if features.TopP {
		badges = append(badges, "![Top-P](https://img.shields.io/badge/top__p-supported-red)")
	}
	if features.TopK {
		badges = append(badges, "![Top-K](https://img.shields.io/badge/top__k-supported-orange)")
	}
	if features.TopA {
		badges = append(badges, "![Top-A](https://img.shields.io/badge/top__a-supported-orange)")
	}
	if features.MinP {
		badges = append(badges, "![Min-P](https://img.shields.io/badge/min__p-supported-orange)")
	}
	if features.TypicalP {
		badges = append(badges, "![Typical-P](https://img.shields.io/badge/typical__p-supported-orange)")
	}
	if features.TFS {
		badges = append(badges, "![TFS](https://img.shields.io/badge/tfs-supported-orange)")
	}

	if len(badges) == 0 {
		return ""
	}

	return "**Sampling Controls:** " + strings.Join(badges, " ")
}

// repetitionBadgeSection creates a section for repetition control badges.
func repetitionBadgeSection(features *catalogs.ModelFeatures) string {
	var badges []string

	if features.FrequencyPenalty {
		badges = append(badges, "![Frequency](https://img.shields.io/badge/frequency__penalty-supported-purple)")
	}
	if features.PresencePenalty {
		badges = append(badges, "![Presence](https://img.shields.io/badge/presence__penalty-supported-purple)")
	}
	if features.RepetitionPenalty {
		badges = append(badges, "![Repetition](https://img.shields.io/badge/repetition__penalty-supported-purple)")
	}
	if features.NoRepeatNgramSize {
		badges = append(badges, "![No-Repeat](https://img.shields.io/badge/no__repeat__ngram-supported-indigo)")
	}

	if len(badges) == 0 {
		return ""
	}

	return "**Repetition Controls:** " + strings.Join(badges, " ")
}

// observabilityBadgeSection creates a section for observability badges.
func observabilityBadgeSection(features *catalogs.ModelFeatures) string {
	var badges []string

	if features.Logprobs {
		badges = append(badges, "![Logprobs](https://img.shields.io/badge/logprobs-supported-teal)")
	}
	if features.TopLogprobs {
		badges = append(badges, "![Top-Logprobs](https://img.shields.io/badge/top__logprobs-supported-teal)")
	}
	if features.Echo {
		badges = append(badges, "![Echo](https://img.shields.io/badge/echo-supported-teal)")
	}

	if len(badges) == 0 {
		return ""
	}

	return "**Observability:** " + strings.Join(badges, " ")
}

// advancedBadgeSection creates a section for advanced/niche feature badges.
func advancedBadgeSection(features *catalogs.ModelFeatures) string {
	var badges []string

	// Alternative sampling
	if features.Mirostat || features.MirostatTau || features.MirostatEta {
		badges = append(badges, "![Mirostat](https://img.shields.io/badge/mirostat-supported-gray)")
	}
	if features.ContrastiveSearchPenaltyAlpha {
		badges = append(badges, "![Contrastive](https://img.shields.io/badge/contrastive-supported-gray)")
	}

	// Beam search
	if features.NumBeams {
		badges = append(badges, "![Beam-Search](https://img.shields.io/badge/beam__search-supported-brown)")
	}

	// Determinism
	if features.Seed {
		badges = append(badges, "![Seed](https://img.shields.io/badge/seed-deterministic-green)")
	}

	// Multiplicity
	if features.N {
		badges = append(badges, "![N](https://img.shields.io/badge/n-multiple-pink)")
	}
	if features.BestOf {
		badges = append(badges, "![Best-Of](https://img.shields.io/badge/best__of-reranking-pink)")
	}

	if len(badges) == 0 {
		return ""
	}

	return "**Advanced Features:** " + strings.Join(badges, " ")
}
