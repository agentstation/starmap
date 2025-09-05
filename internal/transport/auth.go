package transport

import (
	"net/http"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// Authenticator applies authentication to HTTP requests.
type Authenticator interface {
	Apply(req *http.Request, apiKey string)
}

// NoAuth implements no authentication.
type NoAuth struct{}

// Apply implements the Authenticator interface for NoAuth.
func (a *NoAuth) Apply(_ *http.Request, _ string) {
	// No authentication applied
}

// BearerAuth implements Bearer token authentication.
type BearerAuth struct{}

// Apply implements the Authenticator interface for BearerAuth.
func (a *BearerAuth) Apply(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
}

// HeaderAuth implements custom header authentication.
type HeaderAuth struct {
	Header string
}

// Apply implements the Authenticator interface for HeaderAuth.
func (a *HeaderAuth) Apply(req *http.Request, apiKey string) {
	req.Header.Set(a.Header, apiKey)
}

// QueryAuth implements API key as query parameter authentication.
type QueryAuth struct {
	Param string
}

// Apply implements the Authenticator interface for QueryAuth.
func (a *QueryAuth) Apply(req *http.Request, apiKey string) {
	if req.URL == nil {
		return
	}

	// Parse existing query parameters
	query := req.URL.Query()
	query.Set(a.Param, apiKey)
	req.URL.RawQuery = query.Encode()
}

// ProviderAuth implements provider-specific authentication using catalog configuration.
type ProviderAuth struct {
	Provider *catalogs.Provider
}

// Apply implements the Authenticator interface for ProviderAuth.
func (a *ProviderAuth) Apply(req *http.Request, apiKey string) {
	if a.Provider == nil || a.Provider.APIKey == nil {
		return
	}

	// Handle query parameter authentication (e.g., Google AI Studio)
	if a.Provider.APIKey.QueryParam != "" {
		if req.URL != nil {
			query := req.URL.Query()
			query.Set(a.Provider.APIKey.QueryParam, apiKey)
			req.URL.RawQuery = query.Encode()
		}
		return
	}

	// Handle header authentication
	header := a.Provider.APIKey.Header
	if header == "" {
		header = "Authorization"
	}

	var value string
	switch a.Provider.APIKey.Scheme {
	case catalogs.ProviderAPIKeySchemeBearer:
		value = "Bearer " + apiKey
	case catalogs.ProviderAPIKeySchemeBasic:
		value = "Basic " + apiKey
	case catalogs.ProviderAPIKeySchemeDirect:
		// Direct value (no scheme prefix) - covers both empty string and explicit "Direct"
		value = apiKey
	default:
		// Unknown scheme - treat as direct
		value = apiKey
	}

	req.Header.Set(header, value)
}
