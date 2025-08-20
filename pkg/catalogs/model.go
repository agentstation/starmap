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
	Delivery *ModelDelivery `json:"delivery,omitempty" yaml:"delivery,omitempty"`

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

// ModelTag represents a use case or category tag for models.
type ModelTag string

// String returns the string representation of a ModelTag.
func (tag ModelTag) String() string {
	return string(tag)
}

// Model tags for categorizing models by use case and capabilities.
const (
	// Core Use Cases
	ModelTagCoding    ModelTag = "coding"    // Programming and code generation
	ModelTagWriting   ModelTag = "writing"   // Creative and technical writing
	ModelTagReasoning ModelTag = "reasoning" // Logical reasoning and problem solving
	ModelTagMath      ModelTag = "math"      // Mathematical problem solving
	ModelTagChat      ModelTag = "chat"      // Conversational AI
	ModelTagInstruct  ModelTag = "instruct"  // Instruction following
	ModelTagResearch  ModelTag = "research"  // Research and analysis
	ModelTagCreative  ModelTag = "creative"  // Creative content generation
	ModelTagRoleplay  ModelTag = "roleplay"  // Character roleplay and simulation

	// Technical Capabilities
	ModelTagFunctionCalling ModelTag = "function_calling"   // Tool/function calling
	ModelTagEmbedding       ModelTag = "embedding"          // Text embeddings
	ModelTagSummarization   ModelTag = "summarization"      // Text summarization
	ModelTagTranslation     ModelTag = "translation"        // Language translation
	ModelTagQA              ModelTag = "question_answering" // Question answering

	// Modality-Specific
	ModelTagVision       ModelTag = "vision"         // Computer vision
	ModelTagMultimodal   ModelTag = "multimodal"     // Multiple input modalities
	ModelTagAudio        ModelTag = "audio"          // Audio processing
	ModelTagTextToImage  ModelTag = "text_to_image"  // Text-to-image generation
	ModelTagTextToSpeech ModelTag = "text_to_speech" // Text-to-speech synthesis
	ModelTagSpeechToText ModelTag = "speech_to_text" // Speech recognition
	ModelTagImageToText  ModelTag = "image_to_text"  // Image captioning/OCR

	// Domain-Specific
	ModelTagMedical   ModelTag = "medical"   // Medical and healthcare
	ModelTagLegal     ModelTag = "legal"     // Legal document processing
	ModelTagFinance   ModelTag = "finance"   // Financial analysis
	ModelTagScience   ModelTag = "science"   // Scientific applications
	ModelTagEducation ModelTag = "education" // Educational content
)

// ModelArchitecture represents the technical architecture details of a model.
type ModelArchitecture struct {
	ParameterCount string           `json:"parameter_count,omitempty" yaml:"parameter_count,omitempty"` // Model size (e.g., "7B", "70B", "405B")
	Type           ArchitectureType `json:"type,omitempty" yaml:"type,omitempty"`                       // Type of architecture
	Tokenizer      Tokenizer        `json:"tokenizer,omitempty" yaml:"tokenizer,omitempty"`             // Tokenizer type used by the model
	Precision      *string          `json:"precision,omitempty" yaml:"precision,omitempty"`             // Legacy precision format (use Quantization for filtering)
	Quantization   Quantization     `json:"quantization,omitempty" yaml:"quantization,omitempty"`       // Quantization level used by the model
	Quantized      bool             `json:"quantized" yaml:"quantized"`                                 // Whether the model has been quantized
	FineTuned      bool             `json:"fine_tuned" yaml:"fine_tuned"`                               // Whether this is a fine-tuned variant
	BaseModel      *string          `json:"base_model,omitempty" yaml:"base_model,omitempty"`           // Base model ID if fine-tuned
}

// ArchitectureType represents the type of model architecture.
type ArchitectureType string

// String returns the string representation of an ArchitectureType.
func (at ArchitectureType) String() string {
	return string(at)
}

// Architecture types.
const (
	ArchitectureTypeTransformer ArchitectureType = "transformer" // Transformer-based models (GPT, BERT, LLaMA, etc.)
	ArchitectureTypeMoE         ArchitectureType = "moe"         // Mixture of Experts (Mixtral, GLaM, Switch Transformer)
	ArchitectureTypeCNN         ArchitectureType = "cnn"         // Convolutional Neural Networks
	ArchitectureTypeRNN         ArchitectureType = "rnn"         // Recurrent Neural Networks
	ArchitectureTypeLSTM        ArchitectureType = "lstm"        // Long Short-Term Memory networks
	ArchitectureTypeGRU         ArchitectureType = "gru"         // Gated Recurrent Unit networks
	ArchitectureTypeVAE         ArchitectureType = "vae"         // Variational Autoencoders
	ArchitectureTypeGAN         ArchitectureType = "gan"         // Generative Adversarial Networks
	ArchitectureTypeDiffusion   ArchitectureType = "diffusion"   // Diffusion models (Stable Diffusion, DALL-E, etc.)
)

