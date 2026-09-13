package runtime

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestManualHistoryRetiresSupersededDistinctInventoriesBeforeByteLimit(t *testing.T) {
	t.Parallel()
	t.Run("ordinary", func(t *testing.T) { testManualHistoryRetirementAtCapacity(t, false) })
	t.Run("reset", func(t *testing.T) { testManualHistoryRetirementAtCapacity(t, true) })
}

func testManualHistoryRetirementAtCapacity(t *testing.T, reset bool) {
	t.Helper()
	const retainedCount = 47
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	padding := strings.Repeat("x", 1<<20)
	baseline := starmap.CatalogState{GenerationID: "distinct-baseline", Catalog: manualProviderObservation(t, 100, at).Catalog, GeneratedAt: at.Add(-time.Minute)}
	var history *manualBatch
	var first sources.Observation
	for index := range retainedCount {
		observation := distinctRetentionObservation(t, index, padding, at.Add(time.Duration(index)*time.Minute))
		if index == 0 {
			first = observation
		}
		prepared, err := prepareManualObservations(t.Context(), []sources.Observation{observation})
		if err != nil {
			t.Fatal(err)
		}
		history = &manualBatch{parent: history, observations: prepared}
	}
	if _, err := selectManualObservations(t.Context(), history.parent, history.observations, nil); err != nil {
		t.Fatalf("starting history already exceeds capacity: %v", err)
	}
	latest := distinctRetentionObservation(t, retainedCount, padding, at.Add(retainedCount*time.Minute))
	input, err := prepareManualObservations(t.Context(), []sources.Observation{latest})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := selectManualObservations(t.Context(), history, input, nil); err != manualHistoryCapacity {
		t.Fatalf("incoming observation did not reach capacity: %v", err)
	}
	var resets []ObservationReset
	if reset {
		resets = []ObservationReset{{ProviderID: "provider"}}
	}
	layers := layerSet{manual: history, embedded: baseline}
	selected, err := layers.prepareManualInputs(t.Context(), input, nil, resets)
	if err != nil || len(selected) != 1 {
		t.Fatalf("superseded distinct inventories blocked acquisition: selected=%d error=%v", len(selected), err)
	}
	if len(manualBatches(history)) != retainedCount {
		t.Fatal("retirement changed accepted history in place")
	}
	before, err := layers.build(t.Context(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := before.Catalog.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	if (provider.Models["omitted"] != nil) == reset || provider.Models["model"].Name != fmt.Sprintf("Model %d", retainedCount) {
		t.Fatalf("retirement lost a current or omitted offering: %v", err)
	}
	store, err := newLayerStore(privateRuntimeDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	reference, err := store.stageManualBatch(t.Context(), layers.manual)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.saveManualHead(t.Context(), reference); err != nil {
		t.Fatal(err)
	}
	restored, err := store.loadManualHistory(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	restarted := layerSet{manual: restored, embedded: baseline}
	after, err := restarted.build(t.Context(), baseline)
	if err != nil || before.GenerationID != after.GenerationID || before.PayloadChecksum != after.PayloadChecksum {
		t.Fatalf("restart changed the compacted generation: %v", err)
	}
	kept := make(map[string]bool)
	for _, batch := range manualBatches(restored) {
		for _, retained := range batch.observations {
			if _, err := retained.restore(); err != nil {
				t.Fatal(err)
			}
			kept[retained.Receipt.Link.ObservationID] = true
		}
	}
	if len(kept) >= retainedCount || !kept[first.ID] || !kept[latest.ID] {
		t.Fatalf("retirement retained %d observations or lost original receipts", len(kept))
	}
}

func distinctRetentionObservation(t *testing.T, index int, padding string, at time.Time) sources.Observation {
	t.Helper()
	base := manualProviderObservation(t, int64(200+index), at)
	builder, err := catalogs.NewBuilderFrom(base.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := builder.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	provider.Models["model"].Name = fmt.Sprintf("Model %d", index)
	provider.Models["model"].Description = padding
	if index == 0 {
		omitted := catalogs.DeepCopyModel(*provider.Models["model"])
		omitted.ID, omitted.Name = "omitted", "Omitted offering"
		omitted.Description = ""
		provider.Models[omitted.ID] = &omitted
	}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	catalog, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	return observation
}
