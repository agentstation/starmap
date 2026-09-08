package reconciler

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/utc"
)

func TestStableChangeTimeKeepsCurrentPricingValidation(t *testing.T) {
	changedAt := time.Date(2001, 1, 1, 12, 0, 0, 0, time.FixedZone("fixture", -6*60*60))
	expired := utc.New(time.Date(2002, 1, 1, 0, 0, 0, 0, time.UTC))
	engine, err := New(WithChangeTime(changedAt))
	if err != nil {
		t.Fatal(err)
	}
	model, history := engine.createMerger().model("provider", "model", map[sources.ID]*catalogs.Model{
		sources.ProvidersID: {ID: "model", Name: "Model", Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD, EffectiveUntil: &expired,
			Tokens: &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: 1}},
		}},
	})
	if model.Pricing != nil {
		t.Fatal("stable change time allowed currently expired pricing")
	}
	if !model.CreatedAt.Time().Equal(changedAt) || model.CreatedAt.Time().Location() != time.UTC {
		t.Fatalf("generated creation time = %v, want normalized change time", model.CreatedAt)
	}
	rejection := history[modelProvenancePricing].Current
	if len(rejection.Rejections) != 1 || !rejection.Timestamp.Equal(changedAt) {
		t.Fatalf("expired pricing lost stable rejection evidence: %#v", rejection)
	}
	if _, err := New(WithChangeTime(time.Time{})); err == nil {
		t.Fatal("accepted an unspecified change time")
	}
}

func TestPricingIntervalTransitionsKeepStableEvidence(t *testing.T) {
	start := utc.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	end := utc.New(start.Time().Add(time.Hour))
	price := &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, EffectiveFrom: &start, EffectiveUntil: &end,
		Tokens: &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: 1}}}
	engine, err := New(WithChangeTime(start.Time()))
	if err != nil {
		t.Fatal(err)
	}
	var before, after string
	for _, tc := range []struct {
		name  string
		at    time.Time
		valid bool
	}{
		{"before", start.Time().Add(-time.Hour), false},
		{"just-before", start.Time().Add(-time.Nanosecond), false},
		{"start-inclusive", start.Time(), true},
		{"just-before-end", end.Time().Add(-time.Nanosecond), true},
		{"end-exclusive", end.Time(), false},
		{"later", end.Time().Add(time.Hour), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			merger := engine.createMerger()
			merger.pricingAt = tc.at
			model, history := merger.model("provider", "model", map[sources.ID]*catalogs.Model{
				sources.ProvidersID: {ID: "model", Pricing: price},
			})
			if (model.Pricing != nil) != tc.valid {
				t.Fatalf("pricing validity at %s = %t", tc.at, model.Pricing != nil)
			}
			if tc.valid {
				return
			}
			rejected := history[modelProvenancePricing].Current.Rejections
			if len(rejected) != 1 {
				t.Fatal("missing pricing interval rejection")
			}
			previous := &after
			if tc.at.Before(start.Time()) {
				previous = &before
			}
			if *previous == "" {
				*previous = rejected[0].Reason
			} else if *previous != rejected[0].Reason {
				t.Fatal("same interval phase changed rejection evidence")
			}
		})
	}
	if before == after {
		t.Fatal("future and expired pricing received the same explanation")
	}
}
