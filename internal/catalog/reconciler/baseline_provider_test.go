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
)

func TestSparseProviderDeclarationPreservesBaselineReceipts(t *testing.T) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	input := sourceIdentityCatalog(t, "https://provider.example/original", catalogs.Model{ID: "shared", Name: "Shared"})
	original := sourceIdentityObservation(t, sources.LocalCatalogID, input, at)
	baseline := snapshotForTest(t, sourceIdentityReconcile(t, sources.LocalCatalogID, original).Catalog)
	for _, test := range []struct{ name, localName string }{
		{name: "identity only"},
		{name: "present unchanged fact", localName: "Provider A"},
		{name: "edited fact", localName: "Edited Provider"},
	} {
		t.Run(test.name, func(t *testing.T) {
			builder, err := catalogs.NewBuilderFrom(sourceIdentityCatalog(t, "", catalogs.Model{ID: "added", Name: "Added"}))
			if err != nil {
				t.Fatal(err)
			}
			provider, err := builder.Provider("provider-a")
			if err != nil {
				t.Fatal(err)
			}
			if err := builder.SetProvider(catalogs.Provider{ID: provider.ID, Name: test.localName, Models: provider.Models}); err != nil {
				t.Fatal(err)
			}
			addition := sourceIdentityObservation(t, sources.LocalCatalogID, snapshotForTest(t, builder), at.Add(time.Hour))
			result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{
				{SourceID: sources.ReleaseArtifactID, Catalog: baseline}, {SourceID: sources.EmbeddedCatalogID, Catalog: baseline}, addition,
			}, WithChangeTime(addition.ObservedAt))
			if err != nil {
				t.Fatal(err)
			}
			current, err := result.Catalog.Provider("provider-a")
			if err != nil {
				t.Fatal(err)
			}
			wantName := "Provider A"
			if test.localName != "" {
				wantName = test.localName
			}
			if current.Name != wantName || current.Catalog == nil || current.Catalog.Endpoint.URL != "https://provider.example/original" {
				t.Fatalf("provider metadata changed: %#v", current)
			}
			for _, field := range []string{"Name", "Catalog", "Credentials"} {
				want := baseline.Provenance().FindByField(evidence.ResourceTypeProvider, "provider-a", field)
				got := result.Catalog.Provenance().FindByField(evidence.ResourceTypeProvider, "provider-a", field)
				if len(want) != 1 || len(got) != 1 {
					t.Fatalf("missing %s receipt", field)
				}
				if field == "Name" && test.localName != "" {
					if got[0].ObservationID != addition.ID {
						t.Fatal("real local provider fact lost fresh authority")
					}
					continue
				}
				wantJSON, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				gotJSON, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(gotJSON, wantJSON) {
					t.Fatalf("provider %s receipt changed", field)
				}
			}
		})
	}
}

func TestDeniedProviderReceiptCannotUseAlternateCarrier(t *testing.T) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	original := sourceIdentityObservation(t, sources.LocalCatalogID, sourceIdentityCatalog(t, "", catalogs.Model{ID: "shared", Name: "Shared"}), at)
	baseline := snapshotForTest(t, sourceIdentityReconcile(t, sources.LocalCatalogID, original).Catalog)
	provider, err := baseline.Provider("provider-a")
	if err != nil {
		t.Fatal(err)
	}
	policies := authority.New()
	merger := newMerger(policies, NewAuthorityStrategy(policies), baseline)
	merger.setObservations([]sources.Observation{{SourceID: sources.ReleaseArtifactID, Catalog: baseline}, {SourceID: sources.EmbeddedCatalogID, Catalog: baseline}})
	merger.projectedEvidence = func(catalogs.ProviderID, provenance.Entry) bool { return false }
	policy, found := policies.Find(evidence.ResourceTypeProvider, "Name")
	if !found {
		t.Fatal("provider name policy missing")
	}
	resolved := merger.providerSourcesForPolicy("provider-a", policy, map[sources.ID]*catalogs.Provider{sources.ReleaseArtifactID: &provider, sources.EmbeddedCatalogID: &provider})
	if resolved[sources.LocalCatalogID] != nil || len(resolved) != 2 {
		t.Fatalf("denied provider claim changed fallback authority: %#v", resolved)
	}
}
