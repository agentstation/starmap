package catalogs

import (
	"math"
	"testing"
	"time"

	"github.com/agentstation/utc"
)

func TestPriceUnitCurrencyAndZeroValidation(t *testing.T) {
	tests := []struct {
		name    string
		pricing *ModelPricing
		wantErr bool
	}{
		{
			name: "per million",
			pricing: &ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{
				Input: &ModelTokenCost{Per1M: 2.5},
			}},
		},
		{
			name: "consistent units",
			pricing: &ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{
				Input: &ModelTokenCost{PerToken: 0.0000025, Per1M: 2.5},
			}},
		},
		{
			name: "explicit free price",
			pricing: &ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{
				Input: &ModelTokenCost{},
			}},
		},
		{
			name: "operation zero",
			pricing: &ModelPricing{Currency: ModelPricingCurrencyUSD, Operations: &ModelOperationPricing{
				Request: pricingFloat64Pointer(0),
			}},
		},
		{name: "missing currency", pricing: &ModelPricing{Tokens: &ModelTokenPricing{Input: &ModelTokenCost{Per1M: 1}}}, wantErr: true},
		{name: "malformed currency", pricing: &ModelPricing{Currency: "usd", Tokens: &ModelTokenPricing{Input: &ModelTokenCost{Per1M: 1}}}, wantErr: true},
		{name: "missing price", pricing: &ModelPricing{Currency: ModelPricingCurrencyUSD}, wantErr: true},
		{name: "negative", pricing: &ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{Input: &ModelTokenCost{Per1M: -1}}}, wantErr: true},
		{name: "not finite", pricing: &ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{Input: &ModelTokenCost{Per1M: math.Inf(1)}}}, wantErr: true},
		{name: "inconsistent units", pricing: &ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{Input: &ModelTokenCost{PerToken: 0.000001, Per1M: 2}}}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.pricing.Validate()
			if test.wantErr && err == nil {
				t.Fatal("Validate returned nil error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

func TestPriceUnitTierValidation(t *testing.T) {
	valid := &ModelPricing{
		Currency: ModelPricingCurrencyUSD,
		Tiers: []ModelPricingTier{{
			Name: "long context",
			Type: ModelPricingTierTypeContext,
			Size: 200_000,
			Tokens: &ModelTokenPricing{
				Input: &ModelTokenCost{Per1M: 5},
			},
		}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate valid tier: %v", err)
	}

	invalid := *valid
	invalid.Tiers = append([]ModelPricingTier(nil), valid.Tiers...)
	invalid.Tiers[0].Size = 0
	if err := invalid.Validate(); err == nil {
		t.Fatal("Validate accepted tier without a positive threshold")
	}
}

func TestPriceUnitEffectiveTimeValidation(t *testing.T) {
	now := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	from := utc.New(now.Add(-time.Hour))
	until := utc.New(now.Add(time.Hour))
	pricing := &ModelPricing{
		Currency:       ModelPricingCurrencyUSD,
		EffectiveFrom:  &from,
		EffectiveUntil: &until,
		Tokens:         &ModelTokenPricing{Input: &ModelTokenCost{Per1M: 1}},
	}
	if err := pricing.Validate(); err != nil {
		t.Fatalf("Validate effective interval: %v", err)
	}
	if !pricing.IsEffectiveAt(now) {
		t.Fatal("pricing was not effective inside its interval")
	}
	if pricing.IsEffectiveAt(until.Time) {
		t.Fatal("pricing was effective at the exclusive upper boundary")
	}

	invalidUntil := utc.New(from.Time)
	pricing.EffectiveUntil = &invalidUntil
	if err := pricing.Validate(); err == nil {
		t.Fatal("Validate accepted an empty effective interval")
	}
}

func pricingFloat64Pointer(value float64) *float64 {
	return &value
}