// Tokenizer represents the tokenizer type used by a model.
type Tokenizer string

// String returns the string representation of a Tokenizer.
func (t Tokenizer) String() string {
	return string(t)
}

// Tokenizer types.
const (
	TokenizerClaude   Tokenizer = "claude"   // Claude tokenizer
	TokenizerCohere   Tokenizer = "cohere"   // Cohere tokenizer
	TokenizerDeepSeek Tokenizer = "deepseek" // DeepSeek tokenizer
	TokenizerGPT      Tokenizer = "gpt"      // GPT tokenizer (OpenAI)
	TokenizerGemini   Tokenizer = "gemini"   // Gemini tokenizer (Google)
	TokenizerGrok     Tokenizer = "grok"     // Grok tokenizer (xAI)
	TokenizerLlama2   Tokenizer = "llama2"   // LLaMA 2 tokenizer
	TokenizerLlama3   Tokenizer = "llama3"   // LLaMA 3 tokenizer
	TokenizerLlama4   Tokenizer = "llama4"   // LLaMA 4 tokenizer
	TokenizerMistral  Tokenizer = "mistral"  // Mistral tokenizer
	TokenizerNova     Tokenizer = "nova"     // Nova tokenizer (Amazon)
	TokenizerQwen     Tokenizer = "qwen"     // Qwen tokenizer
	TokenizerQwen3    Tokenizer = "qwen3"    // Qwen 3 tokenizer
	TokenizerRouter   Tokenizer = "router"   // Router-based tokenizer
	TokenizerYi       Tokenizer = "yi"       // Yi tokenizer
	TokenizerUnknown  Tokenizer = "unknown"  // Unknown tokenizer type
)

// Quantization represents the quantization level used by a model.
// Quantization reduces model size and computational requirements while aiming to preserve performance.
type Quantization string

// String returns the string representation of a Quantization.
func (q Quantization) String() string {
	return string(q)
}

// Quantization levels.
const (
	QuantizationINT4    Quantization = "int4"    // Integer (4 bit)
	QuantizationINT8    Quantization = "int8"    // Integer (8 bit)
	QuantizationFP4     Quantization = "fp4"     // Floating point (4 bit)
	QuantizationFP6     Quantization = "fp6"     // Floating point (6 bit)
	QuantizationFP8     Quantization = "fp8"     // Floating point (8 bit)
	QuantizationFP16    Quantization = "fp16"    // Floating point (16 bit)
	QuantizationBF16    Quantization = "bf16"    // Brain floating point (16 bit)
	QuantizationFP32    Quantization = "fp32"    // Floating point (32 bit)
	QuantizationUnknown Quantization = "unknown" // Unknown quantization
)

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
	StopTokenIds    bool `json:"stop_token_ids" yaml:"stop_token_ids"`       // [Advanced] Supports stop token IDs (numeric)

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
	ModelModalityText  ModelModality = "text"
	ModelModalityAudio ModelModality = "audio"
	ModelModalityImage ModelModality = "image"
	ModelModalityVideo ModelModality = "video"
	ModelModalityPDF   ModelModality = "pdf"
)

// ModelGeneration - core chat completions generation controls
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
	// Note: Specific tool names can also be used as values to force calling a particular tool
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

