package oci

import (
	"testing"

	"github.com/agentstation/starmap/internal/providerdata"
)

func TestExactTokenPricing(t *testing.T) {
	tests := []struct {
		id       string
		input    float64
		output   float64
		cache    float64
		tierSize int64
	}{
		{id: "google.gemini-2.5-pro", input: 1.25, output: 10, tierSize: 200000},
		{id: "google.gemini-2.5-flash", input: 0.30, output: 2.50},
		{id: "openai.gpt-oss-120b", input: 0.15, output: 0.60},
		{id: "openai.gpt-oss-20b", input: 0.07, output: 0.30},
		{id: "xai.grok-4.3", input: 1.25, output: 2.50, cache: 0.20, tierSize: 200000},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			catalog, err := providerdata.LoadPricingCatalog(ProviderID)
			if err != nil {
				t.Fatalf("LoadPricingCatalog: %v", err)
			}
			pricing := catalog.Models[test.id].Pricing
			if pricing == nil || pricing.Tokens.Input.Per1M != test.input || pricing.Tokens.Output.Per1M != test.output {
				t.Fatalf("pricing = %#v", pricing)
			}
			if test.cache > 0 && (pricing.Tokens.CacheRead == nil || pricing.Tokens.CacheRead.Per1M != test.cache) {
				t.Fatalf("cache pricing = %#v", pricing.Tokens)
			}
			if test.tierSize > 0 && (len(pricing.Tiers) != 1 || pricing.Tiers[0].Size != test.tierSize) {
				t.Fatalf("tier pricing = %#v", pricing.Tiers)
			}
		})
	}
	catalog, err := providerdata.LoadPricingCatalog(ProviderID)
	if err != nil {
		t.Fatalf("LoadPricingCatalog: %v", err)
	}
	if _, found := catalog.Models["unknown.model"]; found {
		t.Fatal("unknown model inherited pricing")
	}
}
