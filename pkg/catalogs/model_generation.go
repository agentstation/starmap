package catalogs

// ModelGeneration - core chat completions generation controls.
type ModelGeneration struct {
	// Core sampling and decoding
	Temperature *FloatRange `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	TopP        *FloatRange `json:"top_p,omitempty" yaml:"top_p,omitempty"`
	TopK        *IntRange   `json:"top_k,omitempty" yaml:"top_k,omitempty"`
	TopA        *FloatRange `json:"top_a,omitempty" yaml:"top_a,omitempty"`
	MinP        *FloatRange `json:"min_p,omitempty" yaml:"min_p,omitempty"`
	TypicalP    *FloatRange `json:"typical_p,omitempty" yaml:"typical_p,omitempty"`
	TFS         *FloatRange `json:"tfs,omitempty" yaml:"tfs,omitempty"`

	// Length and termination
	MaxTokens       *int `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	MaxOutputTokens *int `json:"max_output_tokens,omitempty" yaml:"max_output_tokens,omitempty"`

	// Repetition control
	FrequencyPenalty  *FloatRange `json:"frequency_penalty,omitempty" yaml:"frequency_penalty,omitempty"`
	PresencePenalty   *FloatRange `json:"presence_penalty,omitempty" yaml:"presence_penalty,omitempty"`
	RepetitionPenalty *FloatRange `json:"repetition_penalty,omitempty" yaml:"repetition_penalty,omitempty"`
	NoRepeatNgramSize *IntRange   `json:"no_repeat_ngram_size,omitempty" yaml:"no_repeat_ngram_size,omitempty"`
	LengthPenalty     *FloatRange `json:"length_penalty,omitempty" yaml:"length_penalty,omitempty"`

	// Observability
	TopLogprobs *int `json:"top_logprobs,omitempty" yaml:"top_logprobs,omitempty"` // Number of top log probabilities to return

	// Multiplicity and reranking
	N      *IntRange `json:"n,omitempty" yaml:"n,omitempty"`             // Number of candidates to generate
	BestOf *IntRange `json:"best_of,omitempty" yaml:"best_of,omitempty"` // Server-side sampling with best selection

	// Alternative sampling strategies (niche)
	MirostatTau                   *FloatRange `json:"mirostat_tau,omitempty" yaml:"mirostat_tau,omitempty"`
	MirostatEta                   *FloatRange `json:"mirostat_eta,omitempty" yaml:"mirostat_eta,omitempty"`
	ContrastiveSearchPenaltyAlpha *FloatRange `json:"contrastive_search_penalty_alpha,omitempty" yaml:"contrastive_search_penalty_alpha,omitempty"`

	// Beam search (niche)
	NumBeams         *IntRange   `json:"num_beams,omitempty" yaml:"num_beams,omitempty"`
	DiversityPenalty *FloatRange `json:"diversity_penalty,omitempty" yaml:"diversity_penalty,omitempty"`
}

// FloatRange represents a range of float values.
type FloatRange struct {
	Min     float64 `json:"min" yaml:"min"`         // Minimum value
	Max     float64 `json:"max" yaml:"max"`         // Maximum value
	Default float64 `json:"default" yaml:"default"` // Default value
}

// IntRange represents a range of integer values.
type IntRange struct {
	Min     int `json:"min" yaml:"min"`         // Minimum value
	Max     int `json:"max" yaml:"max"`         // Maximum value
	Default int `json:"default" yaml:"default"` // Default value
}
