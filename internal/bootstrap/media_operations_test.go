package bootstrap

import (
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestEveryPublishedMediaOperationMatchesItsDefinition(t *testing.T) {
	catalog, _, err := Embedded()
	if err != nil {
		t.Fatalf("Embedded: %v", err)
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

// TestTheResidualOfferingsAreRealtimeAlone requires explicit WebSocket delivery
// for each offering without a supported request operation.
func TestTheResidualOfferingsAreRealtimeAlone(t *testing.T) {
	catalog, _, err := Embedded()
	if err != nil {
		t.Fatalf("Embedded: %v", err)
	}
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
			if model == nil || model.Delivery == nil ||
				!slices.Equal(model.Delivery.Protocols, []catalogs.ModelResponseProtocol{catalogs.ModelResponseProtocolWebSocket}) {
				t.Fatalf("%s/%s has no operation and lacks exclusive WebSocket delivery", provider.ID, offering.ProviderModelID)
			}
		}
	}
}

// TestEveryRecognitionOfferingDeclaresActualBillingUnits checks usable prices
// in the declared unit. Token billing includes input and output. A display
// estimate cannot substitute for either rate.
func TestEveryRecognitionOfferingDeclaresActualBillingUnits(t *testing.T) {
	catalog, _, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, provider := range catalog.Providers().List() {
		offerings, err := catalog.ProviderOfferings(provider.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, offering := range offerings {
			if !offering.Supports(catalogs.ProviderOperationDocumentsRecognition) {
				continue
			}
			checked++
			name := string(provider.ID) + "/" + string(offering.ProviderModelID)
			if offering.Billing == nil || offering.Billing.Recognition == nil || offering.Pricing == nil {
				t.Fatalf("%s lacks billing units or prices", name)
			}
			switch offering.Billing.Recognition.Basis {
			case catalogs.RecognitionBillingPages:
				if offering.Pricing.Operations == nil || offering.Pricing.Operations.PageInput == nil {
					t.Fatalf("%s lacks a page rate", name)
				}
			case catalogs.RecognitionBillingTokens:
				if offering.Pricing.Tokens == nil || offering.Pricing.Tokens.Input == nil || offering.Pricing.Tokens.Output == nil {
					t.Fatalf("%s lacks input or output token rates", name)
				}
				if offering.Pricing.Operations != nil && offering.Pricing.Operations.PageInput != nil {
					t.Fatalf("%s presents an estimate as a fixed page charge", name)
				}
			default:
				t.Fatalf("%s has unknown billing units", name)
			}
			if offering.Limits == nil || offering.Limits.DocumentPages <= 0 {
				t.Fatalf("%s has no page limit", name)
			}
			if _, found := offering.Endpoint(catalogs.ProviderOperationDocumentsRecognition); !found {
				t.Fatalf("%s has no recognition endpoint", name)
			}
		}
	}
	if checked != 11 {
		t.Fatalf("recognition offerings = %d, want 11", checked)
	}
}
