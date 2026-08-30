package bootstrap

import (
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// nonChatTags name the model kinds that never answer a chat-completions
// request, whichever provider serves them.
var nonChatTags = []catalogs.ModelTag{
	"embed",
	catalogs.ModelTagEmbedding,
	"image-gen",
	"video-gen",
	"tts",
	"stt",
	catalogs.ModelTagRerank,
	catalogs.ModelTagModeration,
	catalogs.ModelTagTextToImage,
	catalogs.ModelTagTextToSpeech,
	catalogs.ModelTagSpeechToText,
}

// TestOfferedChatCompletionsAgreeWithCanonicalModelFacts cross-checks the two
// authorities that must not disagree. A provider offering claims the
// operations it serves; the canonical definition states what the model is.
// When an offering advertises chat-completions for a model the author
// classifies as an embedding, image, speech, or video model, the offering is
// unroutable in practice and every consumer that trusts it sends a request the
// provider rejects. Provider-level facts that omit the model's classification
// tag are the way this drift enters the catalog.
func TestOfferedChatCompletionsAgreeWithCanonicalModelFacts(t *testing.T) {
	builder, err := NewEmbeddedBuilder()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	checked := 0
	for _, provider := range catalog.Providers().List() {
		offerings, err := catalog.ProviderOfferings(provider.ID)
		if err != nil {
			t.Fatalf("ProviderOfferings(%s): %v", provider.ID, err)
		}
		for _, offering := range offerings {
			if !slices.Contains(offering.Service.Operations, catalogs.ProviderOperationChatCompletions) {
				continue
			}
			definition, err := catalog.Definition(offering.DefinitionID)
			if err != nil {
				t.Fatalf("Definition(%s): %v", offering.DefinitionID, err)
			}
			checked++
			for _, tag := range definition.Metadata.Tags {
				if slices.Contains(nonChatTags, tag) {
					t.Errorf(
						"%s/%s offers chat-completions but %s is tagged %q",
						offering.ProviderID, offering.ProviderModelID, definition.ID, tag,
					)
				}
			}
			features := definition.Capabilities.Features
			if features == nil || len(features.Modalities.Output) == 0 {
				continue
			}
			if !slices.Contains(features.Modalities.Output, catalogs.ModelModalityText) {
				t.Errorf(
					"%s/%s offers chat-completions but %s outputs %v",
					offering.ProviderID, offering.ProviderModelID,
					definition.ID, features.Modalities.Output,
				)
			}
		}
	}
	if checked == 0 {
		t.Fatal("embedded catalog published no chat-completions offerings to check")
	}
}
