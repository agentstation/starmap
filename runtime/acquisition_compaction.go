package runtime

import (
	"bytes"
	"context"
	"reflect"
	"slices"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// PrepareAcquisitionReplay returns a catalog generation and the original inputs needed for later replay.
// It retains compacted inputs only when catalog facts, provenance, membership, and current reviews remain exact.
// Current metadata reviews retain their latest original observation. Omission preserves the last review.
// The caller authenticates the baseline and inputs. This function reads no sources or storage.
func PrepareAcquisitionReplay(ctx context.Context, baseline catalogs.Generation, publisherID string, bindings []sources.ProviderAcquisitionBinding, observations []sources.Observation, runID string, completedAt time.Time) (catalogs.Generation, []sources.Observation, error) {
	candidate, err := replayAcquisition(ctx, baseline, publisherID, bindings, observations, true)
	if err != nil {
		return catalogs.Generation{}, nil, err
	}
	expected, err := candidate.Generation(runID, completedAt)
	if err != nil {
		return catalogs.Generation{}, nil, err
	}
	selected, err := compactAcquisitionHistory(ctx, observations)
	if err != nil {
		return catalogs.Generation{}, nil, err
	}
	if len(selected) == len(observations) {
		return expected, selected, nil
	}
	candidate, err = replayAcquisition(ctx, baseline, publisherID, bindings, selected, true)
	if err != nil {
		return catalogs.Generation{}, nil, err
	}
	actual, err := candidate.Generation(runID, completedAt)
	if err != nil {
		return catalogs.Generation{}, nil, err
	}
	if !bytes.Equal(actual.Payload, expected.Payload) || !reflect.DeepEqual(actual.Manifest.ReviewCandidates, expected.Manifest.ReviewCandidates) {
		return expected, observations, ctx.Err()
	}
	return expected, selected, ctx.Err()
}

func compactAcquisitionHistory(ctx context.Context, observations []sources.Observation) ([]sources.Observation, error) {
	if ctx == nil {
		return nil, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if len(observations) > maxManualHistoryBatches {
		return nil, &errors.ValidationError{Field: "replay.observations", Message: "exceeds the retained history bound"}
	}
	var providers *manualBatch
	var metadata []manualObservation
	seen := make(map[string]bool, len(observations))
	size := 0
	for _, input := range observations {
		if seen[input.ID] {
			return nil, &errors.ValidationError{Field: "replay.observations", Message: "contains a duplicate observation identity"}
		}
		seen[input.ID] = true
		prepared, err := prepareManualObservations(ctx, []sources.Observation{input})
		if err != nil {
			return nil, err
		}
		observation := prepared[0]
		if len(observation.Payload) > maxLayerBytes-size {
			return nil, &errors.ValidationError{Field: "replay.observations", Message: "exceeds the retained payload bound"}
		}
		size += len(observation.Payload)
		if input.SourceID == sources.ProvidersID {
			providers = &manualBatch{parent: providers, observations: prepared}
		} else {
			metadata = appendReplayMetadata(metadata, observation)
		}
	}
	// Replay applies all metadata before providers and has no reset batches.
	// Provider compaction therefore uses one epoch regardless of metadata collection order.
	compacted, err := compactProviderHistory(ctx, compactRepeatedReplayRounds(providers))
	if err != nil {
		return nil, err
	}
	selected := make(map[string]bool)
	for _, batch := range manualBatches(compacted) {
		for _, observation := range batch.observations {
			selected[observation.Receipt.Link.ObservationID] = true
		}
	}
	for _, observation := range metadata {
		selected[observation.Receipt.Link.ObservationID] = true
	}
	retained := make([]sources.Observation, 0, len(selected))
	for _, observation := range observations {
		if !selected[observation.ID] {
			continue
		}
		observation.Issues = slices.Clone(observation.Issues)
		if observation.ProviderBinding != nil {
			binding := *observation.ProviderBinding
			observation.ProviderBinding = &binding
		}
		retained = append(retained, observation)
	}
	return retained, ctx.Err()
}

func appendReplayMetadata(history []manualObservation, current manualObservation) []manualObservation {
	if len(history) >= 2 {
		first, previous := history[len(history)-2], history[len(history)-1]
		if sameReplayInput(first, previous) && sameReplayInput(previous, current) &&
			first.Receipt.Link.ObservedAt.Before(previous.Receipt.Link.ObservedAt) &&
			previous.Receipt.Link.ObservedAt.Before(current.Receipt.Link.ObservedAt) {
			history[len(history)-1] = current
			return history
		}
	}
	return append(history, current)
}

// Equal-time provider observations reconcile together. Keep complete repeated rounds together too.
func compactRepeatedReplayRounds(history *manualBatch) *manualBatch {
	var rounds [][]manualObservation
	for _, batch := range manualBatches(history) {
		for _, observation := range batch.observations {
			last := len(rounds) - 1
			if last >= 0 && rounds[last][0].Receipt.Link.ObservedAt.Equal(observation.Receipt.Link.ObservedAt) {
				rounds[last] = append(rounds[last], observation)
			} else {
				rounds = append(rounds, []manualObservation{observation})
			}
		}
	}
	var selected [][]manualObservation
	for _, round := range rounds {
		if last := len(selected) - 1; last > 0 && sameReplayRound(selected[last-1], selected[last]) && sameReplayRound(selected[last], round) {
			selected[last] = round
		} else {
			selected = append(selected, round)
		}
	}
	var compacted *manualBatch
	for _, round := range selected {
		compacted = &manualBatch{parent: compacted, observations: round}
	}
	return compacted
}

func sameReplayRound(left, right []manualObservation) bool {
	if len(left) != len(right) || !left[0].Receipt.Link.ObservedAt.Before(right[0].Receipt.Link.ObservedAt) {
		return false
	}
	for index := range left {
		if !sameReplayInput(left[index], right[index]) {
			return false
		}
	}
	return true
}

func sameReplayInput(left, right manualObservation) bool {
	a, b := left.Receipt, right.Receipt
	return a.Link.Source == b.Link.Source && a.Link.Revision == b.Link.Revision &&
		a.Link.Completeness == sources.ObservationCompletenessComplete && b.Link.Completeness == a.Link.Completeness &&
		a.Link.Status == sources.ObservationStatusSucceeded && b.Link.Status == a.Link.Status &&
		len(a.Issues) == 0 && len(b.Issues) == 0 && a.Records == b.Records &&
		reflect.DeepEqual(a.ProviderBinding, b.ProviderBinding) && bytes.Equal(left.Payload, right.Payload)
}

// Current metadata reviews retain the latest original evidence for each unresolved offering.
// An omitted offering keeps its last review. Provider reviews retain their account scopes.
func currentReplayReviews(manifest catalogs.GenerationManifest) ([]evidence.ReviewCandidate, error) {
	type identity struct {
		source   sources.ID
		provider string
		model    string
		code     evidence.ReviewCandidateCode
	}
	links := make(map[string]catalogs.SourceObservationLink, len(manifest.SourceObservations))
	for _, link := range manifest.SourceObservations {
		links[link.ObservationID] = link
	}
	winners := make(map[identity]evidence.ReviewCandidate)
	var selected []evidence.ReviewCandidate
	for _, review := range manifest.ReviewCandidates {
		if review.SourceID == sources.ProvidersID {
			selected = append(selected, review)
			continue
		}
		link, found := links[review.SourceObservationID]
		if !found || link.Source != review.SourceID || link.Revision != review.SourceRevision || link.EvidenceChecksum != review.EvidenceChecksum {
			return nil, &errors.ConflictError{Resource: "review observation", Message: "does not match its original source evidence"}
		}
		key := identity{review.SourceID, review.ProviderID, review.ProviderModelID, review.Code}
		previous, exists := winners[key]
		prior := links[previous.SourceObservationID]
		if !exists || link.ObservedAt.After(prior.ObservedAt) ||
			(link.ObservedAt.Equal(prior.ObservedAt) && link.ObservationID > prior.ObservationID) {
			winners[key] = review
		}
	}
	for _, review := range winners {
		selected = append(selected, review)
	}
	slices.SortFunc(selected, evidence.CompareReviewCandidates)
	return selected, nil
}
