package bootstrap

import (
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestWhisperPublishesSpeechRecognition(t *testing.T) {
	catalog, _, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	offering, err := catalog.Offering(catalogs.ProviderIDOpenAI, "whisper-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []catalogs.ProviderOperation{
		catalogs.ProviderOperationAudioTranscriptions,
		catalogs.ProviderOperationAudioTranslations,
	} {
		if !offering.Supports(operation) {
			t.Errorf("Whisper does not publish %s", operation)
		}
	}
	if offering.Supports(catalogs.ProviderOperationChatCompletions) {
		t.Error("Whisper must not publish chat completions")
	}
	definition, err := catalog.Definition(offering.DefinitionID)
	if err != nil {
		t.Fatal(err)
	}
	features := definition.Capabilities.Features
	if features == nil || !slices.Contains(features.Modalities.Input, catalogs.ModelModalityAudio) ||
		!slices.Equal(features.Modalities.Output, []catalogs.ModelModality{catalogs.ModelModalityText}) || features.Streaming {
		t.Errorf("Whisper must declare audio input, text output, and no streaming: %+v", features)
	}
}
