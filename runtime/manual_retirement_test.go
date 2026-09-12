package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestDistinctProviderRetirementPreservesReplay(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	for _, scenario := range []string{"increasing", "return-to-first", "independent-fields", "interleaved-accounts"} {
		t.Run(scenario, func(t *testing.T) {
			var observations []sources.Observation
			bindings := make(map[string]sources.ProviderAcquisitionBinding)
			for index := range 16 {
				limit := int64(200 + index)
				var binding *sources.ProviderAcquisitionBinding
				switch scenario {
				case "return-to-first":
					if index >= 8 {
						limit = 200
					}
				case "interleaved-accounts":
					id := "account-a"
					limit = int64(200 + index/2)
					if index%2 != 0 {
						id, limit = "account-b", 207
					}
					binding = scopedProviderLayer(t, id, "1", at).Receipt.ProviderBinding
					bindings[id] = *binding
				}
				observedAt := at.Add(time.Duration(index) * time.Minute)
				catalog := manualProviderObservation(t, limit, observedAt).Catalog
				if scenario == "independent-fields" {
					catalog = retentionModelCatalog(t, catalog, func(model *catalogs.Model) {
						model.Name = fmt.Sprintf("Name %d", min(index, 7))
						model.Description = fmt.Sprintf("Description %d", min(index/2, 5))
						model.Limits.OutputTokens = int64(400 + min(index, 11))
					})
				}
				observations = append(observations, providerResetObservation(t, sources.ProvidersID, catalog, observedAt, binding))
			}
			history := retentionHistory(t, observations)
			compacted, err := compactProviderHistory(t.Context(), history)
			if err != nil {
				t.Fatal(err)
			}
			if len(manualBatches(compacted)) >= len(manualBatches(history)) {
				t.Fatal("distinct observations did not retire")
			}
			selections := map[string]*providerBindingPolicy{"unscoped": nil}
			if len(bindings) != 0 {
				selections = map[string]*providerBindingPolicy{"both": {bindings: bindings}, "none": {bindings: map[string]sources.ProviderAcquisitionBinding{}}}
				for id, binding := range bindings {
					selections[id] = &providerBindingPolicy{bindings: map[string]sources.ProviderAcquisitionBinding{id: binding}}
				}
			}
			baselines := []starmap.CatalogState{unreviewedBaseline(t)}
			for _, limit := range []int64{100, 200, 207, 215} {
				baselines = append(baselines, starmap.CatalogState{GenerationID: fmt.Sprintf("baseline-%d", limit),
					Catalog: manualProviderObservation(t, limit, at).Catalog, GeneratedAt: at.Add(-time.Minute)})
			}
			for name, policy := range selections {
				for _, baseline := range baselines {
					t.Run(name+"/"+baseline.GenerationID, func(t *testing.T) {
						assertRetirementReplay(t, layerSet{manual: history, providerBindings: policy}, layerSet{manual: compacted, providerBindings: policy}, baseline)
					})
				}
			}
		})
	}
}

func retentionHistory(t *testing.T, observations []sources.Observation) *manualBatch {
	t.Helper()
	var history *manualBatch
	for _, observation := range observations {
		prepared, err := prepareManualObservations(t.Context(), []sources.Observation{observation})
		if err != nil {
			t.Fatal(err)
		}
		history = &manualBatch{parent: history, observations: prepared}
	}
	return history
}

