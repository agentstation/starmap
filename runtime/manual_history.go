package runtime

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	manualHistoryName          = "manual.json"
	manualHistoryVersion       = 2
	manualHistoryLegacyVersion = 1
	// Histories remain bounded until compaction replaces superseded evidence.
	maxManualHistoryBatches = 4096
)

// manualBatch is an immutable accepted-input node. Parents precede their children.
type manualBatch struct {
	reference    string
	parent       *manualBatch
	observations []manualObservation
	resets       []ProviderObservationReset
}

type manualBatchRecord struct {
	Version      int                        `json:"version"`
	Parent       string                     `json:"parent,omitempty"`
	Observations []string                   `json:"observations"`
	Resets       []ProviderObservationReset `json:"resets,omitempty"`
}

type manualHistoryHead struct {
	Version int    `json:"version"`
	Batch   string `json:"batch"`
}

func (s *layerStore) stageManualBatch(ctx context.Context, batch *manualBatch) (string, error) {
	record := manualBatchRecord{Version: manualHistoryVersion, Resets: batch.resets}
	if batch.parent != nil {
		if batch.parent.reference == "" {
			reference, err := s.stageManualBatch(ctx, batch.parent)
			if err != nil {
				return "", err
			}
			batch.parent.reference = reference
		}
		record.Parent = batch.parent.reference
	}
	for _, observation := range batch.observations {
		name, err := s.stageInput(ctx, observation)
		if err != nil {
			return "", err
		}
		record.Observations = append(record.Observations, name)
	}
	return s.stageInput(ctx, record)
}

func (s *layerStore) saveManualHead(ctx context.Context, reference string) error {
	if !s.durable() {
		return ctx.Err()
	}
	return s.writeContext(ctx, s.directory, manualHistoryName, manualHistoryHead{Version: manualHistoryVersion, Batch: reference})
}

func (s *layerStore) loadManualHistory(ctx context.Context) (*manualBatch, error) {
	if !s.durable() {
		return nil, nil
	}
	raw, err := readLayerFile(s.directory, manualHistoryName)
	if err != nil || raw == nil {
		return nil, err
	}
	var head manualHistoryHead
	if err := decodeInputRecord(raw, &head); err != nil {
		return nil, err
	}
	if (head.Version != manualHistoryVersion && head.Version != manualHistoryLegacyVersion) || head.Batch == "" {
		return nil, invalidInputPublication("invalid manual history head")
	}
	return s.readManualHistory(ctx, head.Batch)
}

func (s *layerStore) readManualHistory(ctx context.Context, reference string) (*manualBatch, error) {
	directory, err := s.directory.ExistingChild(inputPublicationDirectory)
	if err != nil {
		return nil, err
	}
	var newest, child *manualBatch
	seen := make(map[string]bool)
	bytesRead := 0
	for reference != "" {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seen[reference] {
			return nil, invalidInputPublication("manual history contains a repeated batch")
		}
		seen[reference] = true
		if len(seen) > maxManualHistoryBatches {
			return nil, invalidInputPublication("manual history exceeds the batch bound")
		}
		batch, parent, err := s.readManualBatch(ctx, directory, reference, &bytesRead)
		if err != nil {
			return nil, err
		}
		if child == nil {
			newest = batch
		} else {
			child.parent = batch
		}
		child, reference = batch, parent
	}
	return newest, nil
}

func selectManualObservations(ctx context.Context, history *manualBatch, input []manualObservation, resets []ProviderObservationReset) ([]manualObservation, error) {
	if len(input) == 0 {
		return nil, nil
	}
	incomingBytes, err := providerResetBytes(resets)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	bytesRetained, batches := incomingBytes, 0
	for batch := history; batch != nil; batch = batch.parent {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		batches++
		size, err := providerResetBytes(batch.resets)
		if err != nil {
			return nil, err
		}
		bytesRetained += size
		for _, observation := range batch.observations {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			seen[observation.Receipt.Link.ObservationID] = true
			encoded, err := json.Marshal(observation)
			if err != nil {
				return nil, err
			}
			bytesRetained += len(encoded)
		}
	}
	selected := make([]manualObservation, 0, len(input))
	incomingSeen := make(map[string]bool)
	for _, observation := range input {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if incomingSeen[observation.Receipt.Link.ObservationID] || (len(resets) == 0 && seen[observation.Receipt.Link.ObservationID]) {
			continue
		}
		incomingSeen[observation.Receipt.Link.ObservationID] = true
		encoded, err := json.Marshal(observation)
		if err != nil {
			return nil, err
		}
		bytesRetained += len(encoded)
		selected = append(selected, observation)
	}
	if len(selected) != 0 && (batches >= maxManualHistoryBatches || bytesRetained > maxLayerBytes) {
		return nil, invalidInputPublication("manual history requires compaction before another batch")
	}
	return selected, nil
}

