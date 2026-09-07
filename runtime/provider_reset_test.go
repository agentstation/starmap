package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProviderResetReplacesHistoryAndSurvivesRestart(t *testing.T) {
	connected, options, at := providerResetRuntime(t, storage.NewMemory())
	original := manualProviderObservation(t, 200, at)
	if _, err := connected.PublishObservations(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	replacement := manualProviderObservation(t, 0, at.Add(time.Minute))
	state, err := connected.UpdateObservations(t.Context(), func(context.Context, ObservationInputs) ([]sources.Observation, error) {
		return []sources.Observation{replacement}, nil
	}, ProviderObservationReset{ProviderID: "provider"})
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := state.Catalog.Providers().Get("provider")
	if provider.Models["model"].Limits.ContextWindow != 100 {
		t.Fatalf("reset retained old local limit: %d", provider.Models["model"].Limits.ContextWindow)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != state.GenerationID || reopened.State().PayloadChecksum != state.PayloadChecksum {
		t.Fatal("restart changed reset generation")
	}
	provider, _ = reopened.State().Catalog.Providers().Get("provider")
	if provider.Models["model"].Limits.ContextWindow != 100 {
		t.Fatal("restart restored a reset provider fact")
	}
}

func providerResetRuntime(t *testing.T, store storage.Store, extra ...Option) (*Runtime, []Option, time.Time) {
	t.Helper()
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	baseline := manualProviderObservation(t, 100, at.Add(-time.Hour))
	source := newStubSource("reset-baseline")
	payload, err := catalogs.EncodeCatalogPayload(baseline.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	source.replies = []SourceRead{testSourceRead(t, "reset-baseline-generation", payload, at.Add(-time.Hour))}
	options := []Option{WithSource(source), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store))}
	options = append(options, extra...)
	connected := openTestRuntime(t, options...)
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	return connected, options, at
}

func publishProviderReset(t *testing.T, connected *Runtime, replacement sources.Observation, resets ...ProviderObservationReset) starmap.CatalogState {
	t.Helper()
	state, err := connected.UpdateObservations(t.Context(), func(context.Context, ObservationInputs) ([]sources.Observation, error) {
		return []sources.Observation{replacement}, nil
	}, resets...)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestProviderResetAnchorsEarlierScheduledFiles(t *testing.T) {
	connected, options, at := providerResetRuntime(t, storage.NewMemory())
	original := manualProviderObservation(t, 200, at)
	layer, err := NewProviderLayer("provider", original)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connected.publishProviders(t.Context(), []ProviderLayer{layer}, connected.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	if connected.layers.manual != nil {
		t.Fatal("fixture already has manual history")
	}
	reset := publishProviderReset(t, connected, manualProviderObservation(t, 0, at.Add(time.Minute)), ProviderObservationReset{ProviderID: "provider"})
	if len(manualBatches(connected.layers.manual)) != 2 {
		t.Fatal("reset did not anchor scheduled evidence separately")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != reset.GenerationID {
		t.Fatal("restart changed anchored reset identity")
	}
	provider, _ := reopened.State().Catalog.Providers().Get("provider")
	if provider.Models["model"].Limits.ContextWindow != 100 {
		t.Fatal("retained provider file restored a cleared fact")
	}
}

func TestProviderResetPreservesOtherProviderInAggregate(t *testing.T) {
	connected, options, at := providerResetRuntime(t, storage.NewMemory())
	original := manualProviderObservation(t, 200, at)
	builder, err := catalogs.NewBuilderFrom(original.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := builder.Provider("provider")
	provider.ID, provider.Name = "other", "Other"
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	catalog, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	aggregate, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Status: sources.ObservationStatusSucceeded, Completeness: sources.ObservationCompletenessComplete, Records: sources.ObservationRecordCounts{Accepted: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connected.PublishObservations(t.Context(), aggregate); err != nil {
		t.Fatal(err)
	}
	reset := publishProviderReset(t, connected, manualProviderObservation(t, 0, at.Add(time.Minute)), ProviderObservationReset{ProviderID: "provider"})
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != reset.GenerationID {
		t.Fatal("aggregate reset identity changed")
	}
	for id, want := range map[catalogs.ProviderID]int64{"provider": 100, "other": 200} {
		provider, _ := reopened.State().Catalog.Providers().Get(id)
		if provider.Models["model"].Limits.ContextWindow != want {
			t.Fatalf("reset changed wrong scope: %s", id)
		}
	}
	entry := reopened.State().Catalog.Provenance().FindModelField("other", "model", "limits.context_window")
	if len(entry) != 1 || entry[0].ObservationID != aggregate.ID || entry[0].EvidenceChecksum != aggregate.EvidenceChecksum {
		t.Fatal("remaining provider lost original aggregate receipt")
	}
	history, err := reopened.store.loadManualHistory(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manualBatches(history)[0].observations[0].restore(); err != nil {
		t.Fatal("reset rewrote original observation")
	}
}

func TestProviderResetIdentityBindsUnchangedReplacement(t *testing.T) {
	store := &retentionAmbiguousStore{Memory: storage.NewMemory()}
	connected, options, at := providerResetRuntime(t, store)
	original := manualProviderObservation(t, 200, at)
	if _, err := connected.PublishObservations(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	before := connected.State()
	store.ambiguous.Store(true)
	_, err := connected.UpdateObservations(t.Context(), func(context.Context, ObservationInputs) ([]sources.Observation, error) {
		return []sources.Observation{original}, nil
	}, ProviderObservationReset{ProviderID: "provider"})
	if err == nil {
		t.Fatal("lost commit reply did not fail")
	}
	accepted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Manifest.GenerationID == before.GenerationID || accepted.Manifest.Payload.Checksum != before.PayloadChecksum {
		t.Fatal("reset identity did not distinguish unchanged payload")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != accepted.Manifest.GenerationID {
		t.Fatal("lost reply did not recover the reset")
	}
	if len(reopened.layers.manual.resets) != 1 {
		t.Fatal("recovery discarded reset scope")
	}
	if pending, err := reopened.store.loadInputPublication(); err != nil || pending != nil {
		t.Fatal("reset recovery left a pending journal")
	}
}
