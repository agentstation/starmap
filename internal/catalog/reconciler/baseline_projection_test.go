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

func TestSparseLocalAdditionPreservesBaselineReceipts(t *testing.T) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	original := sourceIdentityObservation(t, sources.ModelsDevHTTPID, sourceIdentityCatalog(t, "", catalogs.Model{
		ID: "shared", Name: "Shared", Description: "Reviewed description", Status: catalogs.ModelStatusBeta,
		Limits: &catalogs.ModelLimits{ContextWindow: 8192}, Features: &catalogs.ModelFeatures{ToolCalls: true,
			Modalities: catalogs.ModelModalities{Input: []catalogs.ModelModality{catalogs.ModelModalityText}, Output: []catalogs.ModelModality{catalogs.ModelModalityText}}},
		Extensions: catalogs.SourceExtensions{"source": {Fields: map[string]any{"value": "reviewed"}}},
	}), at)
	baseline := snapshotForTest(t, sourceIdentityReconcile(t, sources.ModelsDevHTTPID, original).Catalog)
	addition := sourceIdentityObservation(t, sources.LocalCatalogID, sourceIdentityCatalog(t, "", catalogs.Model{
		ID: "added", Name: "Added", Description: "New authored model",
	}), at.Add(time.Hour))
	for _, carrier := range []sources.ID{sources.ReleaseArtifactID, sources.EmbeddedCatalogID} {
		t.Run(string(carrier), func(t *testing.T) {
			result, err := ReconcileObservations(t.Context(), baseline, []sources.Observation{
				{SourceID: carrier, Catalog: baseline}, addition,
			}, WithChangeTime(addition.ObservedAt))
			if err != nil {
				t.Fatal(err)
			}
			provider, err := result.Catalog.Provider("provider-a")
			if err != nil || provider.Models["added"] == nil {
				t.Fatalf("local addition missing: %v", err)
			}
			originalProvider, err := baseline.Provider("provider-a")
			if err != nil {
				t.Fatal(err)
			}
			if !originalProvider.Models["shared"].Equal(*provider.Models["shared"]) {
				want, _ := json.Marshal(originalProvider.Models["shared"])
				got, _ := json.Marshal(provider.Models["shared"])
				t.Fatalf("sparse addition changed accepted model: got %s, want %s", got, want)
			}
			for _, field := range []string{"Name", "Description", "Status", "limits.context_window", "Features.tool_calls", `extensions["source"].fields["value"]`} {
				want := baseline.Provenance().FindModelField("provider-a", "shared", field)
				if len(want) == 0 {
					t.Fatalf("original receipt missing for %s", field)
				}
				got := result.Catalog.Provenance().FindModelField("provider-a", "shared", field)
				gotBytes, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				wantBytes, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(gotBytes, wantBytes) {
					t.Errorf("%s receipt changed: got %#v, want %#v", field, got, want)
				}
			}
			retained := snapshotForTest(t, result.Catalog)
			lowerPriority := sourceIdentityObservation(t, sources.ProvidersID, sourceIdentityCatalog(t, "", catalogs.Model{
				ID: "shared", Name: "Shared", Description: "Older provider description", Status: catalogs.ModelStatusActive,
			}), at)
			next, err := ReconcileObservations(t.Context(), retained, []sources.Observation{
				{SourceID: sources.LocalCatalogID, Catalog: retained}, lowerPriority,
			}, WithChangeTime(addition.ObservedAt))
			if err != nil {
				t.Fatal(err)
			}
			nextProvider, err := next.Catalog.Provider("provider-a")
			if err != nil {
				t.Fatal(err)
			}
			if nextProvider.Models["shared"].Description != "Reviewed description" || nextProvider.Models["shared"].Status != catalogs.ModelStatusBeta {
				t.Fatal("sparse addition lowered authority before retained provider replay")
			}
		})
	}
}

