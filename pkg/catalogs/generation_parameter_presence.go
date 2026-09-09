package catalogs

import "slices"

// GenerationParameter identifies one optional generation control.
type GenerationParameter string

const (
	// GenerationTemperature identifies the temperature parameter.
	GenerationTemperature GenerationParameter = "temperature"
	// GenerationTopP identifies the top_p parameter.
	GenerationTopP GenerationParameter = "top_p"
	// GenerationTopK identifies the top_k parameter.
	GenerationTopK GenerationParameter = "top_k"
	// GenerationTopA identifies the top_a parameter.
	GenerationTopA GenerationParameter = "top_a"
	// GenerationMinP identifies the min_p parameter.
	GenerationMinP GenerationParameter = "min_p"
	// GenerationTypicalP identifies the typical_p parameter.
	GenerationTypicalP GenerationParameter = "typical_p"
	// GenerationTFS identifies the tfs parameter.
	GenerationTFS GenerationParameter = "tfs"
	// GenerationMaxTokens identifies the max_tokens parameter.
	GenerationMaxTokens GenerationParameter = "max_tokens"
	// GenerationMaxOutputTokens identifies the max_output_tokens parameter.
	GenerationMaxOutputTokens GenerationParameter = "max_output_tokens"
	// GenerationFrequencyPenalty identifies the frequency_penalty parameter.
	GenerationFrequencyPenalty GenerationParameter = "frequency_penalty"
	// GenerationPresencePenalty identifies the presence_penalty parameter.
	GenerationPresencePenalty GenerationParameter = "presence_penalty"
	// GenerationRepetitionPenalty identifies the repetition_penalty parameter.
	GenerationRepetitionPenalty GenerationParameter = "repetition_penalty"
	// GenerationNoRepeatNgramSize identifies the no_repeat_ngram_size parameter.
	GenerationNoRepeatNgramSize GenerationParameter = "no_repeat_ngram_size"
	// GenerationLengthPenalty identifies the length_penalty parameter.
	GenerationLengthPenalty GenerationParameter = "length_penalty"
	// GenerationTopLogprobs identifies the top_logprobs parameter.
	GenerationTopLogprobs GenerationParameter = "top_logprobs"
	// GenerationN identifies the n parameter.
	GenerationN GenerationParameter = "n"
	// GenerationBestOf identifies the best_of parameter.
	GenerationBestOf GenerationParameter = "best_of"
	// GenerationMirostatTau identifies the mirostat_tau parameter.
	GenerationMirostatTau GenerationParameter = "mirostat_tau"
	// GenerationMirostatEta identifies the mirostat_eta parameter.
	GenerationMirostatEta GenerationParameter = "mirostat_eta"
	// GenerationContrastiveSearchPenaltyAlpha identifies the contrastive_search_penalty_alpha parameter.
	GenerationContrastiveSearchPenaltyAlpha GenerationParameter = "contrastive_search_penalty_alpha"
	// GenerationNumBeams identifies the num_beams parameter.
	GenerationNumBeams GenerationParameter = "num_beams"
	// GenerationDiversityPenalty identifies the diversity_penalty parameter.
	GenerationDiversityPenalty GenerationParameter = "diversity_penalty"
)

var generationParameters = []GenerationParameter{
	GenerationTemperature,
	GenerationTopP,
	GenerationTopK,
	GenerationTopA,
	GenerationMinP,
	GenerationTypicalP,
	GenerationTFS,
	GenerationMaxTokens,
	GenerationMaxOutputTokens,
	GenerationFrequencyPenalty,
	GenerationPresencePenalty,
	GenerationRepetitionPenalty,
	GenerationNoRepeatNgramSize,
	GenerationLengthPenalty,
	GenerationTopLogprobs,
	GenerationN,
	GenerationBestOf,
	GenerationMirostatTau,
	GenerationMirostatEta,
	GenerationContrastiveSearchPenaltyAlpha,
	GenerationNumBeams,
	GenerationDiversityPenalty,
}

// PublishedGenerationParameters returns the supported generation controls.
// The caller owns the returned slice.
func PublishedGenerationParameters() []GenerationParameter { return slices.Clone(generationParameters) }

func generationParameterMask(parameter GenerationParameter) uint32 {
	switch parameter {
	case GenerationTemperature:
		return 1 << 0
	case GenerationTopP:
		return 1 << 1
	case GenerationTopK:
		return 1 << 2
	case GenerationTopA:
		return 1 << 3
	case GenerationMinP:
		return 1 << 4
	case GenerationTypicalP:
		return 1 << 5
	case GenerationTFS:
		return 1 << 6
	case GenerationMaxTokens:
		return 1 << 7
	case GenerationMaxOutputTokens:
		return 1 << 8
	case GenerationFrequencyPenalty:
		return 1 << 9
	case GenerationPresencePenalty:
		return 1 << 10
	case GenerationRepetitionPenalty:
		return 1 << 11
	case GenerationNoRepeatNgramSize:
		return 1 << 12
	case GenerationLengthPenalty:
		return 1 << 13
	case GenerationTopLogprobs:
		return 1 << 14
	case GenerationN:
		return 1 << 15
	case GenerationBestOf:
		return 1 << 16
	case GenerationMirostatTau:
		return 1 << 17
	case GenerationMirostatEta:
		return 1 << 18
	case GenerationContrastiveSearchPenaltyAlpha:
		return 1 << 19
	case GenerationNumBeams:
		return 1 << 20
	case GenerationDiversityPenalty:
		return 1 << 21
	default:
		return 0
	}
}

