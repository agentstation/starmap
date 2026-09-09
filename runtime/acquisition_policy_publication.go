package runtime

import (
	"context"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// acquisitionPolicyClientOptions gives an explicit policy a writable memory default.
// A caller-supplied store takes precedence over this process-local store.
func (o options) acquisitionPolicyClientOptions() []starmap.Option {
	if o.providerBindings == nil && o.acquisitionSources == nil {
		return o.client
	}
	return append([]starmap.Option{starmap.WithCatalogStore(storage.NewMemory())}, o.client...)
}

// publishAcquisitionPolicyStartup applies declarations and removal of prior scoped evidence.
// It aligns the client and runtime before either can serve.
func (r *Runtime) publishAcquisitionPolicyStartup(ctx context.Context) error {
	if r.config.providerBindings == nil && r.config.acquisitionSources == nil && !storedProviderPolicyRequired(r.client.CurrentCatalogState()) {
		return nil
	}
	r.mu.RLock()
	state := r.effective
	evidence := r.layers.buildEvidence
	r.mu.RUnlock()
	current := r.client.CurrentCatalogState()
	if current.GenerationID == state.GenerationID && current.PayloadChecksum == state.PayloadChecksum {
		return nil
	}
	committed, err := r.commit(ctx, state, r.lease.epoch(), evidence)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.effective = committed
	r.mu.Unlock()
	return nil
}

// restoreAcquisitionPolicyGeneration reuses immutable bytes only for the selected identity.
// A prior policy can return only when the current declarations select it again.
func (r *Runtime) restoreAcquisitionPolicyGeneration(ctx context.Context, state starmap.CatalogState, epoch uint64) (bool, error) {
	if (r.config.providerBindings == nil && r.config.acquisitionSources == nil && !storedProviderPolicyRequired(r.client.CurrentCatalogState())) || state.GenerationID == "" || state.GenerationID == r.client.CurrentGenerationID() {
		return false, nil
	}
	generation, err := r.client.Generation(ctx, state.GenerationID)
	if errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if generation.Manifest.GenerationID != state.GenerationID || generation.Manifest.Payload.Checksum != state.PayloadChecksum {
		return false, &errors.ConflictError{Resource: "acquisition policy generation", Message: "retained generation does not match the selected identity and bytes"}
	}
	if err := r.lease.fence(epoch); err != nil {
		return false, err
	}
	if _, err := r.client.Activate(ctx, generation); err != nil {
		return false, err
	}
	return true, nil
}

// validateAcquisitionPublication checks source and binding policy before retention.
func (o options) validateAcquisitionPublication(providers []ProviderLayer, manual []manualObservation) error {
	if len(providers) != 0 && !o.acquisitionSources.permits(sources.ProvidersID) {
		return &errors.ConflictError{Resource: "acquisition source", Message: "provider observations are excluded"}
	}
	if err := o.acquisitionSources.validateManual(manual); err != nil {
		return err
	}
	if err := o.providerBindings.validatePublication(providers); err != nil {
		return err
	}
	return o.providerBindings.validateManual(manual, true)
}