func TestBaselineCarrierRequiresTrustedUnchangedReceipt(t *testing.T) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	original := sourceIdentityObservation(t, sources.ModelsDevHTTPID, sourceIdentityCatalog(t, "", catalogs.Model{
		ID: "shared", Name: "Shared", Description: "Reviewed description",
	}), at)
	baseline := snapshotForTest(t, sourceIdentityReconcile(t, sources.ModelsDevHTTPID, original).Catalog)
	provider, err := baseline.Provider("provider-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name          string
		observed      bool
		otherCatalog  bool
		edited        bool
		denied        bool
		ineligible    bool
		current       bool
		syntheticOnly bool
		wantCarried   bool
	}{
		{name: "accepted baseline", wantCarried: true},
		{name: "real release observation", observed: true},
		{name: "unaccepted catalog", otherCatalog: true},
		{name: "edited value", edited: true},
		{name: "hidden receipt", denied: true},
		{name: "ineligible source", ineligible: true},
		{name: "current observation", current: true},
		{name: "no real local delta", syntheticOnly: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			catalog := baseline
			if test.otherCatalog {
				builder, err := catalogs.NewBuilderFrom(baseline)
				if err != nil {
					t.Fatal(err)
				}
				catalog = snapshotForTest(t, builder)
			}
			observation := sources.Observation{SourceID: sources.ReleaseArtifactID, Catalog: catalog}
			if test.observed {
				observation = sourceIdentityObservation(t, sources.ReleaseArtifactID, catalog, at.Add(time.Hour))
			}
			merger := newMerger(authority.New(), NewAuthorityStrategy(authority.New()), baseline)
			addition := sourceIdentityObservation(t, sources.LocalCatalogID, sourceIdentityCatalog(t, "", catalogs.Model{ID: "added", Name: "Added"}), at.Add(time.Hour))
			inputs := []sources.Observation{observation}
			if !test.syntheticOnly {
				inputs = append(inputs, addition)
			}
			merger.setObservations(inputs)
			if test.denied {
				merger.projectedEvidence = func(catalogs.ProviderID, provenance.Entry) bool { return false }
			}
			model := catalogs.DeepCopyModel(*provider.Models["shared"])
			if test.edited {
				model.Description = "Edited description"
			}
			policy, ok := authority.New().Find(evidence.ResourceTypeModel, "Description")
			if !ok {
				t.Fatal("description policy missing")
			}
			if test.ineligible {
				policy.SourceOrder = []sources.ID{sources.ReleaseArtifactID}
			}
			models := map[sources.ID]*catalogs.Model{
				sources.ReleaseArtifactID: &model,
			}
			if test.current {
				current := catalogs.DeepCopyModel(model)
				current.Description = "Current observation"
				models[sources.ModelsDevHTTPID] = &current
			}
			resolved := merger.modelSourcesForPolicy("provider-a", "shared", policy, models)
			carried := resolved[sources.ModelsDevHTTPID] == &model
			if carried != test.wantCarried {
				t.Fatalf("carried original source = %v, want %v", carried, test.wantCarried)
			}
			if _, exists := merger.carried(evidence.ResourceTypeModel, provenance.ModelResourceID("provider-a", "shared"), "Description", sources.ModelsDevHTTPID); exists != test.wantCarried {
				t.Fatalf("remembered original receipt = %v, want %v", exists, test.wantCarried)
			}
		})
	}
}

func TestDeniedBaselineReceiptCannotUseAlternateCarrier(t *testing.T) {
	original := scopedReconciliationObservation(t, "retained", "shared", 1000, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC))
	baseline := snapshotForTest(t, sourceIdentityReconcile(t, sources.ProvidersID, original).Catalog)
	addition := sourceIdentityObservation(t, sources.LocalCatalogID, sourceIdentityCatalog(t, "", catalogs.Model{ID: "added", Name: "Added"}), original.ObservedAt.Add(time.Hour))
	inputs := []sources.Observation{
		{SourceID: sources.ReleaseArtifactID, Catalog: baseline},
		{SourceID: sources.EmbeddedCatalogID, Catalog: baseline}, addition,
	}
	for _, name := range []string{"hidden binding", "invalid binding revision"} {
		t.Run(name, func(t *testing.T) {
			permit := func(_ catalogs.ProviderID, entry provenance.Entry) bool {
				if entry.Source != sources.ProvidersID {
					return true
				}
				if name == "hidden binding" {
					return false
				}
				return entry.ProviderBindingID == original.ProviderBinding.ID && entry.ProviderBindingRevision == "other-revision"
			}
			result, err := ReconcileObservations(t.Context(), baseline, inputs, WithProjectedEvidencePolicy(permit), WithChangeTime(addition.ObservedAt))
			if err != nil {
				t.Fatal(err)
			}
			wantProvider, err := baseline.Provider("provider-a")
			if err != nil {
				t.Fatal(err)
			}
			gotProvider, err := result.Catalog.Provider("provider-a")
			if err != nil {
				t.Fatal(err)
			}
			if !wantProvider.Models["shared"].Equal(*gotProvider.Models["shared"]) {
				t.Fatal("denied carrier changed accepted historical model")
			}
			want, err := json.Marshal(baseline.Provenance().FindModelField("provider-a", "shared", "limits.context_window"))
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(result.Catalog.Provenance().FindModelField("provider-a", "shared", "limits.context_window"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("denied carrier replaced historical receipt: %s", got)
			}
			policies := authority.New()
			merger := newMerger(policies, NewAuthorityStrategy(policies), baseline)
			merger.setObservations(inputs)
			merger.projectedEvidence = permit
			policy, found := policies.Find(evidence.ResourceTypeModel, "Limits")
			if !found {
				t.Fatal("context window policy missing")
			}
			policy.EvidencePath = "limits.context_window"
			resolved := merger.modelSourcesForValue("provider-a", "shared", policy, map[sources.ID]*catalogs.Model{
				sources.ReleaseArtifactID: wantProvider.Models["shared"], sources.EmbeddedCatalogID: wantProvider.Models["shared"],
			}, func(model *catalogs.Model) any { return model.Limits.ContextWindow })
			if resolved[sources.ProvidersID] != nil || len(resolved) != 2 {
				t.Fatalf("denied claim changed fallback authority: %#v", resolved)
			}
		})
	}
}

