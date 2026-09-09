package catalogs

import (
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestTokenCostPresenceCopiesAndRoundTrips(t *testing.T) {
	states := []struct {
		name  string
		state ValuePresence
		value float64
	}{
		{"missing", ValueMissing, 0}, {"unknown", ValueUnknown, 0}, {"zero", ValueKnown, 0}, {"positive", ValueKnown, 1.25},
	}
	for _, a := range states {
		for _, b := range states {
			for _, format := range []string{"json", "yaml"} {
				t.Run(a.name+"/"+b.name+"/"+format, func(t *testing.T) {
					raw := make(map[string]any)
					for i, claim := range []struct {
						state ValuePresence
						value float64
					}{{a.state, a.value}, {b.state, b.value}} {
						if claim.state == ValueMissing {
							continue
						}
						unit := []TokenCostUnit{CostUnitPerToken, CostUnitPerMillion}[i]
						var value any = claim.value
						if claim.state == ValueUnknown {
							value = nil
						}
						raw[string(unit)] = value
					}
					input, err := json.Marshal(raw)
					if err != nil {
						t.Fatal(err)
					}
					cost := &ModelTokenCost{}
					if err := json.Unmarshal(input, cost); err != nil {
						t.Fatal(err)
					}
					model := &Model{ID: "model-1", Pricing: &ModelPricing{Currency: ModelPricingCurrencyUSD, Tokens: &ModelTokenPricing{Input: cost}}}
					copy := DeepCopyModel(*model)
					if copy.Pricing.Tokens.Input == cost {
						t.Fatal("copy shares token cost")
					}
					for range 3 {
						var data []byte
						var err error
						decoded := &ModelTokenCost{Per1M: 99}
						if format == "json" {
							data, err = json.Marshal(copy.Pricing.Tokens.Input)
							if err == nil {
								err = json.Unmarshal(data, decoded)
							}
						} else {
							data, err = yaml.Marshal(copy.Pricing.Tokens.Input)
							if err == nil {
								err = yaml.Unmarshal(data, decoded)
							}
						}
						if format == "yaml" && a.state == ValueMissing && b.state == ValueMissing {
							if err == nil {
								t.Fatal("missing units encoded as legacy free YAML")
							}
							return
						}
						if err != nil {
							t.Fatal(err)
						}
						for i, want := range []struct {
							state ValuePresence
							value float64
						}{{a.state, a.value}, {b.state, b.value}} {
							unit := []TokenCostUnit{CostUnitPerToken, CostUnitPerMillion}[i]
							value, state := decoded.Amount(unit)
							other := b
							if i == 1 {
								other = a
							}
							if want.state == ValueKnown && want.value == 0 && other.state == ValueKnown && other.value != 0 {
								want.state = ValueMissing
							}
							if value != want.value || state != want.state {
								t.Fatalf("%s = (%v,%v), want (%v,%v): %s", unit, value, state, want.value, want.state, data)
							}
						}
						copy.Pricing.Tokens.Input = decoded
					}
				})
			}
		}
	}
}

func TestTokenCostLegacyJSONBytes(t *testing.T) {
	for _, input := range []string{`{"per_token":0,"per_1m_tokens":0}`, `{"per_token":0,"per_1m_tokens":1.5}`, `{"per_token":0.000001,"per_1m_tokens":1}`} {
		t.Run(input, func(t *testing.T) {
			var cost ModelTokenCost
			if err := json.Unmarshal([]byte(input), &cost); err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(cost)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != input {
				t.Fatalf("legacy bytes changed: %s", got)
			}
		})
	}
}

func TestTokenCostPresenceReuseAndNil(t *testing.T) {
	var cost ModelTokenCost
	for _, input := range []string{`{"per_1m_tokens":null}`, `{"per_1m_tokens":1.5}`, `{}`} {
		if err := json.Unmarshal([]byte(input), &cost); err != nil {
			t.Fatal(err)
		}
	}
	if _, state := cost.Amount(CostUnitPerMillion); state != ValueMissing {
		t.Fatal("reuse kept old claim")
	}
	cost.SetAmountUnknown(CostUnitPerMillion)
	cost.Per1M = 4
	if value, state := cost.Amount(CostUnitPerMillion); state != ValueKnown || value != 4 {
		t.Fatal("positive assignment did not replace unknown")
	}
	cost.SetAmountUnknown(CostUnitPerMillion)
	cost.SetAmount(CostUnitPerMillion, 0)
	if value, state := cost.Amount(CostUnitPerMillion); state != ValueKnown || value != 0 {
		t.Fatal("setter did not record zero")
	}
	for _, item := range []*ModelTokenCost{nil, &cost} {
		if item.SetAmount("invalid", 1) || item.SetAmountUnknown("invalid") || item.UnsetAmount("invalid") {
			t.Fatal("invalid unit accepted")
		}
		if _, state := item.Amount("invalid"); state != ValueMissing {
			t.Fatal("invalid unit claimed a value")
		}
	}
	var absent *ModelTokenCost
	if absent.SetAmount(CostUnitPerToken, 1) || absent.SetAmountUnknown(CostUnitPerToken) || absent.UnsetAmount(CostUnitPerToken) {
		t.Fatal("nil receiver accepted mutation")
	}
}
