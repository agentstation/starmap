package catalogs

import (
	"github.com/agentstation/utc"
)

// Model represents a model configuration.
type Model struct {
	// Core identity
	ID          string   `json:"id" yaml:"id"`                                       // Unique model identifier
	Name        string   `json:"name" yaml:"name"`                                   // Display name (must not be empty)
	Authors     []Author `json:"authors,omitempty" yaml:"authors,omitempty"`         // Authors/organizations of the model (if known)
	Description string   `json:"description,omitempty" yaml:"description,omitempty"` // Description of the model and its use cases

	// Metadata - version and timing information
	Metadata *ModelMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"` // Metadata for the model

	// Features - what this model can do
	Features *ModelFeatures `json:"features,omitempty" yaml:"features,omitempty"`

	// Attachments - attachment support details
	Attachments *ModelAttachments `json:"attachments,omitempty" yaml:"attachments,omitempty"`

	// Generation - core chat completions generation controls
	Generation *ModelGeneration `json:"generation,omitempty" yaml:"generation,omitempty"`

	// Reasoning - reasoning effort levels
	Reasoning *ModelControlLevels `json:"reasoning,omitempty" yaml:"reasoning,omitempty"`

	// ReasoningTokens - specific token allocation for reasoning processes
	ReasoningTokens *IntRange `json:"reasoning_tokens,omitempty" yaml:"reasoning_tokens,omitempty"`

	// Verbosity - response verbosity levels
	Verbosity *ModelControlLevels `json:"verbosity,omitempty" yaml:"verbosity,omitempty"`

	// Tools - external tool and capability integrations
	Tools *ModelTools `json:"tools,omitempty" yaml:"tools,omitempty"`

	// Delivery - technical response delivery capabilities (formats, protocols, streaming)
	Delivery *ModelDelivery `json:"response,omitempty" yaml:"response,omitempty"`

	// Operational characteristics
	Pricing *ModelPricing `json:"pricing,omitempty" yaml:"pricing,omitempty"` // Optional pricing information
	Limits  *ModelLimits  `json:"limits,omitempty" yaml:"limits,omitempty"`   // Model limits

	// Timestamps for record keeping and auditing
	CreatedAt utc.Time `json:"created_at" yaml:"created_at"` // Created date (YYYY-MM or YYYY-MM-DD format)
	UpdatedAt utc.Time `json:"updated_at" yaml:"updated_at"` // Last updated date (YYYY-MM or YYYY-MM-DD format)
}

// ModelMetadata represents the metadata for a model.
type ModelMetadata struct {
	ReleaseDate     utc.Time           `json:"release_date" yaml:"release_date"`                             // Release date (YYYY-MM or YYYY-MM-DD format)
	OpenWeights     bool               `json:"open_weights" yaml:"open_weights"`                             // Whether model weights are open
	KnowledgeCutoff *utc.Time          `json:"knowledge_cutoff,omitempty" yaml:"knowledge_cutoff,omitempty"` // Knowledge cutoff date (YYYY-MM or YYYY-MM-DD format)
	Tags            []ModelTag         `json:"tags,omitempty" yaml:"tags,omitempty"`                         // Use case tags for categorizing the model
	Architecture    *ModelArchitecture `json:"architecture,omitempty" yaml:"architecture,omitempty"`         // Technical architecture details
}

