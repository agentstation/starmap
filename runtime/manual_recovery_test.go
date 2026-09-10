package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestManualRecoveryValidatesParentBeforeApplyingProviderInputs(t *testing.T) {
	store, err := newLayerStore(privateRuntimeDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	prepared, err := prepareManualObservations(t.Context(), []sources.Observation{manualTestObservation(t, "parent", at, false)})
	if err != nil {
		t.Fatal(err)
	}
	parent := &manualBatch{observations: prepared}
	parent.reference, err = store.stageManualBatch(t.Context(), parent)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err = prepareManualObservations(t.Context(), []sources.Observation{manualTestObservation(t, "child", at.Add(time.Minute), false)})
	if err != nil {
		t.Fatal(err)
	}
	child, err := store.stageManualBatch(t.Context(), &manualBatch{parent: parent, observations: prepared})
	if err != nil {
		t.Fatal(err)
	}
	layer := scopedProviderLayer(t, "binding", "1", at)
	provider, err := store.stageInput(t.Context(), layer)
	if err != nil {
		t.Fatal(err)
	}
	record := inputPublication{Version: inputPublicationVersion, Phase: inputPublicationCommitted, GenerationID: "accepted", PayloadChecksum: "accepted-checksum", Manual: child, Providers: []string{provider}}
	if err := store.writeInputPublication(t.Context(), record); err != nil {
		t.Fatal(err)
	}
	directory, err := store.directory.ExistingChild(inputPublicationDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.WriteFile(parent.reference, []byte("{}"), ".test-"); err != nil {
		t.Fatal(err)
	}
	if err := store.recoverInputPublication(t.Context(), starmap.CatalogState{}); !errors.IsConflict(err) {
		t.Fatalf("damaged parent recovery = %v", err)
	}
	providers, err := store.loadProviders()
	if err != nil || len(providers) != 0 {
		t.Fatalf("recovery installed provider inputs before checking history: %v", err)
	}
	if history, err := store.loadManualHistory(t.Context()); err != nil || history != nil {
		t.Fatalf("recovery installed damaged history: %v", err)
	}
	if pending, err := store.loadInputPublication(); err != nil || pending == nil {
		t.Fatalf("failed recovery lost the journal: %v", err)
	}
}

func TestManualPublicationJournalVersionCompatibility(t *testing.T) {
	for _, test := range []struct {
		name, raw string
		valid     bool
	}{
		{name: "legacy-idle", raw: `{"version":1,"phase":"idle"}`, valid: true},
		{name: "manual-idle", raw: `{"version":2,"phase":"idle"}`, valid: true},
		{name: "current-idle", raw: `{"version":3,"phase":"idle"}`, valid: true},
		{name: "legacy-removals", raw: `{"version":2,"phase":"committed","generation_id":"accepted","payload_checksum":"digest","removals":"policy.json"}`},
		{name: "idle-removals", raw: `{"version":3,"phase":"idle","removals":"policy.json"}`},
		{name: "legacy-manual", raw: `{"version":1,"phase":"committed","generation_id":"accepted","payload_checksum":"digest","manual":"history.json"}`},
		{name: "idle-manual", raw: `{"version":2,"phase":"idle","manual":"history.json"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, err := newLayerStore(privateRuntimeDirectory(t))
			if err != nil {
				t.Fatal(err)
			}
			if err := store.directory.WriteFile(inputPublicationName, []byte(test.raw), ".test-"); err != nil {
				t.Fatal(err)
			}
			_, err = store.loadInputPublication()
			if (err == nil) != test.valid {
				t.Fatalf("journal compatibility = %v, want valid %v", err, test.valid)
			}
		})
	}
}

func TestManualHistoryRefusesNewBatchAtCapacityWithoutChangingHead(t *testing.T) {
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	prepared, err := prepareManualObservations(t.Context(), []sources.Observation{manualTestObservation(t, "existing", at, false)})
	if err != nil {
		t.Fatal(err)
	}
	var history *manualBatch
	for range maxManualHistoryBatches {
		history = &manualBatch{parent: history, observations: prepared}
	}
	if selected, err := selectManualObservations(t.Context(), history, prepared, nil); err != nil || len(selected) != 0 {
		t.Fatalf("duplicate at capacity = %v", err)
	}
	incoming, err := prepareManualObservations(t.Context(), []sources.Observation{manualTestObservation(t, "new", at.Add(time.Minute), false)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := selectManualObservations(t.Context(), history, incoming, nil); !errors.IsConflict(err) {
		t.Fatalf("new batch at capacity = %v", err)
	}
	if len(manualBatches(history)) != maxManualHistoryBatches || history.observations[0].Receipt.Link.ObservationID != prepared[0].Receipt.Link.ObservationID {
		t.Fatal("capacity refusal changed retained history")
	}
}
