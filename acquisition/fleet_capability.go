package acquisition

import (
	"context"

	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
)

// CheckFleetAcquisition resolves required acquisition profiles without listing models.
// Scope equivalence comes from the accepted deployment bindings, not secret comparison.
func (a *Acquirer) CheckFleetAcquisition(ctx context.Context, request runtime.FleetAcquisitionRequirements) error {
	if a == nil {
		return fleetCapabilityError()
	}
	checker, ok := a.observer.(runtime.FleetAcquisitionChecker)
	if !ok {
		return fleetCapabilityError()
	}
	return checker.CheckFleetAcquisition(ctx, request)
}

func (o *providerSourceObserver) CheckFleetAcquisition(ctx context.Context, request runtime.FleetAcquisitionRequirements) error {
	if request.Catalog == nil || o.resolver == nil {
		return fleetCapabilityError()
	}
	for _, id := range request.Providers {
		provider, err := request.Catalog.Provider(id)
		if err != nil {
			return err
		}
		if _, err := o.resolver.ResolveCatalog(ctx, &provider); err != nil {
			return err
		}
	}
	for _, binding := range request.Bindings {
		provider, err := request.Catalog.Provider(binding.ProviderID)
		if err != nil {
			return err
		}
		if err := providers.ValidateBinding(&provider, binding); err != nil {
			return err
		}
		selected := catalogs.DeepCopyProvider(provider)
		selected.Credentials.CatalogAcquisition.Alternatives = []catalogs.ProviderCredentialProfileID{binding.CredentialProfileID}
		material, err := o.resolver.ResolveCatalog(ctx, &selected)
		if err != nil {
			return err
		}
		if material.Profile().ID != binding.CredentialProfileID {
			return fleetCapabilityError()
		}
	}
	return ctx.Err()
}

func fleetCapabilityError() error {
	return &errors.ConfigError{Component: "fleet acquisition", Message: "required acquisition profiles are unavailable"}
}
