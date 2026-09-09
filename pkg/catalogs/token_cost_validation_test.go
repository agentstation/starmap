package catalogs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestTokenCostObservationRequiresKnownAmount(t *testing.T) {
	cases := []struct {
		name, input string
		valid       bool
	}{
		{"missing", `{}`, false},
		{"unknown per token", `{"per_token":null}`, false},
		{"unknown per million", `{"per_1m_tokens":null}`, false},
		{"both unknown", `{"per_token":null,"per_1m_tokens":null}`, false},
		{"known zero per token", `{"per_token":0}`, true},
		{"known zero per million", `{"per_1m_tokens":0}`, true},
		{"known positive", `{"per_1m_tokens":1.5}`, true},
		{"known with unknown other unit", `{"per_token":null,"per_1m_tokens":1.5}`, true},
	}
	for _, tc := range cases {
		for _, format := range []string{"json", "yaml"} {
			t.Run(tc.name+"/"+format, func(t *testing.T) {
				var cost ModelTokenCost
				var err error
				if format == "json" {
					err = json.Unmarshal([]byte(tc.input), &cost)
				} else {
					err = yaml.Unmarshal([]byte(strings.ReplaceAll(tc.input, "per_1m_tokens", "per_1m")), &cost)
				}
				if err != nil {
					t.Fatal(err)
				}
				pricing := &ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{Input: &cost}}
				// The old YAML encoder used an empty mapping for a known free price.
				wantValid := tc.valid || (format == "yaml" && tc.input == `{}`)
				if err := pricing.Validate(); (err == nil) != wantValid {
					t.Fatalf("Validate()=%v, want valid=%v for %s", err, wantValid, tc.input)
				}
			})
		}
	}
	t.Run("legacy constructed zero", func(t *testing.T) {
		pricing := &ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{Input: &ModelTokenCost{}}}
		if err := pricing.Validate(); err != nil {
			t.Fatal(err)
		}
	})
}
