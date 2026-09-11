package runtime

import (
	"context"
	"encoding/json"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// manualObservation retains an original payload and its validated receipt.
// An aggregate provider observation keeps its original provider set and digest.
type manualObservation struct {
	Payload []byte                     `json:"payload"`
	Receipt sources.ObservationReceipt `json:"receipt"`
}

func prepareManualObservations(ctx context.Context, input []sources.Observation) ([]manualObservation, error) {
	if len(input) == 0 {
		return nil, &errors.ValidationError{Field: "manual.observations", Message: "at least one observation is required"}
	}
	owned := make([]manualObservation, 0, len(input))
	seen := make(map[string]bool, len(input))
	for _, observation := range input {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		receipt, err := observation.Receipt()
		if err != nil {
			return nil, err
		}
		if seen[receipt.Link.ObservationID] {
			return nil, &errors.ValidationError{Field: "manual.observations", Message: "duplicate observation identity"}
		}
		seen[receipt.Link.ObservationID] = true
		payload, err := catalogs.EncodeCatalogPayload(observation.Catalog)
		if err != nil {
			return nil, err
		}
		record := manualObservation{Payload: payload, Receipt: receipt}
		if _, err := record.restore(); err != nil {
			return nil, err
		}
		owned = append(owned, record)
	}
	return owned, nil
}

func (o manualObservation) restore() (sources.Observation, error) {
	encoded, err := json.Marshal(o)
	if err != nil {
		return sources.Observation{}, errors.WrapResource("encode", "manual observation", "", err)
	}
	if len(o.Payload) == 0 || len(encoded) > maxLayerBytes {
		return sources.Observation{}, &errors.ValidationError{Field: "manual.observation", Message: "must contain a bounded observation record"}
	}
	catalog, err := catalogs.DecodeSourceObservationPayload(o.Payload)
	if err != nil {
		return sources.Observation{}, err
	}
	return o.Receipt.Restore(catalog)
}

func (p *providerBindingPolicy) permitsManual(observation manualObservation) bool {
	if observation.Receipt.Link.Source != sources.ProvidersID {
		return true
	}
	return p.permits(ProviderLayer{Receipt: observation.Receipt})
}

func (p *providerBindingPolicy) validateManual(observations []manualObservation, publishing bool) error {
	for _, observation := range observations {
		if publishing && !p.permitsManual(observation) {
			return &errors.ConflictError{Resource: "manual provider evidence", Message: "observation has no matching active binding declaration"}
		}
		binding := observation.Receipt.ProviderBinding
		if p == nil || binding == nil {
			continue
		}
		active, exists := p.bindings[binding.ID]
		if exists && active.Revision == binding.Revision && active != *binding {
			return &errors.ConflictError{Resource: "provider binding revision", Message: "active selectors differ from retained evidence without a new revision"}
		}
	}
	return nil
}

// PublishObservations retains caller-supplied observations with their accepted catalog.
// The runtime validates receipts and active bindings before it publishes any input.
// This operation reads no source and joins runtime cancellation and shutdown.
func (r *Runtime) PublishObservations(ctx context.Context, observations ...sources.Observation) (starmap.CatalogState, error) {
	if len(observations) == 0 {
		return starmap.CatalogState{}, &errors.ValidationError{Field: "manual.observations", Message: "at least one observation is required"}
	}
	if r == nil {
		return starmap.CatalogState{}, &errors.ValidationError{Field: "runtime", Message: "is required"}
	}
	if err := r.validateAuthorityPublication(nil, len(observations), nil); err != nil {
		return starmap.CatalogState{}, err
	}
	return r.updateAcquisition(ctx, func(context.Context, ObservationInputs) (ObservationUpdate, error) {
		return ObservationUpdate{Observations: observations}, nil
	}, nil)
}