func TestDeniedBaselineModalityRetainsHistoricalFallback(t *testing.T) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	features := &catalogs.ModelFeatures{Modalities: catalogs.ModelModalities{
		Input: []catalogs.ModelModality{catalogs.ModelModalityText}, Output: []catalogs.ModelModality{catalogs.ModelModalityEmbedding},
	}}
	features.SetSupport(catalogs.ModelFeatureStreaming, true)
	original := sourceIdentityObservation(t, sources.ModelsDevHTTPID, sourceIdentityCatalog(t, "", catalogs.Model{
		ID: "shared", Name: "Shared", Features: features, Metadata: &catalogs.ModelMetadata{Tags: []catalogs.ModelTag{"embedding"}},
	}), at)
	baseline := snapshotForTest(t, sourceIdentityReconcile(t, sources.ModelsDevHTTPID, original).Catalog)
	addition := sourceIdentityObservation(t, sources.LocalCatalogID, sourceIdentityCatalog(t, "", catalogs.Model{ID: "added", Name: "Added"}), at.Add(time.Hour))
	for _, current := range []bool{false, true} {
		name := "historical fallback"
		if current {
			name = "authorized current winner"
		}
		t.Run(name, func(t *testing.T) {
			inputs := []sources.Observation{{SourceID: sources.ReleaseArtifactID, Catalog: baseline}, {SourceID: sources.EmbeddedCatalogID, Catalog: baseline}, addition}
			var observed sources.Observation
			if current {
				currentFeatures := catalogs.DeepCopyModel(catalogs.Model{Features: features}).Features
				currentFeatures.SetSupport(catalogs.ModelFeatureStreaming, false)
				observed = sourceIdentityObservation(t, sources.ProvidersID, sourceIdentityCatalog(t, "", catalogs.Model{ID: "shared", Name: "Shared", Features: currentFeatures}), at.Add(time.Hour))
				inputs = append(inputs, observed)
			}
			result, err := ReconcileObservations(t.Context(), baseline, inputs, WithProjectedEvidencePolicy(func(_ catalogs.ProviderID, entry provenance.Entry) bool {
				return entry.Source != sources.ModelsDevHTTPID
			}), WithChangeTime(addition.ObservedAt))
			if err != nil {
				t.Fatal(err)
			}
			provider, err := result.Catalog.Provider("provider-a")
			if err != nil {
				t.Fatal(err)
			}
			model := provider.Models["shared"]
			if len(model.Features.Modalities.Output) != 1 || model.Features.Modalities.Output[0] != catalogs.ModelModalityEmbedding {
				t.Fatal("denied receipt erased accepted embedding output")
			}
			if _, err := result.Catalog.Build(); err != nil {
				t.Fatalf("accepted embedding model became invalid: %v", err)
			}
			got := result.Catalog.Provenance().FindModelField("provider-a", "shared", "Features.streaming")
			if current {
				if model.Features.Streaming || len(got) != 1 || got[0].ObservationID != observed.ID {
					t.Fatal("historical fallback blocked authorized current capability")
				}
			} else {
				want := baseline.Provenance().FindModelField("provider-a", "shared", "Features.streaming")
				wantJSON, err := json.Marshal(want)
				if err != nil {
					t.Fatal(err)
				}
				gotJSON, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				if !model.Features.Streaming || !bytes.Equal(gotJSON, wantJSON) {
					t.Fatal("weak fallback changed historical capability receipt")
				}
			}
		})
	}
}
