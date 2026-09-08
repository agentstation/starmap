package acquisition

import (
	"context"
	"slices"
	"strings"

	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

var _ runtime.BindingAcquirer = (*Acquirer)(nil)

// AcquireProviderBindings observes only the supplied declarations through their selected profiles.
// It checks the complete selection before credential resolution or provider I/O.
// Empty bindings select nothing. Request providers can restrict this explicit set.
func (a *Acquirer) AcquireProviderBindings(ctx context.Context, request runtime.AcquisitionRequest, bindings []sources.ProviderAcquisitionBinding) (runtime.AcquisitionResult, error) {
	if a == nil || a.observer == nil {
		return runtime.AcquisitionResult{}, &errors.ValidationError{Field: "acquisition.acquirer", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return runtime.AcquisitionResult{}, err
	}
	targets, err := bindingTargets(request, bindings)
	if err != nil {
		return runtime.AcquisitionResult{}, err
	}
	if len(targets) == 0 {
		return runtime.AcquisitionResult{}, nil
	}
	if _, ok := a.observer.(ProviderBindingObserver); !ok {
		return runtime.AcquisitionResult{}, &errors.ValidationError{Field: "acquisition.provider_observer", Message: "must support declared acquisition bindings"}
	}
	return a.acquireTargets(ctx, request, targets)
}

func bindingTargets(request runtime.AcquisitionRequest, bindings []sources.ProviderAcquisitionBinding) ([]providerAttemptTarget, error) {
	selected := make([]providerAttemptTarget, 0, len(bindings))
	seen := make(map[string]bool, len(bindings))
	for _, binding := range bindings {
		if err := binding.Validate(); err != nil {
			return nil, err
		}
		if seen[binding.ID] {
			return nil, &errors.ValidationError{Field: "acquisition.provider_bindings", Message: "each binding identity must have exactly one active declaration"}
		}
		seen[binding.ID] = true
		if len(request.Providers) > 0 && !slices.Contains(request.Providers, binding.ProviderID) {
			continue
		}
		if request.Current == nil {
			return nil, &errors.ValidationError{Field: "acquisition.current_catalog", Message: "is required"}
		}
		provider, _ := request.Current.Providers().Get(binding.ProviderID)
		if err := providers.ValidateBinding(provider, binding); err != nil {
			return nil, err
		}
		selected = append(selected, providerAttemptTarget{providerID: binding.ProviderID, binding: binding})
	}
	for _, provider := range request.Providers {
		if !slices.ContainsFunc(selected, func(target providerAttemptTarget) bool { return target.providerID == provider }) {
			return nil, &errors.ValidationError{Field: "acquisition.providers", Message: "requested provider has no active binding"}
		}
	}
	slices.SortFunc(selected, func(left, right providerAttemptTarget) int {
		if order := strings.Compare(string(left.providerID), string(right.providerID)); order != 0 {
			return order
		}
		return strings.Compare(left.binding.ID, right.binding.ID)
	})
	return selected, nil
}
