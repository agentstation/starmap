package runtime

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestFleetRecoveryPreservesNextPartialMerge(t *testing.T) {
	at := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	layers := fleetTestLayers(t)
	embedded := layers.embedded
	first := manualTestObservation(t, "first", at.Add(time.Minute), false)
	prepared, err := prepareManualObservations(t.Context(), []sources.Observation{first})
	if err != nil {
		t.Fatal(err)
	}
	layers.manual = &manualBatch{observations: prepared}
	before, err := layers.build(t.Context(), embedded)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := decodeFleetRecovery(t.Context(), raw)
	if err != nil {
		t.Fatal(err)
	}
	current, err := restored.build(t.Context(), restored.embedded)
	if err != nil || current.GenerationID != before.GenerationID || current.PayloadChecksum != before.PayloadChecksum {
		t.Fatalf("recovery changed the accepted state: %v", err)
	}
	second := manualTestObservation(t, "second", at.Add(2*time.Minute), true)
	incoming, err := prepareManualObservations(t.Context(), []sources.Observation{second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restored.prepareManualInputs(t.Context(), incoming, nil, nil); err != nil {
		t.Fatal(err)
	}
	next, err := restored.build(t.Context(), restored.embedded)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := next.Catalog.Provider("manual-provider")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"first", "second"} {
		if provider.Models[id] == nil {
			t.Errorf("next partial merge lost model %q", id)
		}
	}
	if len(manualBatches(layers.manual)) != 1 {
		t.Fatal("recovery mutated the original history")
	}
}

func TestFleetRecoveryBindsExactGenerationAndBytes(t *testing.T) {
	data := []byte(`{"version":1,"publisher_id":"deployment"}`)
	generation := catalogs.Generation{Manifest: catalogs.GenerationManifest{GenerationID: "generation"}}
	generation.Manifest.Payload.Checksum = "payload-checksum"
	valid := FleetRecovery{GenerationID: "generation", PayloadChecksum: "payload-checksum", Checksum: fleetRecoveryChecksum(data), Data: data}
	if err := valid.Validate(generation); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []struct {
		name  string
		apply func(*FleetRecovery)
	}{
		{"generation", func(r *FleetRecovery) { r.GenerationID = "other" }},
		{"payload", func(r *FleetRecovery) { r.PayloadChecksum = "other" }},
		{"checksum", func(r *FleetRecovery) { r.Checksum = "other" }},
		{"bytes", func(r *FleetRecovery) { r.Data = append(bytes.Clone(r.Data), ' ') }},
		{"empty", func(r *FleetRecovery) { r.Data = nil }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			candidate := valid
			mutate.apply(&candidate)
			if err := candidate.Validate(generation); err == nil {
				t.Fatal("recovery accepted a different generation or bytes")
			}
		})
	}
}

