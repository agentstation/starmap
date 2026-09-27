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
	// Catalog contains reconciled provider metadata and need not match the serving catalog.
	Catalog *catalogs.Catalog
	// Providers contains retained scopes whose acquisition credentials remain required.
	Providers []catalogs.ProviderID
	// Candidates contains unobserved providers that can lack configured credentials.
	Candidates []catalogs.ProviderID
	Bindings   []sources.ProviderAcquisitionBinding
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
	catalog := layers.fleetAcquisitionCatalog
	if catalog == nil {
		catalog = r.client.CurrentCatalogState().Catalog
	}
	request := FleetAcquisitionRequirements{Catalog: catalog}
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
			if r.config.acquirer != nil && request.Catalog != nil {
				for _, provider := range request.Catalog.Providers().List() {
					if !slices.Contains(request.Providers, provider.ID) {
						request.Candidates = append(request.Candidates, provider.ID)
					}
				}
			}
		}
	}
	slices.Sort(request.Providers)
	slices.Sort(request.Candidates)
	var err error
	if len(request.Providers)+len(request.Candidates)+len(request.Bindings) > 0 {
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

// retainFleetAcquisitionCatalog keeps credential metadata independent of serving pins and removals.
func (l *layerSet) retainFleetAcquisitionCatalog(ctx context.Context, catalog *catalogs.Catalog) error {
	if l.fleetBaseline == nil {
		return nil
	}
	builder := catalogs.NewEmpty()
	for _, author := range catalog.Authors().List() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := builder.SetAuthor(author); err != nil {
			return err
		}
	}
	for _, provider := range catalog.Providers().List() {
		if err := ctx.Err(); err != nil {
			return err
		}
		provider.Models = nil
		if err := builder.SetProvider(provider); err != nil {
			return err
		}
	}
	metadata, err := builder.Build()
	if err != nil {
		return err
	}
	l.fleetAcquisitionCatalog = metadata
	return nil
}
