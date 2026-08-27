package catalogs

import (
	"slices"
	"testing"
)

func mediaModel(in, out []ModelModality, tags ...ModelTag) Model {
	model := Model{
		Features: &ModelFeatures{
			Modalities: ModelModalities{Input: in, Output: out},
		},
	}
	if len(tags) > 0 {
		model.Metadata = &ModelMetadata{Tags: tags}
	}
	return model
}

// TestMediaOperationsFollowModalityAndTag states the whole derivation rule. The
// discriminator is the exact output set, because a model that also writes text
// answers through chat completions. Transcription is the one shape modalities
// cannot name on their own, so it needs the tag.
func TestMediaOperationsFollowModalityAndTag(t *testing.T) {
	text := []ModelModality{ModelModalityText}
	for _, test := range []struct {
		name  string
		model Model
		want  []ProviderOperation
	}{
		{
			name:  "text to image serves generation alone",
			model: mediaModel(text, []ModelModality{ModelModalityImage}, "image-gen"),
			want:  []ProviderOperation{ProviderOperationImagesGenerations},
		},
		{
			name: "an image that also reads an image serves editing too",
			model: mediaModel(
				[]ModelModality{ModelModalityText, ModelModalityImage},
				[]ModelModality{ModelModalityImage},
				ModelTagTextToImage,
			),
			want: []ProviderOperation{
				ProviderOperationImagesGenerations,
				ProviderOperationImagesEdits,
			},
		},
		{
			name:  "text to audio serves speech, with or without the tag",
			model: mediaModel(text, []ModelModality{ModelModalityAudio}),
			want:  []ProviderOperation{ProviderOperationAudioSpeech},
		},
		{
			name: "audio to text serves both transcription paths",
			model: mediaModel(
				[]ModelModality{ModelModalityText, ModelModalityAudio},
				text,
				"stt",
			),
			want: []ProviderOperation{
				ProviderOperationAudioTranscriptions,
				ProviderOperationAudioTranslations,
			},
		},
		{
			name: "an untagged model that hears is a chat model, not a transcriber",
			model: mediaModel(
				[]ModelModality{ModelModalityText, ModelModalityAudio},
				text,
			),
			want: nil,
		},
		{
			name: "a chat model that returns a picture serves no image path",
			model: mediaModel(
				[]ModelModality{ModelModalityText, ModelModalityImage},
				[]ModelModality{ModelModalityText, ModelModalityImage},
				ModelTagTextToImage,
			),
			want: nil,
		},
		{
			name:  "text in and video out is video generation",
			model: mediaModel(text, []ModelModality{ModelModalityVideo}, "video-gen"),
			want:  []ProviderOperation{ProviderOperationVideosGenerations},
		},
		{
			name: "a tagged pdf reader serves recognition",
			model: mediaModel(
				[]ModelModality{ModelModalityText, ModelModalityImage, ModelModalityPDF},
				text,
				"ocr",
			),
			want: []ProviderOperation{ProviderOperationDocumentsRecognition},
		},
		{
			name: "an untagged pdf reader is a chat model, not a recognizer",
			model: mediaModel(
				[]ModelModality{ModelModalityText, ModelModalityPDF},
				text,
			),
			want: nil,
		},
		{
			name: "a model that also writes text serves no video path",
			model: mediaModel(
				text,
				[]ModelModality{ModelModalityText, ModelModalityVideo},
				"video-gen",
			),
			want: nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := mediaOperations(test.model)
			if !slices.Equal(got, test.want) {
				t.Fatalf("operations = %v, want %v", got, test.want)
			}
		})
	}
}

// TestMediaOperationsIgnorePrice holds the rule pull request 102 established. A
// price describes what a turn costs. It never decides what a model serves, and
// a media model that charges nothing serves the same operation as one that
// charges.
func TestMediaOperationsIgnorePrice(t *testing.T) {
	free := mediaModel(
		[]ModelModality{ModelModalityText},
		[]ModelModality{ModelModalityImage},
		"image-gen",
	)
	priced := free
	priced.Pricing = &ModelPricing{
		Operations: &ModelOperationPricing{ImageGen: price(0.04)},
	}
	if !slices.Equal(mediaOperations(free), mediaOperations(priced)) {
		t.Fatalf("free = %v, priced = %v", mediaOperations(free), mediaOperations(priced))
	}
}

// TestAMediaModelReachesItsOfferingOperations is the acceptance case for the
// whole task. Before it, a dedicated media model derived no operation, so it
// derived no endpoint and resolved to no route.
func TestAMediaModelReachesItsOfferingOperations(t *testing.T) {
	speech := mediaModel(
		[]ModelModality{ModelModalityText},
		[]ModelModality{ModelModalityAudio},
		"tts",
	)
	got := deriveOfferingCapabilities(speech).Operations
	want := []ProviderOperation{ProviderOperationAudioSpeech}
	if !slices.Equal(got, want) {
		t.Fatalf("operations = %v, want %v", got, want)
	}
}

