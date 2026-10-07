package runtime

import (
	"bytes"
	"context"
	stderrors "errors"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestAcquisitionCompactionRejectsInvalidInput(t *testing.T) {
	observation := manualProviderObservation(t, 200, time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC))
	for _, name := range []string{"nil-context", "canceled", "duplicate", "changed-evidence", "history-limit"} {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			inputs := []sources.Observation{observation}
			switch name {
			case "nil-context":
				ctx = nil
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "duplicate":
				inputs = append(inputs, observation)
			case "changed-evidence":
				inputs[0].ObservedAt = inputs[0].ObservedAt.Add(time.Second)
			case "history-limit":
				inputs = make([]sources.Observation, maxManualHistoryBatches+1)
			}
			selected, _, err := compactAcquisitionHistory(ctx, inputs, nil)
			if err == nil || selected != nil {
				t.Fatal("invalid input produced a retained history")
			}
			if name == "canceled" && !stderrors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost its identity: %v", err)
			}
		})
	}
}

func TestAcquisitionCompactionReturnsOwnedOriginalEvidence(t *testing.T) {
	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	binding := scopedProviderLayer(t, "account", "1", at).Receipt.ProviderBinding
	var inputs []sources.Observation
	for index := range 5 {
		inputs = append(inputs, providerResetObservation(t, sources.ProvidersID, manualProviderObservation(t, 200, at).Catalog, at.Add(time.Duration(index)*time.Hour), binding))
	}
	selected, _, err := compactAcquisitionHistory(t.Context(), inputs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 || selected[0].ID != inputs[0].ID || selected[1].ID != inputs[4].ID {
		t.Fatal("stable inventories lost their first or latest original evidence")
	}
	for _, observation := range selected {
		if err := observation.Validate(); err != nil {
			t.Fatal("compaction changed original evidence", err)
		}
	}
	selected[0].ProviderBinding.AccountID = "caller-change"
	selected[0].ID = "caller-change"
	if inputs[0].ProviderBinding.AccountID != binding.AccountID || inputs[0].ID == "caller-change" {
		t.Fatal("caller changed accepted input")
	}
}

func TestAcquisitionReplayCompactsSupersededMetadataBeforeRetainedBound(t *testing.T) {
	at := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	baseline := replayBaselineGeneration(t, at)
	for _, name := range []string{"latest", "cited-review", "stamped-update"} {
		t.Run(name, func(t *testing.T) {
			var inputs []sources.Observation
			total := 0
			description := ""
			for index := 0; total <= maxLayerBytes; index++ {
				var unresolved []string
				// The first snapshot adds the review provider, so a later snapshot can raise a review in it.
				if name == "cited-review" && index == 0 {
					unresolved = []string{"seed"}
				} else if name == "cited-review" && index == 1 {
					unresolved = []string{"retired"}
				}
				description = strings.Repeat(string(rune('a'+index)), 6<<20)
				observation := metadataSnapshot(t, at.Add(time.Duration(index)*time.Hour), description, unresolved...)
				payload, err := catalogs.EncodeCatalogPayload(observation.Catalog)
				if err != nil {
					t.Fatal(err)
				}
				total += len(payload)
				inputs = append(inputs, observation)
			}
			if name == "stamped-update" {
				// The latest snapshot repeats the offering, so the previous snapshot keeps the update time.
				inputs = append(inputs, metadataSnapshot(t, at.Add(time.Duration(len(inputs))*time.Hour), description))
			}
			generation, retained, err := PrepareAcquisitionReplay(t.Context(), baseline, "publisher", nil, inputs, "compaction", at.Add(time.Duration(len(inputs))*time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			// The first snapshot stamps the offering creation time. The latest change stamps its update time.
			want := []string{inputs[0].ID, inputs[len(inputs)-1].ID}
			switch name {
			case "cited-review":
				want = []string{inputs[0].ID, inputs[1].ID, inputs[len(inputs)-1].ID}
			case "stamped-update":
				want = []string{inputs[0].ID, inputs[len(inputs)-2].ID, inputs[len(inputs)-1].ID}
			}
			if len(retained) != len(want) {
				t.Fatalf("retained %d of %d snapshots; want %d", len(retained), len(inputs), len(want))
			}
			size := 0
			for index, observation := range retained {
				if observation.ID != want[index] {
					t.Fatalf("retained snapshot %d is %s; want %s", index, observation.ID, want[index])
				}
				payload, err := catalogs.EncodeCatalogPayload(observation.Catalog)
				if err != nil {
					t.Fatal(err)
				}
				size += len(payload)
			}
			if size > maxLayerBytes {
				t.Fatalf("retained %d payload bytes above the %d byte bound", size, maxLayerBytes)
			}
			if name == "cited-review" && !citesReview(generation, inputs[1].ID, "retired") {
				t.Fatal("omission dropped the review of the older snapshot")
			}
			candidate, err := ReplayAcquisition(t.Context(), baseline, "publisher", nil, retained)
			if err != nil {
				t.Fatal(err)
			}
			replayed, err := candidate.Generation("compaction", generation.Manifest.GeneratedAt)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(replayed.Payload, generation.Payload) {
				t.Fatal("compacted history changed catalog facts or provenance")
			}
		})
	}
}

func TestAcquisitionReplayRejectsRetainedHistoryAboveBound(t *testing.T) {
	at := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	baseline := replayBaselineGeneration(t, at)
	var inputs []sources.Observation
	total := 0
	for index := 0; total <= maxLayerBytes; index++ {
		// Each snapshot owns a current review, so compaction must retain all of them.
		description := strings.Repeat(string(rune('a'+index)), 6<<20)
		observation := metadataSnapshot(t, at.Add(time.Duration(index)*time.Hour), description, "unresolved-"+string(rune('a'+index)))
		payload, err := catalogs.EncodeCatalogPayload(observation.Catalog)
		if err != nil {
			t.Fatal(err)
		}
		total += len(payload)
		inputs = append(inputs, observation)
	}
	if _, err := ReplayAcquisition(t.Context(), baseline, "publisher", nil, inputs); err != nil {
		t.Fatalf("accepted history above the retained bound did not replay: %v", err)
	}
	_, retained, err := PrepareAcquisitionReplay(t.Context(), baseline, "publisher", nil, inputs, "compaction", at.Add(time.Duration(len(inputs))*time.Hour))
	var validation *errors.ValidationError
	if !stderrors.As(err, &validation) || validation.Field != "replay.observations" || validation.Message != "exceeds the retained payload bound" || retained != nil {
		t.Fatalf("cited history above the retained bound was retained: %v", err)
	}
}

func TestAcquisitionReplayRejectsOneObservationAboveRecordBound(t *testing.T) {
	at := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	baseline := replayBaselineGeneration(t, at)
	// A catalog payload cannot reach the record bound, so the receipt carries the excess bytes.
	oversized, err := sources.NewObservation(sources.ModelsDevHTTPID, metadataSnapshot(t, at, "small").Catalog, sources.ObservationMetadata{
		ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindETag, Value: strings.Repeat("x", maxLayerBytes)},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"replay", "prepare"} {
		t.Run(name, func(t *testing.T) {
			var err error
			if name == "replay" {
				_, err = ReplayAcquisition(t.Context(), baseline, "publisher", nil, []sources.Observation{oversized})
			} else {
				_, _, err = PrepareAcquisitionReplay(t.Context(), baseline, "publisher", nil, []sources.Observation{oversized}, "oversized", at)
			}
			var validation *errors.ValidationError
			if !stderrors.As(err, &validation) || validation.Field != "manual.observation" {
				t.Fatalf("one observation above the record bound was accepted: %v", err)
			}
		})
	}
}

func replayBaselineGeneration(t *testing.T, at time.Time) catalogs.Generation {
	t.Helper()
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := starmap.NewCandidate(catalog, starmap.CandidateEvidence{}, starmap.WithCandidateGenerationID("baseline"))
	if err != nil {
		t.Fatal(err)
	}
	generation, err := candidate.Generation("baseline", at)
	if err != nil {
		t.Fatal(err)
	}
	return generation
}

// metadataSnapshot returns one complete models.dev observation for the baseline provider.
// Each unresolved model has no canonical reference, so replay records a review candidate for it.
func metadataSnapshot(t *testing.T, at time.Time, description string, unresolved ...string) sources.Observation {
	t.Helper()
	builder, err := catalogs.NewBuilderFrom(manualProviderObservation(t, 0, at).Catalog)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := builder.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	provider.Models["model"].Description = description
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	if len(unresolved) != 0 {
		review := catalogs.Provider{ID: "review", Name: "Review", Models: make(map[string]*catalogs.Model)}
		for _, id := range unresolved {
			review.Models[id] = &catalogs.Model{ID: id, Name: id}
		}
		if err := builder.SetProvider(review); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	return providerResetObservation(t, sources.ModelsDevHTTPID, catalog, at, nil)
}

func citesReview(generation catalogs.Generation, observationID, modelID string) bool {
	for _, review := range generation.Manifest.ReviewCandidates {
		if review.SourceObservationID == observationID && review.ProviderModelID == modelID {
			return true
		}
	}
	return false
}
