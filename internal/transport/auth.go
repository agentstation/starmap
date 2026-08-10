package transport

import (
	"net/http"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// Authenticator applies one catalog-declared credential placement.
type Authenticator interface {
	Apply(*http.Request, catalogs.ProviderCredentialPlacement, string)
}

// HeaderAuth applies header credential placement.
type HeaderAuth struct{}

// Apply places an exact credential value in one header.
func (HeaderAuth) Apply(
	req *http.Request,
	placement catalogs.ProviderCredentialPlacement,
	value string,
) {
	req.Header.Set(placement.Name, credentialPlacementValue(placement.Scheme, value))
}

// QueryAuth applies query credential placement.
type QueryAuth struct{}

// Apply places an exact credential value in one query parameter.
func (QueryAuth) Apply(
	req *http.Request,
	placement catalogs.ProviderCredentialPlacement,
	value string,
) {
	if req.URL == nil {
		return
	}
	query := req.URL.Query()
	query.Set(placement.Name, value)
	req.URL.RawQuery = query.Encode()
}

var authenticators = map[catalogs.ProviderCredentialPlacementKind]Authenticator{
	catalogs.ProviderCredentialPlacementHeader: HeaderAuth{},
	catalogs.ProviderCredentialPlacementQuery:  QueryAuth{},
}

func applyCredentialMaterial(
	req *http.Request,
	material sources.ProviderCredentialMaterial,
) {
	profile := material.Profile()
	for _, placement := range profile.Placements {
		value, exists := material.Value(placement.Field)
		if !exists || value == "" {
			continue
		}
		authenticator, exists := authenticators[placement.Kind]
		if !exists {
			continue
		}
		authenticator.Apply(req, placement, value)
	}
}

func credentialPlacementValue(scheme catalogs.ProviderCredentialScheme, value string) string {
	switch scheme {
	case catalogs.ProviderCredentialSchemeBearer:
		return "Bearer " + value
	case catalogs.ProviderCredentialSchemeBasic:
		return "Basic " + value
	default:
		return value
	}
}
