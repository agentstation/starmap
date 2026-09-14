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

func TestManualCheckpointRetainsResetHistoryAtBatchLimit(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	original := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 200, at).Catalog, at, nil)
	observations, err := prepareManualObservations(t.Context(), []sources.Observation{original})
	if err != nil {
		t.Fatal(err)
	}
	resets := []ObservationReset{{SourceID: sources.ModelsDevHTTPID}}
	history := &manualBatch{observations: observations}
	for range maxManualHistoryBatches - 1 {
		history = &manualBatch{parent: history, observations: observations, resets: resets}
	}
	incoming := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 300, at).Catalog, at.Add(time.Minute), nil)
	input, err := prepareManualObservations(t.Context(), []sources.Observation{incoming})
	if err != nil {
		t.Fatal(err)
	}
	baseline := starmap.CatalogState{GenerationID: "baseline", Catalog: manualProviderObservation(t, 100, at).Catalog, GeneratedAt: at.Add(-time.Minute)}
	layers := layerSet{manual: history, embedded: baseline}
	selected, err := layers.prepareManualInputs(t.Context(), input, nil, resets)
	if err != nil || len(selected) != 1 {
		t.Fatalf("reset history blocked the next acquisition: %v", err)
	}
	expected := layerSet{manual: &manualBatch{parent: history, observations: input, resets: resets}, embedded: baseline}
	before, err := expected.build(t.Context(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	after, err := layers.build(t.Context(), baseline)
	if err != nil || before.GenerationID != after.GenerationID || before.PayloadChecksum != after.PayloadChecksum {
		t.Fatalf("checkpoint changed catalog facts or reset identity: %v", err)
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
	actual, err := restarted.build(t.Context(), baseline)
	if err != nil || actual.GenerationID != before.GenerationID || actual.PayloadChecksum != before.PayloadChecksum {
		t.Fatalf("checkpoint restart changed accepted reset history: %v", err)
	}
	batches := manualBatches(restored)
	if len(batches) != maxManualHistoryBatches+1 {
		t.Fatalf("checkpoint lost ordered batches: %d", len(batches))
	}
	for index, batch := range batches {
		want := original.ID
		if index == len(batches)-1 {
			want = incoming.ID
		}
		if len(batch.observations) != 1 || batch.observations[0].Receipt.Link.ObservationID != want || (index > 0 && len(batch.resets) != 1) {
			t.Fatalf("checkpoint changed original evidence at batch %d", index)
		}
	}
	if len(manualBatches(history)) != maxManualHistoryBatches {
		t.Fatal("checkpoint changed the accepted history in place")
	}
	// Reaccept an original receipt after a checkpoint and a later reset.
	if _, err := restarted.prepareManualInputs(t.Context(), observations, nil, resets); err != nil {
		t.Fatal(err)
	}
	expected.manual = &manualBatch{parent: expected.manual, observations: observations, resets: resets}
	want, err := expected.build(t.Context(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	nextReference, err := store.stageManualBatch(t.Context(), restarted.manual)
	if err != nil {
		t.Fatal(err)
	}
	next, err := store.readManualHistory(t.Context(), nextReference)
	if err != nil {
		t.Fatal(err)
	}
	afterAppend := layerSet{manual: next, embedded: baseline}
	got, err := afterAppend.build(t.Context(), baseline)
	if err != nil || got.GenerationID != want.GenerationID || got.PayloadChecksum != want.PayloadChecksum {
		t.Fatalf("checkpoint append changed reaccepted evidence: %v", err)
	}
	if next.parent == nil || next.parent.checkpoint == nil {
		t.Fatal("an ordinary append lost the retained checkpoint parent")
	}
}

func TestManualCheckpointPreservesMixedReplay(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	baseline := starmap.CatalogState{GenerationID: "baseline", Catalog: manualProviderObservation(t, 100, at).Catalog, GeneratedAt: at.Add(-time.Minute)}
	metadata := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 200, at).Catalog, at, nil)
	provider := manualProviderObservation(t, 300, at.Add(time.Minute))
	peer := providerResetObservation(t, sources.ModelsDevGitID, manualProviderObservation(t, 400, at).Catalog, at.Add(2*time.Minute), nil)
	empty, err := catalogs.NewObservationCatalog(catalogs.NewEmpty())
	if err != nil {
		t.Fatal(err)
	}
	replacement := providerResetObservation(t, sources.ModelsDevHTTPID, empty, at.Add(3*time.Minute), nil)
	var history *manualBatch
	for _, observation := range []sources.Observation{metadata, provider, peer, replacement} {
		prepared, err := prepareManualObservations(t.Context(), []sources.Observation{observation})
		if err != nil {
			t.Fatal(err)
		}
		history = &manualBatch{parent: history, observations: prepared}
	}
	history.resets = []ObservationReset{{SourceID: sources.ModelsDevHTTPID}}
	compacted, err := checkpointManualHistory(t.Context(), history)
	if err != nil {
		t.Fatal(err)
	}
	store, err := newLayerStore(privateRuntimeDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	reference, err := store.stageManualBatch(t.Context(), compacted)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := store.readManualHistory(t.Context(), reference)
	if err != nil {
		t.Fatal(err)
	}
	for _, selectedBaseline := range []starmap.CatalogState{baseline, {GenerationID: "new-baseline", Catalog: manualProviderObservation(t, 900, at).Catalog, GeneratedAt: at.Add(time.Hour)}} {
		original := layerSet{manual: history, embedded: selectedBaseline}
		before, err := original.build(t.Context(), selectedBaseline)
		if err != nil {
			t.Fatal(err)
		}
		restarted := layerSet{manual: restored, embedded: selectedBaseline}
		after, err := restarted.build(t.Context(), selectedBaseline)
		if err != nil || before.GenerationID != after.GenerationID || before.PayloadChecksum != after.PayloadChecksum {
			t.Fatalf("checkpoint changed source precedence or reset evidence: %v", err)
		}
		selection, err := selectObservationResetHistory(t.Context(), restored, selectedBaseline.Catalog)
		if err != nil || !selection.excluded[metadata.ID][""] || selection.excluded[peer.ID][""] {
			t.Fatalf("checkpoint changed reset exclusions: %v", err)
		}
	}
}

func TestManualCheckpointRejectsInvalidRecords(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	observation := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 200, at).Catalog, at, nil)
	prepared, err := prepareManualObservations(t.Context(), []sources.Observation{observation})
	if err != nil {
		t.Fatal(err)
	}
	valid, err := checkpointManualHistory(t.Context(), &manualBatch{observations: prepared})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"payload-index", "observation-index", "duplicate-observation", "duplicate-payload", "unused-observation", "unused-payload", "forged-receipt", "unmatched-reset", "reference-limit", "mixed-record", "old-version", "canceled"} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(valid.checkpoint)
			if err != nil {
				t.Fatal(err)
			}
			var checkpoint manualCheckpoint
			if err := json.Unmarshal(raw, &checkpoint); err != nil {
				t.Fatal(err)
			}
			record := manualBatchRecord{Version: manualHistoryVersion, Checkpoint: &checkpoint}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch name {
			case "payload-index":
				checkpoint.Observations[0].Payload = 1
			case "observation-index":
				checkpoint.Batches[0].Observations[0] = 1
			case "duplicate-observation":
				checkpoint.Batches[0].Observations = []uint32{0, 0}
			case "duplicate-payload":
				checkpoint.Payloads = append(checkpoint.Payloads, checkpoint.Payloads[0])
			case "unused-observation":
				other := providerResetObservation(t, sources.ModelsDevHTTPID, observation.Catalog, at.Add(time.Minute), nil)
				receipt, err := other.Receipt()
				if err != nil {
					t.Fatal(err)
				}
				checkpoint.Observations = append(checkpoint.Observations, manualCheckpointObservation{Receipt: receipt})
			case "unused-payload":
				checkpoint.Payloads = append(checkpoint.Payloads, []byte("{}"))
			case "forged-receipt":
				checkpoint.Observations[0].Receipt.Link.ObservationID = "forged"
			case "unmatched-reset":
				checkpoint.Batches[0].Resets = []ObservationReset{{SourceID: sources.ModelsDevGitID}}
			case "reference-limit":
				checkpoint.Batches[0].Observations = make([]uint32, maxManualCheckpointReferences+1)
			case "mixed-record":
				record.Parent = "parent.json"
			case "old-version":
				record.Version = manualHistoryResetVersion
			case "canceled":
				cancel()
			}
			store, err := newLayerStore(privateRuntimeDirectory(t))
			if err != nil {
				t.Fatal(err)
			}
			reference, err := store.stageInput(t.Context(), record)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.readManualHistory(ctx, reference); err == nil || (name == "canceled" && !errors.Is(err, context.Canceled)) {
				t.Fatalf("invalid checkpoint accepted: %v", err)
			}
			if head, err := store.loadManualHistory(t.Context()); err != nil || head != nil {
				t.Fatalf("invalid checkpoint changed accepted history: %v", err)
			}
		})
	}
}

