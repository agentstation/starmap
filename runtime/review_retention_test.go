package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"slices"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/sources"
)

func providerReviewHistory(t *testing.T, count int, distinct bool) (*manualBatch, sources.Observation) {
	t.Helper()
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	var history *manualBatch
	var latest sources.Observation
	for i := range count {
		limit := int64(200)
		if distinct {
			limit += int64(i)
		}
		latest = manualProviderObservation(t, limit, at.Add(time.Duration(i)*time.Minute))
		prepared, err := prepareManualObservations(t.Context(), []sources.Observation{latest})
		if err != nil {
			t.Fatal(err)
		}
		history = &manualBatch{parent: history, observations: prepared}
	}
	return history, latest
}

func scopedReviewObservation(t *testing.T, binding, revision string, minute int) sources.Observation {
	t.Helper()
	layer := scopedProviderLayer(t, binding, revision, time.Date(2026, 9, 12, 0, minute, 0, 0, time.UTC))
	observation, err := (manualObservation{Payload: layer.Payload, Receipt: layer.Receipt}).restore()
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func reviewForObservation(observation sources.Observation) evidence.ReviewCandidate {
	return evidence.ReviewCandidate{Code: evidence.ReviewCandidateUnresolvedModelReference, ProviderID: "provider", ProviderModelID: "model",
		SourceID: observation.SourceID, SourceObservationID: observation.ID, SourceRevision: observation.Revision, EvidenceChecksum: observation.EvidenceChecksum}
}

func TestProviderReviewSelectionKeepsAccountRevisionsAndOmissions(t *testing.T) {
	old := scopedReviewObservation(t, "account-a", "1", 0)
	current := scopedReviewObservation(t, "account-a", "1", 1)
	peer := scopedReviewObservation(t, "account-b", "1", 2)
	revised := scopedReviewObservation(t, "account-a", "2", 3)
	omitted := scopedReviewObservation(t, "omitted", "1", 0)
	base := scopedReviewObservation(t, "omitted", "1", 4)
	builder, err := catalogs.NewBuilderFrom(base.Catalog)
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
	catalog, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ObservedAt: base.ObservedAt, ProviderBinding: base.ProviderBinding, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded})
	if err != nil {
		t.Fatal(err)
	}
	observations := []sources.Observation{old, current, peer, revised, omitted, empty}
	var reviews []evidence.ReviewCandidate
	for _, observation := range observations[:len(observations)-1] {
		reviews = append(reviews, reviewForObservation(observation))
	}
	for _, reverse := range []bool{false, true} {
		input := slices.Clone(reviews)
		if reverse {
			slices.Reverse(input)
		}
		selected, err := selectCurrentProviderReviews(t.Context(), observations, input)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, candidate := range selected {
			ids = append(ids, candidate.SourceObservationID)
		}
		want := []string{current.ID, peer.ID, revised.ID, omitted.ID}
		slices.Sort(ids)
		slices.Sort(want)
		if !slices.Equal(ids, want) {
			t.Fatalf("account or omission boundary changed: %v, want %v", ids, want)
		}
	}
}

func TestProviderReviewSelectionPreservesPriorityAndMetadata(t *testing.T) {
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	direct := manualProviderObservation(t, 200, at)
	for _, fallback := range []bool{false, true} {
		t.Run(map[bool]string{false: "partial-direct", true: "stale-fallback"}[fallback], func(t *testing.T) {
			metadata := sources.ObservationMetadata{ObservedAt: at.Add(time.Minute), Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
				Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
				Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "provider/other", Message: "A source record is invalid."}}}
			if fallback {
				metadata.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeStaleFallback, Code: sources.ObservationIssueCodeStaleFallback, Message: "Provider response uses retained evidence"}}
			}
			later, err := sources.NewObservation(sources.ProvidersID, direct.Catalog, metadata)
			if err != nil {
				t.Fatal(err)
			}
			metadataObservation := providerResetObservation(t, sources.ModelsDevHTTPID, direct.Catalog, at, nil)
			metadataReview := reviewForObservation(metadataObservation)
			input := []evidence.ReviewCandidate{reviewForObservation(later), metadataReview, reviewForObservation(direct)}
			selected, err := selectCurrentProviderReviews(t.Context(), []sources.Observation{direct, later}, input)
			if err != nil {
				t.Fatal(err)
			}
			want := later.ID
			if fallback {
				want = direct.ID
			}
			if len(selected) != 2 {
				t.Fatalf("selected %d reviews, want provider and metadata", len(selected))
			}
			for _, candidate := range selected {
				if candidate.SourceID == sources.ProvidersID && candidate.SourceObservationID != want {
					t.Fatal("review selected a lower-priority observation")
				}
			}
			if !slices.Contains(selected, metadataReview) {
				t.Fatal("provider review selection changed metadata evidence")
			}
		})
	}
}

