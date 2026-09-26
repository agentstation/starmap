package runtime

import (
	"context"
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// FleetAcquisitionRequirements identifies the acquisition access a refresh owner must retain.
// Bindings declare scopes. Unbound providers retain the deployment's implicit acquisition policy.
// No credential value or credential digest belongs in this record.
type FleetAcquisitionRequirements struct {
	Catalog   *catalogs.Catalog
	Providers []catalogs.ProviderID
	Bindings  []sources.ProviderAcquisitionBinding
}

// FleetAcquisitionChecker verifies access before grant acquisition or renewal.
// It must use acquisition credentials and must not fetch provider inventories.
type FleetAcquisitionChecker interface {
	CheckFleetAcquisition(context.Context, FleetAcquisitionRequirements) error
}

func (r *Runtime) checkFleetAcquisition(ctx context.Context) error {
	r.mu.RLock()
	layers := r.layers
	replayErr := r.fleetReplayError
	r.mu.RUnlock()
	if replayErr != nil {
		return replayErr
	}
	request := FleetAcquisitionRequirements{Catalog: r.client.CurrentCatalogState().Catalog}
	if layers.acquisitionSources.permits(sources.ProvidersID) {
		if layers.providerBindings != nil {
			request.Bindings, _ = layers.providerBindings.selected(nil)
		} else {
			for _, key := range layers.activeProviderOrder() {
				id := layers.providers[key].ProviderID
				if !slices.Contains(request.Providers, id) {
					request.Providers = append(request.Providers, id)
				}
			}
		}
	}
	var err error
	if len(request.Providers)+len(request.Bindings) > 0 {
		checker, ok := r.config.acquirer.(FleetAcquisitionChecker)
		if !ok {
			err = fleetConflict("refresh ownership requires an acquisition capability checker")
		} else {
			bounded, cancel := context.WithTimeout(ctx, LeaseRenewInterval)
			err = checker.CheckFleetAcquisition(bounded, request)
			cancel()
			if err != nil {
				err = fleetConflict("required catalog acquisition access is unavailable")
			}
		}
	}
	r.mu.Lock()
	r.fleetCapabilityError = err
	r.mu.Unlock()
	return err
}
