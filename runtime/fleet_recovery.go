package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

const fleetRecoveryVersion = 1

// MaxFleetRecoveryBytes bounds the private inputs attached to one fleet publication.
const MaxFleetRecoveryBytes = storage.DefaultRetentionInputMaxBytes

// FleetRecovery binds private acquisition inputs to one immutable generation.
// Stores retain these bytes with the generation and never expose them as public catalog data.
// The runtime owns the encoding. Hosts preserve the bytes without modification.
type FleetRecovery struct {
	GenerationID    string `json:"generation_id"`
	PayloadChecksum string `json:"payload_checksum"`
	Checksum        string `json:"checksum"`
	Data            []byte `json:"data"`
}

// Validate checks the generation binding and the complete input checksum.
// Runtime recovery separately validates the input schema and acquisition policy.
func (r FleetRecovery) Validate(generation catalogs.Generation) error {
	if r.GenerationID == "" || r.GenerationID != generation.Manifest.GenerationID ||
		r.PayloadChecksum == "" || r.PayloadChecksum != generation.Manifest.Payload.Checksum {
		return invalidInputPublication("fleet recovery does not match the selected generation")
	}
	if len(r.Data) == 0 || len(r.Data) > MaxFleetRecoveryBytes || r.Checksum != fleetRecoveryChecksum(r.Data) {
		return invalidInputPublication("fleet recovery has invalid bytes or checksum")
	}
	return nil
}

// fleetRecoveryRecord retains semantic inputs without private filesystem references.
type fleetRecoveryRecord struct {
	Version        int                            `json:"version"`
	PublisherID    string                         `json:"publisher_id"`
	Compatibility  string                         `json:"compatibility"`
	Pin            *generationPinRecord           `json:"pin,omitempty"`
	ReplayChecksum string                         `json:"replay_checksum,omitempty"`
	Source         *sourceLayer                   `json:"source,omitempty"`
	Providers      []ProviderLayer                `json:"providers,omitempty"`
	Manual         *manualCheckpoint              `json:"manual,omitempty"`
	Removals       *catalogs.CatalogRemovalPolicy `json:"removals,omitempty"`
}

func fleetRecoveryChecksum(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func validFleetChecksum(value string) bool {
	digest, err := hex.DecodeString(value)
	return err == nil && len(digest) == sha256.Size && hex.EncodeToString(digest) == value
}

func encodeFleetRecovery(ctx context.Context, layers layerSet) ([]byte, error) {
	return encodeFleetRecoveryWithPin(ctx, layers, nil)
}

func encodeFleetRecoveryWithPin(ctx context.Context, layers layerSet, pin *generationPinRecord) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if layers.publisherID == "" {
		return nil, invalidInputPublication("fleet recovery requires a publisher identity")
	}
	compatibility, err := fleetLayerCompatibility(layers)
	if err != nil {
		return nil, err
	}
	record := fleetRecoveryRecord{Version: fleetRecoveryVersion, PublisherID: layers.publisherID, Compatibility: compatibility,
		Source: layers.source, Removals: layers.removals}
	if pin != nil {
		if err := pin.validate(); err != nil {
			return nil, err
		}
		if pin.Phase != pinAccepted {
			return nil, pinRecordConflict("fleet recovery requires an accepted pin record")
		}
		state, err := layers.build(ctx, layers.embedded)
		if err != nil {
			return nil, err
		}
		record.Pin, record.ReplayChecksum = pin, state.PayloadChecksum
	}
	for _, key := range layers.providerOrder() {
		record.Providers = append(record.Providers, layers.providers[key])
	}
	if layers.manual != nil {
		checkpoint, err := checkpointManualHistory(ctx, layers.manual)
		if err != nil {
			return nil, err
		}
		record.Manual = checkpoint.checkpoint
	}
	data, err := json.Marshal(record, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFleetRecoveryBytes {
		return nil, invalidInputPublication("fleet recovery exceeds the input byte bound")
	}
	return data, nil
}

func decodeFleetRecovery(ctx context.Context, data []byte) (layerSet, error) {
	record, err := readFleetRecovery(ctx, data)
	if err != nil {
		return layerSet{}, err
	}
	return decodeFleetRecoveryRecord(ctx, record)
}

func readFleetRecovery(ctx context.Context, data []byte) (fleetRecoveryRecord, error) {
	var record fleetRecoveryRecord
	if err := ctx.Err(); err != nil {
		return record, err
	}
	if len(data) == 0 || len(data) > MaxFleetRecoveryBytes {
		return record, invalidInputPublication("fleet recovery exceeds the input byte bound")
	}
	if err := json.Unmarshal(data, &record, json.RejectUnknownMembers(true)); err != nil {
		return record, err
	}
	if record.Version != fleetRecoveryVersion || record.PublisherID == "" || !validFleetChecksum(record.Compatibility) {
		return record, invalidInputPublication("fleet recovery has an unsupported version or incomplete identity")
	}
	if record.Pin != nil {
		if err := record.Pin.validate(); err != nil {
			return record, err
		}
		if record.Pin.Phase != pinAccepted || !strings.HasPrefix(record.ReplayChecksum, "sha256:") || !validFleetChecksum(strings.TrimPrefix(record.ReplayChecksum, "sha256:")) {
			return record, pinRecordConflict("fleet pin recovery has an invalid acceptance or replay checksum")
		}
	} else if record.ReplayChecksum != "" {
		return record, pinRecordConflict("an alternate replay checksum requires an accepted pin")
	}
	return record, nil
}

func decodeFleetRecoveryRecord(ctx context.Context, record fleetRecoveryRecord) (layerSet, error) {
	var layers layerSet
	if record.Source != nil {
		if err := validateSourceInput(record.Source); err != nil {
			return layers, err
		}
	}
	layers.publisherID, layers.source = record.PublisherID, record.Source
	layers.providers = make(map[providerEvidenceKey]ProviderLayer, len(record.Providers))
	for _, provider := range record.Providers {
		if err := ctx.Err(); err != nil {
			return layerSet{}, err
		}
		if err := provider.validate(); err != nil {
			return layerSet{}, err
		}
		if _, duplicate := layers.providers[provider.evidenceKey()]; duplicate {
			return layerSet{}, invalidInputPublication("fleet recovery contains a duplicate provider scope")
		}
		layers.setProvider(provider)
	}
	if err := validateProviderScopes(record.Providers, nil); err != nil {
		return layerSet{}, err
	}
	if record.Manual != nil {
		history, err := restoreManualCheckpoint(ctx, record.Manual)
		if err != nil {
			return layerSet{}, err
		}
		layers.manual = history
	}
	if record.Removals != nil {
		policy, err := validateRemovalRecord(removalPolicyRecord{Version: removalPolicyVersion, Policy: *record.Removals})
		if err != nil {
			return layerSet{}, err
		}
		layers.removals = policy
	}
	return layers, nil
}
