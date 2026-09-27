package embedded_test

import (
	"testing"

	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestEmbeddedImageBillingContract(t *testing.T) {
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
		basis catalogs.ImageBillingBasis
		price float64
	}{
		{"black-forest-labs/FLUX-1.1-pro", catalogs.ImageBillingImages, 0.04},
		{"black-forest-labs/FLUX-1-schnell", catalogs.ImageBillingPixelIterations, 0.0005},
		{"black-forest-labs/FLUX-2-klein-4b", catalogs.ImageBillingPixelIterations, 0.014},
		{"black-forest-labs/FLUX-2-klein-9b", catalogs.ImageBillingPixelIterations, 0.015},
	} {
		offering, err := catalog.Offering("deepinfra", tc.model)
		if err != nil {
			t.Fatal(err)
		}
		if offering.Billing == nil || offering.Billing.Validate() != nil || offering.Billing.Images == nil || offering.Billing.Images.Basis != tc.basis {
			t.Fatalf("missing image contract: %s", tc.model)
		}
		if offering.Pricing == nil || offering.Pricing.Operations == nil || offering.Pricing.Operations.ImageGen != nil || offering.Pricing.Operations.ImageUnit == nil || *offering.Pricing.Operations.ImageUnit != tc.price {
			t.Fatalf("incorrect image price: %s", tc.model)
		}
		if _, ok := offering.Endpoint(catalogs.ProviderOperationImagesGenerations); !ok {
			t.Fatalf("missing image endpoint: %s", tc.model)
		}
	}
}