func TestFleetRecoveryRejectsAmbiguousAndInvalidInputs(t *testing.T) {
	layers := fleetTestLayers(t)
	raw, err := encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = decompressFleetRecovery(t.Context(), raw, MaxFleetRecoveryBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name   string
		change func(string) string
	}{
		{"version", func(s string) string { return strings.Replace(s, `"version":2`, `"version":1`, 1) }},
		{"publisher", func(s string) string {
			return strings.Replace(s, `"publisher_id":"deployment"`, `"publisher_id":""`, 1)
		}},
		{"unknown", func(s string) string { return strings.Replace(s, `"version":2`, `"version":2,"unknown":true`, 1) }},
		{"duplicate", func(s string) string { return strings.Replace(s, `"version":2`, `"version":2,"version":2`, 1) }},
		{"trailing", func(s string) string { return s + "{}" }},
		{"source", func(s string) string { return strings.TrimSuffix(s, "}") + `,"source":{"generation_id":"x"}}` }},
		{"manual", func(s string) string { return strings.TrimSuffix(s, "}") + `,"manual":{}}` }},
		{"provider", func(s string) string { return strings.TrimSuffix(s, "}") + `,"providers":[{"provider_id":"x"}]}` }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if _, err := decodeFleetRecovery(t.Context(), fleetCompressedTest(t, []byte(scenario.change(string(raw))))); err == nil {
				t.Fatal("accepted invalid recovery")
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := decodeFleetRecovery(ctx, raw); err == nil {
		t.Fatal("recovery ignored cancellation")
	}
}

func TestFleetRecoveryRetainsProviderScopesAndRejectsDuplicates(t *testing.T) {
	at := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	layer := scopedProviderLayer(t, "account-a", "1", at)
	peer := scopedProviderLayer(t, "account-b", "1", at)
	compatibility, err := fleetLayerCompatibility(layerSet{})
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := fleetBaseline(fleetTestLayers(t))
	if err != nil {
		t.Fatal(err)
	}
	record := fleetRecoveryRecord{Version: fleetRecoveryVersion, Baseline: baseline, PublisherID: "deployment", Compatibility: compatibility, Providers: []ProviderLayer{layer, peer}}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := decodeFleetRecovery(t.Context(), fleetCompressedTest(t, raw))
	if err != nil || len(restored.providers) != 2 {
		t.Fatalf("recovery lost provider evidence: %v", err)
	}
	for _, expected := range record.Providers {
		if !reflect.DeepEqual(expected, restored.providers[expected.evidenceKey()]) {
			t.Fatal("recovery changed an original provider scope or receipt")
		}
	}
	record.Providers = append(record.Providers, layer)
	raw, err = json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeFleetRecovery(t.Context(), fleetCompressedTest(t, raw)); err == nil {
		t.Fatal("recovery accepted a duplicate provider scope")
	}
}

func TestFleetRecoveryPreservesSourceResetsAndRemovals(t *testing.T) {
	at := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	upstream := aliasGeneration(t, "upstream")
	source := &sourceLayer{Identity: "upstream", GenerationID: upstream.Manifest.GenerationID,
		Checksum: upstream.Manifest.Payload.Checksum, Payload: upstream.Payload, Manifest: &upstream.Manifest,
		PublishedAt: upstream.Manifest.GeneratedAt, Chain: []SourceHop{{Identity: "upstream"}}}
	original := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 200, at).Catalog, at, nil)
	replacement := providerResetObservation(t, sources.ModelsDevHTTPID, manualProviderObservation(t, 300, at).Catalog, at.Add(time.Minute), nil)
	var history *manualBatch
	for _, observation := range []sources.Observation{original, replacement} {
		prepared, err := prepareManualObservations(t.Context(), []sources.Observation{observation})
		if err != nil {
			t.Fatal(err)
		}
		history = &manualBatch{parent: history, observations: prepared}
	}
	history.resets = []ObservationReset{{SourceID: sources.ModelsDevHTTPID}}
	target, err := catalogs.NewCanonicalRemovalTarget("author/current")
	if err != nil {
		t.Fatal(err)
	}
	layers := fleetTestLayers(t)
	layers.source, layers.manual = source, history
	layers.removals = &catalogs.CatalogRemovalPolicy{PublisherID: "deployment", Targets: []catalogs.CatalogRemovalTarget{target}}
	raw, err := encodeFleetRecoveryWithPin(t.Context(), layers, nil)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := decodeFleetRecovery(t.Context(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(recovered.source, source) || !reflect.DeepEqual(recovered.removals, layers.removals) {
		t.Fatal("recovery changed the source or explicit removal policy")
	}
	batches := manualBatches(recovered.manual)
	if len(batches) != 2 || batches[0].observations[0].Receipt.Link.ObservationID != original.ID ||
		batches[1].observations[0].Receipt.Link.ObservationID != replacement.ID || !reflect.DeepEqual(batches[1].resets, history.resets) {
		t.Fatal("recovery changed ordered receipts or reset history")
	}
	if recovered.manual.reference != "" || recovered.manual.parent != nil {
		t.Fatal("recovery depends on a former leader filesystem reference")
	}
}

func decodeFleetRecovery(ctx context.Context, data []byte) (layerSet, error) {
	record, err := readFleetRecovery(ctx, data)
	if err != nil {
		return layerSet{}, err
	}
	return decodeFleetRecoveryRecord(ctx, record, layerSet{})
}

func fleetTestLayers(t *testing.T) layerSet {
	t.Helper()
	generation := aliasGeneration(t, "baseline")
	baseline, err := catalogs.DecodeCatalogGeneration(generation)
	if err != nil {
		t.Fatal(err)
	}
	return layerSet{publisherID: "deployment", embeddedManifest: &generation.Manifest,
		embedded: starmap.CatalogState{Catalog: baseline, GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum, GeneratedAt: generation.Manifest.GeneratedAt}}
}

func TestFleetRecoveryRejectsMissingOrCorruptBaseline(t *testing.T) {
	raw, err := encodeFleetRecoveryWithPin(t.Context(), fleetTestLayers(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	original, err := readFleetRecovery(t.Context(), raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name   string
		change func(*catalogs.Generation)
	}{
		{"missing", func(g *catalogs.Generation) { *g = catalogs.Generation{} }},
		{"manifest", func(g *catalogs.Generation) { g.Manifest.GenerationID = "" }},
		{"payload", func(g *catalogs.Generation) { g.Payload = append(g.Payload, ' ') }},
		{"schema", func(g *catalogs.Generation) { g.Manifest.SchemaVersion++ }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			record := original
			record.Baseline = original.Baseline.Copy()
			scenario.change(&record.Baseline)
			raw, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeFleetRecovery(t.Context(), fleetCompressedTest(t, raw)); err == nil {
				t.Fatal("accepted an unusable baseline")
			}
		})
	}
}

func fleetCompressedTest(t *testing.T, data []byte) []byte {
	t.Helper()
	compressed, err := compressFleetRecovery(data)
	if err != nil {
		t.Fatal(err)
	}
	return compressed
}
