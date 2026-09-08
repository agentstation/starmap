package acquisition

import (
	"bytes"
	"context"

	"github.com/agentstation/starmap/internal/sources/providers"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ProviderBindingObserver observes a declared scope through its acquisition profile.
// Custom observers must implement this contract before they can serve bound requests.
type ProviderBindingObserver interface {
	ObserveProviderBinding(context.Context, *catalogs.Catalog, sources.ProviderAcquisitionBinding) (ProviderObservation, error)
}

// ObserveProviderBinding gets one provider observation without retaining or publishing it.
// Runtime scope policy must approve the returned evidence before activation.
func (a *Acquirer) ObserveProviderBinding(ctx context.Context, current *catalogs.Catalog, binding sources.ProviderAcquisitionBinding) (ProviderObservation, error) {
	if err := binding.Validate(); err != nil {
		return ProviderObservation{}, err
	}
	if err := ctx.Err(); err != nil {
		return ProviderObservation{}, err
	}
	if a == nil {
		return ProviderObservation{}, &errors.ValidationError{Field: "acquisition.acquirer", Message: "is required"}
	}
	observer, ok := a.observer.(ProviderBindingObserver)
	if !ok {
		return ProviderObservation{}, &errors.ValidationError{Field: "acquisition.provider_observer", Message: "must support declared acquisition bindings"}
	}
	result, err := observer.ObserveProviderBinding(ctx, current, binding)
	if err != nil {
		return ProviderObservation{}, err
	}
	result.Layer.Payload = bytes.Clone(result.Layer.Payload)
	result.Layer.Receipt = result.Layer.Receipt.Clone()
	if result.Attempt.ProviderID != binding.ProviderID ||
		(result.Attempt.BindingID != "" && result.Attempt.BindingID != binding.ID) ||
		(result.Attempt.BindingRevision != "" && result.Attempt.BindingRevision != binding.Revision) {
		return ProviderObservation{}, &errors.ValidationError{Field: "acquisition.provider_attempt", Message: "returned attempt must match the requested binding"}
	}
	result.Attempt.BindingID, result.Attempt.BindingRevision = binding.ID, binding.Revision
	if err := result.Attempt.Validate(); err != nil {
		return ProviderObservation{}, err
	}
	if (result.Attempt.Outcome == sources.ProviderOutcomeSucceeded) != (len(result.Layer.Payload) > 0) {
		return ProviderObservation{}, &errors.ValidationError{Field: "acquisition.provider_attempt", Message: "only a successful attempt can return an observation payload"}
	}
	if len(result.Layer.Payload) > 0 && (result.Layer.ProviderID != binding.ProviderID || result.Layer.Receipt.ProviderBinding == nil || *result.Layer.Receipt.ProviderBinding != binding) {
		return ProviderObservation{}, &errors.ValidationError{Field: "acquisition.provider_binding", Message: "returned receipt must match the requested binding"}
	}
	return result, nil
}

func (o *providerSourceObserver) ObserveProviderBinding(ctx context.Context, current *catalogs.Catalog, binding sources.ProviderAcquisitionBinding) (ProviderObservation, error) {
	scoped, err := scopeToProvider(current, binding.ProviderID)
	if err != nil {
		return ProviderObservation{}, err
	}
	source := providers.New(scoped, o.options...)
	observed, attempts, err := source.ObserveBinding(ctx, binding)
	if err != nil {
		return ProviderObservation{}, err
	}
	return retainedProviderObservation(binding.ProviderID, observed, attempts)
}
