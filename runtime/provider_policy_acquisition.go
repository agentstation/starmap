package runtime

import (
	"context"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// BindingAcquirer observes only the supplied active declarations.
// It must preserve each declaration from credential selection through its receipt.
// Implementations also implement Acquirer for runtime injection.
type BindingAcquirer interface {
	AcquireProviderBindings(context.Context, AcquisitionRequest, []sources.ProviderAcquisitionBinding) (AcquisitionResult, error)
}

func (p *providerBindingPolicy) selected(providers []catalogs.ProviderID) ([]sources.ProviderAcquisitionBinding, error) {
	selected := make([]sources.ProviderAcquisitionBinding, 0, len(p.bindings))
	matched := make(map[catalogs.ProviderID]bool, len(providers))
	for _, binding := range p.bindings {
		if len(providers) == 0 || slices.Contains(providers, binding.ProviderID) {
			selected = append(selected, binding)
			matched[binding.ProviderID] = true
		}
	}
	for _, provider := range providers {
		if !matched[provider] {
			return nil, &errors.ValidationError{Field: "provider_bindings.providers", Message: "requested provider has no active binding"}
		}
	}
	slices.SortFunc(selected, func(left, right sources.ProviderAcquisitionBinding) int { return strings.Compare(left.ID, right.ID) })
	return selected, nil
}

// acquireSelectedProviders prevents an explicit policy from invoking an unscoped role.
func (r *Runtime) acquireSelectedProviders(ctx context.Context, request AcquisitionRequest) (AcquisitionResult, error) {
	if r.config.providerBindings == nil {
		return r.config.acquirer.AcquireProviders(ctx, request)
	}
	bindings, err := r.config.providerBindings.selected(request.Providers)
	if err != nil {
		return AcquisitionResult{}, err
	}
	if len(bindings) == 0 {
		return AcquisitionResult{}, nil
	}
	acquirer, ok := r.config.acquirer.(BindingAcquirer)
	if !ok {
		return AcquisitionResult{}, &errors.ConfigError{Component: "binding acquisition", Message: "acquirer must support active provider bindings"}
	}
	return acquirer.AcquireProviderBindings(ctx, request, bindings)
}
