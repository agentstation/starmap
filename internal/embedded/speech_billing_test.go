package embedded_test

import (
	"testing"

	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestEmbeddedSpeechBillingContract(t *testing.T) {
	builder, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		model catalogs.ProviderModelID
		rate  float64
	}{{"tts-1", 0.000015}, {"tts-1-hd", 0.00003}} {
		offering, err := catalog.Offering(catalogs.ProviderIDOpenAI, tc.model)
		if err != nil {
			t.Fatal(err)
		}
		if offering.Billing == nil || offering.Billing.Validate() != nil || offering.Billing.Speech == nil || offering.Billing.Speech.MaxInputCharacters != 4096 || *offering.Billing.Speech.RequestCharge {
			t.Fatalf("missing complete speech contract: %s", tc.model)
		}
		if offering.Pricing == nil || offering.Pricing.Operations == nil || offering.Pricing.Operations.CharacterInput == nil || *offering.Pricing.Operations.CharacterInput != tc.rate {
			t.Fatalf("missing character price: %s", tc.model)
		}
		if _, ok := offering.Endpoint(catalogs.ProviderOperationAudioSpeech); !ok {
			t.Fatalf("missing speech endpoint: %s", tc.model)
		}
		if _, ok := offering.Endpoint(catalogs.ProviderOperationChatCompletions); ok {
			t.Fatalf("speech model advertises chat: %s", tc.model)
		}
	}
}
