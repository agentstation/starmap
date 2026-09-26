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

// CredentialProduct selects the catalog acquisition policy family.
type CredentialProduct string

const (
	// CredentialProductStarmap selects standalone Starmap acquisition.
	CredentialProductStarmap CredentialProduct = "starmap"
	// CredentialProductStarport selects embedded Starport acquisition.
	CredentialProductStarport CredentialProduct = "starport"
)

// CredentialResolverConfig configures Starmap catalog acquisition without inference or account storage.
type CredentialResolverConfig struct {
	References []CredentialReference
	// Product defaults to Starmap. It selects both current and legacy policy ordering.
	Product CredentialProduct
	// Lookup reads the host environment. Nil uses the process environment.
	Lookup func(string) (string, bool)
	// State selects explicit persistence. Nil uses the current policy without migration history.
	State *CredentialPolicyState
}

// OpenCredentialResolver composes the built-in acquisition secret sources and selection policy.
// It opens only explicitly selected policy storage and never reads credential sources during construction.
// Product selects ambient precedence. Explicit references override ambient selection.
func OpenCredentialResolver(ctx context.Context, config CredentialResolverConfig) (sources.ProviderCredentialResolver, error) {
	if ctx == nil {
		return nil, &errors.ValidationError{Field: "acquisition.credentials.context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	current := auth.EnvironmentPolicyCurrent
	legacy := auth.EnvironmentPolicyLegacy
	switch config.Product {
	case "", CredentialProductStarmap:
	case CredentialProductStarport:
		current = auth.EnvironmentPolicyStarportCurrent
		legacy = auth.EnvironmentPolicyStarportLegacy
	default:
		return nil, &errors.ValidationError{Field: "acquisition.credentials.product", Message: "is not supported"}
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
	resolver := auth.NewResolver(auth.WithReferencePolicies(policies), auth.WithEnvironmentPolicy(current), auth.WithEnvironmentLookup(config.Lookup))
	if config.State == nil {
		return resolver, nil
	}
	state := *config.State
	initial := current
	if state.LegacyInstallation {
		initial = legacy
	}
	store, err := auth.OpenFilePolicyStore(ctx, state.Directory, auth.PolicyOwner{
		Product: state.Product, Deployment: state.DeploymentID, Instance: state.InstanceID,
	}, initial)
	if err != nil {
		return nil, err
	}
	return auth.NewMigrationResolver(resolver, store)
}
