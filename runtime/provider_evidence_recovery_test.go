package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestCompleteProviderRecoveryRetiresSupersededPartialReceipt(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		retainPrice bool
		scoped      bool
		peerScope   bool
		uncovered   bool
	}{
		{name: "fully-replaced"},
		{name: "retained-price", retainPrice: true},
		{name: "same-binding", scoped: true},
		{name: "peer-binding", scoped: true, peerScope: true},
		{name: "uncovered-offering", uncovered: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			retainPrice := scenario.retainPrice
			keepPartial := retainPrice || scenario.peerScope || scenario.uncovered
			store := storage.NewMemory()
			var extra []Option
			binding := sources.ProviderAcquisitionBinding{SchemaVersion: 1, ID: "catalog-one", Revision: "1", ProviderID: "provider", AccountID: "account-one", Region: "global", APISurface: "models.list", CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "catalog-one"}
			peer := binding
			peer.ID, peer.AccountID, peer.CredentialProfileID = "catalog-two", "account-two", "catalog-two"
			if scenario.scoped {
				extra = append(extra, WithProviderBindings(binding, peer))
			}
			connected, options, at := providerResetRuntime(t, store, extra...)
			observation := func(limit int64, price *catalogs.ModelPricing, partial bool, when time.Time) sources.Observation {
				base := manualProviderObservation(t, limit, when)
				builder, err := catalogs.NewBuilderFrom(base.Catalog)
				if err != nil {
					t.Fatal(err)
				}
				provider, err := builder.Provider("provider")
				if err != nil {
					t.Fatal(err)
				}
				provider.Models["model"].Pricing = price
				if partial && scenario.uncovered {
					model := catalogs.DeepCopyModel(*provider.Models["model"])
					model.ID = "peer"
					provider.Models["peer"] = &model
				}
				if err := builder.SetProvider(provider); err != nil {
					t.Fatal(err)
				}
				catalog, err := builder.Build()
				if err != nil {
					t.Fatal(err)
				}
				metadata := sources.ObservationMetadata{ObservedAt: when, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded}
				if scenario.scoped {
					metadata.ProviderBinding = &binding
					if !partial && scenario.peerScope {
						metadata.ProviderBinding = &peer
					}
				}
				if partial {
					metadata.Completeness, metadata.Status = sources.ObservationCompletenessPartial, sources.ObservationStatusDegraded
					metadata.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "provider/invalid", Message: "A source record is invalid."}}
				}
				result, err := sources.NewObservation(sources.ProvidersID, catalog, metadata)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			price := func(value float64) *catalogs.ModelPricing {
				return &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: &catalogs.ModelTokenPricing{Input: &catalogs.ModelTokenCost{Per1M: value}}}
			}
			partial := observation(200, price(2), true, at)
			if _, err := connected.PublishObservations(t.Context(), partial); err != nil {
				t.Fatal(err)
			}
			prior, err := store.Current(t.Context())
			if err != nil || !prior.Manifest.Degraded {
				t.Fatalf("partial generation = %#v, %v", prior.Manifest, err)
			}
			replacementPrice := price(3)
			if retainPrice {
				replacementPrice = nil
			}
			complete := observation(300, replacementPrice, false, at.Add(time.Minute))
			if _, err := connected.PublishObservations(t.Context(), complete); err != nil {
				t.Fatal(err)
			}
			accepted, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			p, err := connected.State().Catalog.Provider("provider")
			if err != nil {
				t.Fatal(err)
			}
			wantPrice := 3.0
			if retainPrice {
				wantPrice = 2
			}
			model := p.Models["model"]
			if model.Pricing == nil || model.Pricing.Tokens == nil || model.Pricing.Tokens.Input == nil || model.Pricing.Tokens.Input.Per1M != wantPrice {
				t.Fatalf("recovery lost expected price %v", wantPrice)
			}
			if model.Limits == nil || model.Limits.ContextWindow != 300 {
				t.Fatal("recovery lost current context limit")
			}
			wantPriceReceipt := complete.ID
			if retainPrice {
				wantPriceReceipt = partial.ID
			}
			priceEvidence := connected.State().Catalog.Provenance().FindModelField("provider", "model", "pricing")
			if len(priceEvidence) != 1 || priceEvidence[0].ObservationID != wantPriceReceipt {
				t.Error("price provenance lost the original provider receipt")
			}
			if accepted.Manifest.Degraded != keepPartial {
				t.Errorf("degraded after complete recovery = %t, want %t", accepted.Manifest.Degraded, keepPartial)
			}
			foundOld, foundNew := false, false
			for _, link := range accepted.Manifest.SourceObservations {
				foundOld = foundOld || link.ObservationID == partial.ID
				foundNew = foundNew || link.ObservationID == complete.ID
			}
			if foundOld != keepPartial || !foundNew {
				t.Errorf("active receipts: old=%t new=%t, want old=%t new=true", foundOld, foundNew, keepPartial)
			}
			if len(manualBatches(connected.layers.manual)) != 2 {
				t.Fatal("recovery changed original retained history")
			}
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			reopened := openTestRuntime(t, options...)
			if reopened.State().GenerationID != accepted.Manifest.GenerationID {
				t.Fatal("restart changed accepted recovery generation")
			}
			after, err := store.Current(t.Context())
			if err != nil || after.Manifest.Degraded != keepPartial {
				t.Errorf("restart lost the recovery state: degraded=%t err=%v", after.Manifest.Degraded, err)
			}
		})
	}
}
