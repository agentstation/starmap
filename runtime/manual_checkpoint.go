package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// maxManualCheckpointReferences bounds decoded replay work independently of bytes.
const maxManualCheckpointReferences = 1 << 16

// manualCheckpoint shares payload bytes while retaining ordered original receipts.
// A checkpoint has no parent. Later linked batches can append to it.
type manualCheckpoint struct {
	Payloads     [][]byte                      `json:"payloads"`
	Observations []manualCheckpointObservation `json:"observations"`
	Batches      []manualCheckpointBatch       `json:"batches"`
}

type manualCheckpointObservation struct {
	Payload uint32                     `json:"payload"`
	Receipt sources.ObservationReceipt `json:"receipt"`
}

type manualCheckpointBatch struct {
	Observations []uint32           `json:"observations"`
	Resets       []ObservationReset `json:"resets,omitempty"`
}

func checkpointManualHistory(ctx context.Context, history *manualBatch) (*manualBatch, error) {
	checkpoint := &manualCheckpoint{}
	payloads := make(map[[sha256.Size]byte]uint32)
	observations := make(map[string]uint32)
	originals := make(map[string]manualObservation)
	references := 0
	for _, batch := range manualBatches(history) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		references += len(batch.observations)
		if references > maxManualCheckpointReferences {
			return nil, invalidInputPublication("checkpoint exceeds the replay reference bound")
		}
		record := manualCheckpointBatch{Resets: slices.Clone(batch.resets)}
		for _, observation := range batch.observations {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			id := observation.Receipt.Link.ObservationID
			index, exists := observations[id]
			if exists {
				prior := originals[id]
				if !bytes.Equal(prior.Payload, observation.Payload) || !reflect.DeepEqual(prior.Receipt, observation.Receipt) {
					return nil, invalidInputPublication("observation identity contains different checkpoint evidence")
				}
			} else {
				digest := sha256.Sum256(observation.Payload)
				payload, exists := payloads[digest]
				if !exists {
					payload = uint32(len(checkpoint.Payloads))
					payloads[digest] = payload
					checkpoint.Payloads = append(checkpoint.Payloads, bytes.Clone(observation.Payload))
				}
				index = uint32(len(checkpoint.Observations))
				observations[id], originals[id] = index, observation
				checkpoint.Observations = append(checkpoint.Observations, manualCheckpointObservation{Payload: payload, Receipt: observation.Receipt.Clone()})
			}
			record.Observations = append(record.Observations, index)
		}
		checkpoint.Batches = append(checkpoint.Batches, record)
	}
	return restoreManualCheckpoint(ctx, checkpoint)
}

func restoreManualCheckpoint(ctx context.Context, checkpoint *manualCheckpoint) (*manualBatch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if checkpoint == nil || len(checkpoint.Payloads) == 0 || len(checkpoint.Observations) == 0 || len(checkpoint.Batches) == 0 {
		return nil, invalidInputPublication("checkpoint must contain payloads, observations, and replay batches")
	}
	for _, count := range []int{len(checkpoint.Payloads), len(checkpoint.Observations), len(checkpoint.Batches)} {
		if count > maxManualCheckpointReferences {
			return nil, invalidInputPublication("checkpoint exceeds the replay reference bound")
		}
	}
	referenceCount := 0
	for _, batch := range checkpoint.Batches {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		referenceCount += len(batch.Observations)
		if len(batch.Observations) == 0 || referenceCount > maxManualCheckpointReferences {
			return nil, invalidInputPublication("checkpoint batch exceeds the replay reference bound")
		}
	}
	raw, err := json.Marshal(manualBatchRecord{Version: manualHistoryVersion, Checkpoint: checkpoint})
	if err != nil {
		return nil, err
	}
	if len(raw) > maxLayerBytes {
		return nil, manualHistoryCapacity
	}
	catalogsByPayload := make(map[uint32]*catalogs.Catalog)
	payloadDigests := make(map[[sha256.Size]byte]bool)
	for _, payload := range checkpoint.Payloads {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		digest := sha256.Sum256(payload)
		if len(payload) == 0 || payloadDigests[digest] {
			return nil, invalidInputPublication("checkpoint payloads must be nonempty and unique")
		}
		payloadDigests[digest] = true
	}
	observations := make([]manualObservation, len(checkpoint.Observations))
	restored := make([]sources.Observation, len(checkpoint.Observations))
	identities := make(map[string]bool)
	for index, entry := range checkpoint.Observations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if uint64(entry.Payload) >= uint64(len(checkpoint.Payloads)) || identities[entry.Receipt.Link.ObservationID] {
			return nil, invalidInputPublication("checkpoint observation has an invalid payload or repeated identity")
		}
		identities[entry.Receipt.Link.ObservationID] = true
		catalog, exists := catalogsByPayload[entry.Payload]
		if !exists {
			catalog, err = catalogs.DecodeSourceObservationPayload(checkpoint.Payloads[entry.Payload])
			if err != nil {
				return nil, err
			}
			catalogsByPayload[entry.Payload] = catalog
		}
		restored[index], err = entry.Receipt.Restore(catalog)
		if err != nil {
			return nil, err
		}
		observations[index] = manualObservation{Payload: checkpoint.Payloads[entry.Payload], Receipt: entry.Receipt}
	}
	if len(catalogsByPayload) != len(checkpoint.Payloads) {
		return nil, invalidInputPublication("checkpoint contains an unreferenced payload")
	}
	head := &manualBatch{checkpoint: checkpoint, checkpointBytes: len(raw)}
	used := make(map[uint32]bool)
	references := 0
	for _, record := range checkpoint.Batches {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		references += len(record.Observations)
		if len(record.Observations) == 0 || references > maxManualCheckpointReferences {
			return nil, invalidInputPublication("checkpoint batch exceeds the replay reference bound")
		}
		resets, err := prepareObservationResets(record.Resets)
		if err != nil {
			return nil, err
		}
		batch := &manualBatch{resets: resets}
		seen := make(map[uint32]bool)
		selected := make([]sources.Observation, 0, len(record.Observations))
		for _, index := range record.Observations {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if uint64(index) >= uint64(len(observations)) || seen[index] {
				return nil, invalidInputPublication("checkpoint batch has an invalid or repeated observation")
			}
			seen[index], used[index] = true, true
			batch.observations = append(batch.observations, observations[index])
			selected = append(selected, restored[index])
		}
		if err := validateRestoredObservationReplacement(ctx, resets, selected); err != nil {
			return nil, err
		}
		head.packed = append(head.packed, batch)
	}
	if len(used) != len(observations) {
		return nil, invalidInputPublication("checkpoint contains an unreferenced observation")
	}
	return head, nil
}
