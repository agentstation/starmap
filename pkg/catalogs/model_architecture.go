package catalogs

// ModelArchitecture represents the technical architecture details of a model.
type ModelArchitecture struct {
	ParameterCount string           `json:"parameter_count,omitempty" yaml:"parameter_count,omitempty"`
	Type           ArchitectureType `json:"type,omitempty" yaml:"type,omitempty"`                 // Type of architecture
	Tokenizer      Tokenizer        `json:"tokenizer,omitempty" yaml:"tokenizer,omitempty"`       // Tokenizer type used by the model
	Quantization   Quantization     `json:"quantization,omitempty" yaml:"quantization,omitempty"` // Quantization level used by the model
	Quantized      bool             `json:"quantized" yaml:"quantized"`
	FineTuned      bool             `json:"fine_tuned" yaml:"fine_tuned"`                     // Whether this is a fine-tuned variant
	BaseModel      *string          `json:"base_model,omitempty" yaml:"base_model,omitempty"` // Base model ID if fine-tuned
}

// ArchitectureType represents the type of model architecture.
type ArchitectureType string

// String returns text for ArchitectureType.
func (at ArchitectureType) String() string {
	return string(at)
}

// Architecture types.
const (
	ArchitectureTypeTransformer ArchitectureType = "transformer"
	ArchitectureTypeMoE         ArchitectureType = "moe"
	ArchitectureTypeCNN         ArchitectureType = "cnn"
	ArchitectureTypeRNN         ArchitectureType = "rnn"
	ArchitectureTypeLSTM        ArchitectureType = "lstm"
	ArchitectureTypeGRU         ArchitectureType = "gru"
	ArchitectureTypeVAE         ArchitectureType = "vae"
	ArchitectureTypeGAN         ArchitectureType = "gan"
	ArchitectureTypeDiffusion   ArchitectureType = "diffusion"
)

// Tokenizer represents the tokenizer type used by a model.
type Tokenizer string

// String returns text for Tokenizer.
func (t Tokenizer) String() string {
	return string(t)
}

// Tokenizer types.
const (
	TokenizerClaude   Tokenizer = "claude"
	TokenizerCohere   Tokenizer = "cohere"
	TokenizerDeepSeek Tokenizer = "deepseek"
	TokenizerGPT      Tokenizer = "gpt"
	TokenizerGemini   Tokenizer = "gemini"
	TokenizerGrok     Tokenizer = "grok"
	TokenizerLlama2   Tokenizer = "llama2"
	TokenizerLlama3   Tokenizer = "llama3"
	TokenizerLlama4   Tokenizer = "llama4"
	TokenizerMistral  Tokenizer = "mistral"
	TokenizerNova     Tokenizer = "nova"
	TokenizerQwen     Tokenizer = "qwen"
	TokenizerQwen3    Tokenizer = "qwen3"
	TokenizerRouter   Tokenizer = "router"
	TokenizerYi       Tokenizer = "yi"
	TokenizerUnknown  Tokenizer = "unknown"
)

// Quantization represents the quantization level used by a model.
// Quantization reduces model size and computational requirements while aiming to preserve performance.
type Quantization string

// String returns text for Quantization.
func (q Quantization) String() string {
	return string(q)
}

// Quantization levels.
const (
	QuantizationINT4    Quantization = "int4"
	QuantizationINT8    Quantization = "int8"
	QuantizationFP4     Quantization = "fp4"
	QuantizationFP6     Quantization = "fp6"
	QuantizationFP8     Quantization = "fp8"
	QuantizationFP16    Quantization = "fp16"
	QuantizationBF16    Quantization = "bf16"
	QuantizationFP32    Quantization = "fp32"
	QuantizationUnknown Quantization = "unknown"
)
