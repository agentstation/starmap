package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func manualProviderObservation(t *testing.T, limit int64, at time.Time) sources.Observation {
	t.Helper()
	catalog, err := catalogs.DecodeCatalogPayload(testCatalogPayload(t, "provider", "model", "Model"))
	if err != nil {
		t.Fatal(err)
	}
	builder, err := catalogs.NewBuilderFrom(catalog)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := builder.Provider("provider")
	if limit != 0 {
		provider.Models["model"].Limits = &catalogs.ModelLimits{ContextWindow: limit}
	}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	observed, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, observed, sources.ObservationMetadata{
		ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
		Records: sources.ObservationRecordCounts{Accepted: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func TestManualProviderHistoryUsesFallbackAndConflictPolicy(t *testing.T) {
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	for _, fallback := range []bool{true, false} {
		name := "equal-time-conflict"
		if fallback {
			name = "later-fallback"
		}
		t.Run(name, func(t *testing.T) {
			direct := manualProviderObservation(t, 200, at)
			metadata := sources.ObservationMetadata{ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded}
			if fallback {
				metadata.ObservedAt = at.Add(time.Hour)
				metadata.Status = sources.ObservationStatusDegraded
				metadata.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeStaleFallback, Code: sources.ObservationIssueCodeStaleFallback, Message: "Provider response uses retained evidence"}}
			}
			other, err := sources.NewObservation(sources.ProvidersID, manualProviderObservation(t, 300, at).Catalog, metadata)
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := prepareManualObservations(t.Context(), []sources.Observation{direct, other})
			if err != nil {
				t.Fatal(err)
			}
			layers := layerSet{manual: &manualBatch{observations: prepared}}
			state, err := layers.build(t.Context(), starmap.CatalogState{GenerationID: "baseline", Catalog: manualProviderObservation(t, 100, at).Catalog, GeneratedAt: at.Add(-time.Hour)})
			if !fallback {
				if !errors.IsConflict(err) {
					t.Fatalf("equal-time conflict = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			provider, err := state.Catalog.Provider("provider")
			if err != nil || provider.Models["model"].Limits.ContextWindow != 200 {
				t.Fatal("a later fallback replaced direct provider facts")
			}
		})
	}
}

func TestManualAggregateProviderReceiptSurvivesRestart(t *testing.T) {
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	base := manualProviderObservation(t, 200, at).Catalog
	builder, err := catalogs.NewBuilderFrom(base)
	if err != nil {
		t.Fatal(err)
	}
	peer, err := base.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	peer.ID, peer.Name = "peer", "Peer"
	if err := builder.SetProvider(peer); err != nil {
		t.Fatal(err)
	}
	aggregate, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, aggregate, sources.ObservationMetadata{ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded, Records: sources.ObservationRecordCounts{Accepted: 2}})
	if err != nil {
		t.Fatal(err)
	}
	connected, options := manualTestRuntime(t, storage.NewMemory())
	if _, err := connected.PublishObservations(t.Context(), observation); err != nil {
		t.Fatal(err)
	}
	before := connected.State()
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != before.GenerationID || reopened.State().PayloadChecksum != before.PayloadChecksum {
		t.Fatal("restart changed aggregate provider evidence")
	}
	retained := reopened.layers.manual.observations
	if len(retained) != 1 {
		t.Fatal("aggregate receipt was split across retained inputs")
	}
	actual, err := retained[0].restore()
	if err != nil || actual.ID != observation.ID || actual.EvidenceChecksum != observation.EvidenceChecksum || actual.Catalog.Providers().Len() != 2 {
		t.Fatalf("aggregate receipt did not retain its exact provider set: %v", err)
	}
}

func TestManualProviderHistoryAndScheduledLayerUseObservationTime(t *testing.T) {
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	base := manualProviderObservation(t, 100, at).Catalog
	manual := manualProviderObservation(t, 300, at.Add(time.Minute))
	prepared, err := prepareManualObservations(t.Context(), []sources.Observation{manual})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		at    time.Time
		limit int64
		want  int64
	}{
		{name: "scheduled-older", at: at, limit: 200, want: 300},
		{name: "scheduled-newer", at: at.Add(2 * time.Minute), limit: 400, want: 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			layer, err := NewProviderLayer("provider", manualProviderObservation(t, test.limit, test.at))
			if err != nil {
				t.Fatal(err)
			}
			layers := layerSet{manual: &manualBatch{observations: prepared}}
			layers.setProvider(layer)
			state, err := layers.build(t.Context(), starmap.CatalogState{GenerationID: "baseline", Catalog: base, GeneratedAt: at.Add(-time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			provider, err := state.Catalog.Provider("provider")
			if err != nil || provider.Models["model"] == nil || provider.Models["model"].Limits == nil || provider.Models["model"].Limits.ContextWindow != test.want {
				t.Fatalf("provider facts did not select limit %d: %v", test.want, err)
			}
		})
	}
}

func TestManualHistoryRetainsPriorProviderFactsAfterScheduledOmission(t *testing.T) {
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	source := newStubSource("manual-provider-baseline")
	payload, err := catalogs.EncodeCatalogPayload(manualProviderObservation(t, 100, at).Catalog)
	if err != nil {
		t.Fatal(err)
	}
	source.replies = []SourceRead{testSourceRead(t, "provider-baseline", payload, at)}
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithSource(source), WithClientOptions(starmap.WithCatalogStore(storage.NewMemory()))}
	connected := openTestRuntime(t, options...)
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	first, err := NewProviderLayer("provider", manualProviderObservation(t, 200, at.Add(time.Minute)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connected.publishProviders(t.Context(), []ProviderLayer{first}, connected.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	if _, err := connected.PublishObservations(t.Context(), manualProviderObservation(t, 0, at.Add(2*time.Minute))); err != nil {
		t.Fatal(err)
	}
	latest, err := NewProviderLayer("provider", manualProviderObservation(t, 0, at.Add(3*time.Minute)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connected.publishProviders(t.Context(), []ProviderLayer{latest}, connected.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	provider, err := connected.Catalog().Provider("provider")
	if err != nil || provider.Models["model"] == nil || provider.Models["model"].Limits == nil || provider.Models["model"].Limits.ContextWindow != 200 {
		t.Fatal("a later provider omission discarded a previously accepted fact")
	}
	before := connected.State()
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != before.GenerationID || reopened.State().PayloadChecksum != before.PayloadChecksum {
		t.Fatal("restart did not retain the provider history")
	}
}
