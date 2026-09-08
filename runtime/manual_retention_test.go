package runtime

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func manualTestObservation(t *testing.T, model string, at time.Time, partial bool) sources.Observation {
	t.Helper()
	catalog, err := catalogs.DecodeCatalogPayload(testCatalogPayload(t, "manual-provider", model, model))
	if err != nil {
		t.Fatal(err)
	}
	metadata := sources.ObservationMetadata{ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded, Records: sources.ObservationRecordCounts{Accepted: 1}}
	if partial {
		metadata.Completeness, metadata.Status = sources.ObservationCompletenessPartial, sources.ObservationStatusDegraded
		metadata.Records.Rejected = 1
		metadata.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "manual-provider/invalid", Message: "private manual diagnostic sentinel"}}
	}
	observation, err := sources.NewObservation(sources.ReleaseArtifactID, catalog, metadata)
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func manualTestRuntime(t *testing.T, store storage.Store) (*Runtime, []Option) {
	t.Helper()
	source := newStubSource("manual-baseline")
	source.replies = []SourceRead{testSourceRead(t, "manual-baseline-generation", testCatalogPayload(t, "baseline-provider", "baseline-model", "Baseline"), time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))}
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithSource(source), WithClientOptions(starmap.WithCatalogStore(store))}
	connected := openTestRuntime(t, options...)
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	return connected, options
}

func TestManualObservationsRetainPartialHistoryAndReceiptsAcrossRestart(t *testing.T) {
	store := storage.NewMemory()
	connected, options := manualTestRuntime(t, store)
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	first := manualTestObservation(t, "first", at, false)
	second := manualTestObservation(t, "second", at.Add(time.Minute), true)
	for _, observation := range []sources.Observation{first, second} {
		if _, err := connected.PublishObservations(t.Context(), observation); err != nil {
			t.Fatal(err)
		}
	}
	before := connected.State()
	provider, err := before.Catalog.Provider("manual-provider")
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{"first", "second"} {
		if provider.Models[model] == nil {
			t.Fatalf("manual history lost %s", model)
		}
	}
	baseline, err := before.Catalog.Provider("baseline-provider")
	if err != nil || baseline.Models["baseline-model"] == nil {
		t.Fatal("manual history discarded the selected source baseline")
	}
	generation, err := store.Current(t.Context())
	if err != nil || len(generation.Manifest.SourceObservations) != 2 {
		t.Fatalf("manual receipts = %d, error = %v", len(generation.Manifest.SourceObservations), err)
	}
	entries, err := os.ReadDir(filepath.Join(connected.store.root, inputPublicationDirectory))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(connected.store.root, inputPublicationDirectory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte("private manual diagnostic sentinel")) {
			t.Fatal("manual retention persisted diagnostic text")
		}
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != before.GenerationID || reopened.State().PayloadChecksum != before.PayloadChecksum {
		t.Fatal("restart changed the manual catalog generation")
	}
	sequence := reopened.State().Sequence
	if _, err := reopened.PublishObservations(t.Context(), second); err != nil {
		t.Fatal(err)
	}
	if reopened.State().GenerationID != before.GenerationID || reopened.State().Sequence != sequence || len(manualBatches(reopened.layers.manual)) != 2 {
		t.Fatal("repeated observation appended or changed accepted history")
	}
}

func TestRejectedManualPublicationCannotChangeRetainedHistory(t *testing.T) {
	store := &retentionRejectingStore{Memory: storage.NewMemory()}
	connected, options := manualTestRuntime(t, store)
	before := connected.State()
	store.reject.Store(true)
	observation := manualTestObservation(t, "rejected", time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC), false)
	if _, err := connected.PublishObservations(t.Context(), observation); err == nil {
		t.Fatal("rejected manual publication succeeded")
	}
	if history, err := connected.store.loadManualHistory(t.Context()); err != nil || history != nil {
		t.Fatalf("rejected publication changed history: %v", err)
	}
	if connected.State().GenerationID != before.GenerationID {
		t.Fatal("rejected publication changed active state")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	store.reject.Store(false)
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != before.GenerationID {
		t.Fatal("restart activated rejected manual observations")
	}
}

func TestManualRetentionRecoversLostCatalogCommitReply(t *testing.T) {
	store := &retentionAmbiguousStore{Memory: storage.NewMemory()}
	connected, options := manualTestRuntime(t, store)
	store.ambiguous.Store(true)
	observation := manualTestObservation(t, "accepted", time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC), false)
	if _, err := connected.PublishObservations(t.Context(), observation); err == nil {
		t.Fatal("lost catalog reply did not return an error")
	}
	accepted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != accepted.Manifest.GenerationID {
		t.Fatal("recovery did not reconstruct the accepted manual generation")
	}
	if len(manualBatches(reopened.layers.manual)) != 1 {
		t.Fatal("recovery did not retain the accepted manual batch")
	}
}

func TestManualSingleBatchRetainsSeparateReviewedInputs(t *testing.T) {
	connected, _ := manualTestRuntime(t, storage.NewMemory())
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	first := manualTestObservation(t, "first", at, false)
	second := manualTestObservation(t, "second", at.Add(time.Minute), false)
	if _, err := connected.PublishObservations(t.Context(), first, second); err != nil {
		t.Fatal(err)
	}
	provider, err := connected.Catalog().Provider("manual-provider")
	if err != nil || provider.Models["first"] == nil || provider.Models["second"] == nil {
		t.Fatal("one reviewed input hid another input in the same batch")
	}
}
