package reconciler

import (
	"bytes"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestMetadataSelectionPreservesReceiptAndBaseline(t *testing.T) {
	for _, source := range []sources.ID{sources.ModelsDevHTTPID, sources.ModelsDevGitID} {
		t.Run(source.String(), func(t *testing.T) {
			baseline, original := providerSelectionFixture(t)
			observation := sourceIdentityObservation(t, source, original.Catalog, original.ObservedAt)
			before, err := catalogs.EncodeCatalogPayload(observation.Catalog)
			if err != nil {
				t.Fatal(err)
			}
			result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{observation}, WithProviderObservationSelection(map[string][]catalogs.ProviderID{observation.ID: {"provider-a"}}))
			if err != nil {
				t.Fatal(err)
			}
			provider, ok := result.Catalog.Providers().Get("provider-a")
			if !ok || provider.Models["shared"].Limits.ContextWindow != 200 {
				t.Fatal("selected metadata did not enrich the baseline")
			}
			if _, ok := result.Catalog.Providers().Get("provider-b"); ok {
				t.Fatal("metadata added a provider outside the baseline")
			}
			entries := result.Catalog.Provenance().FindModelField("provider-a", "shared", "limits.context_window")
			if len(entries) != 1 || entries[0].ObservationID != observation.ID || entries[0].EvidenceChecksum != observation.EvidenceChecksum {
				t.Fatal("metadata lost its original receipt")
			}
			after, err := catalogs.EncodeCatalogPayload(observation.Catalog)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("selection changed the original payload")
			}
			if err := observation.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMetadataSelectionPreservesAliasesAndPeers(t *testing.T) {
	baseline, original := providerSelectionFixture(t)
	builder, err := catalogs.NewBuilderFrom(baseline)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := builder.Providers().Get("provider-a")
	provider.Aliases = []catalogs.ProviderID{"metadata-a"}
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatal(err)
	}
	peer := *provider
	peer.ID, peer.Name, peer.Aliases = "provider-b", "Provider B", nil
	if err := builder.SetProvider(peer); err != nil {
		t.Fatal(err)
	}
	baseline, err = builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	observed, err := catalogs.NewBuilderFrom(original.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	alias, _ := observed.Providers().Get("provider-a")
	if err := observed.DeleteProvider("provider-a"); err != nil {
		t.Fatal(err)
	}
	alias.ID, alias.Aliases = "metadata-a", nil
	if err := observed.SetProvider(*alias); err != nil {
		t.Fatal(err)
	}
	snapshot, err := observed.Build()
	if err != nil {
		t.Fatal(err)
	}
	observation := sourceIdentityObservation(t, sources.ModelsDevHTTPID, snapshot, original.ObservedAt)
	for _, test := range []struct {
		name     string
		selected []catalogs.ProviderID
		a, b     int64
	}{
		{"alias", []catalogs.ProviderID{"metadata-a"}, 200, 100},
		{"peer", []catalogs.ProviderID{"provider-b"}, 100, 200},
		{"empty", []catalogs.ProviderID{}, 100, 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{observation}, WithProviderObservationSelection(map[string][]catalogs.ProviderID{observation.ID: test.selected}))
			if err != nil {
				t.Fatal(err)
			}
			for id, want := range map[catalogs.ProviderID]int64{"provider-a": test.a, "provider-b": test.b} {
				provider, ok := result.Catalog.Providers().Get(id)
				if !ok || provider.Models["shared"].Limits.ContextWindow != want {
					t.Fatalf("provider %s did not retain the selected value %d", id, want)
				}
			}
			if _, ok := result.Catalog.Providers().Get("metadata-a"); ok {
				t.Fatal("source alias became a separate provider")
			}
			if err := observation.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMetadataSelectionWithoutBaselineKeepsSelectedMembership(t *testing.T) {
	_, original := providerSelectionFixture(t)
	observation := sourceIdentityObservation(t, sources.ModelsDevGitID, original.Catalog, original.ObservedAt)
	for _, selected := range [][]catalogs.ProviderID{{"provider-a"}, {}} {
		engine, err := New(WithProviderObservationSelection(map[string][]catalogs.ProviderID{observation.ID: selected}))
		if err != nil {
			t.Fatal(err)
		}
		result, err := engine.Sources(t.Context(), sources.ModelsDevGitID, []sources.Observation{observation})
		if err != nil {
			t.Fatal(err)
		}
		if result.Catalog.Providers().Len() != len(selected) {
			t.Fatal("primary membership ignored metadata selection")
		}
		if len(result.Catalog.AuthoredModels()) != len(observation.Catalog.AuthoredModels()) {
			t.Fatal("provider selection removed the authored corpus")
		}
	}
}

func TestMetadataSelectionRejectsCorruptOriginalAndProtectedSources(t *testing.T) {
	baseline, original := providerSelectionFixture(t)
	for _, source := range []sources.ID{sources.ModelsDevHTTPID, sources.ModelsDevGitID, sources.LocalCatalogID, sources.ReleaseArtifactID, sources.EmbeddedCatalogID} {
		t.Run(source.String(), func(t *testing.T) {
			observation := sourceIdentityObservation(t, source, original.Catalog, original.ObservedAt)
			if isModelsDevSource(source) {
				observation.EvidenceChecksum = "sha256:invalid"
			}
			result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{observation}, WithProviderObservationSelection(map[string][]catalogs.ProviderID{observation.ID: {}}))
			if result != nil || err == nil {
				t.Fatal("selection accepted corrupt evidence or a protected source")
			}
		})
	}
}
