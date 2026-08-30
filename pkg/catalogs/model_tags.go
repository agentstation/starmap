package catalogs

// ModelTag represents a use case or category tag for models.
type ModelTag string

// String returns text for ModelTag.
func (tag ModelTag) String() string {
	return string(tag)
}

// Model tags for categorizing models by use case and capabilities.
const (
	// Core Use Cases.
	ModelTagCoding    ModelTag = "coding"
	ModelTagWriting   ModelTag = "writing"
	ModelTagReasoning ModelTag = "reasoning"
	ModelTagMath      ModelTag = "math"
	ModelTagChat      ModelTag = "chat"
	ModelTagInstruct  ModelTag = "instruct"
	ModelTagResearch  ModelTag = "research"
	ModelTagCreative  ModelTag = "creative"
	ModelTagRoleplay  ModelTag = "roleplay"

	// Technical Capabilities.
	ModelTagFunctionCalling ModelTag = "function_calling"   // Tool/function calling
	ModelTagEmbedding       ModelTag = "embedding"          // Text embeddings
	ModelTagRerank          ModelTag = "rerank"             // Relevance ranking of documents
	ModelTagModeration      ModelTag = "moderation"         // Harm-category classification
	ModelTagSummarization   ModelTag = "summarization"      // Text summarization
	ModelTagTranslation     ModelTag = "translation"        // Language translation
	ModelTagQA              ModelTag = "question_answering" // Question answering

	// Modality-Specific.
	ModelTagVision       ModelTag = "vision"         // Computer vision
	ModelTagMultimodal   ModelTag = "multimodal"     // Multiple input modalities
	ModelTagAudio        ModelTag = "audio"          // Audio processing
	ModelTagTextToImage  ModelTag = "text_to_image"  // Text-to-image generation
	ModelTagTextToVideo  ModelTag = "text_to_video"  // Text-to-video generation
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
