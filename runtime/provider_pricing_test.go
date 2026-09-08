package runtime

import (
	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"testing"
	"time"
)

func TestProviderPricingSurvivesRetentionAndRestart(t *testing.T) {
	for _, cost := range []float64{2, 0} {
		t.Run(map[float64]string{2: "new-price", 0: "explicit-free"}[cost], func(t *testing.T) {
			base, err := catalogs.DecodeCatalogPayload(testCatalogPayload(t, "provider", "model", "Model"))
			if err != nil {
				t.Fatal(err)
			}
			priced := func(price float64) *catalogs.Catalog {
				builder := catalogs.NewEmpty()
				if err := builder.MergeWith(base, catalogs.WithStrategy(catalogs.MergeReplaceAll)); err != nil {
					t.Fatal(err)
				}
				provider, found := base.Providers().Get("provider")
				if !found {
					t.Fatal("missing provider fixture")
				}
				provider.Models["model"].Pricing = &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: price}, Output: &catalogs.ModelTokenCost{Per1M: price * 2}}}
				if err := builder.SetProvider(*provider); err != nil {
					t.Fatal(err)
				}
				result, err := builder.Build()
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			baseline := priced(10)
			payload, err := catalogs.EncodeCatalogPayload(priced(cost))
			if err != nil {
				t.Fatal(err)
			}
			layer := testProviderLayerFromPayload(t, "provider", payload, time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
			if err := layer.validate(); err != nil {
				t.Fatal(err)
			}
			store, err := newLayerStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			r := &Runtime{store: store}
			if err := r.retainProviders(t.Context(), []ProviderLayer{layer}); err != nil {
				t.Fatal(err)
			}
			retained, err := store.loadProviders()
			if err != nil {
				t.Fatal(err)
			}
			layers := layerSet{providers: retained}
			state, err := layers.build(t.Context(), starmap.CatalogState{GenerationID: "baseline", Catalog: baseline})
			if err != nil {
				t.Fatal(err)
			}
			provider, found := state.Catalog.Providers().Get("provider")
			if !found {
				t.Fatal("provider lost")
			}
			got := provider.Models["model"].Pricing.Tokens.Input.Per1M
			if got != cost {
				t.Fatalf("provider input price = %v, want latest provider observation %v", got, cost)
			}
			if got := provider.Models["model"].Pricing.Tokens.Output.Per1M; got != cost*2 {
				t.Fatalf("provider output price = %v, want %v", got, cost*2)
			}
			prior, _ := baseline.Providers().Get("provider")
			if prior.Models["model"].Pricing.Tokens.Input.Per1M != 10 {
				t.Fatal("merge changed baseline")
			}
		})
	}
}
