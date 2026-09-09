package runtime

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestRuntimePricingAuthorityPreservesCompleteRecord(t *testing.T) {
	for _, cost := range []string{"0", "0.000002"} {
		t.Run(cost, func(t *testing.T) {
			at := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
			seed := manualProviderObservation(t, 0, at).Catalog
			decode := func(input string) *catalogs.ModelPricing {
				var pricing catalogs.ModelPricing
				if err := json.Unmarshal([]byte(input), &pricing); err != nil {
					t.Fatal(err)
				}
				if err := pricing.Validate(); err != nil {
					t.Fatal(err)
				}
				return &pricing
			}
			baselinePrice := decode(`{"currency":"USD","tokens":{"input":{"per_1m_tokens":10},"output":{"per_1m_tokens":20}},"operations":{"request":0.1},"tiers":[{"type":"context","size":200000,"tokens":{"output":{"per_1m_tokens":40}}}],"effective_from":"2026-01-01T00:00:00Z","effective_until":"2099-01-01T00:00:00Z"}`)
			providerPrice := decode(`{"currency":"EUR","tokens":{"input":{"per_token":` + cost + `}},"operations":{"request":0},"tiers":[{"type":"context","size":100000,"tokens":{"input":{"per_1m_tokens":3}}}],"effective_from":"2026-02-01T00:00:00Z","effective_until":"2098-01-01T00:00:00Z"}`)
			build := func(pricing *catalogs.ModelPricing) *catalogs.Catalog {
				builder, err := catalogs.NewBuilderFrom(seed)
				if err != nil {
					t.Fatal(err)
				}
				provider, err := builder.Provider("provider")
				if err != nil {
					t.Fatal(err)
				}
				provider.Models["model"].Pricing = pricing
				if err := builder.SetProvider(provider); err != nil {
					t.Fatal(err)
				}
				catalog, err := catalogs.NewObservationCatalog(builder)
				if err != nil {
					t.Fatal(err)
				}
				return catalog
			}
			payload, err := catalogs.EncodeCatalogPayload(build(baselinePrice))
			if err != nil {
				t.Fatal(err)
			}
			upstream := newStubSource("atomic-pricing")
			upstream.replies = []SourceRead{testSourceRead(t, "price-baseline", payload, at), testSourceRead(t, "price-refresh", payload, at.Add(time.Hour))}
			store, err := storage.NewFilesystem(privateRuntimeDirectory(t))
			if err != nil {
				t.Fatal(err)
			}
			options := []Option{WithSource(upstream), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store))}
			connected := openTestRuntime(t, options...)
			if _, err := connected.RefreshSource(t.Context()); err != nil {
				t.Fatal(err)
			}
			observation, err := sources.NewObservation(sources.ProvidersID, build(providerPrice), sources.ObservationMetadata{ObservedAt: at.Add(time.Minute), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := connected.PublishObservations(t.Context(), observation); err != nil {
				t.Fatal(err)
			}
			want, err := json.Marshal(providerPrice)
			if err != nil {
				t.Fatal(err)
			}
			assertState := func(active *Runtime) {
				t.Helper()
				catalog := active.State().Catalog
				provider, err := catalog.Provider("provider")
				if err != nil {
					t.Fatal(err)
				}
				offering, err := catalog.Offering("provider", "model")
				if err != nil {
					t.Fatal(err)
				}
				for _, pricing := range []*catalogs.ModelPricing{provider.Models["model"].Pricing, offering.Pricing} {
					actual, err := json.Marshal(pricing)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(actual, want) {
						t.Errorf("pricing=%s, want complete record %s", actual, want)
					}
				}
				receipts := catalog.Provenance().FindModelField("provider", "model", "pricing")
				if len(receipts) != 1 || receipts[0].ObservationID != observation.ID {
					t.Errorf("pricing receipts=%+v, want original provider %s", receipts, observation.ID)
				}
				generation, err := store.Current(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, receipt := range generation.Manifest.SourceObservations {
					found = found || receipt.ObservationID == observation.ID
				}
				if !found {
					t.Fatal("committed generation lost pricing receipt")
				}
			}
			assertState(connected)
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			reopened := openTestRuntime(t, options...)
			assertState(reopened)
			if _, err := reopened.RefreshSource(t.Context()); err != nil {
				t.Fatal(err)
			}
			assertState(reopened)
		})
	}
}
