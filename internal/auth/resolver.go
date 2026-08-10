package auth

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

const starmapCredentialProduct = "STARMAP"

type environmentLookup func(string) (string, bool)

type profileResolver func(
	context.Context,
	*catalogs.Provider,
	catalogs.ProviderCredentialProfile,
	map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (sources.ProviderCredentialMaterial, bool, error)

// Resolver selects catalog-declared authentication primitives and resolves
// their ambient parameters. It contains no provider roster.
type Resolver struct {
	lookup   environmentLookup
	handlers map[catalogs.ProviderAuthenticationPrimitive]profileResolver
}

// NewResolver creates the built-in catalog credential resolver.
func NewResolver() *Resolver {
	return newResolver(os.LookupEnv)
}

func newResolver(lookup environmentLookup) *Resolver {
	resolver := &Resolver{lookup: lookup}
	resolver.handlers = map[catalogs.ProviderAuthenticationPrimitive]profileResolver{
		catalogs.ProviderAuthenticationNone:          resolver.resolveAmbientProfile,
		catalogs.ProviderAuthenticationAPIKey:        resolver.resolveAmbientProfile,
		catalogs.ProviderAuthenticationBearerToken:   resolver.resolveAmbientProfile,
		catalogs.ProviderAuthenticationGoogleDefault: resolver.resolveDefaultChainProfile,
		catalogs.ProviderAuthenticationAzureDefault:  resolveUnavailableDefaultChainProfile,
		catalogs.ProviderAuthenticationAWSDefault:    resolveUnavailableDefaultChainProfile,
	}
	return resolver
}

func resolveUnavailableDefaultChainProfile(
	context.Context,
	*catalogs.Provider,
	catalogs.ProviderCredentialProfile,
	map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (sources.ProviderCredentialMaterial, bool, error) {
	// A declared primitive is not usable until its source passes dependency,
	// lifecycle, and redaction admission. Until then, selection fails closed.
	return sources.ProviderCredentialMaterial{}, false, nil
}

// ResolveCatalog resolves the first configured catalog-acquisition profile.
// A malformed selected ambient value is terminal and cannot fall through.
func (r *Resolver) ResolveCatalog(
	ctx context.Context,
	provider *catalogs.Provider,
) (sources.ProviderCredentialMaterial, error) {
	if err := ctx.Err(); err != nil {
		return sources.ProviderCredentialMaterial{}, err
	}
	if provider == nil || provider.Credentials == nil {
		return sources.ProviderCredentialMaterial{}, &errors.ValidationError{
			Field: "provider.credentials", Message: "catalog credential metadata is required",
		}
	}
	fields := indexCredentialFields(provider.Credentials.Fields)
	profiles := indexCredentialProfiles(provider.Credentials.Profiles)
	plane := provider.Credentials.CatalogAcquisition
	for _, profileID := range plane.Alternatives {
		profile, exists := profiles[profileID]
		if !exists {
			return sources.ProviderCredentialMaterial{}, &errors.ValidationError{
				Field: "provider.credentials.catalog_acquisition.alternatives",
				Value: profileID, Message: "references an unknown profile",
			}
		}
		handler, exists := r.handlers[profile.Primitive]
		if !exists {
			return sources.ProviderCredentialMaterial{}, &errors.ValidationError{
				Field: "provider.credentials.profiles.primitive",
				Value: profile.Primitive, Message: "is not supported",
			}
		}
		material, configured, err := handler(ctx, provider, profile, fields)
		if err != nil {
			return sources.ProviderCredentialMaterial{}, err
		}
		if configured {
			return material, nil
		}
	}
	if !plane.Required {
		return sources.ProviderCredentialMaterial{}, nil
	}
	return sources.ProviderCredentialMaterial{}, &errors.AuthenticationError{
		Provider: string(provider.ID),
		Method:   "catalog-declared",
		Message:  "no catalog-acquisition credential profile is configured",
	}
}

func (r *Resolver) resolveAmbientProfile(
	ctx context.Context,
	provider *catalogs.Provider,
	profile catalogs.ProviderCredentialProfile,
	fields map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (sources.ProviderCredentialMaterial, bool, error) {
	values := make(map[catalogs.ProviderCredentialFieldID]string, len(profile.Fields))
	for _, fieldID := range profile.Fields {
		if err := ctx.Err(); err != nil {
			return sources.ProviderCredentialMaterial{}, false, err
		}
		field := fields[fieldID]
		value, selected, err := r.resolveField(provider.ID, field)
		if err != nil {
			return sources.ProviderCredentialMaterial{}, false, err
		}
		if !selected && field.Required {
			return sources.ProviderCredentialMaterial{}, false, nil
		}
		if selected {
			values[fieldID] = value
			continue
		}
		if field.Required && !defaultChainCanResolveField(profile, fieldID) {
			return sources.ProviderCredentialMaterial{}, false, nil
		}
	}
	return sources.NewProviderCredentialMaterial(profile, values), true, nil
}

func defaultChainCanResolveField(
	profile catalogs.ProviderCredentialProfile,
	fieldID catalogs.ProviderCredentialFieldID,
) bool {
	if options := profile.ProtocolOptions.GoogleDefault; options != nil {
		return options.ProjectField == fieldID || options.QuotaProjectField == fieldID
	}
	if options := profile.ProtocolOptions.AWSDefault; options != nil {
		return options.RegionField == fieldID
	}
	return false
}

func (r *Resolver) resolveDefaultChainProfile(
	ctx context.Context,
	provider *catalogs.Provider,
	profile catalogs.ProviderCredentialProfile,
	fields map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (sources.ProviderCredentialMaterial, bool, error) {
	values := make(map[catalogs.ProviderCredentialFieldID]string, len(profile.Fields))
	for _, fieldID := range profile.Fields {
		if err := ctx.Err(); err != nil {
			return sources.ProviderCredentialMaterial{}, false, err
		}
		field := fields[fieldID]
		if field.Kind == catalogs.ProviderCredentialFieldSecret {
			continue
		}
		value, selected, err := r.resolveField(provider.ID, field)
		if err != nil {
			return sources.ProviderCredentialMaterial{}, false, err
		}
		if selected {
			values[fieldID] = value
		}
	}
	return sources.NewProviderCredentialMaterial(profile, values), true, nil
}

func (r *Resolver) resolveField(
	providerID catalogs.ProviderID,
	field catalogs.ProviderCredentialField,
) (string, bool, error) {
	candidates := append([]string(nil), field.Environment...)
	derived, err := catalogs.DerivedCredentialEnvironmentName(
		starmapCredentialProduct,
		providerID,
		field.ID,
	)
	if err != nil {
		return "", false, err
	}
	candidates = append(candidates, derived)
	for _, name := range candidates {
		value, exists := r.lookup(name)
		if !exists || value == "" {
			continue
		}
		if field.Pattern != "" {
			matched, matchErr := regexp.MatchString(field.Pattern, value)
			if matchErr != nil {
				return "", false, errors.WrapParse("regex", field.Pattern, matchErr)
			}
			if !matched {
				return "", false, &errors.ValidationError{
					Field:   "provider.credentials.environment",
					Value:   name,
					Message: fmt.Sprintf("selected value does not match field %s", field.ID),
				}
			}
		}
		return value, true, nil
	}
	if field.Default != "" {
		return field.Default, true, nil
	}
	return "", false, nil
}

func indexCredentialFields(
	fields []catalogs.ProviderCredentialField,
) map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField {
	indexed := make(map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField, len(fields))
	for _, field := range fields {
		indexed[field.ID] = field
	}
	return indexed
}

func indexCredentialProfiles(
	profiles []catalogs.ProviderCredentialProfile,
) map[catalogs.ProviderCredentialProfileID]catalogs.ProviderCredentialProfile {
	indexed := make(map[catalogs.ProviderCredentialProfileID]catalogs.ProviderCredentialProfile, len(profiles))
	for _, profile := range profiles {
		indexed[profile.ID] = profile
	}
	return indexed
}
