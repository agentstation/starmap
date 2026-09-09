package runtime

import (
	"bytes"
	"context"
	"maps"
	"slices"

	"github.com/agentstation/starmap"
)

// publishInputChanges stages retained inputs before catalog publication.
// Only an accepted catalog can install those inputs into active retention.
func (r *Runtime) publishInputChanges(ctx context.Context, source *sourceLayer, providers []ProviderLayer, epoch uint64) (starmap.CatalogState, error) {
	return r.publishInputs(ctx, source, providers, nil, epoch, nil)
}

func (r *Runtime) publishInputs(ctx context.Context, source *sourceLayer, providers []ProviderLayer, manual []manualObservation, epoch uint64, resets []ObservationReset) (starmap.CatalogState, error) {
	manualRequested := len(manual) != 0
	if err := ctx.Err(); err != nil {
		return starmap.CatalogState{}, err
	}
	prepared, err := prepareProviderEvidence(providers)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	if err := r.config.validateAcquisitionPublication(prepared, manual); err != nil {
		return starmap.CatalogState{}, err
	}
	if err := validateObservationReplacement(ctx, resets, manual); err != nil {
		return starmap.CatalogState{}, err
	}
	r.publicationMu.Lock()
	defer r.publicationMu.Unlock()
	r.providerRetentionMu.Lock()
	defer r.providerRetentionMu.Unlock()
	if err := r.store.refuseInputPublication(); err != nil {
		return starmap.CatalogState{}, err
	}
	if err := r.lease.fence(epoch); err != nil {
		return starmap.CatalogState{}, err
	}
	r.mu.RLock()
	candidate := r.layers
	candidate.providers = maps.Clone(r.layers.providers)
	r.mu.RUnlock()
	selected, err := selectProviderEvidence(prepared, candidate.providers)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	manual, err = candidate.prepareManualInputs(ctx, manual, selected, resets)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	if source != nil {
		owned := *source
		owned.Payload = bytes.Clone(source.Payload)
		owned.Chain = slices.Clone(source.Chain)
		source = &owned
		candidate.source = source
	}
	for _, layer := range selected {
		candidate.setProvider(layer)
	}
	if manualRequested && len(manual) == 0 && source == nil && len(selected) == 0 {
		return r.State(), nil
	}
	if len(manual) != 0 {
		candidate.manual = &manualBatch{parent: candidate.manual, observations: manual, resets: resets}
	}
	state, err := candidate.build(ctx, candidate.embedded)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	current := r.client.CurrentCatalogState()
	record := inputPublication{Version: inputPublicationVersion, Phase: inputPublicationPrepared, ExpectedID: current.GenerationID, ExpectedChecksum: current.PayloadChecksum, GenerationID: state.GenerationID, PayloadChecksum: state.PayloadChecksum}
	if source != nil {
		record.Source, err = r.store.stageInput(ctx, source)
		if err != nil {
			return starmap.CatalogState{}, err
		}
	}
	for _, layer := range selected {
		name, err := r.store.stageInput(ctx, layer)
		if err != nil {
			return starmap.CatalogState{}, err
		}
		record.Providers = append(record.Providers, name)
	}
	if len(manual) != 0 {
		record.Manual, err = r.store.stageManualBatch(ctx, candidate.manual)
		if err != nil {
			return starmap.CatalogState{}, err
		}
		candidate.manual.reference = record.Manual
	}
	changed := source != nil || len(selected) > 0 || len(manual) != 0
	if changed {
		if err := r.store.writeInputPublication(ctx, record); err != nil {
			return starmap.CatalogState{}, err
		}
	}
	durable, err := r.commit(ctx, state, epoch, candidate.buildEvidence)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	r.mu.Lock()
	r.layers = candidate
	r.effective = durable
	r.mu.Unlock()
	r.broadcast(durable)
	if !changed {
		return durable, nil
	}
	// Catalog acceptance cannot roll back when the caller cancels afterward.
	// A bounded completion attempt leaves recovery evidence on any storage failure.
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeJoinTimeout)
	defer cancel()
	return durable, r.store.completeInputPublication(finish, record, source, selected)
}
