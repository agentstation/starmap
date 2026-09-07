package reconciler

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProviderObservationSelectionPreservesAggregateReceipt(t *testing.T) {
	baseline, original := providerSelectionFixture(t)
	payload, err := catalogs.EncodeCatalogPayload(original.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := original.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	selection := map[string][]catalogs.ProviderID{original.ID: {"provider-b"}}
	option := WithProviderObservationSelection(selection)
	selection[original.ID][0] = "provider-a"
	delete(selection, original.ID)
	result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{original}, option)
	if err != nil {
		t.Fatal(err)
	}
	kept, _ := result.Catalog.Providers().Get("provider-a")
	if kept.Models["shared"].Limits.ContextWindow != 100 {
		t.Fatal("excluded provider overwrote the baseline")
	}
	accepted, ok := result.Catalog.Providers().Get("provider-b")
	if !ok || accepted.Models["shared"].Limits.ContextWindow != 200 {
		t.Fatal("selection discarded the unrelated provider")
	}
	entries := result.Catalog.Provenance().FindModelField("provider-b", "shared", "limits.context_window")
	if len(entries) != 1 || entries[0].ObservationID != original.ID || entries[0].EvidenceChecksum != original.EvidenceChecksum {
		t.Fatal("selected provider lost its original receipt")
	}
	after, err := catalogs.EncodeCatalogPayload(original.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload, after) {
		t.Fatal("selection rewrote the original payload")
	}
	if _, err := receipt.Restore(original.Catalog); err != nil {
		t.Fatalf("original receipt no longer validates: %v", err)
	}
}

func providerSelectionFixture(t *testing.T) (*catalogs.Catalog, sources.Observation) {
	t.Helper()
	baseline := sourceIdentityCatalog(t, "", catalogs.Model{ID: "shared", Name: "Shared", Limits: &catalogs.ModelLimits{ContextWindow: 100}})
	observed, err := catalogs.NewBuilderFrom(baseline)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := observed.Providers().Get("provider-a")
	provider.Models["shared"].Limits.ContextWindow = 200
	if err := observed.SetProvider(*provider); err != nil {
		t.Fatal(err)
	}
	provider.ID, provider.Name = "provider-b", "Provider B"
	if err := observed.SetProvider(*provider); err != nil {
		t.Fatal(err)
	}
	catalog, err := observed.Build()
	if err != nil {
		t.Fatal(err)
	}
	original := sourceIdentityObservation(t, sources.ProvidersID, catalog, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	return baseline, original
}

func TestProviderObservationSelectionExcludesConflictsBeforeMerge(t *testing.T) {
	baseline, original := providerSelectionFixture(t)
	builder, err := catalogs.NewBuilderFrom(baseline)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := builder.Providers().Get("provider-a")
	provider.Models["shared"].Limits.ContextWindow = 300
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	replacement := sourceIdentityObservation(t, sources.ProvidersID, catalog, original.ObservedAt)
	option := WithProviderObservationSelection(map[string][]catalogs.ProviderID{original.ID: {"provider-b"}})
	var first []byte
	for _, observations := range [][]sources.Observation{{original, replacement}, {replacement, original}} {
		result, err := ReconcileObservations(t.Context(), baseline, observations, option, WithChangeTime(original.ObservedAt))
		if err != nil {
			t.Fatal(err)
		}
		for id, want := range map[catalogs.ProviderID]int64{"provider-a": 300, "provider-b": 200} {
			provider, ok := result.Catalog.Providers().Get(id)
			if !ok || provider.Models["shared"].Limits.ContextWindow != want {
				t.Fatalf("wrong selected value for %s", id)
			}
		}
		payload, err := catalogs.EncodeCatalogPayload(result.Catalog)
		if err != nil {
			t.Fatal(err)
		}
		if first != nil && !bytes.Equal(first, payload) {
			t.Fatal("input order changed selected output")
		}
		first = payload
		if err := original.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProviderObservationSelectionEmptyPreservesBaseline(t *testing.T) {
	baseline, original := providerSelectionFixture(t)
	for _, scoped := range []bool{false, true} {
		t.Run(fmt.Sprint(scoped), func(t *testing.T) {
			observation := original
			if scoped {
				observation = scopedReconciliationObservation(t, "retired", "shared", 200, original.ObservedAt)
			}
			result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{observation}, WithProviderObservationSelection(map[string][]catalogs.ProviderID{observation.ID: {}}))
			if err != nil {
				t.Fatal(err)
			}
			payload, err := catalogs.EncodeCatalogPayload(result.Catalog)
			if err != nil {
				t.Fatal(err)
			}
			want, err := catalogs.EncodeCatalogPayload(baseline)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(payload, want) {
				t.Fatal("empty selection changed baseline facts or evidence")
			}
		})
	}
}

func TestProviderObservationSelectionRejectsInvalidInputs(t *testing.T) {
	baseline, original := providerSelectionFixture(t)
	metadata := sourceIdentityObservation(t, sources.LocalCatalogID, baseline, original.ObservedAt)
	corrupted := original
	corrupted.EvidenceChecksum = "sha256:invalid"
	cases := []struct {
		name        string
		observation sources.Observation
		selection   map[string][]catalogs.ProviderID
	}{
		{"missing-observation", original, map[string][]catalogs.ProviderID{"missing": {}}},
		{"missing-provider", original, map[string][]catalogs.ProviderID{original.ID: {"missing"}}},
		{"metadata", metadata, map[string][]catalogs.ProviderID{metadata.ID: {}}},
		{"corrupt-excluded", corrupted, map[string][]catalogs.ProviderID{original.ID: {}}},
		{"empty-identity", original, map[string][]catalogs.ProviderID{"": {}}},
		{"empty-provider", original, map[string][]catalogs.ProviderID{original.ID: {""}}},
		{"duplicate-provider", original, map[string][]catalogs.ProviderID{original.ID: {"provider-a", "provider-a"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{tc.observation}, WithProviderObservationSelection(tc.selection))
			if result != nil || err == nil {
				t.Fatal("invalid selection did not fail")
			}
		})
	}
}

func TestProviderObservationSelectionOmissionKeepsAllRecords(t *testing.T) {
	baseline, original := providerSelectionFixture(t)
	result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{original}, WithProviderObservationSelection(nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []catalogs.ProviderID{"provider-a", "provider-b"} {
		provider, ok := result.Catalog.Providers().Get(id)
		if !ok || provider.Models["shared"].Limits.ContextWindow != 200 {
			t.Fatal("omitted selection changed existing behavior")
		}
	}
}
