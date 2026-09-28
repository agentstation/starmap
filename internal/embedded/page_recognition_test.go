package embedded_test

import (
	"testing"

	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestEmbeddedPageRecognitionContract(t *testing.T) {
	builder, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	offering, err := catalog.Offering(catalogs.ProviderIDMistralAI, "mistral-ocr-4-0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint, found := offering.Endpoint(catalogs.ProviderOperationDocumentsRecognition)
	if !found || endpoint.Type != catalogs.EndpointTypeMistralOCR || endpoint.URL != "https://api.mistral.ai/v1/ocr" {
		t.Fatalf("recognition endpoint: %#v", endpoint)
	}
	if offering.Billing == nil || offering.Billing.Validate() != nil || offering.Billing.Recognition == nil {
		t.Fatal("missing valid billing contract")
	}
	billing := offering.Billing.Recognition
	if billing.Basis != catalogs.RecognitionBillingPages || billing.RequestCharge == nil || *billing.RequestCharge {
		t.Fatalf("billing: %#v", billing)
	}
	if offering.Pricing == nil || offering.Pricing.Currency != catalogs.ModelPricingCurrencyUSD || offering.Pricing.Operations == nil || offering.Pricing.Operations.PageInput == nil || *offering.Pricing.Operations.PageInput != 0.004 {
		t.Fatalf("pricing: %#v", offering.Pricing)
	}
	if _, found := offering.Endpoint(catalogs.ProviderOperationChatCompletions); found {
		t.Fatal("OCR-only model advertises chat")
	}
}
