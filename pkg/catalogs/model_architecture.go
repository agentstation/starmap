package catalogs

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
