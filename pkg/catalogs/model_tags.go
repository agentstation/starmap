package catalogs

// ModelTag represents a use case or category tag for models.
type ModelTag string

// String returns the string representation of a ModelTag.
func (tag ModelTag) String() string {
	return string(tag)
}

// Model tags for categorizing models by use case and capabilities.
const (
	// Core Use Cases.
	ModelTagCoding    ModelTag = "coding"    // Programming and code generation
	ModelTagWriting   ModelTag = "writing"   // Creative and technical writing
	ModelTagReasoning ModelTag = "reasoning" // Logical reasoning and problem solving
	ModelTagMath      ModelTag = "math"      // Mathematical problem solving
	ModelTagChat      ModelTag = "chat"      // Conversational AI
	ModelTagInstruct  ModelTag = "instruct"  // Instruction following
	ModelTagResearch  ModelTag = "research"  // Research and analysis
	ModelTagCreative  ModelTag = "creative"  // Creative content generation
	ModelTagRoleplay  ModelTag = "roleplay"  // Character roleplay and simulation

	// Technical Capabilities.
	ModelTagFunctionCalling ModelTag = "function_calling"   // Tool/function calling
	ModelTagEmbedding       ModelTag = "embedding"          // Text embeddings
	ModelTagSummarization   ModelTag = "summarization"      // Text summarization
	ModelTagTranslation     ModelTag = "translation"        // Language translation
	ModelTagQA              ModelTag = "question_answering" // Question answering

	// Modality-Specific.
	ModelTagVision       ModelTag = "vision"         // Computer vision
	ModelTagMultimodal   ModelTag = "multimodal"     // Multiple input modalities
	ModelTagAudio        ModelTag = "audio"          // Audio processing
	ModelTagTextToImage  ModelTag = "text_to_image"  // Text-to-image generation
	ModelTagTextToSpeech ModelTag = "text_to_speech" // Text-to-speech synthesis
	ModelTagSpeechToText ModelTag = "speech_to_text" // Speech recognition
	ModelTagImageToText  ModelTag = "image_to_text"  // Image captioning/OCR

	// Domain-Specific.
	ModelTagMedical   ModelTag = "medical"   // Medical and healthcare
	ModelTagLegal     ModelTag = "legal"     // Legal document processing
	ModelTagFinance   ModelTag = "finance"   // Financial analysis
	ModelTagScience   ModelTag = "science"   // Scientific applications
	ModelTagEducation ModelTag = "education" // Educational content
)
