package runtime

import (
	"bytes"
	"encoding/json"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// validate binds a retained provider identity to its complete encoded evidence.
// It does not establish account scope, upstream completeness, or field authority.
func (layer ProviderLayer) validate() error {
	if err := validateProviderLayerID(layer.ProviderID); err != nil {
		return err
	}
	if len(layer.Payload) == 0 || len(layer.Payload) > maxLayerBytes {
		return invalidProviderEvidence("payload", "must contain a bounded provider observation")
	}
	if layer.ObservedAt.IsZero() {
		return invalidProviderEvidence("observed_at", "is required")
	}
	encoded, err := json.Marshal(layer)
	if err != nil {
		return errors.WrapResource("encode", "provider evidence", string(layer.ProviderID), err)
	}
	if len(encoded) > maxLayerBytes {
		return invalidProviderEvidence("record_bytes", "payload and receipt exceed the retained record bound")
	}
	if catalogs.DescribeCatalogPayload(layer.Payload).Checksum != layer.Digest {
		return invalidProviderEvidence("digest", "does not match the observation payload")
	}
	observed, err := catalogs.DecodeSourceObservationPayload(layer.Payload)
	if err != nil {
		return errors.WrapResource("decode", "provider evidence", string(layer.ProviderID), err)
	}
	if observed.Providers().Len() != 1 {
		return invalidProviderEvidence("provider_id", "payload must contain exactly one provider")
	}
	if _, found := observed.Providers().Get(layer.ProviderID); !found {
		return invalidProviderEvidence("provider_id", "does not match the observation payload")
	}
	if layer.Receipt.Link.EvidenceChecksum != layer.Digest || !layer.Receipt.Link.ObservedAt.Equal(layer.ObservedAt) {
		return invalidProviderEvidence("receipt", "must describe the payload digest and observation time")
	}
	if _, err := layer.Receipt.Restore(observed); err != nil {
		return errors.WrapResource("validate", "provider receipt", string(layer.ProviderID), err)
	}
	return nil
}

func invalidProviderEvidence(field, message string) error {
	return &errors.ValidationError{Field: "provider_layer." + field, Message: message}
}

// prepareProviderEvidence validates owned copies before any batch retention.
func prepareProviderEvidence(layers []ProviderLayer) ([]ProviderLayer, error) {
	prepared := make([]ProviderLayer, 0, len(layers))
	for _, layer := range layers {
		if len(layer.Payload) > maxLayerBytes {
			return nil, invalidProviderEvidence("payload", "exceeds the retained observation bound")
		}
		layer.Payload = bytes.Clone(layer.Payload)
		layer.Receipt = layer.Receipt.Clone()
		if err := layer.validate(); err != nil {
			return nil, err
		}
		prepared = append(prepared, layer)
	}
	return prepared, nil
}

// NewProviderLayer encodes one validated provider observation with its durable receipt.
// It does not connect to a network or write files.
func NewProviderLayer(id catalogs.ProviderID, observation sources.Observation) (ProviderLayer, error) {
	receipt, err := observation.Receipt()
	if err != nil {
		return ProviderLayer{}, err
	}
	payload, err := catalogs.EncodeCatalogPayload(observation.Catalog)
	if err != nil {
		return ProviderLayer{}, err
	}
	layer := ProviderLayer{ProviderID: id, Payload: payload, Digest: receipt.Link.EvidenceChecksum, ObservedAt: receipt.Link.ObservedAt, Receipt: receipt}
	if err := layer.validate(); err != nil {
		return ProviderLayer{}, err
	}
	return layer, nil
}
