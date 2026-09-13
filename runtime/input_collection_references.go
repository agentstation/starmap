package runtime

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
)

// selectInputs validates every required reference before selecting any deletion.
func (s *inputCollectionSnapshot) selectInputs(ctx context.Context, store *layerStore, active *manualBatch) error {
	var head manualHistoryHead
	if s.manual != nil {
		if err := decodeInputRecord(s.manual, &head); err != nil {
			return err
		}
		if !supportedManualHistoryVersion(head.Version) || head.Batch == "" {
			return invalidInputPublication("invalid manual history head")
		}
	}
	pending, err := parseInputPublication(s.publication)
	if err != nil {
		return err
	}
	if active != nil && (active.reference == "" || (active.reference != head.Batch && (pending == nil || active.reference != pending.Manual))) {
		return invalidInputPublication("accepted manual history differs from retained input references")
	}
	protected := make(map[string]bool)
	if err := s.protectHistory(ctx, store, head.Batch, protected); err != nil {
		return err
	}
	if pending != nil {
		if err := s.protectPublication(ctx, store, *pending, protected); err != nil {
			return err
		}
	}
	s.report.Protected = slices.Sorted(maps.Keys(protected))
	for _, name := range slices.Sorted(maps.Keys(s.records)) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if protected[name] {
			continue
		}
		if s.recognizedInput(ctx, name) {
			s.report.Candidates = append(s.report.Candidates, name)
		} else {
			s.report.Preserved = append(s.report.Preserved, name)
		}
	}
	slices.Sort(s.report.Preserved)
	return ctx.Err()
}

func (s *inputCollectionSnapshot) protectReference(name string, protected map[string]bool) error {
	if !validInputReference(name) || s.records[name] == nil {
		return invalidInputPublication("required input is missing or unsafe for collection")
	}
	protected[name] = true
	return nil
}

func (s *inputCollectionSnapshot) protectHistory(ctx context.Context, store *layerStore, reference string, protected map[string]bool) error {
	if reference == "" {
		return nil
	}
	if err := s.protectReference(reference, protected); err != nil {
		return err
	}
	// The normal reader owns version, receipt, reset, cycle, and aggregate size checks.
	history, err := store.readManualHistory(ctx, reference)
	if err != nil {
		return err
	}
	for batch := history; batch != nil; batch = batch.parent {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.protectReference(batch.reference, protected); err != nil {
			return err
		}
		var record manualBatchRecord
		if err := decodeInputRecord(s.records[batch.reference], &record); err != nil {
			return err
		}
		// Use recorded filenames. Re-encoding legacy observations can change their hashes.
		for _, observation := range record.Observations {
			if err := s.protectReference(observation, protected); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *inputCollectionSnapshot) protectPublication(ctx context.Context, store *layerStore, pending inputPublication, protected map[string]bool) error {
	if _, _, err := store.publicationInputs(pending); err != nil {
		return err
	}
	if _, err := store.readRemovalInput(pending.Removals); err != nil {
		return err
	}
	for _, name := range append(slices.Clone(pending.Providers), pending.Source, pending.Removals) {
		if name != "" {
			if err := s.protectReference(name, protected); err != nil {
				return err
			}
		}
	}
	return s.protectHistory(ctx, store, pending.Manual, protected)
}

// recognizedInput selects only supported records with valid content and semantics.
// Unknown or damaged orphan records remain available for operator inspection.
func (s *inputCollectionSnapshot) recognizedInput(ctx context.Context, name string) bool {
	raw := s.records[name]
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false
	}
	switch {
	case fields["observations"] != nil || fields["checkpoint"] != nil:
		var record manualBatchRecord
		if err := decodeInputRecord(raw, &record); err != nil {
			return false
		}
		if validateManualBatchShape(record) != nil {
			return false
		}
		if record.Checkpoint != nil {
			_, err := restoreManualCheckpoint(ctx, record.Checkpoint)
			return err == nil
		}
		_, err := prepareObservationResets(record.Resets)
		return err == nil
	case fields["receipt"] != nil:
		var observation manualObservation
		if err := decodeInputRecord(raw, &observation); err != nil {
			return false
		}
		_, err := observation.restore()
		return err == nil
	case fields["ProviderID"] != nil:
		var provider ProviderLayer
		return decodeInputRecord(raw, &provider) == nil && provider.validate() == nil
	case fields["generation_id"] != nil:
		var source sourceLayer
		return decodeInputRecord(raw, &source) == nil && validateSourceInput(&source) == nil
	case fields["policy"] != nil:
		var record removalPolicyRecord
		if err := decodeInputRecord(raw, &record); err != nil {
			return false
		}
		_, err := validateRemovalRecord(record)
		return err == nil
	default:
		return false
	}
}
