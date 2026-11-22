package bootstrap

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestDeepInfraVideoContractValidation(t *testing.T) {
	price := 0.075
	requestCharge := false
	offering := catalogs.ProviderOffering{
		ProviderID:      "deepinfra",
		ProviderModelID: "Wan-AI/Wan2.2-T2V-A14B",
		DefinitionID:    "alibaba/wan2.2-t2v-a14b",
		Availability:    catalogs.OfferingAvailabilityAvailable,
		Lifecycle:       catalogs.OfferingLifecycleActive,
		Service: catalogs.ProviderOfferingServiceCapabilities{
			Operations: []catalogs.ProviderOperation{catalogs.ProviderOperationVideosGenerations},
		},
		Endpoints: []catalogs.ProviderOfferingEndpoint{
			{
				Operation: catalogs.ProviderOperationVideosGenerations,
				Type:      "deepinfra-video",
				URL:       "https://api.deepinfra.com/v1/inference/Wan-AI/Wan2.2-T2V-A14B",
			},
		},
		Billing: &catalogs.ModelBilling{
			Videos: &catalogs.VideoBilling{
				Basis:          catalogs.VideoBillingOutputSeconds,
				DefaultSeconds: 5,
				DefaultSize:    "1280x720",
				Seconds:        []int64{5},
				Sizes:          []string{"1280x720"},
				RequestCharge:  &requestCharge,
			},
		},
		Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
			Operations: &catalogs.ModelOperationPricing{
				OutputSecond: &price,
			},
		},
	}
	if err := offering.Validate(); err != nil {
		t.Fatalf("offering.Validate: %v", err)
	}
	endpoint, found := offering.Endpoint(catalogs.ProviderOperationVideosGenerations)
	if !found || endpoint.Type != "deepinfra-video" || endpoint.URL != "https://api.deepinfra.com/v1/inference/Wan-AI/Wan2.2-T2V-A14B" {
		t.Fatalf("native video endpoint = %#v, found = %t", endpoint, found)
	}
	if offering.Billing == nil || offering.Billing.Videos == nil || offering.Billing.Videos.DefaultSeconds != 5 || offering.Billing.Videos.DefaultSize != "1280x720" {
		t.Fatal("measured model input contract missing")
	}
	if offering.Pricing == nil || offering.Pricing.Operations == nil || offering.Pricing.Operations.OutputSecond == nil || *offering.Pricing.Operations.OutputSecond != 0.075 {
		t.Fatal("output-second price missing")
	}
}

func TestEmbeddedVideoOfferingsHaveValidContracts(t *testing.T) {
	catalog, _, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	catalog.Providers().ForEach(func(providerID catalogs.ProviderID, provider *catalogs.Provider) bool {
		for modelID := range provider.Models {
			offering, err := catalog.Offering(providerID, catalogs.ProviderModelID(modelID))
			if err != nil {
				t.Fatalf("offering %s/%s: %v", providerID, modelID, err)
			}
			endpoint, found := offering.Endpoint(catalogs.ProviderOperationVideosGenerations)
			if !found {
				continue
			}
			checked++
			if endpoint.Type == "" || endpoint.URL == "" {
				t.Fatalf("%s/%s has incomplete video endpoint: %#v", providerID, modelID, endpoint)
			}
			if offering.Billing != nil && offering.Billing.Videos != nil {
				if err := offering.Billing.Validate(); err != nil {
					t.Fatalf("%s/%s invalid video billing: %v", providerID, modelID, err)
				}
			}
			if offering.Pricing != nil && offering.Pricing.Operations != nil && offering.Pricing.Operations.OutputSecond != nil && *offering.Pricing.Operations.OutputSecond < 0 {
				t.Fatalf("%s/%s negative output-second price: %v", providerID, modelID, *offering.Pricing.Operations.OutputSecond)
			}
		}
		return true
	})
	if checked == 0 {
		t.Fatal("embedded catalog has no video generation offerings")
	}
}
