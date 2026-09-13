package runtime

import (
	"context"
	"slices"

	"github.com/agentstation/starmap/internal/catalog/reconciler"
	"github.com/agentstation/starmap/pkg/sources"
)

// compactProviderHistory preserves original receipts and witnesses for later replay.
// Reset batches and metadata keep their publication order. Provider replay uses source priority.
func compactProviderHistory(ctx context.Context, history *manualBatch) (*manualBatch, error) {
	batches := manualBatches(history)
	entries, err := providerHistoryEntries(ctx, batches)
	if err != nil {
		return nil, err
	}
	segments := providerHistorySegments(entries)
	required := make(map[manualHistoryPosition]bool)
	for _, segment := range segments {
		segment.protect(required)
	}
	if err := protectProviderChangeWitnesses(ctx, segments, required); err != nil {
		return nil, err
	}
	dropped := make(map[manualHistoryPosition]bool)
	for _, entry := range entries {
		if entry.eligible && !required[entry.position] {
			dropped[entry.position] = true
		}
	}
	if len(dropped) == 0 {
		return history, ctx.Err()
	}
	return retainManualHistory(batches, dropped), ctx.Err()
}

func providerHistoryEntries(ctx context.Context, batches []*manualBatch) ([]providerHistoryEntry, error) {
	var entries []providerHistoryEntry
	epoch := 0
	for batchIndex, batch := range batches {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		boundary := len(batch.resets) != 0
		for _, observation := range batch.observations {
			boundary = boundary || observation.Receipt.Link.Source != sources.ProvidersID
		}
		if boundary {
			epoch++
		}
		for index, observation := range batch.observations {
			if observation.Receipt.Link.Source != sources.ProvidersID {
				continue
			}
			shape, err := retainedProviderShape(ctx, observation)
			if err != nil {
				return nil, err
			}
			entries = append(entries, providerHistoryEntry{position: manualHistoryPosition{batchIndex, index},
				observation: observation, priority: providerHistoryPriority(observation), shape: shape, epoch: epoch, eligible: !boundary && completeProviderInventory(observation)})
		}
		if boundary {
			epoch++
		}
	}
	slices.SortStableFunc(entries, compareProviderHistoryEntries)
	for index := range entries {
		entries[index].order = index
		if (index > 0 && compareProviderHistoryEntries(entries[index-1], entries[index]) == 0) ||
			(index+1 < len(entries) && compareProviderHistoryEntries(entries[index], entries[index+1]) == 0) {
			entries[index].eligible = false
		}
	}
	return entries, ctx.Err()
}

func compareProviderHistoryEntries(left, right providerHistoryEntry) int {
	return reconciler.CompareProviderObservations(left.priority, right.priority)
}

func providerHistoryPriority(observation manualObservation) sources.Observation {
	priority := sources.Observation{ObservedAt: observation.Receipt.Link.ObservedAt, Status: observation.Receipt.Link.Status}
	for _, issue := range observation.Receipt.Issues {
		priority.Issues = append(priority.Issues, sources.ObservationIssue{Scope: issue.Scope, Code: issue.Code, Subject: issue.Subject})
	}
	return priority
}

func retainManualHistory(batches []*manualBatch, dropped map[manualHistoryPosition]bool) *manualBatch {
	var compacted *manualBatch
	join := false
	for batchIndex, batch := range batches {
		retained := make([]manualObservation, 0, len(batch.observations))
		providerOnly := len(batch.resets) == 0
		for index, observation := range batch.observations {
			providerOnly = providerOnly && observation.Receipt.Link.Source == sources.ProvidersID
			if !dropped[manualHistoryPosition{batchIndex, index}] {
				retained = append(retained, observation)
			}
		}
		if len(retained) == 0 && len(batch.resets) == 0 {
			continue
		}
		if providerOnly && join {
			compacted.observations = append(compacted.observations, retained...)
		} else {
			compacted = &manualBatch{parent: compacted, observations: retained, resets: slices.Clone(batch.resets)}
		}
		join = providerOnly
	}
	return compacted
}

func completeProviderInventory(observation manualObservation) bool {
	return observation.Receipt.Link.Completeness == sources.ObservationCompletenessComplete &&
		observation.Receipt.Link.Status == sources.ObservationStatusSucceeded && len(observation.Receipt.Issues) == 0
}