func TestProviderReviewSelectionRejectsMismatchedEvidenceAndCancellation(t *testing.T) {
	observation := scopedReviewObservation(t, "account", "1", 0)
	for _, field := range []string{"missing", "checksum", "revision"} {
		t.Run(field, func(t *testing.T) {
			candidate := reviewForObservation(observation)
			switch field {
			case "missing":
				candidate.SourceObservationID = "absent"
			case "checksum":
				candidate.EvidenceChecksum = "changed"
			case "revision":
				candidate.SourceRevision.Value = "changed"
			}
			input := []evidence.ReviewCandidate{candidate}
			if selected, err := selectCurrentProviderReviews(t.Context(), []sources.Observation{observation}, input); err == nil || selected != nil {
				t.Fatalf("unverified review accepted: %+v, %v", selected, err)
			}
			if input[0] != candidate {
				t.Fatal("failed selection mutated its input")
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := selectCurrentProviderReviews(ctx, nil, nil); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled empty selection: %v", err)
	}
	if selected, err := selectCurrentProviderReviews(ctx, []sources.Observation{observation}, []evidence.ReviewCandidate{reviewForObservation(observation)}); !stderrors.Is(err, context.Canceled) || selected != nil {
		t.Fatalf("canceled review selection: %+v, %v", selected, err)
	}
}

func unreviewedBaseline(t *testing.T) starmap.CatalogState {
	t.Helper()
	catalog, err := catalogs.DecodeCatalogPayload(testCatalogPayload(t, "provider", "reviewed", "Reviewed"))
	if err != nil {
		t.Fatal(err)
	}
	return starmap.CatalogState{GenerationID: "unreviewed", Catalog: catalog, GeneratedAt: time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)}
}

func TestProviderReviewHistorySelectsCurrentReceipt(t *testing.T) {
	history, latest := providerReviewHistory(t, 12, true)
	layers := layerSet{manual: history}
	state, err := layers.build(t.Context(), unreviewedBaseline(t))
	if err != nil {
		t.Fatal(err)
	}
	reviews := layers.buildEvidence.ReviewCandidates
	if len(reviews) != 1 || reviews[0].SourceObservationID != latest.ID || reviews[0].EvidenceChecksum != latest.EvidenceChecksum {
		t.Fatalf("current review set retained superseded observations: %+v", reviews)
	}
	if len(layers.buildEvidence.SourceObservations) != 1 || layers.buildEvidence.SourceObservations[0].ObservationID != latest.ID {
		t.Fatalf("obsolete reviews pinned old generation receipts: %+v", layers.buildEvidence.SourceObservations)
	}
	provider, err := state.Catalog.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	if provider.Models["model"] != nil {
		t.Fatal("unreviewed model became routable")
	}
	if len(manualBatches(history)) != 12 {
		t.Fatal("review selection discarded durable source observations")
	}
}

func TestProviderReviewHistorySurvivesBaselineChangeAndCompaction(t *testing.T) {
	history, _ := providerReviewHistory(t, 12, false)
	compacted, err := compactRepeatedProviderHistory(t.Context(), history)
	if err != nil {
		t.Fatal(err)
	}
	if len(manualBatches(compacted)[0].observations) >= 12 {
		t.Fatal("fixture did not compact repeated observations")
	}
	for _, baseline := range []starmap.CatalogState{
		{GenerationID: "reviewed", Catalog: manualProviderObservation(t, 100, time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)).Catalog},
		unreviewedBaseline(t),
	} {
		t.Run(baseline.GenerationID, func(t *testing.T) {
			full := layerSet{manual: history}
			before, err := full.build(t.Context(), baseline)
			if err != nil {
				t.Fatal(err)
			}
			reduced := layerSet{manual: compacted}
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
			if before.PayloadChecksum != after.PayloadChecksum || !bytes.Equal(beforeEvidence, afterEvidence) {
				t.Fatalf("compaction changed replay after baseline selection: before=%s after=%s reviews=%d/%d", before.PayloadChecksum, after.PayloadChecksum, len(full.buildEvidence.ReviewCandidates), len(reduced.buildEvidence.ReviewCandidates))
			}
		})
	}
}
