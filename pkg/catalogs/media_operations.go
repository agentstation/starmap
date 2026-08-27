package catalogs

import "slices"

// MediaOperationFacts states the exact model facts one dedicated media
// operation requires. A dedicated media operation is one a consumer names for
// itself rather than one it discovers inside a chat answer, so a chat model
// that returns a picture is not one of these.
//
// Naming it is not the same as reaching a separate path. A provider that reads
// a document serves the read through its chat path, and the endpoint table says
// so. What makes the operation its own is that a consumer asks for it and pays
// for it in the operation's own unit.
//
// The derivation reads this table, and the fact-consistency rule enforces it.
// One statement therefore decides both what Starmap publishes and what Starmap
// refuses, and neither reads a price to do it.
type MediaOperationFacts struct {
	// Operation is the published operation name.
	Operation ProviderOperation
	// Tags name the operation on a model. Any one of them is enough.
	Tags []ModelTag
	// TagRequired means the modalities alone cannot identify the operation, so
	// a model has to carry one of the tags. Transcription reads audio and
	// writes text, which is also the shape of a chat model that hears.
	TagRequired bool
	// Input lists the input modalities the model must declare. It may declare
	// more.
	Input []ModelModality
	// Output is the exact output modality set. A model that also writes text
	// answers through chat completions rather than through a media path.
	Output []ModelModality
}

var mediaOperationFacts = []MediaOperationFacts{
	{
		Operation: ProviderOperationImagesGenerations,
		Tags:      []ModelTag{"image-gen", ModelTagTextToImage},
		Input:     []ModelModality{ModelModalityText},
		Output:    []ModelModality{ModelModalityImage},
	},
	{
		Operation: ProviderOperationImagesEdits,
		Tags:      []ModelTag{"image-gen", ModelTagTextToImage},
		Input:     []ModelModality{ModelModalityText, ModelModalityImage},
		Output:    []ModelModality{ModelModalityImage},
	},
	{
		Operation: ProviderOperationAudioSpeech,
		Tags:      []ModelTag{"tts", ModelTagTextToSpeech},
		Input:     []ModelModality{ModelModalityText},
		Output:    []ModelModality{ModelModalityAudio},
	},
	{
		Operation:   ProviderOperationAudioTranscriptions,
		Tags:        []ModelTag{"stt", ModelTagSpeechToText},
		TagRequired: true,
		Input:       []ModelModality{ModelModalityAudio},
		Output:      []ModelModality{ModelModalityText},
	},
	{
		Operation:   ProviderOperationAudioTranslations,
		Tags:        []ModelTag{"stt", ModelTagSpeechToText},
		TagRequired: true,
		Input:       []ModelModality{ModelModalityAudio},
		Output:      []ModelModality{ModelModalityText},
	},
	{
		Operation: ProviderOperationVideosGenerations,
		Tags:      []ModelTag{"video-gen", ModelTagTextToVideo},
		Input:     []ModelModality{ModelModalityText},
		Output:    []ModelModality{ModelModalityVideo},
	},
	{
		Operation:   ProviderOperationDocumentsRecognition,
		Tags:        []ModelTag{"ocr", ModelTagImageToText},
		TagRequired: true,
		Input:       []ModelModality{ModelModalityPDF},
		Output:      []ModelModality{ModelModalityText},
	},
}

// MediaOperationDefinitions returns the canonical facts for every dedicated
// media operation, in published order.
func MediaOperationDefinitions() []MediaOperationFacts {
	definitions := make([]MediaOperationFacts, 0, len(mediaOperationFacts))
	for _, facts := range mediaOperationFacts {
		definitions = append(definitions, facts.clone())
	}
	return definitions
}

// MediaOperationDefinition returns the canonical facts for one operation.
func MediaOperationDefinition(operation ProviderOperation) (MediaOperationFacts, bool) {
	for _, facts := range mediaOperationFacts {
		if facts.Operation == operation {
			return facts.clone(), true
		}
	}
	return MediaOperationFacts{}, false
}

// IsMediaOperation reports whether an operation names a dedicated media path.
func IsMediaOperation(operation ProviderOperation) bool {
	_, found := MediaOperationDefinition(operation)
	return found
}

func (f MediaOperationFacts) clone() MediaOperationFacts {
	copied := f
	copied.Tags = slices.Clone(f.Tags)
	copied.Input = slices.Clone(f.Input)
	copied.Output = slices.Clone(f.Output)
	return copied
}

// Matches reports whether a model declares the facts this operation requires.
func (f MediaOperationFacts) Matches(model Model) bool {
	if model.Features == nil {
		return false
	}
	if f.TagRequired && !f.tagged(model) {
		return false
	}
	for _, modality := range f.Input {
		if !slices.Contains(model.Features.Modalities.Input, modality) {
			return false
		}
	}
	return sameModalitySet(model.Features.Modalities.Output, f.Output)
}

func (f MediaOperationFacts) tagged(model Model) bool {
	if model.Metadata == nil {
		return false
	}
	for _, tag := range model.Metadata.Tags {
		if slices.Contains(f.Tags, tag) {
			return true
		}
	}
	return false
}

// mediaOperations names the dedicated media operations a model serves. It reads
// the declared modalities and the model tags, and it reads no price. A model
// that charges nothing still serves the operation its facts describe.
func mediaOperations(model Model) []ProviderOperation {
	var operations []ProviderOperation
	for _, facts := range mediaOperationFacts {
		if facts.Matches(model) {
			operations = append(operations, facts.Operation)
		}
	}
	return operations
}

func sameModalitySet(declared, required []ModelModality) bool {
	if len(declared) != len(required) {
		return false
	}
	for _, modality := range required {
		if !slices.Contains(declared, modality) {
			return false
		}
	}
	return true
}
