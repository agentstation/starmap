package providers

import (
	"context"
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ObserveBinding observes one declared provider scope with its selected acquisition profile.
// The receipt records deployment declarations, which do not prove upstream account ownership.
func (s *Source) ObserveBinding(
	ctx context.Context,
	binding sources.ProviderAcquisitionBinding,
) (sources.Observation, []sources.ProviderAttempt, error) {
	if err := binding.Validate(); err != nil {
		return sources.Observation{}, nil, err
	}
	if err := ctx.Err(); err != nil {
		return sources.Observation{}, nil, err
	}
	if err := s.validateBinding(binding); err != nil {
		return sources.Observation{}, nil, err
	}
	// The copy owns its resolver selection. Concurrent calls retain separate run memos.
	selected := *s
	selected.credentialResolver = bindingCredentialResolver{binding: binding, resolver: s.credentialResolver}
	if s.attemptSink != nil {
		selected.attemptSink = func(attempt sources.ProviderAttempt) {
			attempt.BindingID, attempt.BindingRevision = binding.ID, binding.Revision
			s.attemptSink(attempt)
		}
	}
	observed, attempts, err := selected.ObserveAttempts(ctx, sources.WithProviderFilter(binding.ProviderID))
	for i := range attempts {
		attempts[i].BindingID, attempts[i].BindingRevision = binding.ID, binding.Revision
	}
	if err != nil {
		return sources.Observation{}, attempts, err
	}
	observed, err = sources.NewObservation(observed.SourceID, observed.Catalog, sources.ObservationMetadata{
		ProviderBinding: &binding,
		ObservedAt:      observed.ObservedAt, Revision: observed.Revision,
		Completeness: observed.Completeness, Status: observed.Status,
		Records: observed.Records, Issues: observed.Issues,
	})
	return observed, attempts, err
}

func (s *Source) validateBinding(binding sources.ProviderAcquisitionBinding) error {
	if s == nil || s.providers == nil {
		return bindingSelectionError("provider_id", "requires configured providers")
	}
	provider, found := s.providers.Get(binding.ProviderID)
	if !found {
		return &errors.NotFoundError{Resource: "provider", ID: string(binding.ProviderID)}
	}
	if err := ValidateBinding(provider, binding); err != nil {
		return err
	}
	if s.credentialResolver == nil {
		return bindingSelectionError("credential_profile_id", "requires a credential resolver")
	}
	return nil
}

// ValidateBinding checks the declared scope against the provider's acquisition configuration.
// It does not resolve credentials, construct clients, or contact providers.
func ValidateBinding(provider *catalogs.Provider, binding sources.ProviderAcquisitionBinding) error {
	if err := binding.Validate(); err != nil {
		return err
	}
	if provider == nil || provider.ID != binding.ProviderID {
		return bindingSelectionError("provider_id", "requires the declared provider")
	}
	if provider.Catalog == nil {
		return bindingSelectionError("provider_id", "requires a catalog endpoint")
	}
	if provider.Credentials == nil || !slices.Contains(provider.Credentials.CatalogAcquisition.Alternatives, binding.CredentialProfileID) {
		return bindingSelectionError("credential_profile_id", "must be permitted for catalog acquisition")
	}
	if !slices.ContainsFunc(provider.Credentials.Profiles, func(profile catalogs.ProviderCredentialProfile) bool {
		return profile.ID == binding.CredentialProfileID
	}) {
		return bindingSelectionError("credential_profile_id", "requires a declared profile")
	}
	return nil
}

// bindingCredentialResolver checks the profile before the run memo retains its material.
type bindingCredentialResolver struct {
	binding  sources.ProviderAcquisitionBinding
	resolver sources.ProviderCredentialResolver
}

func (r bindingCredentialResolver) ResolveCatalog(ctx context.Context, provider *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
	if err := ctx.Err(); err != nil {
		return sources.ProviderCredentialMaterial{}, err
	}
	if provider == nil || provider.ID != r.binding.ProviderID || provider.Credentials == nil {
		return sources.ProviderCredentialMaterial{}, bindingSelectionError("provider_id", "does not match credential selection")
	}
	selected := catalogs.DeepCopyProvider(*provider)
	selected.Credentials.CatalogAcquisition.Alternatives = []catalogs.ProviderCredentialProfileID{r.binding.CredentialProfileID}
	material, err := r.resolver.ResolveCatalog(ctx, &selected)
	if err != nil {
		return sources.ProviderCredentialMaterial{}, err
	}
	if material.Profile().ID != r.binding.CredentialProfileID {
		return sources.ProviderCredentialMaterial{}, &errors.ConfigError{
			Component: "provider_binding.credential_profile_id", Message: "resolved profile does not match the declared acquisition profile",
		}
	}
	return material, nil
}

func bindingSelectionError(field, message string) error {
	return &errors.ValidationError{Field: "provider_binding." + field, Message: message}
}
