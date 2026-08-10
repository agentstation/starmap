package sources

import (
	"context"
	"net/url"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// ProviderCredentialMaterial carries one selected catalog-acquisition profile
// and its resolved values. Values are private so generic serializers and
// formatters cannot expose them.
type ProviderCredentialMaterial struct {
	profile catalogs.ProviderCredentialProfile
	values  map[catalogs.ProviderCredentialFieldID]string
}

// NewProviderCredentialMaterial creates caller-owned credential material.
func NewProviderCredentialMaterial(
	profile catalogs.ProviderCredentialProfile,
	values map[catalogs.ProviderCredentialFieldID]string,
) ProviderCredentialMaterial {
	return ProviderCredentialMaterial{
		profile: copyCredentialProfile(profile),
		values:  copyCredentialValues(values),
	}
}

// Profile returns a caller-owned copy of the selected profile.
func (m ProviderCredentialMaterial) Profile() catalogs.ProviderCredentialProfile {
	return copyCredentialProfile(m.profile)
}

// Value returns one exact credential or parameter value.
func (m ProviderCredentialMaterial) Value(
	fieldID catalogs.ProviderCredentialFieldID,
) (string, bool) {
	value, exists := m.values[fieldID]
	return value, exists
}

// EndpointBindings returns resolved URL-template bindings for the profile.
func (m ProviderCredentialMaterial) EndpointBindings() map[string]string {
	bindings := make(map[string]string, len(m.profile.EndpointBindings))
	for _, binding := range m.profile.EndpointBindings {
		value, exists := m.values[binding.Field]
		if !exists || value == "" {
			continue
		}
		switch binding.Format {
		case catalogs.ProviderCredentialEndpointBindingURL:
			parsed, err := url.Parse(value)
			if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") ||
				parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
				continue
			}
			bindings[binding.Variable] = value
		case catalogs.ProviderCredentialEndpointBindingPathSegment:
			bindings[binding.Variable] = url.PathEscape(value)
		}
	}
	return bindings
}

// ProviderCredentialResolver resolves one catalog-acquisition profile.
type ProviderCredentialResolver interface {
	ResolveCatalog(context.Context, *catalogs.Provider) (ProviderCredentialMaterial, error)
}

// ProviderCredentialResolverFunc adapts a function to credential resolution.
type ProviderCredentialResolverFunc func(
	context.Context,
	*catalogs.Provider,
) (ProviderCredentialMaterial, error)

// ResolveCatalog implements ProviderCredentialResolver.
func (f ProviderCredentialResolverFunc) ResolveCatalog(
	ctx context.Context,
	provider *catalogs.Provider,
) (ProviderCredentialMaterial, error) {
	return f(ctx, provider)
}

func copyCredentialValues(
	values map[catalogs.ProviderCredentialFieldID]string,
) map[catalogs.ProviderCredentialFieldID]string {
	copied := make(map[catalogs.ProviderCredentialFieldID]string, len(values))
	for fieldID, value := range values {
		copied[fieldID] = value
	}
	return copied
}

func copyCredentialProfile(
	profile catalogs.ProviderCredentialProfile,
) catalogs.ProviderCredentialProfile {
	copied := profile
	copied.Fields = append([]catalogs.ProviderCredentialFieldID(nil), profile.Fields...)
	copied.Placements = append([]catalogs.ProviderCredentialPlacement(nil), profile.Placements...)
	copied.Scopes = append([]string(nil), profile.Scopes...)
	copied.EndpointBindings = append(
		[]catalogs.ProviderCredentialEndpointBinding(nil),
		profile.EndpointBindings...,
	)
	if profile.ProtocolOptions.GoogleDefault != nil {
		options := *profile.ProtocolOptions.GoogleDefault
		copied.ProtocolOptions.GoogleDefault = &options
	}
	if profile.ProtocolOptions.AWSDefault != nil {
		options := *profile.ProtocolOptions.AWSDefault
		copied.ProtocolOptions.AWSDefault = &options
	}
	return copied
}
