package azurefoundry

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestApplyPricingRetainsRegionAndSKUMode(t *testing.T) {
	offerings := []catalogs.ProviderOffering{{
		ProviderID: ProviderID, ProviderModelID: "gpt-4.1@2025-04-14", DefinitionID: "openai/gpt-4.1",
		Availability: catalogs.OfferingAvailabilityRestricted,
		Access:       catalogs.OfferingAccess{Channel: catalogs.OfferingAccessChannelServerToServer, Routability: catalogs.OfferingRoutabilityDiscoverable},
		Regions:      []catalogs.CloudRegion{{ID: "eastus", Realm: "azure-public"}}, Deployment: catalogs.ProviderDeployment{Type: "customer_deployment"}, Lifecycle: catalogs.OfferingLifecycleActive,
		Modes: map[string]catalogs.ProviderOfferingMode{"GlobalStandard": {}},
	}}
	catalog := pricingCatalog{ObservedAt: time.Now(), Meters: []retailMeter{
		{ServiceName: foundryModelsService, ProductName: "Azure OpenAI", SKUName: "gpt 4.1 Inp regnl", MeterName: "gpt 4.1 Inp regnl Tokens", Region: "eastus", Unit: "1K", Type: "Consumption", Currency: "USD", RetailPrice: 0.0022},
		{ServiceName: foundryModelsService, ProductName: "Azure OpenAI", SKUName: "gpt 4.1 Outp regnl", MeterName: "gpt 4.1 Outp regnl Tokens", Region: "eastus", Unit: "1K", Type: "Consumption", Currency: "USD", RetailPrice: 0.0088},
		{ServiceName: foundryModelsService, SKUName: "file-search", MeterName: "file-search Calls", Region: "eastus", Unit: "1K", Type: "Consumption", Currency: "USD", RetailPrice: 2.75},
	}}
	matched, ignored, err := applyPricing(offerings, catalog)
	if err != nil {
		t.Fatalf("applyPricing: %v", err)
	}
	if matched != 2 || ignored != 1 {
		t.Fatalf("matched=%d ignored=%d", matched, ignored)
	}
	for name, mode := range offerings[0].Modes {
		if name == "GlobalStandard" {
			continue
		}
		if mode.Pricing == nil || mode.Pricing.Tokens.Input == nil || mode.Pricing.Tokens.Output == nil {
			t.Fatalf("mode %q pricing=%#v", name, mode.Pricing)
		}
		if mode.Pricing.Tokens.Input.Per1M != 2.2 || mode.Pricing.Tokens.Output.Per1M != 8.8 {
			t.Fatalf("mode %q pricing=%#v", name, mode.Pricing.Tokens)
		}
	}
}

func TestLiveRetailPricing(t *testing.T) {
	if os.Getenv("STARMAP_LIVE_AZURE_PRICING") != "1" {
		t.Skip("set STARMAP_LIVE_AZURE_PRICING=1 to query the public Azure Retail Prices API")
	}
	catalog, err := newHTTPPricingFetcher().Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Meters) == 0 || catalog.ObservedAt.IsZero() {
		t.Fatalf("meters=%d observed_at=%v", len(catalog.Meters), catalog.ObservedAt)
	}
	tokenMeters := 0
	for _, meter := range catalog.Meters {
		if _, _, ok := classifyRetailMeter(meter); ok {
			tokenMeters++
		}
	}
	if tokenMeters == 0 {
		t.Fatal("live Azure price feed contained no supported USD token meters")
	}
	t.Logf("observed_at=%s meters=%d supported_token_meters=%d", catalog.ObservedAt.Format(time.RFC3339), len(catalog.Meters), tokenMeters)
}

func TestClassifyRetailMeterFailsClosed(t *testing.T) {
	tests := []retailMeter{
		{ServiceName: "Virtual Machines", SKUName: "gpt-4 inp", Unit: "1K", Type: "Consumption", Currency: "USD"},
		{ServiceName: foundryModelsService, SKUName: "gpt-4 inp", Unit: "1 Hour", Type: "Consumption", Currency: "USD"},
		{ServiceName: foundryModelsService, SKUName: "gpt-4 session", Unit: "1", Type: "Consumption", Currency: "USD"},
		{ServiceName: foundryModelsService, SKUName: "gpt-4 inp", Unit: "1K", Type: "Reservation", Currency: "USD"},
	}
	for _, meter := range tests {
		if _, _, ok := classifyRetailMeter(meter); ok {
			t.Fatalf("accepted %#v", meter)
		}
	}
}
