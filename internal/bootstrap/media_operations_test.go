package bootstrap

import (
	"math"
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

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
				if servedThroughChat(operation) {
					continue
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
		catalogs.ProviderOperationImagesGenerations:    26,
		catalogs.ProviderOperationAudioSpeech:          14,
		catalogs.ProviderOperationAudioTranscriptions:  7,
		catalogs.ProviderOperationAudioTranslations:    7,
		catalogs.ProviderOperationVideosGenerations:    13,
		catalogs.ProviderOperationDocumentsRecognition: 11,
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

// servedThroughChat reports whether a provider serves a media operation on the
// same path it serves chat.
//
// Every other media operation reaches its own path. An offering that named one
// beside chat completions therefore had an incorrect fact. Document recognition
// is the exception. A provider reads a scanned page through the same model and
// request path as a chat turn. The operation remains distinct because consumers
// request it by name and pay by the page.
func servedThroughChat(operation catalogs.ProviderOperation) bool {
	return operation == catalogs.ProviderOperationDocumentsRecognition
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

// geminiTokensPerPage is the number of input tokens Google bills for one page
// of a document. Google publishes it at
// https://ai.google.dev/gemini-api/docs/document-processing.
const geminiTokensPerPage = 258

// TestEveryRecognitionOfferingCanBeBilledByThePage names the failure a refresh
// would otherwise ship silently.
//
// Starport cannot route a recognition request to an offering without a page
// price. Google publishes a fixed input-token count for each page instead of a
// page price. The catalog derives the page price from the model's input-token
// price. A refresh can change that token price while leaving the derived page
// price stale. This test detects that drift.
func TestEveryRecognitionOfferingCanBeBilledByThePage(t *testing.T) {
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
			if !slices.Contains(offering.Service.Operations, catalogs.ProviderOperationDocumentsRecognition) {
				continue
			}
			checked++
			name := string(provider.ID) + "/" + string(offering.ProviderModelID)

			if offering.Pricing == nil || offering.Pricing.Operations == nil ||
				offering.Pricing.Operations.PageInput == nil {
				t.Fatalf("%s serves recognition with no page price", name)
			}
			page := *offering.Pricing.Operations.PageInput
			if page <= 0 {
				t.Fatalf("%s prices a page at %v", name, page)
			}
			if offering.Limits == nil || offering.Limits.DocumentPages <= 0 {
				t.Fatalf("%s serves recognition and states no page limit", name)
			}
			if _, found := offering.Endpoint(catalogs.ProviderOperationDocumentsRecognition); !found {
				t.Fatalf("%s serves recognition and resolves to no endpoint", name)
			}

			if provider.ID != "google-ai-studio" {
				continue
			}
			if offering.Pricing.Tokens == nil || offering.Pricing.Tokens.Input == nil {
				t.Fatalf("%s derives a page price from an input price it does not carry", name)
			}
			want := geminiTokensPerPage * offering.Pricing.Tokens.Input.Per1M / 1_000_000
			if math.Abs(page-want) > 1e-12 {
				t.Fatalf(
					"%s prices a page at %v; %d tokens at %v per million is %v",
					name,
					page,
					geminiTokensPerPage,
					offering.Pricing.Tokens.Input.Per1M,
					want,
				)
			}
		}
	}

	// The census PLG3 records. Three Gemini models read a document and carry no
	// input price at all, so the catalog does not offer what it cannot bill.
	if checked != 11 {
		t.Fatalf("recognition offerings = %d, want 11", checked)
	}
}
