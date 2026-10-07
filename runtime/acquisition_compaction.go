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
//
// The retained inputs must fit the retained payload bound. The supplied history can exceed it.
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
	cited, err := replayCitations(expected, observations)
	if err != nil {
		return catalogs.Generation{}, nil, err
	}
	selected, sizes, err := compactAcquisitionHistory(ctx, observations, cited)
	if err != nil {
		return catalogs.Generation{}, nil, err
	}
	if len(selected) != len(observations) {
		candidate, err = replayAcquisition(ctx, baseline, publisherID, bindings, selected, true)
		if err != nil {
			return catalogs.Generation{}, nil, err
		}
		actual, err := candidate.Generation(runID, completedAt)
		if err != nil {
			return catalogs.Generation{}, nil, err
		}
		if !bytes.Equal(actual.Payload, expected.Payload) || !reflect.DeepEqual(actual.Manifest.ReviewCandidates, expected.Manifest.ReviewCandidates) {
			selected = observations
		}
	}
	retained := 0
	for _, observation := range selected {
		retained += sizes[observation.ID]
	}
	if retained > maxLayerBytes {
		return catalogs.Generation{}, nil, &errors.ValidationError{Field: "replay.observations", Message: "exceeds the retained payload bound"}
	}
	return expected, selected, ctx.Err()
}

// replayCitations returns the observations that the generation still names.
// Payload provenance, membership scopes, and review candidates each name their original observation.
// A metadata pass stamps the offerings that it adds or changes with its observation time.
// An offering timestamp therefore names each metadata observation at that time.
func replayCitations(generation catalogs.Generation, observations []sources.Observation) (map[string]bool, error) {
	catalog, err := catalogs.DecodeCatalogGeneration(generation)
	if err != nil {
		return nil, err
	}
	stamped := make(map[int64]bool)
	catalog.Providers().ForEach(func(_ catalogs.ProviderID, provider *catalogs.Provider) bool {
		for _, model := range provider.Models {
			stamped[model.CreatedAt.Time().UnixNano()] = true
			stamped[model.UpdatedAt.Time().UnixNano()] = true
		}
		return true
	})
	cited := make(map[string]bool)
	for _, observation := range observations {
		if observation.SourceID != sources.ProvidersID && stamped[observation.ObservedAt.UnixNano()] {
			cited[observation.ID] = true
		}
	}
	for _, entries := range catalog.Provenance().Map() {
		for _, entry := range entries {
			cited[entry.ObservationID] = true
		}
	}
	for _, scope := range catalog.MembershipScopes() {
		if scope.Inventory != nil {
			cited[scope.Inventory.ObservationID] = true
		}
		for _, addition := range scope.Additions {
			cited[addition.ObservationID] = true
		}
	}
	for _, review := range generation.Manifest.ReviewCandidates {
		cited[review.SourceObservationID] = true
	}
	return cited, nil
}

// compactAcquisitionHistory selects inputs for an exact replay and returns the payload size of each input.
// The caller replays the selection and keeps the full history when the result differs.
func compactAcquisitionHistory(ctx context.Context, observations []sources.Observation, cited map[string]bool) ([]sources.Observation, map[string]int, error) {
	if ctx == nil {
		return nil, nil, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if len(observations) > maxManualHistoryBatches {
		return nil, nil, &errors.ValidationError{Field: "replay.observations", Message: "exceeds the retained history bound"}
	}
	var providers *manualBatch
	var metadata []manualObservation
	seen := make(map[string]bool, len(observations))
	sizes := make(map[string]int, len(observations))
	for _, input := range observations {
		if seen[input.ID] {
			return nil, nil, &errors.ValidationError{Field: "replay.observations", Message: "contains a duplicate observation identity"}
		}
		seen[input.ID] = true
		prepared, err := prepareManualObservations(ctx, []sources.Observation{input})
		if err != nil {
			return nil, nil, err
		}
		observation := prepared[0]
		sizes[input.ID] = len(observation.Payload)
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
		return nil, nil, err
	}
	selected := make(map[string]bool)
	for _, batch := range manualBatches(compacted) {
		for _, observation := range batch.observations {
			selected[observation.Receipt.Link.ObservationID] = true
		}
	}
	for _, observation := range supersedeReplayMetadata(metadata, cited) {
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
	return retained, sizes, ctx.Err()
}

// supersedeReplayMetadata drops metadata that a later complete observation from the same source supersedes.
// An older observation stays while the generation cites it. Inputs after the latest complete observation stay too.
func supersedeReplayMetadata(history []manualObservation, cited map[string]bool) []manualObservation {
	latest := make(map[sources.ID]int)
	for index, observation := range history {
		link := observation.Receipt.Link
		if link.Completeness == sources.ObservationCompletenessComplete && link.Status == sources.ObservationStatusSucceeded {
			latest[link.Source] = index
		}
	}
	retained := make([]manualObservation, 0, len(history))
	for index, observation := range history {
		link := observation.Receipt.Link
		if last, complete := latest[link.Source]; complete && index < last && !cited[link.ObservationID] {
			continue
		}
		retained = append(retained, observation)
	}
	return retained
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
		a.Link.Completeness == b.Link.Completeness && a.Link.Status == b.Link.Status &&
		repeatableReplayQuality(a) && slices.Equal(a.Issues, b.Issues) && a.Records == b.Records &&
		reflect.DeepEqual(a.ProviderBinding, b.ProviderBinding) && bytes.Equal(left.Payload, right.Payload)
}

func repeatableReplayQuality(receipt sources.ObservationReceipt) bool {
	if receipt.Link.Completeness == sources.ObservationCompletenessComplete && receipt.Link.Status == sources.ObservationStatusSucceeded && len(receipt.Issues) == 0 {
		return true
	}
	if receipt.Link.Completeness != sources.ObservationCompletenessPartial || receipt.Link.Status != sources.ObservationStatusDegraded {
		return false
	}
	report := evidence.RecordQuarantine{Records: receipt.Records}
	for _, issue := range receipt.Issues {
		report.Issues = append(report.Issues, evidence.QuarantinedRecord{Scope: issue.Scope, Code: issue.Code, Subject: issue.Subject})
	}
	return report.Valid()
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