// ModelFeatures represents a set of feature flags that describe what a model can do.
type ModelFeatures struct {
	// Input/Output modalities
	Modalities ModelModalities `json:"modalities" yaml:"modalities"` // Supported input/output modalities

	// Core capabilities
	// Tool calling system - three distinct aspects:
	ToolCalls   bool `json:"tool_calls" yaml:"tool_calls"`   // Can invoke/call tools in responses (model outputs tool_calls)
	Tools       bool `json:"tools" yaml:"tools"`             // Accepts tool definitions in requests (accepts tools parameter)
	ToolChoice  bool `json:"tool_choice" yaml:"tool_choice"` // Supports tool choice strategies (auto/none/required control)
	WebSearch   bool `json:"web_search" yaml:"web_search"`   // Supports web search capabilities
	Attachments bool `json:"attachments" yaml:"attachments"` // Attachment support details

	// Reasoning & Verbosity
	Reasoning        bool `json:"reasoning" yaml:"reasoning"`                 // Supports basic reasoning
	ReasoningEffort  bool `json:"reasoning_effort" yaml:"reasoning_effort"`   // Supports configurable reasoning intensity
	ReasoningTokens  bool `json:"reasoning_tokens" yaml:"reasoning_tokens"`   // Supports specific reasoning token allocation
	IncludeReasoning bool `json:"include_reasoning" yaml:"include_reasoning"` // Supports including reasoning traces in response
	Verbosity        bool `json:"verbosity" yaml:"verbosity"`                 // Supports verbosity control (GPT-5+)

	// Generation control - Core sampling and decoding
	Temperature bool `json:"temperature" yaml:"temperature"` // [Core] Supports temperature parameter
	TopP        bool `json:"top_p" yaml:"top_p"`             // [Core] Supports top_p parameter (nucleus sampling)
	TopK        bool `json:"top_k" yaml:"top_k"`             // [Advanced] Supports top_k parameter
	TopA        bool `json:"top_a" yaml:"top_a"`             // [Advanced] Supports top_a parameter (top-a sampling)
	MinP        bool `json:"min_p" yaml:"min_p"`             // [Advanced] Supports min_p parameter (minimum probability threshold)
	TypicalP    bool `json:"typical_p" yaml:"typical_p"`     // [Advanced] Supports typical_p parameter (typical sampling)
	TFS         bool `json:"tfs" yaml:"tfs"`                 // [Advanced] Supports tail free sampling

	// Generation control - Length and termination
	MaxTokens       bool `json:"max_tokens" yaml:"max_tokens"`               // [Core] Supports max_tokens parameter
	MaxOutputTokens bool `json:"max_output_tokens" yaml:"max_output_tokens"` // [Core] Supports max_output_tokens parameter (some providers distinguish from max_tokens)
	Stop            bool `json:"stop" yaml:"stop"`                           // [Core] Supports stop sequences/words
	StopTokenIDs    bool `json:"stop_token_ids" yaml:"stop_token_ids"`       // [Advanced] Supports stop token IDs (numeric)

	// Generation control - Repetition control
	FrequencyPenalty  bool `json:"frequency_penalty" yaml:"frequency_penalty"`       // [Core] Supports frequency penalty
	PresencePenalty   bool `json:"presence_penalty" yaml:"presence_penalty"`         // [Core] Supports presence penalty
	RepetitionPenalty bool `json:"repetition_penalty" yaml:"repetition_penalty"`     // [Advanced] Supports repetition penalty
	NoRepeatNgramSize bool `json:"no_repeat_ngram_size" yaml:"no_repeat_ngram_size"` // [Niche] Supports n-gram repetition blocking
	LengthPenalty     bool `json:"length_penalty" yaml:"length_penalty"`             // [Niche] Supports length penalty (seq2seq style)

	// Generation control - Token biasing
	LogitBias     bool `json:"logit_bias" yaml:"logit_bias"`         // [Core] Supports token-level bias adjustment
	BadWords      bool `json:"bad_words" yaml:"bad_words"`           // [Advanced] Supports bad words/disallowed tokens
	AllowedTokens bool `json:"allowed_tokens" yaml:"allowed_tokens"` // [Niche] Supports token whitelist

	// Generation control - Determinism
	Seed bool `json:"seed" yaml:"seed"` // [Advanced] Supports deterministic seeding

	// Generation control - Observability
	Logprobs    bool `json:"logprobs" yaml:"logprobs"`         // [Core] Supports returning log probabilities
	TopLogprobs bool `json:"top_logprobs" yaml:"top_logprobs"` // [Core] Supports returning top N log probabilities
	Echo        bool `json:"echo" yaml:"echo"`                 // [Advanced] Supports echoing prompt with completion

	// Generation control - Multiplicity and reranking
	N      bool `json:"n" yaml:"n"`             // [Advanced] Supports generating multiple candidates
	BestOf bool `json:"best_of" yaml:"best_of"` // [Advanced] Supports server-side sampling with best selection

	// Generation control - Alternative sampling strategies (niche)
	Mirostat                      bool `json:"mirostat" yaml:"mirostat"`                                                 // [Niche] Supports Mirostat sampling
	MirostatTau                   bool `json:"mirostat_tau" yaml:"mirostat_tau"`                                         // [Niche] Supports Mirostat tau parameter
	MirostatEta                   bool `json:"mirostat_eta" yaml:"mirostat_eta"`                                         // [Niche] Supports Mirostat eta parameter
	ContrastiveSearchPenaltyAlpha bool `json:"contrastive_search_penalty_alpha" yaml:"contrastive_search_penalty_alpha"` // [Niche] Supports contrastive decoding

	// Generation control - Beam search (niche)
	NumBeams         bool `json:"num_beams" yaml:"num_beams"`                 // [Niche] Supports beam search
	EarlyStopping    bool `json:"early_stopping" yaml:"early_stopping"`       // [Niche] Supports early stopping in beam search
	DiversityPenalty bool `json:"diversity_penalty" yaml:"diversity_penalty"` // [Niche] Supports diversity penalty in beam search

	// Response delivery
	FormatResponse    bool `json:"format_response" yaml:"format_response"`       // Supports alternative response formats (beyond text)
	StructuredOutputs bool `json:"structured_outputs" yaml:"structured_outputs"` // Supports structured outputs (JSON schema validation)
	Streaming         bool `json:"streaming" yaml:"streaming"`                   // Supports response streaming

}

