package reconciler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestUnknownTokenAmountRetainsPriorValidPricing(t *testing.T) {
	for _, input := range []string{`{}`, `{"per_token":null}`, `{"per_1m_tokens":null}`, `{"per_token":null,"per_1m_tokens":null}`} {
		t.Run(input, func(t *testing.T) {
			baseline := catalogs.NewEmpty()
			prior := &catalogs.Model{ID: "model-1", Name: "Model", Pricing: &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: 0.75}}}}
			if err := baseline.SetProvider(catalogs.Provider{ID: "openai", Name: "OpenAI", Models: map[string]*catalogs.Model{"model-1": prior}}); err != nil {
				t.Fatal(err)
			}
			var observed catalogs.Model
			payload := `{"id":"model-1","pricing":{"currency":"USD","tokens":{"input":` + input + `}}}`
			if err := json.Unmarshal([]byte(payload), &observed); err != nil {
				t.Fatal(err)
			}
			policy := authority.New()
			merger := newMerger(policy, NewAuthorityStrategy(policy), snapshotForTest(t, baseline))
			merged, history := merger.model("openai", "model-1", map[sources.ID]*catalogs.Model{sources.ProvidersID: &observed})
			if merged.Pricing == nil || merged.Pricing.Tokens == nil || merged.Pricing.Tokens.Input == nil || merged.Pricing.Tokens.Input.Per1M != 0.75 {
				t.Fatalf("unknown amount replaced valid prior pricing: %#v", merged.Pricing)
			}
			receipt := history[modelProvenancePricing].Current
			if len(receipt.Rejections) != 1 || !strings.Contains(receipt.Reason, "retained prior pricing") {
				t.Fatalf("missing rejection and retention evidence: %#v", receipt)
			}
		})
	}
}
