package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestSourceResetLostReplyRecoversUnchangedPayload(t *testing.T) {
	store := &retentionAmbiguousStore{Memory: storage.NewMemory()}
	connected, options, at := providerResetRuntime(t, store)
	original := providerResetObservation(t, sources.ModelsDevGitID, manualProviderObservation(t, 200, at).Catalog, at, nil)
	if _, err := connected.PublishObservations(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	before := connected.State()
	store.ambiguous.Store(true)
	_, err := connected.UpdateObservations(t.Context(), func(context.Context, ObservationInputs) ([]sources.Observation, error) {
		return []sources.Observation{original}, nil
	}, ObservationReset{SourceID: sources.ModelsDevGitID})
	if err == nil {
		t.Fatal("lost reply did not fail")
	}
	accepted, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Manifest.GenerationID == before.GenerationID || accepted.Manifest.Payload.Checksum != before.PayloadChecksum {
		t.Fatal("reset did not bind unchanged replacement payload")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, options...)
	if reopened.State().GenerationID != accepted.Manifest.GenerationID || reopened.layers.manual.resets[0].SourceID != sources.ModelsDevGitID {
		t.Fatal("restart lost accepted metadata reset")
	}
}

func TestSourceResetHistoryVersions(t *testing.T) {
	at := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name    string
		version int
		resets  []ObservationReset
		valid   bool
	}{
		{name: "v1", version: 1, valid: true},
		{name: "v2", version: 2, valid: true},
		{name: "v3", version: 3, resets: []ObservationReset{{SourceID: sources.ModelsDevHTTPID}}, valid: true},
		{name: "v2-metadata-reset", version: 2, resets: []ObservationReset{{SourceID: sources.ModelsDevHTTPID}}},
		{name: "wrong-source", version: 3, resets: []ObservationReset{{SourceID: sources.ModelsDevGitID}}},
		{name: "v4", version: 4, resets: []ObservationReset{{SourceID: sources.ModelsDevHTTPID}}, valid: true},
		{name: "future", version: 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, err := newLayerStore(privateRuntimeDirectory(t))
			if err != nil {
				t.Fatal(err)
			}
			original := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 200, at).Catalog, at, nil)
			observations, err := prepareManualObservations(t.Context(), []sources.Observation{original})
			if err != nil {
				t.Fatal(err)
			}
			input, err := store.stageInput(t.Context(), observations[0])
			if err != nil {
				t.Fatal(err)
			}
			ref, err := store.stageInput(t.Context(), manualBatchRecord{Version: test.version, Observations: []string{input}, Resets: test.resets})
			if err != nil {
				t.Fatal(err)
			}
			if err := store.writeContext(t.Context(), store.directory, manualHistoryName, manualHistoryHead{Version: test.version, Batch: ref}); err != nil {
				t.Fatal(err)
			}
			_, err = store.loadManualHistory(t.Context())
			if (err == nil) != test.valid {
				t.Fatalf("history version %d error %v, want valid %v", test.version, err, test.valid)
			}
		})
	}
}
