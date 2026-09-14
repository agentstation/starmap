package auth

import (
	"context"
	stderrors "errors"
	"reflect"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// MigrationResolver refuses changed material until the operator resolves the conflicting selection.
type MigrationResolver struct {
	resolver *Resolver
	store    PolicyStore
}

// NewMigrationResolver adds persistent selection policy to catalog credential resolution.
func NewMigrationResolver(resolver *Resolver, store PolicyStore) (*MigrationResolver, error) {
	if resolver == nil || store == nil || resolver.environmentPolicy != EnvironmentPolicyCurrent {
		return nil, policyStoreError("migration requires a current resolver and policy store")
	}
	return &MigrationResolver{resolver: resolver, store: store}, nil
}

// ResolveCatalog compares complete handles before recording a legacy provider migration.
func (m *MigrationResolver) ResolveCatalog(ctx context.Context, provider *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, credentialResolutionTimeout)
	defer cancel()
	if provider == nil || provider.Credentials == nil {
		return m.resolver.ResolveCatalog(ctx, provider)
	}
	if err := m.resolver.validateEnvironmentAliases(provider); err != nil {
		return sources.ProviderCredentialMaterial{}, err
	}
	policy, err := m.store.Policy(ctx, provider.ID)
	if err != nil {
		return sources.ProviderCredentialMaterial{}, err
	}
	if policy == EnvironmentPolicyCurrent {
		return m.resolver.ResolveCatalog(ctx, provider)
	}
	if policy != EnvironmentPolicyLegacy {
		return sources.ProviderCredentialMaterial{}, policyStoreError("stored selection policy is not supported")
	}
	inputs := newPolicyInputs(m.resolver, provider.ID)
	current, err := inputs.resolver(EnvironmentPolicyCurrent).ResolveCatalog(ctx, provider)
	if err != nil {
		return sources.ProviderCredentialMaterial{}, err
	}
	legacy, err := inputs.resolver(EnvironmentPolicyLegacy).ResolveCatalog(ctx, provider)
	if err != nil && !missingCatalogCredentials(err) {
		return sources.ProviderCredentialMaterial{}, err
	}
	if err == nil && !sameCredentialHandle(current, legacy) {
		return sources.ProviderCredentialMaterial{}, credentialMigrationConflict(provider.ID, legacy, current)
	}
	if err := m.store.Accept(ctx, provider.ID); err != nil {
		return sources.ProviderCredentialMaterial{}, err
	}
	if err := validateMaterial(ctx, provider.ID, current); err != nil {
		return sources.ProviderCredentialMaterial{}, err
	}
	return current, nil
}

func missingCatalogCredentials(err error) bool {
	var missing *errors.AuthenticationError
	return stderrors.As(err, &missing) && missing.Method == "catalog-declared"
}

func sameCredentialHandle(left, right sources.ProviderCredentialMaterial) bool {
	profile := left.Profile()
	if !reflect.DeepEqual(profile, right.Profile()) {
		return false
	}
	for _, field := range profile.Fields {
		leftValue, leftExists := left.Value(field)
		rightValue, rightExists := right.Value(field)
		if leftExists != rightExists || leftValue != rightValue {
			return false
		}
	}
	return true
}

func credentialMigrationConflict(provider catalogs.ProviderID, legacy, current sources.ProviderCredentialMaterial) error {
	var names []string
	for _, material := range []sources.ProviderCredentialMaterial{legacy, current} {
		for _, origin := range material.Origins() {
			if origin.Name != "" && !slices.Contains(names, origin.Name) {
				names = append(names, origin.Name)
			}
		}
	}
	slices.Sort(names)
	return &errors.ConflictError{
		Resource: "catalog credential policy for " + string(provider),
		Message:  "catalog-acquisition selections differ (" + strings.Join(names, ", ") + "); select an explicit credential reference or remove the conflicting variable",
	}
}
