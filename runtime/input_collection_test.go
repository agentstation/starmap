package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

type retainedInputCollector interface {
	CollectRetainedInputs(context.Context, InputCollectionRequest) (InputCollectionReport, error)
}

func collectionObservation(t *testing.T, minute int) manualObservation {
	t.Helper()
	at := time.Date(2026, 9, 12, 0, minute, 0, 0, time.UTC)
	observation := manualProviderObservation(t, 200+int64(minute), at)
	prepared, err := prepareManualObservations(t.Context(), []sources.Observation{observation})
	if err != nil {
		t.Fatal(err)
	}
	return prepared[0]
}

func stageCollectionInput(t *testing.T, store *layerStore, input any) string {
	t.Helper()
	name, err := store.stageInput(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func TestRuntimeInputCollectionPreservesPendingAndUnknown(t *testing.T) {
	r := openTestRuntime(t, WithCatalogSource("embedded"))
	observation := collectionObservation(t, 0)
	observationRef := stageCollectionInput(t, r.store, observation)
	batch := &manualBatch{observations: []manualObservation{observation}}
	manualRef, err := r.store.stageManualBatch(t.Context(), batch)
	if err != nil {
		t.Fatal(err)
	}
	batch.reference = manualRef
	acceptedObservation := collectionObservation(t, 2)
	acceptedRef, err := r.store.stageManualBatch(t.Context(), &manualBatch{observations: []manualObservation{acceptedObservation}})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.store.saveManualHead(t.Context(), acceptedRef); err != nil {
		t.Fatal(err)
	}
	// A committed catalog can advance before its retention journal finishes.
	r.layers.manual = batch
	provider := testProviderLayer(t, "collection", "model", "Model", observation.Receipt.Link.ObservedAt)
	providerRef := stageCollectionInput(t, r.store, provider)
	generation := aliasGeneration(t, "collection-source")
	sourceRef := stageCollectionInput(t, r.store, sourceLayer{Identity: "collection-source", GenerationID: generation.Manifest.GenerationID,
		Checksum: generation.Manifest.Payload.Checksum, Payload: generation.Payload})
	removalRef := stageCollectionInput(t, r.store, removalPolicyRecord{Version: removalPolicyVersion,
		Policy: catalogs.CatalogRemovalPolicy{PublisherID: "collection"}})
	pending := inputPublication{Version: inputPublicationVersion, Phase: inputPublicationPrepared, GenerationID: "pending", PayloadChecksum: "checksum",
		Source: sourceRef, Providers: []string{providerRef}, Manual: manualRef, Removals: removalRef}
	if err := r.store.writeInputPublication(t.Context(), pending); err != nil {
		t.Fatal(err)
	}
	orphan := stageCollectionInput(t, r.store, collectionObservation(t, 1))
	unknown := stageCollectionInput(t, r.store, map[string]any{"version": 99, "future": "record"})
	directory, err := r.store.directory.ExistingChild(inputPublicationDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.PublishFileContext(t.Context(), "operator-note.txt", []byte("keep"), ".test-"); err != nil {
		t.Fatal(err)
	}
	changed := strings.Repeat("f", 64) + ".json"
	if err := directory.PublishFileContext(t.Context(), changed, []byte(`{"payload":"changed"}`), ".test-"); err != nil {
		t.Fatal(err)
	}
	request := InputCollectionRequest{MaxEntries: 64, MaxBytes: maxLayerBytes, DryRun: true}
	report, err := r.CollectRetainedInputs(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Removed) != 0 || !slices.Equal(report.Candidates, []string{orphan}) {
		t.Fatalf("invalid dry run: %+v", report)
	}
	for _, name := range []string{acceptedRef, manualRef, observationRef, providerRef, sourceRef, removalRef} {
		if !slices.Contains(report.Protected, name) {
			t.Fatalf("pending reference not protected: %s, %+v", name, report)
		}
	}
	for _, name := range []string{unknown, changed, "operator-note.txt"} {
		if !slices.Contains(report.Preserved, name) {
			t.Fatalf("unknown input not preserved: %s, %+v", name, report)
		}
	}
	request.DryRun = false
	report, err = r.CollectRetainedInputs(t.Context(), request)
	if err != nil || !slices.Equal(report.Removed, []string{orphan}) {
		t.Fatalf("collection: %+v, %v", report, err)
	}
	if _, err := directory.ReadFile(orphan, maxLayerBytes); !os.IsNotExist(err) {
		t.Fatalf("orphan remains: %v", err)
	}
	if _, err := r.store.readManualHistory(t.Context(), manualRef); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.store.publicationInputs(pending); err != nil {
		t.Fatal(err)
	}
	if _, err := r.store.readRemovalInput(removalRef); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeInputCollectionRefusesUnsafePreflight(t *testing.T) {
	r := openTestRuntime(t, WithCatalogSource("embedded"))
	orphan := stageCollectionInput(t, r.store, collectionObservation(t, 0))
	directory, err := r.store.directory.ExistingChild(inputPublicationDirectory)
	if err != nil {
		t.Fatal(err)
	}
	request := InputCollectionRequest{MaxEntries: 64, MaxBytes: maxLayerBytes}
	for _, test := range []struct {
		name        string
		manual      any
		publication any
		active      *manualBatch
		request     InputCollectionRequest
	}{
		{name: "future head", manual: manualHistoryHead{Version: 99, Batch: orphan}},
		{name: "missing history", manual: manualHistoryHead{Version: manualHistoryVersion, Batch: strings.Repeat("a", 64) + ".json"}},
		{name: "wrong history kind", manual: manualHistoryHead{Version: manualHistoryVersion, Batch: orphan}},
		{name: "future publication", publication: inputPublication{Version: 99, Phase: inputPublicationIdle}},
		{name: "missing accepted head", active: &manualBatch{reference: orphan}},
		{name: "entry limit", request: InputCollectionRequest{MaxEntries: 1, MaxBytes: maxLayerBytes}},
		{name: "byte limit", request: InputCollectionRequest{MaxEntries: 64, MaxBytes: 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for name, value := range map[string]any{manualHistoryName: test.manual, inputPublicationName: test.publication} {
				if value == nil {
					continue
				}
				raw, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if err := r.store.directory.PublishFileContext(t.Context(), name, raw, ".test-"); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if _, err := r.store.directory.CompareAndRemoveFileContext(context.WithoutCancel(t.Context()), name, raw); err != nil {
						t.Error(err)
					}
				})
			}
			r.layers.manual = test.active
			t.Cleanup(func() { r.layers.manual = nil })
			selected := request
			if test.request.MaxEntries != 0 {
				selected = test.request
			}
			report, err := r.CollectRetainedInputs(t.Context(), selected)
			if err == nil || len(report.Removed) != 0 {
				t.Fatalf("unsafe preflight deleted input: %+v, %v", report, err)
			}
			if _, err := directory.ReadFile(orphan, maxLayerBytes); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRuntimeInputCollectionPreservesLegacyBytes(t *testing.T) {
	r := openTestRuntime(t, WithCatalogSource("embedded"))
	directory, err := r.store.directory.Child(inputPublicationDirectory)
	if err != nil {
		t.Fatal(err)
	}
	observation := collectionObservation(t, 0)
	raw, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, raw, "", "  "); err != nil {
		t.Fatal(err)
	}
	legacy := formatted.Bytes()
	digest := sha256.Sum256(legacy)
	reference := hex.EncodeToString(digest[:]) + ".json"
	if err := directory.PublishFileContext(t.Context(), reference, legacy, ".test-"); err != nil {
		t.Fatal(err)
	}
	head := stageCollectionInput(t, r.store, manualBatchRecord{Version: manualHistoryLegacyVersion, Observations: []string{reference}})
	if err := r.store.saveManualHead(t.Context(), head); err != nil {
		t.Fatal(err)
	}
	r.layers.manual, err = r.store.loadManualHistory(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	orphan := stageCollectionInput(t, r.store, observation)
	report, err := r.CollectRetainedInputs(t.Context(), InputCollectionRequest{MaxEntries: 64, MaxBytes: maxLayerBytes})
	if err != nil || !slices.Equal(report.Removed, []string{orphan}) || !slices.Contains(report.Protected, reference) {
		t.Fatalf("legacy filename lost: %+v, %v", report, err)
	}
	retained, err := directory.ReadFile(reference, maxLayerBytes)
	if err != nil || !bytes.Equal(retained, legacy) {
		t.Fatalf("legacy bytes changed: %v", err)
	}
}

func TestRuntimeInputCollectionWorksOfflineWithPin(t *testing.T) {
	store := storage.NewMemory()
	generation := aliasGeneration(t, "collection-pin")
	if err := store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, WithCatalogSource("embedded"), WithGenerationPin(generation.Manifest.GenerationID),
		WithClientOptions(starmap.WithCatalogStore(store)), WithCatalogNetworkMode("offline"))
	orphan := stageCollectionInput(t, r.store, collectionObservation(t, 0))
	before := r.State()
	report, err := r.CollectRetainedInputs(t.Context(), InputCollectionRequest{MaxEntries: 64, MaxBytes: maxLayerBytes})
	if err != nil || !slices.Equal(report.Removed, []string{orphan}) {
		t.Fatalf("pinned maintenance refused: %+v, %v", report, err)
	}
	if after := r.State(); before.GenerationID != after.GenerationID || before.PayloadChecksum != after.PayloadChecksum {
		t.Fatal("collection changed the pinned catalog")
	}
}

func TestRuntimeInputCollectionCancellationAndClose(t *testing.T) {
	r := openTestRuntime(t, WithCatalogSource("embedded"))
	orphan := stageCollectionInput(t, r.store, collectionObservation(t, 0))
	request := InputCollectionRequest{MaxEntries: 64, MaxBytes: maxLayerBytes}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if report, err := r.CollectRetainedInputs(ctx, request); !stderrors.Is(err, context.Canceled) || len(report.Removed) != 0 {
		t.Fatalf("canceled collection: %+v, %v", report, err)
	}
	r.publicationMu.Lock()
	result := make(chan error, 1)
	go func() { _, err := r.CollectRetainedInputs(t.Context(), request); result <- err }()
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		r.runs.mu.Lock()
		active := r.runs.active != nil
		r.runs.mu.Unlock()
		if active {
			break
		}
		select {
		case <-deadline:
			r.publicationMu.Unlock()
			t.Fatal("collection did not join runtime ownership")
		case <-ticker.C:
		}
	}
	closed := make(chan error, 1)
	go func() { closed <- r.Close() }()
	select {
	case <-r.ctx.Done():
	case <-deadline:
		r.publicationMu.Unlock()
		t.Fatal("close did not cancel collection")
	}
	select {
	case err := <-closed:
		r.publicationMu.Unlock()
		t.Fatalf("close released an active collector: %v", err)
	default:
	}
	r.publicationMu.Unlock()
	if err := <-result; !stderrors.Is(err, context.Canceled) {
		t.Fatalf("shutdown collection: %v", err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	directory, err := r.store.directory.ExistingChild(inputPublicationDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := directory.ReadFile(orphan, maxLayerBytes); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CollectRetainedInputs(t.Context(), request); err == nil {
		t.Fatal("closed runtime accepted collection")
	}
}

func TestRuntimeInputCollectionPreservesCheckpointAndRetriesPartialRemoval(t *testing.T) {
	r := openTestRuntime(t, WithCatalogSource("embedded"))
	old := &manualBatch{observations: []manualObservation{collectionObservation(t, 0)}}
	oldRef, err := r.store.stageManualBatch(t.Context(), old)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := checkpointManualHistory(t.Context(), old)
	if err != nil {
		t.Fatal(err)
	}
	child := &manualBatch{parent: checkpoint, observations: []manualObservation{collectionObservation(t, 1)}}
	head, err := r.store.stageManualBatch(t.Context(), child)
	if err != nil {
		t.Fatal(err)
	}
	child.reference = head
	if err := r.store.saveManualHead(t.Context(), head); err != nil {
		t.Fatal(err)
	}
	r.layers.manual = child
	// Represent interruption after removal of an unreachable observation but before its batch.
	directory, err := r.store.directory.ExistingChild(inputPublicationDirectory)
	if err != nil {
		t.Fatal(err)
	}
	observationRef := stageCollectionInput(t, r.store, old.observations[0])
	raw, err := directory.ReadFile(observationRef, maxLayerBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := directory.CompareAndRemoveFileContext(t.Context(), observationRef, raw); err != nil {
		t.Fatal(err)
	}
	report, err := r.CollectRetainedInputs(t.Context(), InputCollectionRequest{MaxEntries: 64, MaxBytes: maxLayerBytes})
	if err != nil || !slices.Equal(report.Removed, []string{oldRef}) {
		t.Fatalf("partial collection did not recover: %+v, %v", report, err)
	}
	restored, err := r.store.loadManualHistory(t.Context())
	if err != nil || restored == nil || restored.parent == nil || restored.parent.checkpoint == nil {
		t.Fatalf("checkpoint lost: %+v, %v", restored, err)
	}
	if !slices.Equal(restored.parent.packed[0].observations[0].Payload, old.observations[0].Payload) {
		t.Fatal("checkpoint lost original payload")
	}
	report, err = r.CollectRetainedInputs(t.Context(), InputCollectionRequest{MaxEntries: 64, MaxBytes: maxLayerBytes})
	if err != nil || len(report.Removed) != 0 || len(report.Candidates) != 0 {
		t.Fatalf("repeated collection: %+v, %v", report, err)
	}
}

func TestRuntimeInputCollectionWaitsForPublication(t *testing.T) {
	store := &gatedReconciliationStore{Store: storage.NewMemory()}
	options := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store))}
	r := openTestRuntime(t, options...)
	entered, release := make(chan struct{}), make(chan struct{})
	releaseOnce := sync.OnceFunc(func() { close(release) })
	defer releaseOnce()
	store.mu.Lock()
	store.entered, store.release = entered, release
	store.mu.Unlock()
	observation := manualTestObservation(t, "collection-concurrent", time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC), false)
	published := make(chan error, 1)
	go func() { _, err := r.PublishObservations(t.Context(), observation); published <- err }()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("publication did not reach the catalog store")
	}
	ctx, cancel := context.WithCancel(t.Context())
	waiting := make(chan error, 1)
	go func() {
		_, err := r.CollectRetainedInputs(ctx, InputCollectionRequest{MaxEntries: 64, MaxBytes: maxLayerBytes})
		waiting <- err
	}()
	// Cancellation must unblock a collector waiting for the unfinished publisher.
	cancel()
	if err := <-waiting; !stderrors.Is(err, context.Canceled) {
		t.Fatalf("waiting collector: %v", err)
	}
	releaseOnce()
	if err := <-published; err != nil {
		t.Fatal(err)
	}
	accepted := r.State()
	report, err := r.CollectRetainedInputs(t.Context(), InputCollectionRequest{MaxEntries: 64, MaxBytes: maxLayerBytes})
	if err != nil || len(report.Protected) < 2 || len(report.Removed) != 0 {
		t.Fatalf("published history not protected: %+v, %v", report, err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != accepted.GenerationID || reopened.State().PayloadChecksum != accepted.PayloadChecksum {
		t.Fatal("collection changed restart state")
	}
}

func TestRuntimeInputCollectionPreservesAcceptedHistory(t *testing.T) {
	r := openTestRuntime(t, WithStateDirectory(privateRuntimeDirectory(t)), WithCatalogSource("embedded"))
	collector, ok := any(r).(retainedInputCollector)
	if !ok {
		t.Fatal("runtime has no collection operation that preserves accepted input references")
	}
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	observation := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 200, at).Catalog, at, nil)
	prepared, err := prepareManualObservations(t.Context(), []sources.Observation{observation})
	if err != nil {
		t.Fatal(err)
	}
	history := &manualBatch{observations: prepared}
	reference, err := r.store.stageManualBatch(t.Context(), history)
	if err != nil {
		t.Fatal(err)
	}
	history.reference = reference
	if err := r.store.saveManualHead(t.Context(), reference); err != nil {
		t.Fatal(err)
	}
	r.layers.manual = history
	orphanObservation := providerResetObservation(t, sources.ModelsDevHTTPID, observation.Catalog, at.Add(time.Minute), nil)
	orphans, err := prepareManualObservations(t.Context(), []sources.Observation{orphanObservation})
	if err != nil {
		t.Fatal(err)
	}
	orphan, err := r.store.stageInput(t.Context(), orphans[0])
	if err != nil {
		t.Fatal(err)
	}
	before := r.State()
	report, err := collector.CollectRetainedInputs(t.Context(), InputCollectionRequest{MaxEntries: 64, MaxBytes: maxLayerBytes})
	if err != nil || len(report.Removed) != 1 || report.Removed[0] != orphan || !slices.Contains(report.Protected, reference) {
		t.Fatalf("collection lost reference boundaries: %+v, %v", report, err)
	}
	reloaded, err := r.store.loadManualHistory(t.Context())
	if err != nil || reloaded == nil || reloaded.reference != reference || len(reloaded.observations) != 1 {
		t.Fatalf("accepted history did not survive collection: %+v, %v", reloaded, err)
	}
	if after := r.State(); after.GenerationID != before.GenerationID || after.PayloadChecksum != before.PayloadChecksum {
		t.Fatal("input collection changed the serving catalog")
	}
}
