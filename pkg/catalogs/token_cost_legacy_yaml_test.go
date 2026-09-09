package catalogs

import (
	"encoding/json"
	"github.com/goccy/go-yaml"
	"testing"
)

func TestLegacyFreeTokenCostYAMLRemainsValid(t *testing.T) {
	// The previous encoder emitted an empty mapping for a non-nil zero cost.
	var pricing ModelPricing
	if err := yaml.Unmarshal([]byte("currency: USD\ntokens:\n  input: {}\n  output: {}\n"), &pricing); err != nil {
		t.Fatal(err)
	}
	if err := pricing.Validate(); err != nil {
		t.Fatalf("legacy free price rejected: %v", err)
	}
	for _, cost := range []*ModelTokenCost{pricing.Tokens.Input, pricing.Tokens.Output} {
		for _, unit := range []TokenCostUnit{CostUnitPerToken, CostUnitPerMillion} {
			if value, state := cost.Amount(unit); value != 0 || state != ValueKnown {
				t.Fatalf("legacy zero became (%v,%v)", value, state)
			}
		}
	}
}

func TestMissingTokenCostCannotEncodeLegacyFreeYAML(t *testing.T) {
	var cost ModelTokenCost
	if err := json.Unmarshal([]byte(`{}`), &cost); err != nil {
		t.Fatal(err)
	}
	if _, err := yaml.Marshal(cost); err == nil {
		t.Fatal("missing price encoded as the legacy free-price mapping")
	}
	if err := (&ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{Input: &cost}}).Validate(); err == nil {
		t.Fatal("missing JSON cost became free")
	}
}