func (s *layerStore) readManualBatch(ctx context.Context, directory *privatefiles.Directory, reference string, bytesRead *int) (*manualBatch, string, error) {
	var record manualBatchRecord
	if err := s.readInput(directory, reference, &record); err != nil {
		return nil, "", err
	}
	if (record.Version != manualHistoryVersion && record.Version != manualHistoryLegacyVersion) || len(record.Observations) == 0 || (record.Version == manualHistoryLegacyVersion && len(record.Resets) > 0) {
		return nil, "", invalidInputPublication("invalid manual observation batch")
	}
	resets, err := prepareProviderResets(record.Resets)
	if err != nil {
		return nil, "", err
	}
	size, err := providerResetBytes(resets)
	if err != nil {
		return nil, "", err
	}
	*bytesRead += size
	if *bytesRead > maxLayerBytes {
		return nil, "", invalidInputPublication("manual history exceeds the retained byte bound")
	}
	batch := &manualBatch{reference: reference, resets: resets}
	seen := make(map[string]bool, len(record.Observations))
	for _, reference := range record.Observations {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		if seen[reference] {
			return nil, "", invalidInputPublication("manual batch contains duplicate observations")
		}
		seen[reference] = true
		var observation manualObservation
		if err := s.readInput(directory, reference, &observation); err != nil {
			return nil, "", err
		}
		encoded, err := json.Marshal(observation)
		if err != nil {
			return nil, "", err
		}
		*bytesRead += len(encoded)
		if *bytesRead > maxLayerBytes {
			return nil, "", invalidInputPublication("manual history exceeds the observation byte bound")
		}
		if _, err := observation.restore(); err != nil {
			return nil, "", err
		}
		batch.observations = append(batch.observations, observation)
	}
	if err := validateProviderReplacement(ctx, batch.resets, batch.observations); err != nil {
		return nil, "", err
	}
	return batch, record.Parent, nil
}

func manualBatches(history *manualBatch) []*manualBatch {
	var batches []*manualBatch
	for batch := history; batch != nil; batch = batch.parent {
		batches = append(batches, batch)
	}
	slices.Reverse(batches)
	return batches
}

func validateManualHistory(history *manualBatch, policy *providerBindingPolicy) error {
	for batch := history; batch != nil; batch = batch.parent {
		if err := policy.validateManual(batch.observations, false); err != nil {
			return errors.WrapResource("validate", "manual history", batch.reference, err)
		}
	}
	return nil
}

// manualInputs includes provider evidence that predates or follows manual publication.
func (l *layerSet) manualInputs(input []manualObservation, providers []ProviderLayer) []manualObservation {
	if len(input) != 0 && l.manual == nil {
		input = append(l.manualProviderAnchors(), input...)
	}
	if l.manual != nil || len(input) != 0 {
		for _, layer := range providers {
			input = append(input, manualObservation{Payload: layer.Payload, Receipt: layer.Receipt})
		}
	}
	return input
}

func (l *layerSet) manualProviderAnchors() []manualObservation {
	anchors := make([]manualObservation, 0, len(l.providers))
	for _, key := range l.activeProviderOrder() {
		layer := l.providers[key]
		anchors = append(anchors, manualObservation{Payload: layer.Payload, Receipt: layer.Receipt})
	}
	return anchors
}

// prepareManualInputs anchors prior provider files before the first reset batch.
func (l *layerSet) prepareManualInputs(ctx context.Context, input []manualObservation, providers []ProviderLayer, resets []ProviderObservationReset) ([]manualObservation, error) {
	if len(resets) > 0 && l.manual == nil {
		anchors := l.manualProviderAnchors()
		if len(anchors) > 0 {
			l.manual = &manualBatch{observations: anchors}
		}
	}
	return selectManualObservations(ctx, l.manual, l.manualInputs(input, providers), resets)
}
