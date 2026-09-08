package catalogs

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestMergeModelsPricingReplacesAsValidatedUnit(t *testing.T) {
	for _, test := range []struct {
		name    string
		pricing *ModelPricing
		accept  bool
	}{
		{"free", &ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{Input: &ModelTokenCost{}}}, true},
		{"new-units", &ModelPricing{Currency: "EUR", Tokens: &ModelTokenPricing{Input: &ModelTokenCost{PerToken: 0.000002}}}, true},
		{"missing", nil, false},
		{"empty", &ModelPricing{}, false},
		{"no-component", &ModelPricing{Currency: "EUR"}, false},
		{"negative", &ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{Input: &ModelTokenCost{Per1M: -1}}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			baseline := &ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{
				Input: &ModelTokenCost{PerToken: 0.000010, Per1M: 10}, Output: &ModelTokenCost{Per1M: 20},
			}}
			before := deepCopyModelPricing(baseline)
			observation := deepCopyModelPricing(test.pricing)
			got := MergeModels(Model{Pricing: baseline}, Model{Pricing: test.pricing})
			want := before
			if test.accept {
				want = observation
			}
			if !reflect.DeepEqual(got.Pricing, want) {
				t.Fatalf("pricing = %+v, want complete selected price %+v", got.Pricing, want)
			}
			if !reflect.DeepEqual(baseline, before) || !reflect.DeepEqual(test.pricing, observation) {
				t.Fatal("merge mutated an input price")
			}
			got.Pricing.Tokens.Input.Per1M = 99
			if !reflect.DeepEqual(baseline, before) || !reflect.DeepEqual(test.pricing, observation) {
				t.Fatal("merged pricing aliases an input price")
			}
		})
	}
}

func TestFreeTokenCostPresenceSurvivesEncoding(t *testing.T) {
	for _, encoding := range []struct {
		name      string
		marshal   func(any) ([]byte, error)
		unmarshal func([]byte, any) error
	}{
		{"json", json.Marshal, json.Unmarshal},
		{"yaml", yaml.Marshal, yaml.Unmarshal},
	} {
		t.Run(encoding.name, func(t *testing.T) {
			original := ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{Input: &ModelTokenCost{}}}
			payload, err := encoding.marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var decoded ModelPricing
			if err := encoding.unmarshal(payload, &decoded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(original, decoded) {
				t.Fatalf("free price lost presence: %s", payload)
			}
			if err := decoded.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
