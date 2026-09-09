package catalogs

import (
	"encoding/json"
	"testing"
)

func TestLegacyUnusedTokenUnitDoesNotClaimFreePrice(t *testing.T) {
	var decoded ModelTokenCost
	if err := json.Unmarshal([]byte(`{"per_token":0,"per_1m_tokens":1.5}`), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, cost := range []*ModelTokenCost{{Per1M: 1.5}, &decoded} {
		if value, state := cost.Amount(CostUnitPerToken); state != ValueMissing || value != 0 {
			t.Errorf("unused legacy unit=(%v,%v), want missing", value, state)
		}
		data, err := json.Marshal(cost)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != `{"per_token":0,"per_1m_tokens":1.5}` {
			t.Fatalf("legacy bytes changed: %s", data)
		}
		cost.UnsetAmount(CostUnitPerMillion)
		if _, state := cost.Amount(CostUnitPerToken); state != ValueMissing {
			t.Error("removing price created a free alternate-unit claim")
		}
	}
}

func TestSetTokenAmountReplacesAlternateUnit(t *testing.T) {
	for _, value := range []float64{0, 0.000002} {
		cost := &ModelTokenCost{Per1M: 1.5}
		if !cost.SetAmount(CostUnitPerToken, value) {
			t.Fatal("valid unit was rejected")
		}
		if _, state := cost.Amount(CostUnitPerMillion); state != ValueMissing {
			t.Error("new amount retained a different alternate-unit price")
		}
		amount, state := cost.Amount(CostUnitPerToken)
		if amount != value || state != ValueKnown {
			t.Fatalf("new amount=(%v,%v)", amount, state)
		}
	}
}
