package auth

import (
	"fmt"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// EnvironmentPolicy identifies the catalog credential selection order.
type EnvironmentPolicy string

const (
	// EnvironmentPolicyLegacy preserves conventional-first lookup for migration checks.
	EnvironmentPolicyLegacy EnvironmentPolicy = "starmap-catalog-v1"
	// EnvironmentPolicyCurrent selects product names before conventional names.
	EnvironmentPolicyCurrent EnvironmentPolicy = "starmap-catalog-v2"
	// EnvironmentPolicyStarportLegacy preserves the original gateway acquisition order.
	EnvironmentPolicyStarportLegacy EnvironmentPolicy = "starport-catalog-v1"
	// EnvironmentPolicyStarportCurrent selects catalog-role names before shared deployment names.
	EnvironmentPolicyStarportCurrent EnvironmentPolicy = "starport-catalog-v2"
)

// WithEnvironmentPolicy selects the policy that a migration guard must compare.
func WithEnvironmentPolicy(policy EnvironmentPolicy) ResolverOption {
	return func(resolver *Resolver) { resolver.environmentPolicy = policy }
}

func (r *Resolver) environmentCandidates(
	providerID catalogs.ProviderID,
	field catalogs.ProviderCredentialField,
) ([]string, error) {
	derived, err := catalogs.DerivedCredentialEnvironmentName(starmapCredentialProduct, providerID, field.ID)
	if err != nil {
		return nil, err
	}
	gateway, err := catalogs.DerivedCredentialEnvironmentName("STARPORT", providerID, field.ID)
	if err != nil {
		return nil, err
	}
	var candidates []string
	switch r.environmentPolicy {
	case EnvironmentPolicyCurrent:
		candidates = append([]string{derived}, field.Environment...)
	case EnvironmentPolicyLegacy:
		candidates = append(slices.Clone(field.Environment), derived)
	case EnvironmentPolicyStarportCurrent:
		role := "STARPORT_CATALOG_" + strings.TrimPrefix(gateway, "STARPORT_")
		candidates = append([]string{role, derived, gateway}, field.Environment...)
	case EnvironmentPolicyStarportLegacy:
		candidates = append([]string{gateway}, field.Environment...)
	default:
		return nil, &errors.ValidationError{
			Field: "credentials.environment_policy", Message: "is not supported",
		}
	}
	unique := make([]string, 0, len(candidates))
	for _, name := range candidates {
		if !credentialEnvironmentPattern.MatchString(name) {
			return nil, &errors.ValidationError{
				Field: "provider.credentials.environment", Message: "requires valid environment names",
			}
		}
		if !slices.Contains(unique, name) {
			unique = append(unique, name)
		}
	}
	return unique, nil
}

func (r *Resolver) validateEnvironmentAliases(provider *catalogs.Provider) error {
	owners := make(map[string]catalogs.ProviderCredentialFieldID)
	fields := make(map[catalogs.ProviderCredentialFieldID]struct{})
	for _, field := range provider.Credentials.Fields {
		if _, exists := fields[field.ID]; exists {
			return &errors.ValidationError{
				Field: "provider.credentials.fields", Value: field.ID, Message: "must be unique",
			}
		}
		fields[field.ID] = struct{}{}
		candidates, err := r.environmentCandidates(provider.ID, field)
		if err != nil {
			return err
		}
		for _, name := range candidates {
			if owner, exists := owners[name]; exists && owner != field.ID {
				return &errors.ValidationError{
					Field: "provider.credentials.environment", Value: name,
					Message: fmt.Sprintf("conflicts between fields %s and %s", owner, field.ID),
				}
			}
			owners[name] = field.ID
		}
	}
	return nil
}

func (p EnvironmentPolicy) current() EnvironmentPolicy {
	switch p {
	case EnvironmentPolicyLegacy, EnvironmentPolicyCurrent:
		return EnvironmentPolicyCurrent
	case EnvironmentPolicyStarportLegacy, EnvironmentPolicyStarportCurrent:
		return EnvironmentPolicyStarportCurrent
	default:
		return ""
	}
}

func (p EnvironmentPolicy) legacy() EnvironmentPolicy {
	switch p.current() {
	case EnvironmentPolicyCurrent:
		return EnvironmentPolicyLegacy
	case EnvironmentPolicyStarportCurrent:
		return EnvironmentPolicyStarportLegacy
	default:
		return ""
	}
}

// WithEnvironmentLookup supplies the host environment without reading it during construction.
func WithEnvironmentLookup(lookup func(string) (string, bool)) ResolverOption {
	return func(resolver *Resolver) {
		if lookup != nil {
			resolver.lookup = lookup
			resolver.sources[referenceBackendEnvironment] = environmentSource{lookup: lookup}
		}
	}
}

func (p EnvironmentPolicy) empty(value string) bool {
	if p.current() == EnvironmentPolicyStarportCurrent {
		return strings.TrimSpace(value) == ""
	}
	return value == ""
}

func (p EnvironmentPolicy) skipsEmpty(value string) bool {
	return p == p.legacy() && p.empty(value)
}
