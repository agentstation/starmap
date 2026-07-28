package reconciler

import (
	"strings"
	"testing"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPricingAuthorityPreservesRejectionsWhenEveryCandidateIsInvalid(t *testing.T) {
	authorities := authority.New()
	merger := newMerger(authorities, NewAuthorityStrategy(authorities), nil)
	expired := utc.New(time.Now().Add(-time.Hour))

	model, history := merger.model("openai", "model-1", map[sources.ID]*catalogs.Model{
		sources.ProvidersID: {
			ID: "model-1",
			Pricing: &catalogs.ModelPricing{
				Currency: catalogs.ModelPricingCurrencyUSD,
				Tokens: &catalogs.ModelTokenPricing{
					Input: &catalogs.ModelTokenCost{Per1M: -1},
				},
			},
		},
		sources.ModelsDevHTTPID: {
			ID: "model-1",
			Pricing: &catalogs.ModelPricing{
				Currency:       catalogs.ModelPricingCurrencyUSD,
				EffectiveUntil: &expired,
				Tokens: &catalogs.ModelTokenPricing{
					Input: &catalogs.ModelTokenCost{Per1M: 1},
				},
			},
		},
	})

	if model.Pricing != nil {
		t.Fatalf("pricing = %#v, want nil when every candidate is invalid", model.Pricing)
	}
	evidence, found := history[modelProvenancePricing]
	if !found {
		t.Fatal("missing pricing rejection evidence")
	}
	if evidence.Current.Source != "" || evidence.Current.Value != nil {
		t.Fatalf("winner = %#v, want no selected source or value", evidence.Current)
	}
	if len(evidence.Current.Rejections) != 2 {
		t.Fatalf("rejections = %#v, want both invalid candidates", evidence.Current.Rejections)
	}
	if evidence.Current.Rejections[0].Source != sources.ProvidersID ||
		evidence.Current.Rejections[1].Source != sources.ModelsDevHTTPID {
		t.Fatalf("rejection order = %#v, want policy order", evidence.Current.Rejections)
	}
	if !strings.Contains(evidence.Current.Reason, "no valid current pricing candidate") {
		t.Fatalf("reason = %q", evidence.Current.Reason)
	}
}

func TestPricingAuthorityRetainsPriorValidPriceWhenCurrentCandidateIsInvalid(t *testing.T) {
	baseline := catalogs.NewEmpty()
	priorPrice := 0.75
	model := catalogs.Model{
		ID:   "model-1",
		Name: "Baseline Model",
		Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
			Tokens: &catalogs.ModelTokenPricing{
				Input: &catalogs.ModelTokenCost{Per1M: priorPrice},
			},
		},
	}
	if err := baseline.SetProvider(catalogs.Provider{
		ID:     "openai",
		Name:   "OpenAI",
		Models: map[string]*catalogs.Model{model.ID: &model},
	}); err != nil {
		t.Fatalf("set baseline provider: %v", err)
	}

	authorities := authority.New()
	merger := newMerger(authorities, NewAuthorityStrategy(authorities), snapshotForTest(t, baseline))
	merged, history := merger.model("openai", "model-1", map[sources.ID]*catalogs.Model{
		sources.ProvidersID: {
			ID: "model-1",
			Pricing: &catalogs.ModelPricing{
				Currency: catalogs.ModelPricingCurrencyUSD,
				Tokens: &catalogs.ModelTokenPricing{
					Input: &catalogs.ModelTokenCost{Per1M: -1},
				},
			},
		},
	})

	if merged.Pricing == nil ||
		merged.Pricing.Tokens == nil ||
		merged.Pricing.Tokens.Input == nil ||
		merged.Pricing.Tokens.Input.Per1M != priorPrice {
		t.Fatalf("retained pricing = %#v, want prior valid price", merged.Pricing)
	}
	evidence := history[modelProvenancePricing].Current
	if len(evidence.Rejections) != 1 || !strings.Contains(evidence.Reason, "retained prior pricing") {
		t.Fatalf("evidence = %#v, want rejection plus retained-prior reason", evidence)
	}
}
