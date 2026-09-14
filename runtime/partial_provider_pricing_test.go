package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPartialProviderReplyPreservesPricingAcrossRestartAndSourceRefresh(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name   string
		prior  float64
		empty  bool
		fields bool
	}{
		{"paid-partial", 2, false, false}, {"free-partial", 0, false, false},
		{"paid-empty", 2, true, false}, {"free-empty", 0, true, false},
		{"paid-omitted-price", 2, false, true}, {"free-omitted-price", 0, false, true},
		{"paid-omitted-prices", 2, true, true}, {"free-omitted-prices", 0, true, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			for _, scope := range []string{"unbound", "account", "public"} {
				t.Run(scope, func(t *testing.T) {
					seed, err := catalogs.DecodeCatalogPayload(testCatalogPayload(t, "provider", "model", "Model"))
					if err != nil {
						t.Fatal(err)
					}
					build := func(price float64, omit bool) *catalogs.Catalog {
						builder, err := catalogs.NewBuilderFrom(seed)
						if err != nil {
							t.Fatal(err)
						}
						provider, err := builder.Provider("provider")
						if err != nil {
							t.Fatal(err)
						}
						provider.Models["model"].Pricing = &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: price}, Output: &catalogs.ModelTokenCost{Per1M: price * 2}}}
						provider.Models["model"].Limits = &catalogs.ModelLimits{}
						provider.Models["model"].Limits.Set(catalogs.ModelLimitContextWindow, int64(price*100+1000))
						peer := *provider.Models["model"]
						peer.ID = "peer"
						provider.Models["peer"] = &peer
						if omit {
							if scenario.fields {
								provider.Models["model"].Pricing = nil
							} else {
								delete(provider.Models, "model")
							}
							if scenario.empty {
								if scenario.fields {
									provider.Models["peer"].Pricing = nil
								} else {
									delete(provider.Models, "peer")
								}
							}
						}
						if err := builder.SetProvider(provider); err != nil {
							t.Fatal(err)
						}
						result, err := builder.Build()
						if err != nil {
							t.Fatal(err)
						}
						return result
					}
					baseline := build(10, false)
					binding := sources.ProviderAcquisitionBinding{SchemaVersion: 1, ID: "fixture", Revision: "1", ProviderID: "provider", Public: scope == "public", Region: "global", APISurface: "models.list", CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "catalog"}
					if scope != "public" {
						binding.AccountID = "account"
					}
					at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
					layer := func(catalog *catalogs.Catalog, partial bool, observed time.Time) ProviderLayer {
						metadata := sources.ObservationMetadata{ObservedAt: observed, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded}
						if scope != "unbound" {
							metadata.ProviderBinding = &binding
						}
						if partial {
							metadata.Completeness = sources.ObservationCompletenessPartial
							metadata.Status = sources.ObservationStatusDegraded
							metadata.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "provider/model", Message: "Fixture excludes an invalid record."}}
						}
						observation, err := sources.NewObservation(sources.ProvidersID, catalog, metadata)
						if err != nil {
							t.Fatal(err)
						}
						result, err := NewProviderLayer("provider", observation)
						if err != nil {
							t.Fatal(err)
						}
						return result
					}
					store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
					if err != nil {
						t.Fatal(err)
					}
					source := newStubSource("partial-price-baseline")
					payload, err := catalogs.EncodeCatalogPayload(baseline)
					if err != nil {
						t.Fatal(err)
					}
					source.replies = []SourceRead{testSourceRead(t, "baseline-generation", payload, at.Add(-time.Hour))}
					options := []Option{WithSource(source), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store))}
					if scope != "unbound" {
						options = append(options, WithProviderBindings(binding))
					}
					connected := openTestRuntime(t, options...)
					if _, err := connected.RefreshSource(t.Context()); err != nil {
						t.Fatal(err)
					}
					read := func() *catalogs.Catalog { return connected.State().Catalog }
					price := func(catalog *catalogs.Catalog, id string) float64 {
						provider, err := catalog.Provider("provider")
						if err != nil {
							t.Fatal(err)
						}
						model := provider.Models[id]
						if model == nil || model.Pricing == nil || model.Pricing.Tokens == nil || model.Pricing.Tokens.Input == nil {
							t.Fatalf("missing price for %s", id)
						}
						if model.Pricing.Currency != catalogs.ModelPricingCurrencyUSD || model.Pricing.Tokens.Output == nil || model.Pricing.Tokens.Output.Per1M != model.Pricing.Tokens.Input.Per1M*2 {
							t.Fatalf("pricing record changed currency or mixed token costs for %s", id)
						}
						return model.Pricing.Tokens.Input.Per1M
					}
					limits := func(catalog *catalogs.Catalog) {
						t.Helper()
						if !scenario.fields {
							return
						}
						provider, err := catalog.Provider("provider")
						if err != nil {
							t.Fatal(err)
						}
						model := provider.Models["model"]
						if model == nil || model.Limits == nil || model.Limits.ContextWindow != 1300 {
							t.Fatal("partial observation lost the current context limit while retaining its omitted price")
						}
					}
					if _, err := connected.publishProviders(t.Context(), []ProviderLayer{layer(build(scenario.prior, false), false, at)}, connected.lease.epoch()); err != nil {
						t.Fatal(err)
					}
					if got := price(read(), "model"); got != scenario.prior {
						t.Fatalf("complete observation price = %v, want %v", got, scenario.prior)
					}
					if _, err := connected.publishProviders(t.Context(), []ProviderLayer{layer(build(3, true), true, at.Add(time.Minute))}, connected.lease.epoch()); err != nil {
						t.Fatal(err)
					}
					actual := read()
					limits(actual)
					wantPeer := 3.0
					if scenario.empty {
						wantPeer = scenario.prior
					}
					if got := price(actual, "peer"); got != wantPeer {
						t.Errorf("valid partial peer price = %v, want %v", got, wantPeer)
					}
					if got := price(actual, "model"); got != scenario.prior {
						t.Errorf("omitted model price after partial retention = %v, want prior accepted price %v", got, scenario.prior)
					}
					if err := connected.Close(); err != nil {
						t.Fatal(err)
					}
					reopened := openTestRuntime(t, options...)
					limits(reopened.State().Catalog)
					if got := price(reopened.State().Catalog, "model"); got != scenario.prior {
						t.Errorf("omitted model price after runtime reopen = %v, want prior accepted price %v", got, scenario.prior)
					}
					nextPayload, err := catalogs.EncodeCatalogPayload(build(12, false))
					if err != nil {
						t.Fatal(err)
					}
					nextReply := testSourceRead(t, "next-baseline-generation", nextPayload, at.Add(2*time.Minute))
					source.mu.Lock()
					source.replies = []SourceRead{nextReply}
					source.mu.Unlock()
					if _, err := reopened.RefreshSource(t.Context()); err != nil {
						t.Fatal(err)
					}
					limits(reopened.State().Catalog)
					if got := price(reopened.State().Catalog, "model"); got != scenario.prior {
						t.Errorf("source refresh lost retained price: %v, want %v", got, scenario.prior)
					}

				})
			}

		})
	}
}
