package runtime

import (
	"context"
	"crypto/sha256"

	"github.com/agentstation/starmap/pkg/sources"
)

// repeatedProviderInventory binds identical payloads to their original scope.
type repeatedProviderInventory struct {
	payload [sha256.Size]byte
	binding sources.ProviderAcquisitionBinding
}

// compactRepeatedProviderHistory retains original observations for every distinct
// inventory. It removes intermediate successful copies only within an identical scope.
// Metadata and reset histories retain their replay boundaries.
func compactRepeatedProviderHistory(ctx context.Context, history *manualBatch) (*manualBatch, error) {
	var observations []manualObservation
	latest := make(map[repeatedProviderInventory]manualObservation)
	first := make(map[repeatedProviderInventory]manualObservation)
	for _, batch := range manualBatches(history) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(batch.resets) != 0 {
			return history, nil
		}
		for _, observation := range batch.observations {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if observation.Receipt.Link.Source != sources.ProvidersID {
				return history, nil
			}
			observations = append(observations, observation)
			if !completeProviderInventory(observation) {
				continue
			}
			key := repeatedInventoryKey(observation)
			previous, exists := latest[key]
			if !exists || observation.Receipt.Link.ObservedAt.After(previous.Receipt.Link.ObservedAt) {
				latest[key] = observation
			}
			previous, exists = first[key]
			if !exists || observation.Receipt.Link.ObservedAt.Before(previous.Receipt.Link.ObservedAt) {
				first[key] = observation
			}
		}
	}
	retained := make([]manualObservation, 0, len(observations))
	seen := make(map[string]bool)
	for _, observation := range observations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seen[observation.Receipt.Link.ObservationID] {
			continue
		}
		if completeProviderInventory(observation) {
			key := repeatedInventoryKey(observation)
			newest, oldest := latest[key], first[key]
			if newest.Receipt.Link.ObservedAt.After(observation.Receipt.Link.ObservedAt) && oldest.Receipt.Link.ObservedAt.Before(observation.Receipt.Link.ObservedAt) {
				continue
			}
		}
		seen[observation.Receipt.Link.ObservationID] = true
		retained = append(retained, observation)
	}
	if len(retained) == 0 {
		return history, nil
	}
	return &manualBatch{observations: retained}, nil
}

func completeProviderInventory(observation manualObservation) bool {
	return observation.Receipt.Link.Completeness == sources.ObservationCompletenessComplete &&
		observation.Receipt.Link.Status == sources.ObservationStatusSucceeded && len(observation.Receipt.Issues) == 0
}

func repeatedInventoryKey(observation manualObservation) repeatedProviderInventory {
	key := repeatedProviderInventory{payload: sha256.Sum256(observation.Payload)}
	if observation.Receipt.ProviderBinding != nil {
		key.binding = *observation.Receipt.ProviderBinding
	}
	return key
}