// ModelModalities represents the input/output modalities supported by a model.
type ModelModalities struct {
	Input  []ModelModality `json:"input" yaml:"input"`   // Supported input modalities
	Output []ModelModality `json:"output" yaml:"output"` // Supported output modalities
}

// ModelModality represents a supported input or output modality for AI models.
type ModelModality string

// String returns the string representation of a ModelModality.
func (m ModelModality) String() string {
	return string(m)
}

// Supported model modalities.
const (
	ModelModalityText      ModelModality = "text"
	ModelModalityAudio     ModelModality = "audio"
	ModelModalityImage     ModelModality = "image"
	ModelModalityVideo     ModelModality = "video"
	ModelModalityPDF       ModelModality = "pdf"
	ModelModalityEmbedding ModelModality = "embedding" // Vector embeddings
)

// ToolChoice represents the strategy for selecting tools.
// Used in API requests as the "tool_choice" parameter value.
type ToolChoice string

// String returns the string representation of a ToolChoice.
func (tc ToolChoice) String() string {
	return string(tc)
}

// Tool choice strategies for controlling tool usage behavior.
const (
	ToolChoiceAuto     ToolChoice = "auto"     // Model autonomously decides whether to call tools based on context
	ToolChoiceNone     ToolChoice = "none"     // Model will never call tools, even if tool definitions are provided
	ToolChoiceRequired ToolChoice = "required" // Model must call at least one tool before responding
	// Note: Specific tool names can also be used as values to force calling a particular tool.
)

// ModelControlLevels represents a set of effort/intensity levels for model controls.
type ModelControlLevels struct {
	Levels  []ModelControlLevel `json:"levels" yaml:"levels"`   // Which levels this model supports
	Default *ModelControlLevel  `json:"default" yaml:"default"` // Default level
}

// ModelControlLevel represents an effort/intensity level for model controls.
type ModelControlLevel string

// String returns the string representation of a ModelControlLevel.
func (mcl ModelControlLevel) String() string {
	return string(mcl)
}

// Supported model control levels.
const (
	ModelControlLevelMinimum ModelControlLevel = "minimum"
	ModelControlLevelLow     ModelControlLevel = "low"
	ModelControlLevelMedium  ModelControlLevel = "medium"
	ModelControlLevelHigh    ModelControlLevel = "high"
	ModelControlLevelMaximum ModelControlLevel = "maximum"
)

// ModelAttachments represents the attachment capabilities of a model.
type ModelAttachments struct {
	MimeTypes   []string `json:"mime_types,omitempty" yaml:"mime_types,omitempty"`       // Supported MIME types
	MaxFileSize *int64   `json:"max_file_size,omitempty" yaml:"max_file_size,omitempty"` // Maximum file size in bytes
	MaxFiles    *int     `json:"max_files,omitempty" yaml:"max_files,omitempty"`         // Maximum number of files per request
}

// ModelLimits represents the limits for a model.
type ModelLimits struct {
	ContextWindow int64 `json:"context_window" yaml:"context_window"` // Context window size in tokens
	OutputTokens  int64 `json:"output_tokens" yaml:"output_tokens"`   // Maximum output tokens
}

// ModelTools represents external tool and capability integrations.
type ModelTools struct {
	// Tool calling configuration
	// Specifies which tool choice strategies this model supports.
	// Requires both Tools=true and ToolChoice=true in ModelFeatures.
	// Common values: ["auto"], ["auto", "none"], ["auto", "none", "required"]
	ToolChoices []ToolChoice `json:"tool_choices,omitempty" yaml:"tool_choices,omitempty"` // Supported tool choice strategies

	// Web search configuration
	// Only applicable if WebSearch=true in ModelFeatures
	WebSearch *ModelWebSearch `json:"web_search,omitempty" yaml:"web_search,omitempty"`
}

// ModelWebSearch represents web search configuration for search-enabled models.
type ModelWebSearch struct {
	// Plugin-based web search options (for models using OpenRouter's web plugin)
	MaxResults   *int    `json:"max_results,omitempty" yaml:"max_results,omitempty"`     // Maximum number of search results (defaults to 5)
	SearchPrompt *string `json:"search_prompt,omitempty" yaml:"search_prompt,omitempty"` // Custom prompt for search results

	// Built-in web search options (for models with native web search like GPT-4.1, Perplexity)
	SearchContextSizes []ModelControlLevel `json:"search_context_sizes,omitempty" yaml:"search_context_sizes,omitempty"` // Supported context sizes (low, medium, high)
	DefaultContextSize *ModelControlLevel  `json:"default_context_size,omitempty" yaml:"default_context_size,omitempty"` // Default search context size
}