func retentionModelCatalog(t *testing.T, catalog *catalogs.Catalog, change func(*catalogs.Model)) *catalogs.Catalog {
	t.Helper()
	builder, err := catalogs.NewBuilderFrom(catalog)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := builder.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	change(provider.Models["model"])
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	result, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertRetirementReplay(t *testing.T, full, reduced layerSet, baseline starmap.CatalogState) {
	t.Helper()
	full.publisherID, reduced.publisherID = "retirement-test", "retirement-test"
	before, err := full.build(t.Context(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	after, err := reduced.build(t.Context(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	beforeEvidence, err := json.Marshal(full.buildEvidence)
	if err != nil {
		t.Fatal(err)
	}
	afterEvidence, err := json.Marshal(reduced.buildEvidence)
	if err != nil {
		t.Fatal(err)
	}
	if before.GenerationID != after.GenerationID || before.PayloadChecksum != after.PayloadChecksum || !bytes.Equal(beforeEvidence, afterEvidence) {
		beforePayload, _ := catalogs.EncodeCatalogPayload(before.Catalog)
		afterPayload, _ := catalogs.EncodeCatalogPayload(after.Catalog)
		t.Fatalf("retirement changed replay\nbefore payload: %s\nafter payload: %s\nbefore evidence: %s\nafter evidence: %s", beforePayload, afterPayload, beforeEvidence, afterEvidence)
	}
}

func TestProviderRetirementPreservesReplayBoundaries(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	for _, scenario := range []string{"metadata", "reset", "partial", "fallback", "empty-inventory", "canonical-link", "out-of-order"} {
		t.Run(scenario, func(t *testing.T) {
			var observations []sources.Observation
			for index := range 16 {
				observedAt := at.Add(time.Duration(index) * time.Minute)
				catalog := manualProviderObservation(t, int64(200+min(index, 10)), observedAt).Catalog
				metadata := sources.ObservationMetadata{ObservedAt: observedAt, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
					Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded}
				source := sources.ProvidersID
				if index == 6 {
					switch scenario {
					case "metadata":
						source = sources.ModelsDevHTTPID
					case "partial", "fallback":
						metadata.Completeness, metadata.Status = sources.ObservationCompletenessPartial, sources.ObservationStatusDegraded
						issue := sources.ObservationIssue{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "provider/other", Message: "Invalid record."}
						if scenario == "fallback" {
							issue.Scope, issue.Code = sources.ObservationIssueScopeStaleFallback, sources.ObservationIssueCodeStaleFallback
						}
						metadata.Issues = []sources.ObservationIssue{issue}
					case "empty-inventory":
						builder, err := catalogs.NewBuilderFrom(catalog)
						if err != nil {
							t.Fatal(err)
						}
						provider, err := builder.Provider("provider")
						if err != nil {
							t.Fatal(err)
						}
						provider.Models = nil
						if err := builder.SetProvider(provider); err != nil {
							t.Fatal(err)
						}
						catalog, err = catalogs.NewObservationCatalog(builder)
						if err != nil {
							t.Fatal(err)
						}
					case "canonical-link":
						catalog = retentionModelCatalog(t, catalog, func(model *catalogs.Model) { model.ModelRef = "other/model" })
					}
				}
				observation, err := sources.NewObservation(source, catalog, metadata)
				if err != nil {
					t.Fatal(err)
				}
				observations = append(observations, observation)
			}
			if scenario == "out-of-order" {
				slices.Reverse(observations)
			}
			history := retentionHistory(t, observations)
			if scenario == "reset" {
				manualBatches(history)[6].resets = []ObservationReset{{ProviderID: "provider"}}
			}
			compacted, err := compactProviderHistory(t.Context(), history)
			if err != nil {
				t.Fatal(err)
			}
			baselines := []starmap.CatalogState{unreviewedBaseline(t), {GenerationID: "reviewed", Catalog: manualProviderObservation(t, 100, at).Catalog, GeneratedAt: at.Add(-time.Minute)}}
			for _, baseline := range baselines {
				for _, selection := range []*acquisitionSourcePolicy{nil, {ids: []sources.ID{sources.ProvidersID}}, {ids: []sources.ID{}}} {
					assertRetirementReplay(t, layerSet{manual: history, acquisitionSources: selection}, layerSet{manual: compacted, acquisitionSources: selection}, baseline)
				}
			}
			second, err := compactProviderHistory(t.Context(), compacted)
			if err != nil {
				t.Fatal(err)
			}
			assertRetirementReplay(t, layerSet{manual: compacted}, layerSet{manual: second}, baselines[1])
		})
	}
}

func TestProviderRetirementPreservesConflictsAndRefusesInvalidInputs(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	var observations []sources.Observation
	for index := range 12 {
		minute := index
		if index == 6 {
			minute = 5
		}
		observations = append(observations, manualProviderObservation(t, int64(200+index), at.Add(time.Duration(minute)*time.Minute)))
	}
	history := retentionHistory(t, observations)
	compacted, err := compactProviderHistory(t.Context(), history)
	if err != nil {
		t.Fatal(err)
	}
	baseline := starmap.CatalogState{GenerationID: "reviewed", Catalog: manualProviderObservation(t, 100, at).Catalog, GeneratedAt: at.Add(-time.Minute)}
	full, reduced := layerSet{manual: history}, layerSet{manual: compacted}
	_, originalErr := full.build(t.Context(), baseline)
	_, retiredErr := reduced.build(t.Context(), baseline)
	if originalErr == nil || retiredErr == nil || originalErr.Error() != retiredErr.Error() {
		t.Fatalf("retirement changed an equal-priority conflict: %v / %v", originalErr, retiredErr)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := compactProviderHistory(ctx, history); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled retirement: %v", err)
	}
	corrupt := &manualBatch{observations: slices.Clone(history.observations)}
	corrupt.observations[0].Receipt.Link.EvidenceChecksum = "invalid"
	if result, err := compactProviderHistory(t.Context(), corrupt); err == nil || result != nil {
		t.Fatal("retirement accepted invalid original evidence")
	}
	if _, err := history.observations[0].restore(); err != nil {
		t.Fatalf("failed retirement changed the original receipt: %v", err)
	}
}

func TestProviderRetirementPreservesAccountSubsets(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	for seed := range uint64(8) {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			random := rand.New(rand.NewPCG(seed, 17))
			var observations []sources.Observation
			var bindings []sources.ProviderAcquisitionBinding
			for index := range 3 {
				bindings = append(bindings, *scopedProviderLayer(t, fmt.Sprintf("account-%d", index), "1", at).Receipt.ProviderBinding)
			}
			for index := range 24 {
				observedAt := at.Add(time.Duration(index) * time.Minute)
				catalog := manualProviderObservation(t, int64(200+random.IntN(4)), observedAt).Catalog
				observations = append(observations, providerResetObservation(t, sources.ProvidersID, catalog, observedAt, &bindings[index%len(bindings)]))
			}
			history := retentionHistory(t, observations)
			compacted, err := compactProviderHistory(t.Context(), history)
			if err != nil {
				t.Fatal(err)
			}
			baselines := []starmap.CatalogState{unreviewedBaseline(t), {GenerationID: "reviewed", Catalog: manualProviderObservation(t, 200, at).Catalog, GeneratedAt: at.Add(-time.Minute)}}
			for mask := 1; mask < 1<<len(bindings); mask++ {
				t.Run(fmt.Sprintf("selection-%d", mask), func(t *testing.T) {
					policy := &providerBindingPolicy{bindings: make(map[string]sources.ProviderAcquisitionBinding)}
					for index, binding := range bindings {
						if mask&(1<<index) != 0 {
							policy.bindings[binding.ID] = binding
						}
					}
					for _, baseline := range baselines {
						assertRetirementReplay(t, layerSet{manual: history, providerBindings: policy}, layerSet{manual: compacted, providerBindings: policy}, baseline)
					}
				})
			}
		})
	}
}
