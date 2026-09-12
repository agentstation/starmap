package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestManualHistoryCompactsRepeatedProviderInventoriesBeforeBatchLimit(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	missing := manualProviderObservation(t, 200, at)
	provider, err := missing.Catalog.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	omitted := catalogs.DeepCopyModel(*provider.Models["model"])
	omitted.ID = "omitted"
	provider.Models["omitted"] = &omitted
	builder, err := catalogs.NewBuilderFrom(missing.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	observed, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	missing, err = sources.NewObservation(sources.ProvidersID, observed, sources.ObservationMetadata{
		ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareManualObservations(t.Context(), []sources.Observation{missing})
	if err != nil {
		t.Fatal(err)
	}
	history := &manualBatch{observations: prepared}
	var latest sources.Observation
	for i := 1; i < maxManualHistoryBatches; i++ {
		latest = manualProviderObservation(t, 300, at.Add(time.Duration(i)*time.Minute))
		input, err := prepareManualObservations(t.Context(), []sources.Observation{latest})
		if err != nil {
			t.Fatal(err)
		}
		history = &manualBatch{parent: history, observations: input}
	}
	incoming := manualProviderObservation(t, 400, at.Add(time.Duration(maxManualHistoryBatches)*time.Minute))
	input, err := prepareManualObservations(t.Context(), []sources.Observation{incoming})
	if err != nil {
		t.Fatal(err)
	}
	baseline := starmap.CatalogState{GenerationID: "baseline", Catalog: manualProviderObservation(t, 100, at).Catalog, GeneratedAt: at.Add(-time.Minute)}
	layers := layerSet{manual: history, embedded: baseline}
	selected, err := layers.prepareManualInputs(t.Context(), input, nil, nil)
	if err != nil || len(selected) != 1 {
		t.Fatalf("repeated inventories blocked the next acquisition: %v", err)
	}
	if count := len(manualBatches(layers.manual)); count >= maxManualHistoryBatches {
		t.Fatalf("compaction retained %d batches", count)
	}
	before, err := layers.build(t.Context(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	provider, err = before.Catalog.Provider("provider")
	if err != nil || provider.Models["omitted"] == nil || provider.Models["model"].Limits.ContextWindow != 400 {
		t.Fatalf("compaction lost current or omitted provider records: %v", err)
	}
	if len(manualBatches(history)) != maxManualHistoryBatches {
		t.Fatal("compaction changed the accepted history in place")
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
		t.Fatalf("restart changed the compacted catalog: %v", err)
	}
	found := false
	for _, batch := range manualBatches(restored) {
		for _, retained := range batch.observations {
			if retained.Receipt.Link.ObservationID != missing.ID {
				continue
			}
			original, err := retained.restore()
			if err != nil || original.EvidenceChecksum != missing.EvidenceChecksum {
				t.Fatalf("compaction rewrote the omitted model receipt: %v", err)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("compaction discarded the omitted model's original observation")
	}
}

func TestRepeatedProviderCompactionPreservesReplayBoundaries(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	for _, scenario := range []string{"repeated", "changed", "partial", "binding", "equal-time", "reset", "metadata", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			prior := manualProviderObservation(t, 200, at)
			metadata := sources.ObservationMetadata{ObservedAt: at.Add(time.Minute), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded}
			source, catalog := sources.ProvidersID, prior.Catalog
			switch scenario {
			case "changed":
				catalog = manualProviderObservation(t, 300, at).Catalog
			case "partial":
				metadata.Completeness, metadata.Status = sources.ObservationCompletenessPartial, sources.ObservationStatusDegraded
				metadata.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "provider/invalid", Message: "Invalid record."}}
			case "binding":
				metadata.ProviderBinding = scopedProviderLayer(t, "other-account", "1", at).Receipt.ProviderBinding
			case "equal-time":
				metadata.ObservedAt = at
				metadata.Revision = sources.Revision{Kind: sources.RevisionKindETag, Value: "new-etag"}
			case "metadata":
				source = sources.ModelsDevHTTPID
			}
			current, err := sources.NewObservation(source, catalog, metadata)
			if err != nil {
				t.Fatal(err)
			}
			input, err := prepareManualObservations(t.Context(), []sources.Observation{prior, current})
			if err != nil {
				t.Fatal(err)
			}
			history := &manualBatch{parent: &manualBatch{observations: input[:1]}, observations: input[1:]}
			if scenario == "reset" {
				history.resets = []ObservationReset{{ProviderID: "provider"}}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if scenario == "canceled" {
				cancel()
			}
			compacted, err := compactRepeatedProviderHistory(ctx, history)
			if scenario == "canceled" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("canceled compaction = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			retained := make(map[string]bool)
			for _, batch := range manualBatches(compacted) {
				for _, original := range batch.observations {
					if _, err := original.restore(); err != nil {
						t.Fatalf("compaction changed original evidence: %v", err)
					}
					retained[original.Receipt.Link.ObservationID] = true
				}
			}
			if !retained[current.ID] || !retained[prior.ID] {
				t.Fatal("compaction selected the wrong original receipts")
			}
			if (scenario == "reset" || scenario == "metadata") && compacted != history {
				t.Fatal("compaction changed a required replay boundary")
			}
			if history.parent == nil || history.parent.observations[0].Receipt.Link.ObservationID != prior.ID {
				t.Fatal("compaction mutated accepted history")
			}
		})
	}
}

func TestManualHistoryCompactsRepeatedProviderInventoriesBeforeByteLimit(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	original := manualProviderObservation(t, 200, at)
	builder, err := catalogs.NewBuilderFrom(original.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := builder.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	provider.Models["model"].Description = strings.Repeat("x", 1<<20)
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	catalog, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	var history *manualBatch
	bytesRetained := 2
	var incoming []manualObservation
	for i := 0; ; i++ {
		observation := providerResetObservation(t, sources.ProvidersID, catalog, at.Add(time.Duration(i)*time.Minute), nil)
		incoming, err = prepareManualObservations(t.Context(), []sources.Observation{observation})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(incoming[0])
		if err != nil {
			t.Fatal(err)
		}
		if bytesRetained+len(raw)+2 > maxLayerBytes {
			break
		}
		bytesRetained += len(raw) + 2
		history = &manualBatch{parent: history, observations: incoming}
	}
	if _, err := selectManualObservations(t.Context(), history, incoming, nil); err != manualHistoryCapacity {
		t.Fatalf("fixture did not reach the byte limit: %v", err)
	}
	layers := layerSet{manual: history}
	selected, err := layers.prepareManualInputs(t.Context(), incoming, nil, nil)
	if err != nil || len(selected) != 1 {
		t.Fatalf("repeated payload bytes blocked acquisition: %v", err)
	}
	if len(manualBatches(layers.manual)) != 2 || len(layers.manual.parent.observations) != 2 {
		t.Fatal("compaction retained redundant payload copies")
	}
	if len(manualBatches(history)) <= 1 {
		t.Fatal("compaction mutated accepted history")
	}
}

func TestRepeatedProviderCompactionPreservesEffectiveCatalog(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	for _, scoped := range []bool{false, true} {
		name := "legacy"
		if scoped {
			name = "bound"
		}
		t.Run(name, func(t *testing.T) {
			baseline := starmap.CatalogState{GenerationID: "baseline", Catalog: manualProviderObservation(t, 100, at).Catalog, GeneratedAt: at.Add(-time.Minute)}
			var history *manualBatch
			var binding *sources.ProviderAcquisitionBinding
			if scoped {
				binding = scopedProviderLayer(t, "account", "1", at).Receipt.ProviderBinding
			}
			for i := range 4 {
				observation := providerResetObservation(t, sources.ProvidersID, manualProviderObservation(t, 200, at).Catalog, at.Add(time.Duration(i)*time.Minute), binding)
				prepared, err := prepareManualObservations(t.Context(), []sources.Observation{observation})
				if err != nil {
					t.Fatal(err)
				}
				history = &manualBatch{parent: history, observations: prepared}
			}
			original := layerSet{publisherID: "publisher", manual: history, embedded: baseline}
			before, err := original.build(t.Context(), baseline)
			if err != nil {
				t.Fatal(err)
			}
			compacted, err := compactRepeatedProviderHistory(t.Context(), history)
			if err != nil {
				t.Fatal(err)
			}
			if len(compacted.observations) != 2 {
				t.Fatal("compaction must preserve first and latest inventory evidence")
			}
			replacement := layerSet{publisherID: "publisher", manual: compacted, embedded: baseline}
			after, err := replacement.build(t.Context(), baseline)
			if err != nil || before.GenerationID != after.GenerationID || before.PayloadChecksum != after.PayloadChecksum {
				beforePayload, _ := catalogs.EncodeCatalogPayload(before.Catalog)
				afterPayload, _ := catalogs.EncodeCatalogPayload(after.Catalog)
				t.Logf("before_payload=%s", beforePayload)
				t.Logf("after_payload=%s", afterPayload)
				t.Logf("before_evidence=%+v after_evidence=%+v", original.buildEvidence, replacement.buildEvidence)
				t.Fatalf("compaction changed effective facts or receipts: %v", err)
			}
		})
	}
}
