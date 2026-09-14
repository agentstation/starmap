package publication

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/sources"
	catalogruntime "github.com/agentstation/starmap/runtime"
)

// MaxPublicationStateBytes bounds a private publisher checkpoint, including retained source payloads.
const MaxPublicationStateBytes = 256 << 20

const publicationStateVersion = 1

type retainedInput struct {
	Receipt sources.ObservationReceipt `json:"receipt"`
	Payload []byte                     `json:"payload"`
}

type publicationStateRecord struct {
	Version     int                 `json:"version"`
	PublisherID string              `json:"publisher_id"`
	Baseline    catalogs.Generation `json:"baseline"`
	Archive     []byte              `json:"archive"`
	Statement   []byte              `json:"statement"`
	Inputs      []retainedInput     `json:"inputs"`
}

// EncodeState returns a private checkpoint and its exact digest.
// It excludes credential values and diagnostic messages. Binding selectors remain private.
func EncodeState(state *State) (ReceiptRecord, error) {
	if state == nil {
		return ReceiptRecord{}, admissionError("state", "is required")
	}
	record := publicationStateRecord{Version: publicationStateVersion, PublisherID: state.publisherID, Baseline: state.baseline.Copy()}
	size := len(record.Baseline.Payload)
	if state.bundle != nil {
		record.Archive = state.bundle.Data
		record.Statement = state.bundle.Attestation
		size += len(record.Archive) + len(record.Statement)
	}
	for _, observation := range state.history {
		receipt, err := observation.Receipt()
		if err != nil {
			return ReceiptRecord{}, err
		}
		payload, err := catalogs.EncodeCatalogPayload(observation.Catalog)
		if err != nil {
			return ReceiptRecord{}, err
		}
		size += len(payload)
		if size > MaxPublicationStateBytes/2 {
			return ReceiptRecord{}, admissionError("state", "exceeds the checkpoint input bound")
		}
		record.Inputs = append(record.Inputs, retainedInput{Receipt: receipt, Payload: payload})
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return ReceiptRecord{}, err
	}
	if len(raw) > MaxPublicationStateBytes {
		return ReceiptRecord{}, admissionError("state", "exceeds the checkpoint byte bound")
	}
	return ReceiptRecord{Data: raw, Checksum: receiptChecksum(raw)}, nil
}

// RestoreState verifies a private checkpoint against a separately trusted digest.
// The digest must come from accepted publisher state or verified publisher provenance, never from the supplied bytes.
// Original observation identities, payloads, and binding selectors must survive validation and replay.
func RestoreState(ctx context.Context, data []byte, expectedChecksum string) (*State, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > MaxPublicationStateBytes || receiptChecksum(data) != expectedChecksum {
		return nil, admissionError("state.checksum", "does not match the trusted checkpoint")
	}
	var wire struct {
		Version     int                 `json:"version"`
		PublisherID string              `json:"publisher_id"`
		Baseline    catalogs.Generation `json:"baseline"`
		Archive     []byte              `json:"archive"`
		Statement   []byte              `json:"statement"`
		Inputs      json.RawMessage     `json:"inputs"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return nil, admissionError("state", "contains an invalid checkpoint")
	}
	if wire.Version != publicationStateVersion {
		return nil, admissionError("state.version", "is unsupported")
	}
	state, err := NewState(wire.Baseline, wire.PublisherID)
	if err != nil {
		return nil, err
	}
	state.history, err = restoreStateInputs(ctx, wire.Inputs)
	if err != nil {
		return nil, err
	}
	if len(wire.Archive) != 0 || len(wire.Statement) != 0 {
		current, err := artifact.Open(wire.Archive, wire.Statement)
		if err != nil {
			return nil, err
		}
		if _, err := catalogs.DecodeCatalogGeneration(current); err != nil {
			return nil, err
		}
		state.current = current
		bundle, err := artifact.Build(current)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(bundle.Data, wire.Archive) || !bytes.Equal(bundle.Attestation, wire.Statement) {
			return nil, admissionError("state.artifact", "must preserve canonical artifact bytes")
		}
		state.bundle = &bundle
	} else if len(state.history) != 0 {
		return nil, admissionError("state.artifact", "accepted observations require their catalog artifact")
	}
	if err := verifyRestoredState(ctx, state); err != nil {
		return nil, err
	}
	canonical, err := EncodeState(state)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical.Data) {
		return nil, admissionError("state", "must use canonical checkpoint encoding")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return state, nil
}

func restoreStateInputs(ctx context.Context, raw json.RawMessage) ([]sources.Observation, error) {
	if bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return nil, admissionError("state.inputs", "must contain an input array")
	}
	var observations []sources.Observation
	for decoder.More() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(observations) >= maxScopes {
			return nil, admissionError("state.inputs", "exceeds the retained observation bound")
		}
		var input retainedInput
		if err := decoder.Decode(&input); err != nil {
			return nil, admissionError("state.inputs", "contains an invalid retained input")
		}
		catalog, err := catalogs.DecodeSourceObservationPayload(input.Payload)
		if err != nil {
			return nil, err
		}
		observation, err := input.Receipt.Restore(catalog)
		if err != nil {
			return nil, err
		}
		observations = append(observations, observation)
	}
	if _, err := decoder.Token(); err != nil {
		return nil, admissionError("state.inputs", "contains an incomplete array")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, admissionError("state.inputs", "contains trailing input")
	}
	return observations, nil
}

func verifyRestoredState(ctx context.Context, state *State) error {
	if state.bundle == nil {
		return nil
	}
	var bindings []sources.ProviderAcquisitionBinding
	seen := make(map[string]sources.ProviderAcquisitionBinding)
	for _, observation := range state.history {
		if observation.ProviderBinding == nil {
			continue
		}
		binding := *observation.ProviderBinding
		if prior, found := seen[binding.ID]; found {
			if prior != binding {
				return admissionError("state.binding", "contains conflicting active declarations")
			}
			continue
		}
		seen[binding.ID] = binding
		bindings = append(bindings, binding)
	}
	candidate, err := catalogruntime.ReplayAcquisition(ctx, state.baseline, state.publisherID, bindings, state.history)
	if err != nil {
		return err
	}
	rebuilt, err := candidate.Generation(state.current.Manifest.SyncRunID, state.current.Manifest.GeneratedAt)
	if err != nil {
		return err
	}
	expected, err := state.current.SemanticChecksum()
	if err != nil {
		return err
	}
	actual, err := rebuilt.SemanticChecksum()
	if err != nil {
		return err
	}
	if expected != actual {
		return admissionError("state.catalog", "retained inputs do not reproduce the accepted catalog")
	}
	return nil
}
