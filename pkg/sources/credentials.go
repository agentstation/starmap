package sources

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// ProviderCredentialMaterial carries one selected catalog-acquisition profile
// and its resolved values. Values are private so generic serializers and
// formatters cannot expose them.
type ProviderCredentialMaterial struct {
	profile  catalogs.ProviderCredentialProfile
	values   map[catalogs.ProviderCredentialFieldID]string
	metadata ProviderCredentialMetadata
}

// ProviderCredentialMetadata describes one resolved material lifecycle.
// Version is opaque and contains no source path or secret digest.
type ProviderCredentialMetadata struct {
	Version   string
	ExpiresAt time.Time
	Lease     *ProviderCredentialLease
}

// ProviderCredentialLease describes renewable credential material.
type ProviderCredentialLease struct {
	Renewable    bool
	RefreshAfter time.Time
}

// NewProviderCredentialMaterial creates caller-owned credential material.
func NewProviderCredentialMaterial(
	profile catalogs.ProviderCredentialProfile,
	values map[catalogs.ProviderCredentialFieldID]string,
	metadata ProviderCredentialMetadata,
) ProviderCredentialMaterial {
	return ProviderCredentialMaterial{
		profile:  copyCredentialProfile(profile),
		values:   copyCredentialValues(values),
		metadata: copyCredentialMetadata(metadata),
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

// Version returns the resolver-owned opaque material version.
func (m ProviderCredentialMaterial) Version() string { return m.metadata.Version }

// ExpiresAt returns the material expiry when the selected source supplied one.
func (m ProviderCredentialMaterial) ExpiresAt() (time.Time, bool) {
	if m.metadata.ExpiresAt.IsZero() {
		return time.Time{}, false
	}
	return m.metadata.ExpiresAt, true
}

// Lease returns caller-owned renewable-material metadata when present.
func (m ProviderCredentialMaterial) Lease() (ProviderCredentialLease, bool) {
	if m.metadata.Lease == nil {
		return ProviderCredentialLease{}, false
	}
	return *m.metadata.Lease, true
}

// String returns a secret-free material summary.
func (m ProviderCredentialMaterial) String() string {
	return fmt.Sprintf("provider credential material (profile=%s, version=%t)", m.profile.ID, m.metadata.Version != "")
}

// GoString returns a secret-free Go-syntax material summary.
func (m ProviderCredentialMaterial) GoString() string { return m.String() }

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

func copyCredentialMetadata(metadata ProviderCredentialMetadata) ProviderCredentialMetadata {
	copied := metadata
	if metadata.Lease != nil {
		lease := *metadata.Lease
		copied.Lease = &lease
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