// ParameterPresence reports an optional generation control's observed presence.
func (g *ModelGeneration) ParameterPresence(parameter GenerationParameter) ValuePresence {
	if g == nil {
		return ValueMissing
	}
	var known bool
	switch parameter {
	case GenerationTemperature:
		known = g.Temperature != nil
	case GenerationTopP:
		known = g.TopP != nil
	case GenerationTopK:
		known = g.TopK != nil
	case GenerationTopA:
		known = g.TopA != nil
	case GenerationMinP:
		known = g.MinP != nil
	case GenerationTypicalP:
		known = g.TypicalP != nil
	case GenerationTFS:
		known = g.TFS != nil
	case GenerationMaxTokens:
		known = g.MaxTokens != nil
	case GenerationMaxOutputTokens:
		known = g.MaxOutputTokens != nil
	case GenerationFrequencyPenalty:
		known = g.FrequencyPenalty != nil
	case GenerationPresencePenalty:
		known = g.PresencePenalty != nil
	case GenerationRepetitionPenalty:
		known = g.RepetitionPenalty != nil
	case GenerationNoRepeatNgramSize:
		known = g.NoRepeatNgramSize != nil
	case GenerationLengthPenalty:
		known = g.LengthPenalty != nil
	case GenerationTopLogprobs:
		known = g.TopLogprobs != nil
	case GenerationN:
		known = g.N != nil
	case GenerationBestOf:
		known = g.BestOf != nil
	case GenerationMirostatTau:
		known = g.MirostatTau != nil
	case GenerationMirostatEta:
		known = g.MirostatEta != nil
	case GenerationContrastiveSearchPenaltyAlpha:
		known = g.ContrastiveSearchPenaltyAlpha != nil
	case GenerationNumBeams:
		known = g.NumBeams != nil
	case GenerationDiversityPenalty:
		known = g.DiversityPenalty != nil
	default:
		return ValueMissing
	}
	if known {
		return ValueKnown
	}
	if g.unknownParameters&generationParameterMask(parameter) != 0 {
		return ValueUnknown
	}
	return ValueMissing
}

// SetParameterUnknown records an explicit unknown control. Invalid controls return false.
func (g *ModelGeneration) SetParameterUnknown(parameter GenerationParameter) bool {
	if g == nil {
		return false
	}
	switch parameter {
	case GenerationTemperature:
		g.Temperature = nil
	case GenerationTopP:
		g.TopP = nil
	case GenerationTopK:
		g.TopK = nil
	case GenerationTopA:
		g.TopA = nil
	case GenerationMinP:
		g.MinP = nil
	case GenerationTypicalP:
		g.TypicalP = nil
	case GenerationTFS:
		g.TFS = nil
	case GenerationMaxTokens:
		g.MaxTokens = nil
	case GenerationMaxOutputTokens:
		g.MaxOutputTokens = nil
	case GenerationFrequencyPenalty:
		g.FrequencyPenalty = nil
	case GenerationPresencePenalty:
		g.PresencePenalty = nil
	case GenerationRepetitionPenalty:
		g.RepetitionPenalty = nil
	case GenerationNoRepeatNgramSize:
		g.NoRepeatNgramSize = nil
	case GenerationLengthPenalty:
		g.LengthPenalty = nil
	case GenerationTopLogprobs:
		g.TopLogprobs = nil
	case GenerationN:
		g.N = nil
	case GenerationBestOf:
		g.BestOf = nil
	case GenerationMirostatTau:
		g.MirostatTau = nil
	case GenerationMirostatEta:
		g.MirostatEta = nil
	case GenerationContrastiveSearchPenaltyAlpha:
		g.ContrastiveSearchPenaltyAlpha = nil
	case GenerationNumBeams:
		g.NumBeams = nil
	case GenerationDiversityPenalty:
		g.DiversityPenalty = nil
	default:
		return false
	}
	g.unknownParameters |= generationParameterMask(parameter)
	return true
}

// UnsetParameter removes a control claim. Invalid controls return false.
func (g *ModelGeneration) UnsetParameter(parameter GenerationParameter) bool {
	if !g.SetParameterUnknown(parameter) {
		return false
	}
	g.unknownParameters &^= generationParameterMask(parameter)
	return true
}
