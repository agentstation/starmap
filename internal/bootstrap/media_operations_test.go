package bootstrap

import (
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestEveryPublishedMediaOperationMatchesItsDefinition cross-checks the shipped
// catalog against the canonical media-operation table. The table is what the
// derivation reads, so this test is the one place that proves the shipped bytes
// agree with it rather than with an earlier version of it.
func TestEveryPublishedMediaOperationMatchesItsDefinition(t *testing.T) {
	builder, err := NewEmbeddedBuilder()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	counts := map[catalogs.ProviderOperation]int{}
	checked := 0
	for _, provider := range catalog.Providers().List() {
		offerings, err := catalog.ProviderOfferings(provider.ID)
		if err != nil {
			t.Fatalf("ProviderOfferings(%s): %v", provider.ID, err)
		}
		for _, offering := range offerings {
			for _, operation := range offering.Service.Operations {
				facts, found := catalogs.MediaOperationDefinition(operation)
				if !found {
					continue
				}
				counts[operation]++
				checked++
				model := provider.Models[string(offering.ProviderModelID)]
				if model == nil {
					t.Fatalf(
						"%s/%s publishes %s with no model",
						provider.ID,
						offering.ProviderModelID,
						operation,
					)
				}
				if !facts.Matches(*model) {
					t.Fatalf(
						"%s/%s publishes %s, but its facts are input %v output %v",
						provider.ID,
						offering.ProviderModelID,
						operation,
						model.Features.Modalities.Input,
						model.Features.Modalities.Output,
					)
				}
				if slices.Contains(offering.Service.Operations, catalogs.ProviderOperationChatCompletions) {
					t.Fatalf(
						"%s/%s publishes both %s and chat completions",
						provider.ID,
						offering.ProviderModelID,
						operation,
					)
				}
			}
		}
	}

	if checked == 0 {
		t.Fatal("the shipped catalog publishes no media operation")
	}
	// The census MOD12 records. A change here is a real catalog change, and the
	// proof file states what each number means.
	want := map[catalogs.ProviderOperation]int{
		catalogs.ProviderOperationImagesGenerations:   26,
		catalogs.ProviderOperationAudioSpeech:         14,
		catalogs.ProviderOperationAudioTranscriptions: 7,
		catalogs.ProviderOperationAudioTranslations:   7,
		catalogs.ProviderOperationVideosGenerations:   13,
	}
	for operation, wantCount := range want {
		if counts[operation] != wantCount {
			t.Fatalf("%s offerings = %d, want %d", operation, counts[operation], wantCount)
		}
	}
	if counts[catalogs.ProviderOperationImagesEdits] != 0 {
		t.Fatalf(
			"images-edits offerings = %d, want 0; no shipped model reads an image and answers with one alone",
			counts[catalogs.ProviderOperationImagesEdits],
		)
	}
}

// TestTheResidualOfferingsAreRealtimeAlone names every offering the shipped
// catalog still leaves without an operation. MOD0 counted 63 of them, and MOD12
// left 16: 13 that generate video and 3 that serve a realtime session. AMJ3
// gave the video ones an operation, so only the realtime shape remains, and
// holding the number here is what makes a new residual visible.
func TestTheResidualOfferingsAreRealtimeAlone(t *testing.T) {
	builder, err := NewEmbeddedBuilder()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	video, realtime, other := 0, 0, 0
	for _, provider := range catalog.Providers().List() {
		offerings, err := catalog.ProviderOfferings(provider.ID)
		if err != nil {
			t.Fatalf("ProviderOfferings(%s): %v", provider.ID, err)
		}
		for _, offering := range offerings {
			if len(offering.Service.Operations) > 0 {
				continue
			}
			model := provider.Models[string(offering.ProviderModelID)]
			if model == nil || model.Features == nil {
				other++
				continue
			}
			output := model.Features.Modalities.Output
			switch {
			case slices.Contains(output, catalogs.ModelModalityVideo):
				video++
			case slices.Contains(output, catalogs.ModelModalityAudio) &&
				slices.Contains(output, catalogs.ModelModalityText):
				realtime++
			default:
				t.Fatalf(
					"%s/%s has no operation and no recorded reason: input %v output %v",
					provider.ID,
					offering.ProviderModelID,
					model.Features.Modalities.Input,
					output,
				)
			}
		}
	}

	if video != 0 || realtime != 3 || other != 0 {
		t.Fatalf(
			"residual offerings: video = %d want 0, realtime = %d want 3, unexplained = %d want 0",
			video,
			realtime,
			other,
		)
	}
}