func TestManualCheckpointSharesPayloadsAtByteLimit(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	builder, err := catalogs.NewBuilderFrom(manualProviderObservation(t, 200, at).Catalog)
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
	for index := 0; ; index++ {
		observation := providerResetObservation(t, sources.ModelsDevHTTPID, catalog, at.Add(time.Duration(index)*time.Minute), nil)
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
		t.Fatalf("fixture did not reach the original byte limit: %v", err)
	}
	layers := layerSet{manual: history}
	selected, err := layers.prepareManualInputs(t.Context(), incoming, nil, nil)
	if err != nil || len(selected) != 1 || layers.manual.checkpoint == nil {
		t.Fatalf("metadata payload repetition blocked acquisition: %v", err)
	}
	if len(layers.manual.checkpoint.Payloads) != 1 || len(layers.manual.checkpoint.Observations) != len(manualBatches(history))+1 {
		t.Fatal("checkpoint failed to share bytes or discarded original receipts")
	}
	if layers.manual.checkpointBytes >= maxLayerBytes/2 {
		t.Fatalf("checkpoint did not reduce retained bytes: %d", layers.manual.checkpointBytes)
	}
}

func TestManualCheckpointRecoveryPreservesPublicationBoundary(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	observation := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 200, at).Catalog, at, nil)
	prepared, err := prepareManualObservations(t.Context(), []sources.Observation{observation})
	if err != nil {
		t.Fatal(err)
	}
	for _, corrupt := range []bool{false, true} {
		name := "accepted"
		if corrupt {
			name = "invalid-before-inputs"
		}
		t.Run(name, func(t *testing.T) {
			history, err := checkpointManualHistory(t.Context(), &manualBatch{observations: prepared})
			if err != nil {
				t.Fatal(err)
			}
			if corrupt {
				history.checkpoint.Observations[0].Payload = 1
			}
			store, err := newLayerStore(privateRuntimeDirectory(t))
			if err != nil {
				t.Fatal(err)
			}
			reference, err := store.stageManualBatch(t.Context(), history)
			if err != nil {
				t.Fatal(err)
			}
			provider, err := store.stageInput(t.Context(), scopedProviderLayer(t, "binding", "1", at))
			if err != nil {
				t.Fatal(err)
			}
			record := inputPublication{Version: inputPublicationVersion, Phase: inputPublicationCommitted, GenerationID: "accepted", PayloadChecksum: "accepted-checksum", Manual: reference, Providers: []string{provider}}
			if err := store.writeInputPublication(t.Context(), record); err != nil {
				t.Fatal(err)
			}
			err = store.recoverInputPublication(t.Context(), starmap.CatalogState{})
			if corrupt {
				if err == nil {
					t.Fatal("recovery accepted a corrupted checkpoint")
				}
				if providers, err := store.loadProviders(); err != nil || len(providers) != 0 {
					t.Fatalf("recovery applied inputs before validating the checkpoint: %v", err)
				}
				if head, err := store.loadManualHistory(t.Context()); err != nil || head != nil {
					t.Fatalf("recovery installed a corrupted checkpoint: %v", err)
				}
				if pending, err := store.loadInputPublication(); err != nil || pending == nil {
					t.Fatalf("failed recovery lost its publication journal: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			restored, err := store.loadManualHistory(t.Context())
			if err != nil || restored == nil || len(manualBatches(restored)) != 1 || manualBatches(restored)[0].observations[0].Receipt.Link.ObservationID != observation.ID {
				t.Fatalf("accepted checkpoint did not recover: %v", err)
			}
			if pending, err := store.loadInputPublication(); err != nil || pending != nil {
				t.Fatalf("successful recovery retained a pending journal: %v", err)
			}
		})
	}
}
