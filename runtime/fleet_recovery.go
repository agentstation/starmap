package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"reflect"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

const fleetRecoveryVersion = 2

// MaxFleetRecoveryBytes bounds both encoded and decoded private inputs for one fleet publication.
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
	Baseline       catalogs.Generation            `json:"baseline,omitzero"`
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

func encodeFleetRecoveryWithPin(ctx context.Context, layers layerSet, pin *generationPinRecord) ([]byte, error) {
	record, err := makeFleetRecoveryRecord(ctx, layers, pin)
	if err != nil {
		return nil, err
	}
	record.Baseline, err = fleetBaseline(layers)
	if err != nil {
		return nil, err
	}
	return encodeFleetRecoveryRecord(record)
}

func encodeFleetRecoveryRecord(record fleetRecoveryRecord) ([]byte, error) {
	data, err := json.Marshal(record, json.Deterministic(true), jsonv1.FormatDurationAsNano(true))
	if err != nil {
		return nil, err
	}
	return compressFleetRecovery(data)
}

// makeFleetRecoveryRecord captures semantic inputs independently of baseline storage.
func makeFleetRecoveryRecord(ctx context.Context, layers layerSet, pin *generationPinRecord) (fleetRecoveryRecord, error) {
	if err := ctx.Err(); err != nil {
		return fleetRecoveryRecord{}, err
	}
	if layers.publisherID == "" {
		return fleetRecoveryRecord{}, invalidInputPublication("fleet recovery requires a publisher identity")
	}
	compatibility, err := fleetLayerCompatibility(layers)
	if err != nil {
		return fleetRecoveryRecord{}, err
	}
	record := fleetRecoveryRecord{Version: fleetRecoveryVersion, PublisherID: layers.publisherID, Compatibility: compatibility,
		Source: layers.source, Removals: layers.removals}
	if pin != nil {
		if err := pin.validate(); err != nil {
			return fleetRecoveryRecord{}, err
		}
		if pin.Phase != pinAccepted {
			return fleetRecoveryRecord{}, pinRecordConflict("fleet recovery requires an accepted pin record")
		}
		state, err := layers.build(ctx, layers.embedded)
		if err != nil {
			return fleetRecoveryRecord{}, err
		}
		record.Pin, record.ReplayChecksum = pin, state.PayloadChecksum
	}
	for _, key := range layers.providerOrder() {
		record.Providers = append(record.Providers, layers.providers[key])
	}
	if layers.manual != nil {
		checkpoint, err := checkpointManualHistory(ctx, layers.manual)
		if err != nil {
			return fleetRecoveryRecord{}, err
		}
		record.Manual = checkpoint.checkpoint
	}
	return record, nil
}

func readFleetRecovery(ctx context.Context, data []byte) (fleetRecoveryRecord, error) {
	var record fleetRecoveryRecord
	if err := ctx.Err(); err != nil {
		return record, err
	}
	decoded, err := decompressFleetRecovery(ctx, data, MaxFleetRecoveryBytes)
	if err != nil {
		return record, err
	}
	if err := json.Unmarshal(decoded, &record, json.RejectUnknownMembers(true), jsonv1.FormatDurationAsNano(true)); err != nil {
		return record, err
	}
	if err := validateFleetRecoveryRecord(record); err != nil {
		return record, err
	}
	return record, nil
}

func validateFleetRecoveryRecord(record fleetRecoveryRecord) error {
	if record.Version != fleetRecoveryVersion || record.PublisherID == "" || !validFleetChecksum(record.Compatibility) {
		return invalidInputPublication("fleet recovery has an unsupported version or incomplete identity")
	}
	if record.Pin != nil {
		if err := record.Pin.validate(); err != nil {
			return err
		}
		if record.Pin.Phase != pinAccepted || !strings.HasPrefix(record.ReplayChecksum, "sha256:") || !validFleetChecksum(strings.TrimPrefix(record.ReplayChecksum, "sha256:")) {
			return pinRecordConflict("fleet pin recovery has an invalid acceptance or replay checksum")
		}
	} else if record.ReplayChecksum != "" {
		return pinRecordConflict("an alternate replay checksum requires an accepted pin")
	}
	return nil
}

func decodeFleetRecoveryRecord(ctx context.Context, record fleetRecoveryRecord, prior layerSet) (layerSet, error) {
	var layers layerSet
	baseline := prior.embedded.Catalog
	if baseline == nil || prior.fleetBaseline == nil || !reflect.DeepEqual(prior.fleetBaseline.Manifest, record.Baseline.Manifest) || !bytes.Equal(prior.fleetBaseline.Payload, record.Baseline.Payload) {
		var err error
		baseline, err = catalogs.DecodeCatalogGeneration(record.Baseline)
		if err != nil {
			return layers, err
		}
	}
	layers.fleetBaseline = &record.Baseline
	manifest := record.Baseline.Manifest.Copy()
	layers.embedded = starmap.CatalogState{Catalog: baseline, GenerationID: manifest.GenerationID,
		PayloadChecksum: manifest.Payload.Checksum, GeneratedAt: manifest.GeneratedAt}
	layers.embeddedManifest = &manifest
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

// fleetBaseline retains the complete reconstruction baseline independently of the binary.
func fleetBaseline(layers layerSet) (catalogs.Generation, error) {
	if layers.embeddedManifest == nil || layers.embedded.Catalog == nil {
		return catalogs.Generation{}, invalidInputPublication("fleet recovery requires the complete baseline")
	}
	manifest := layers.embeddedManifest.Copy()
	if manifest.GenerationID != layers.embedded.GenerationID || manifest.Payload.Checksum != layers.embedded.PayloadChecksum || !manifest.GeneratedAt.Equal(layers.embedded.GeneratedAt) {
		return catalogs.Generation{}, invalidInputPublication("fleet baseline state differs from its manifest")
	}
	if layers.fleetBaseline != nil {
		if !reflect.DeepEqual(manifest, layers.fleetBaseline.Manifest) {
			return catalogs.Generation{}, invalidInputPublication("fleet baseline manifest differs from its retained bytes")
		}
		return *layers.fleetBaseline, nil
	}
	payload, err := catalogs.EncodeCatalogPayload(layers.embedded.Catalog)
	if err != nil {
		return catalogs.Generation{}, err
	}
	generation := catalogs.Generation{Manifest: manifest, Payload: payload}
	if err := generation.Validate(); err != nil {
		return catalogs.Generation{}, err
	}
	return generation, nil
}
