package bootstrap

import (
	"github.com/agentstation/starmap/pkg/catalogs"
	"testing"
)

func TestWanVideoUsesMeasuredNativeContract(t *testing.T) {
	catalog, _, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}
	offering, err := catalog.Offering("deepinfra", "Wan-AI/Wan2.2-T2V-A14B")
	if err != nil {
		t.Fatal(err)
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
