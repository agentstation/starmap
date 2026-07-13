package catalogs

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeProviderTokenPrice(t *testing.T) {
	tests := []struct {
		name       string
		value      float64
		unit       ProviderNormalizationUnit
		perToken   float64
		perMillion float64
	}{
		{name: "per million", value: 2.5, unit: ProviderNormalizationUnitPerMillionTokens, perToken: 0.0000025, perMillion: 2.5},
		{name: "per token", value: 0.0000025, unit: ProviderNormalizationUnitPerToken, perToken: 0.0000025, perMillion: 2.5},
		{name: "cents per one hundred million", value: 25, unit: ProviderNormalizationUnitCentsPer100MillionTokens, perToken: 0.0000000025, perMillion: 0.0025},
		{name: "milli currency per million", value: 2500, unit: ProviderNormalizationUnitMilliCurrencyPerMillionTokens, perToken: 0.0000025, perMillion: 2.5},
		{name: "exact zero", value: 0, unit: ProviderNormalizationUnitPerToken, perToken: 0, perMillion: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cost, err := NormalizeProviderTokenPrice(test.value, test.unit)
			require.NoError(t, err)
			assert.InDelta(t, test.perToken, cost.PerToken, 1e-15)
			assert.InDelta(t, test.perMillion, cost.Per1M, 1e-12)
		})
	}
}

func TestNormalizeProviderTokenPriceRejectsUnsafeValues(t *testing.T) {
	for name, value := range map[string]float64{
		"negative":          -1,
		"nan":               math.NaN(),
		"positive infinity": math.Inf(1),
		"overflow":          math.MaxFloat64,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NormalizeProviderTokenPrice(value, ProviderNormalizationUnitPerToken)
			require.Error(t, err)
		})
	}
	_, err := NormalizeProviderTokenPrice(1, "credits_per_fortnight")
	require.Error(t, err)
}

func TestScaleProviderTokenPrice(t *testing.T) {
	cost, err := NormalizeProviderTokenPrice(2, ProviderNormalizationUnitPerMillionTokens)
	require.NoError(t, err)
	discounted, err := ScaleProviderTokenPrice(cost, 0.5)
	require.NoError(t, err)
	assert.Equal(t, 1.0, discounted.Per1M)
	assert.Equal(t, 0.000001, discounted.PerToken)
	for _, scale := range []float64{-1, math.NaN(), math.Inf(1)} {
		_, err := ScaleProviderTokenPrice(cost, scale)
		require.Error(t, err)
	}
}

func TestValidateProviderFieldMappings(t *testing.T) {
	valid := []FieldMapping{
		{From: "context_window", To: "limits.context_window"},
		{From: "pricing.prompt", To: "pricing.tokens.input", Unit: ProviderNormalizationUnitPerToken, Currency: ModelPricingCurrencyUSD},
		{From: "pricing.completion", To: "pricing.tokens.output", Unit: ProviderNormalizationUnitPerMillionTokens, Currency: ModelPricingCurrencyUSD, Mode: "batch"},
		{From: "pricing.prompt", To: "pricing.tokens.input", Unit: ProviderNormalizationUnitCentsPer100MillionTokens, Currency: ModelPricingCurrencyUSD, Tier: &ProviderPricingTier{Name: "long", Type: ModelPricingTierTypeContext, Size: 200_000}},
		{From: "pricing.image", To: "pricing.operations.image_gen", Unit: ProviderNormalizationUnitPerOperation, Currency: ModelPricingCurrencyUSD},
		{From: "owned_by", To: "extensions.provider.owner"},
		{From: "archived", To: "lifecycle", Values: map[string]string{"false": "active", "true": "deprecated"}},
	}
	require.NoError(t, ValidateProviderFieldMappings(valid))

	tests := map[string][]FieldMapping{
		"unknown target":      {{From: "pricing.prompt", To: "pricing.magic", Unit: ProviderNormalizationUnitPerToken, Currency: ModelPricingCurrencyUSD}},
		"unknown unit":        {{From: "pricing.prompt", To: "pricing.tokens.input", Unit: "credits", Currency: ModelPricingCurrencyUSD}},
		"incompatible unit":   {{From: "pricing.prompt", To: "pricing.operations.request", Unit: ProviderNormalizationUnitPerToken, Currency: ModelPricingCurrencyUSD}},
		"missing currency":    {{From: "pricing.prompt", To: "pricing.tokens.input", Unit: ProviderNormalizationUnitPerToken}},
		"duplicate target":    {{From: "pricing.prompt", To: "pricing.tokens.input", Unit: ProviderNormalizationUnitPerToken, Currency: ModelPricingCurrencyUSD}, {From: "pricing.completion", To: "pricing.tokens.input", Unit: ProviderNormalizationUnitPerToken, Currency: ModelPricingCurrencyUSD}},
		"bad tier":            {{From: "pricing.prompt", To: "pricing.tokens.input", Unit: ProviderNormalizationUnitPerToken, Currency: ModelPricingCurrencyUSD, Tier: &ProviderPricingTier{Type: ModelPricingTierTypeContext}}},
		"unit on plain field": {{From: "context_window", To: "limits.context_window", Unit: ProviderNormalizationUnitPerToken}},
		"unsafe mode":         {{From: "pricing.prompt", To: "pricing.tokens.input", Unit: ProviderNormalizationUnitPerToken, Currency: ModelPricingCurrencyUSD, Mode: "batch/unsafe"}},
		"secret-like target":  {{From: "owned_by", To: "extensions.provider.api_key"}},
		"values on pricing":   {{From: "pricing.prompt", To: "pricing.tokens.input", Unit: ProviderNormalizationUnitPerToken, Currency: ModelPricingCurrencyUSD, Values: map[string]string{"0": "active"}}},
		"invalid lifecycle":   {{From: "archived", To: "lifecycle", Values: map[string]string{"true": "retired-later-maybe"}}},
	}
	for name, mappings := range tests {
		t.Run(name, func(t *testing.T) {
			require.Error(t, ValidateProviderFieldMappings(mappings))
		})
	}
}
