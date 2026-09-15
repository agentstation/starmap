package acquisition

import (
	"context"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// CredentialReference selects a deployment source for one catalog credential field.
type CredentialReference struct {
	ProviderID      catalogs.ProviderID
	FieldID         catalogs.ProviderCredentialFieldID
	Reference       string
	FallbackAmbient bool
}

// CredentialPolicyState selects private storage for persistent credential selection history.
// The host must classify installations without a policy record from its existing state.
// Existing policy records take precedence over LegacyInstallation.
type CredentialPolicyState struct {
	Directory          string
	Product            string
	DeploymentID       string
	InstanceID         string
	LegacyInstallation bool
}

// CredentialResolverConfig configures Starmap catalog acquisition without inference or account storage.
type CredentialResolverConfig struct {
	References []CredentialReference
	// State selects explicit persistence. Nil uses the current policy without migration history.
	State *CredentialPolicyState
}

// OpenCredentialResolver composes the built-in acquisition secret sources and selection policy.
// It opens only explicitly selected policy storage and never reads credential sources during construction.
// Ambient lookup uses STARMAP names before catalog-declared conventional names.
// Hosts can select role-specific environment names through explicit references.
func OpenCredentialResolver(ctx context.Context, config CredentialResolverConfig) (sources.ProviderCredentialResolver, error) {
	if ctx == nil {
		return nil, &errors.ValidationError{Field: "acquisition.credentials.context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	policies := make(map[auth.CredentialFieldKey]auth.ReferencePolicy, len(config.References))
	for _, selected := range config.References {
		key := auth.CredentialFieldKey{ProviderID: selected.ProviderID, FieldID: selected.FieldID}
		if key.ProviderID == "" || key.FieldID == "" {
			return nil, &errors.ValidationError{Field: "acquisition.credentials.reference", Message: "requires provider and field IDs"}
		}
		if _, exists := policies[key]; exists {
			return nil, &errors.ValidationError{Field: "acquisition.credentials.reference", Message: "duplicates a provider field"}
		}
		reference, err := auth.ParseReference(selected.Reference)
		if err != nil {
			return nil, err
		}
		policies[key] = auth.ReferencePolicy{Reference: reference, FallbackAmbient: selected.FallbackAmbient}
	}
	resolver := auth.NewResolver(auth.WithReferencePolicies(policies))
	if config.State == nil {
		return resolver, nil
	}
	state := *config.State
	initial := auth.EnvironmentPolicyCurrent
	if state.LegacyInstallation {
		initial = auth.EnvironmentPolicyLegacy
	}
	store, err := auth.OpenFilePolicyStore(ctx, state.Directory, auth.PolicyOwner{
		Product: state.Product, Deployment: state.DeploymentID, Instance: state.InstanceID,
	}, initial)
	if err != nil {
		return nil, err
	}
	return auth.NewMigrationResolver(resolver, store)
}
