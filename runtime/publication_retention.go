package runtime

import (
	"bytes"
	"context"
	"maps"
	"slices"
	"time"

	"github.com/agentstation/starmap"
)

// publishInputChanges stages retained inputs before catalog publication.
// Only an accepted catalog can install those inputs into active retention.
func (r *Runtime) publishInputChanges(ctx context.Context, source *sourceLayer, providers []ProviderLayer, epoch uint64) (starmap.CatalogState, error) {
	return r.publishInputs(ctx, source, providers, nil, epoch, nil)
}

func (r *Runtime) publishInputs(ctx context.Context, source *sourceLayer, providers []ProviderLayer, manual []manualObservation, epoch uint64, resets []ObservationReset) (starmap.CatalogState, error) {
	return r.publishInputsWithRemovals(ctx, source, providers, manual, epoch, resets, nil)
}

func (r *Runtime) publishInputsWithRemovals(ctx context.Context, source *sourceLayer, providers []ProviderLayer, manual []manualObservation, epoch uint64, resets []ObservationReset, removal *removalUpdate) (starmap.CatalogState, error) {
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
	if removal != nil {
		if err := candidate.prepareRemovalUpdate(r.State(), removal); err != nil {
			return starmap.CatalogState{}, err
		}
	}
	selected, err := selectProviderEvidence(prepared, candidate.providers)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	manual, err = candidate.prepareManualInputs(ctx, manual, selected, resets)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	if source != nil {
		if err := candidate.validateSourceRemovalTransition(source); err != nil {
			return starmap.CatalogState{}, err
		}
		owned := *source
		owned.Payload = bytes.Clone(source.Payload)
		owned.Chain = slices.Clone(source.Chain)
		if source.Manifest != nil {
			manifest := source.Manifest.Copy()
			owned.Manifest = &manifest
		}
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
	changes := inputChanges{source: source, providers: selected}
	if len(manual) != 0 {
		changes.manual = candidate.manual
	}
	if removal != nil {
		changes.removals = candidate.removals
	}
	record, err = changes.stage(ctx, r.store, record)
	if err != nil {
		return starmap.CatalogState{}, err
	}
	changed := !changes.empty()
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
	// Caller cancellation starts a bounded grace period for retained-input writes.
	finish, cancel := publicationCompletionContext(ctx)
	defer cancel()
	return durable, changes.complete(finish, r.store, record)
}

func publicationCompletionContext(ctx context.Context) (context.Context, context.CancelFunc) {
	finish, cancel := context.WithCancel(context.WithoutCancel(ctx))
	stop := context.AfterFunc(ctx, func() {
		timer := time.NewTimer(closeJoinTimeout)
		defer timer.Stop()
		select {
		case <-timer.C:
			cancel()
		case <-finish.Done():
		}
	})
	return finish, func() {
		stop()
		cancel()
	}
}
