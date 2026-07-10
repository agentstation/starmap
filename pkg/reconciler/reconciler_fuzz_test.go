package reconciler

import (
	"math"
	"testing"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func FuzzReconciliationNoPanic(f *testing.F) {
	f.Add("model-a", "Provider Model", 1.0)
	f.Add("", "", -1.0)
	f.Add("nested/model", "Model", math.Inf(1))
	f.Fuzz(func(t *testing.T, id, name string, price float64) {
		if len(id) > catalogsMaxFuzzString || len(name) > catalogsMaxFuzzString {
			t.Skip()
		}
		authorities := authority.New()
		merger := newMerger(authorities, NewAuthorityStrategy(authorities), nil)
		_, _, _ = merger.Models(map[sources.ID][]*catalogs.Model{
			sources.ProvidersID: {{
				ID: id, Name: name,
				Pricing: &catalogs.ModelPricing{
					Currency: catalogs.ModelPricingCurrencyUSD,
					Tokens:   &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: price}},
				},
			}},
			sources.ModelsDevHTTPID: {{ID: id, Name: "fallback"}},
		})
	})
}

const catalogsMaxFuzzString = 4 * 1024