// TestMediaFactsRefuseAContradictoryTag states the consistency rule. A tag that
// names an operation while the modalities beside it name another is a catalog
// defect, and it is one the derivation would answer by publishing the wrong
// operation rather than by failing.
func TestMediaFactsRefuseAContradictoryTag(t *testing.T) {
	for _, test := range []struct {
		name  string
		model Model
	}{
		{
			name: "a speech model that claims a text answer",
			model: mediaModel(
				[]ModelModality{ModelModalityText},
				[]ModelModality{ModelModalityText, ModelModalityAudio},
				"tts",
			),
		},
		{
			name: "an image model that claims a text answer",
			model: mediaModel(
				[]ModelModality{ModelModalityText},
				[]ModelModality{ModelModalityText, ModelModalityImage},
				"image-gen",
			),
		},
		{
			name: "a recognizer that reads no document",
			model: mediaModel(
				[]ModelModality{ModelModalityText, ModelModalityImage},
				[]ModelModality{ModelModalityText},
				"ocr",
			),
		},
		{
			name: "a transcriber that reads no audio",
			model: mediaModel(
				[]ModelModality{ModelModalityText},
				[]ModelModality{ModelModalityText},
				"stt",
			),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateModelFactConsistency(test.model); err == nil {
				t.Fatal("validateModelFactConsistency accepted contradictory facts")
			}
		})
	}
}

// TestEveryMediaOperationIsAValidCatalogOperation keeps the published set and
// the validated set in step. A member the validator refuses would derive an
// offering that then fails its own validation.
func TestEveryMediaOperationIsAValidCatalogOperation(t *testing.T) {
	definitions := MediaOperationDefinitions()
	if len(definitions) == 0 {
		t.Fatal("no media operations are defined")
	}
	for _, facts := range definitions {
		if !validProviderOperation(facts.Operation) {
			t.Fatalf("%s is not a valid provider operation", facts.Operation)
		}
		if !IsMediaOperation(facts.Operation) {
			t.Fatalf("%s is not reported as a media operation", facts.Operation)
		}
		if len(facts.Output) == 0 {
			t.Fatalf("%s names no output modality", facts.Operation)
		}
	}
}

// TestTheRecognitionOperationDecodesFromItsWireValue holds the name a catalog
// writes and a consumer reads.
//
// The constant and the wire value are two different things, and only the wire
// value crosses a process boundary. A rename of the constant is a compile
// error. A change to the string it carries is a silent one that leaves every
// shipped catalog naming an operation the reader no longer resolves.
func TestTheRecognitionOperationDecodesFromItsWireValue(t *testing.T) {
	const wire = "documents-recognition"

	if string(ProviderOperationDocumentsRecognition) != wire {
		t.Fatalf("wire value = %q, want %q", ProviderOperationDocumentsRecognition, wire)
	}
	decoded := ProviderOperation(wire)
	if !validProviderOperation(decoded) {
		t.Fatalf("%q is not a valid provider operation", wire)
	}
	facts, found := MediaOperationDefinition(decoded)
	if !found {
		t.Fatalf("%q resolves to no media operation definition", wire)
	}
	if !slices.Contains(facts.Input, ModelModalityPDF) {
		t.Fatalf("recognition reads %v, want it to read a document", facts.Input)
	}
	if !slices.Equal(facts.Output, []ModelModality{ModelModalityText}) {
		t.Fatalf("recognition writes %v, want text alone", facts.Output)
	}
	if !facts.TagRequired {
		t.Fatal("recognition must require a tag: a chat model that reads a document has the same modalities")
	}
}

// TestAPagePriceIsAValidatedPriceOfItsOwn covers the unit a recognition
// provider bills.
//
// A page is not a token. A provider that reads a document charges the same for
// a page of dense text and a page of white space, so a catalog that carried
// only token prices could state no recognition cost at all. This asserts the
// page price counts as a price, survives a round trip, and is refused when it
// is not a price a caller can be charged.
func TestAPagePriceIsAValidatedPriceOfItsOwn(t *testing.T) {
	pricing := &ModelPricing{
		Operations: &ModelOperationPricing{PageInput: price(0.001)},
		Currency:   "USD",
	}
	if err := pricing.Validate(); err != nil {
		t.Fatalf("a page price alone is not a price: %v", err)
	}

	copied := deepCopyModelPricing(pricing)
	if copied.Operations.PageInput == nil {
		t.Fatal("the page price did not survive a copy")
	}
	if *copied.Operations.PageInput != 0.001 {
		t.Fatalf("copied page price = %v, want 0.001", *copied.Operations.PageInput)
	}
	if copied.Operations.PageInput == pricing.Operations.PageInput {
		t.Fatal("the copy shares the original's pointer")
	}

	negative := &ModelPricing{
		Operations: &ModelOperationPricing{PageInput: price(-0.001)},
		Currency:   "USD",
	}
	if err := negative.Validate(); err == nil {
		t.Fatal("a negative page price was accepted")
	}
}