// ModelDelivery represents technical response delivery capabilities.
type ModelDelivery struct {
	// Response delivery mechanisms
	Protocols []ModelResponseProtocol `json:"protocols,omitempty" yaml:"protocols,omitempty"` // Supported delivery protocols (HTTP, gRPC, etc.)
	Streaming []ModelStreaming        `json:"streaming,omitempty" yaml:"streaming,omitempty"` // Supported streaming modes (sse, websocket, chunked)
	Formats   []ModelResponseFormat   `json:"formats,omitempty" yaml:"formats,omitempty"`     // Available response formats (if format_response feature enabled)
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

// ModelResponseFormat represents a supported response format.
type ModelResponseFormat string

// String returns the string representation of a ModelResponseFormat.
func (mrf ModelResponseFormat) String() string {
	return string(mrf)
}

// Model response formats.
const (
	// Basic formats
	ModelResponseFormatText ModelResponseFormat = "text" // Plain text responses (default)

	// JSON formats
	ModelResponseFormatJSON       ModelResponseFormat = "json"        // JSON encouraged via prompting
	ModelResponseFormatJSONMode   ModelResponseFormat = "json_mode"   // Forced valid JSON (OpenAI style)
	ModelResponseFormatJSONObject ModelResponseFormat = "json_object" // Same as json_mode (OpenAI API name)

	// Structured formats
	ModelResponseFormatJSONSchema       ModelResponseFormat = "json_schema"       // Schema-validated JSON (OpenAI structured output)
	ModelResponseFormatStructuredOutput ModelResponseFormat = "structured_output" // General structured output support

	// Function calling (alternative to JSON schema)
	ModelResponseFormatFunctionCall ModelResponseFormat = "function_call" // Tool/function calling for structured data
)

// ModelResponseProtocol represents a supported delivery protocol.
type ModelResponseProtocol string

// Model delivery protocols.
const (
	ModelResponseProtocolHTTP      ModelResponseProtocol = "http"      // HTTP/HTTPS REST API
	ModelResponseProtocolGRPC      ModelResponseProtocol = "grpc"      // gRPC protocol
	ModelResponseProtocolWebSocket ModelResponseProtocol = "websocket" // WebSocket protocol
)

// ModelStreaming represents how responses can be delivered.
type ModelStreaming string

// String returns the string representation of a ModelStreaming.
func (ms ModelStreaming) String() string {
	return string(ms)
}

// Model streaming modes.
const (
	ModelStreamingSSE       ModelStreaming = "sse"       // Server-Sent Events streaming
	ModelStreamingWebSocket ModelStreaming = "websocket" // WebSocket streaming
	ModelStreamingChunked   ModelStreaming = "chunked"   // HTTP chunked transfer encoding
)

// ModelAttachments represents the attachment capabilities of a model.
type ModelAttachments struct {
	MimeTypes   []string `json:"mime_types,omitempty" yaml:"mime_types,omitempty"`       // Supported MIME types
	MaxFileSize *int64   `json:"max_file_size,omitempty" yaml:"max_file_size,omitempty"` // Maximum file size in bytes
	MaxFiles    *int     `json:"max_files,omitempty" yaml:"max_files,omitempty"`         // Maximum number of files per request
}

// ModelPricing represents the pricing structure for a model.
type ModelPricing struct {
	// Token-based costs
	Tokens *TokenPricing `json:"tokens,omitempty" yaml:"tokens,omitempty"`

	// Fixed costs per operation
	Operations *OperationPricing `json:"operations,omitempty" yaml:"operations,omitempty"`

	// Metadata
	Currency string `json:"currency" yaml:"currency"` // "USD", "EUR", etc.
}

// TokenPricing represents all token-based costs.
type TokenPricing struct {
	// Core tokens
	Input  *TokenCost `json:"input,omitempty" yaml:"input,omitempty"`   // Input/prompt tokens
	Output *TokenCost `json:"output,omitempty" yaml:"output,omitempty"` // Standard output tokens

	// Advanced token types
	Reasoning *TokenCost `json:"reasoning,omitempty" yaml:"reasoning,omitempty"` // Internal reasoning tokens
	Cache     *CacheCost `json:"cache,omitempty" yaml:"cache,omitempty"`         // Cache operations

	// Alternative flat cache structure (for backward compatibility)
	CacheRead  *TokenCost `json:"cache_read,omitempty" yaml:"cache_read,omitempty"`   // Cache read costs (flat structure)
	CacheWrite *TokenCost `json:"cache_write,omitempty" yaml:"cache_write,omitempty"` // Cache write costs (flat structure)
}

// TokenCost represents cost per token with flexible units.
type TokenCost struct {
	PerToken float64 `json:"per_token" yaml:"per_token"`  // Cost per individual token
	Per1M    float64 `json:"per_1m_tokens" yaml:"per_1m"` // Cost per 1M tokens
}

// CacheCost represents cache-specific pricing.
type CacheCost struct {
	Read  *TokenCost `json:"read,omitempty" yaml:"read,omitempty"`   // Cache read costs
	Write *TokenCost `json:"write,omitempty" yaml:"write,omitempty"` // Cache write costs
}

// OperationPricing represents fixed costs for operations.
type OperationPricing struct {
	// Core operations
	Request *float64 `json:"request,omitempty" yaml:"request,omitempty"` // Cost per API request

	// Media operations
	ImageInput *float64 `json:"image_input,omitempty" yaml:"image_input,omitempty"` // Cost per image processed
	AudioInput *float64 `json:"audio_input,omitempty" yaml:"audio_input,omitempty"` // Cost per audio input
	VideoInput *float64 `json:"video_input,omitempty" yaml:"video_input,omitempty"` // Cost per video input

	// Generation operations
	ImageGen *float64 `json:"image_gen,omitempty" yaml:"image_gen,omitempty"` // Cost per image generated
	AudioGen *float64 `json:"audio_gen,omitempty" yaml:"audio_gen,omitempty"` // Cost per audio generated
	VideoGen *float64 `json:"video_gen,omitempty" yaml:"video_gen,omitempty"` // Cost per video generated

	// Service operations
	WebSearch    *float64 `json:"web_search,omitempty" yaml:"web_search,omitempty"`       // Cost per web search
	FunctionCall *float64 `json:"function_call,omitempty" yaml:"function_call,omitempty"` // Cost per function call
	ToolUse      *float64 `json:"tool_use,omitempty" yaml:"tool_use,omitempty"`           // Cost per tool usage
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
