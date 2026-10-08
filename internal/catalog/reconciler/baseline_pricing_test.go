package reconciler

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/utc"
)

func TestBaselinePricingPreservesRejectionEvidence(t *testing.T) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	for _, valid := range []bool{false, true} {
		name := "retained invalid diagnostic"
		if valid {
			name = "carried price with rejection"
		}
		t.Run(name, func(t *testing.T) {
			price := &catalogs.ModelPricing{Currency: catalogs.ModelPricingCurrencyUSD, Tokens: &catalogs.ModelTokenPricing{}}
			if valid {
				price = sourceIdentityPricing(1)
			}
			baselineInput := sourceIdentityCatalog(t, "", catalogs.Model{ID: "shared", Name: "Shared", Pricing: price})
			invalid := sourceIdentityObservation(t, sources.ProvidersID, sourceIdentityCatalog(t, "", catalogs.Model{ID: "shared", Name: "Shared", Pricing: sourceIdentityPricing(-1)}), at)
			local := sourceIdentityObservation(t, sources.LocalCatalogID, baselineInput, at)
			baseline := snapshotForTest(t, sourceIdentityReconcile(t, sources.ProvidersID, invalid, local).Catalog)
			provider, err := baseline.Provider("provider-a")
			if err != nil {
				t.Fatal(err)
			}
			policies := authority.New()
			merger := newMerger(policies, NewAuthorityStrategy(policies), baseline)
			addition := sourceIdentityObservation(t, sources.LocalCatalogID, sourceIdentityCatalog(t, "", catalogs.Model{ID: "added", Name: "Added"}), at.Add(time.Hour))
			merger.setObservations([]sources.Observation{{SourceID: sources.ReleaseArtifactID, Catalog: baseline}, addition})
			merger.pricingAt = at
			policy, found := policies.Find(evidence.ResourceTypeModel, "Pricing")
			if !found {
				t.Fatal("pricing policy missing")
			}
			models := merger.modelSourcesForPolicy("provider-a", "shared", policy, map[sources.ID]*catalogs.Model{sources.ReleaseArtifactID: provider.Models["shared"]})
			history := make(map[string]provenance.Field)
			target := catalogs.DeepCopyModel(*provider.Models["shared"])
			merger.mergeModelPricing(modelIdentity{providerID: "provider-a", modelID: "shared"}, &target, policy, models, &history)
			want := baseline.Provenance().FindModelField("provider-a", "shared", "pricing")
			if len(want) != 1 || len(want[0].Rejections) == 0 {
				t.Fatal("baseline rejection evidence missing")
			}
			got := want[0]
			if entry, exists := history["pricing"]; exists {
				got = entry.Current
			}
			wantJSON, err := json.Marshal(want[0])
			if err != nil {
				t.Fatal(err)
			}
			gotJSON, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(gotJSON, wantJSON) {
				t.Fatalf("baseline price evidence changed: got %s, want %s", gotJSON, wantJSON)
			}
			newInvalid := sourceIdentityObservation(t, sources.ModelsDevGitID, sourceIdentityCatalog(t, "", catalogs.Model{ID: "shared", Name: "Shared", Pricing: sourceIdentityPricing(-2)}), at.Add(time.Hour))
			merger.setObservations([]sources.Observation{{SourceID: sources.ReleaseArtifactID, Catalog: baseline}, addition, newInvalid})
			models[sources.ModelsDevGitID] = newInvalid.Catalog.Providers().List()[0].Models["shared"]
			merger.mergeModelPricing(modelIdentity{providerID: "provider-a", modelID: "shared"}, &target, policy, models, &history)
			foundNew := false
			for _, rejection := range history["pricing"].Current.Rejections {
				foundNew = foundNew || rejection.Source == sources.ModelsDevGitID
			}
			if !foundNew {
				t.Fatal("new rejected source evidence disappeared")
			}
		})
	}
}

func TestBaselinePricingBecomesEligibleAtEffectiveFrom(t *testing.T) {
	start := utc.New(time.Now().UTC().Add(time.Hour))
	price := sourceIdentityPricing(1)
	price.EffectiveFrom = &start
	input := sourceIdentityCatalog(t, "", catalogs.Model{ID: "shared", Name: "Shared", Pricing: price})
	original := sourceIdentityObservation(t, sources.LocalCatalogID, input, start.Add(-time.Hour).Time())
	baseline := snapshotForTest(t, sourceIdentityReconcile(t, sources.LocalCatalogID, original).Catalog)
	prior := baseline.Provenance().FindModelField("provider-a", "shared", "pricing")
	if len(prior) != 1 || prior[0].Source != "" || len(prior[0].Rejections) == 0 {
		t.Fatal("future price rejection missing")
	}
	provider, err := baseline.Provider("provider-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		at       time.Time
		eligible bool
	}{
		{name: "before interval", at: start.Add(-time.Nanosecond).Time()},
		{name: "interval starts", at: start.Time(), eligible: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			policies := authority.New()
			merger := newMerger(policies, NewAuthorityStrategy(policies), baseline)
			addition := sourceIdentityObservation(t, sources.LocalCatalogID, sourceIdentityCatalog(t, "", catalogs.Model{ID: "added", Name: "Added"}), start.Time())
			merger.setObservations([]sources.Observation{{SourceID: sources.ReleaseArtifactID, Catalog: baseline}, addition})
			merger.pricingAt = test.at
			policy, found := policies.Find(evidence.ResourceTypeModel, "Pricing")
			if !found {
				t.Fatal("pricing policy missing")
			}
			history := make(map[string]provenance.Field)
			target := catalogs.DeepCopyModel(*provider.Models["shared"])
			merger.mergeModelPricing(modelIdentity{providerID: "provider-a", modelID: "shared"}, &target, policy, map[sources.ID]*catalogs.Model{sources.ReleaseArtifactID: &target}, &history)
			got := history["pricing"].Current
			if (got.Source != "") != test.eligible {
				t.Fatalf("selected price source %q, eligible=%v", got.Source, test.eligible)
			}
		})
	}
}
