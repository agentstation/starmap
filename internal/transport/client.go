package transport

import (
	"context"
	"net/http"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Client provides HTTP client functionality with authentication.
type Client struct {
	http *http.Client
	auth Authenticator
}

// New creates a new transport client with the specified authenticator.
func New(provider *catalogs.Provider) *Client {
	return &Client{
		http: &http.Client{
			Timeout: constants.DefaultHTTPTimeout,
			// Provider credentials are scoped to the configured endpoint.
			// Never replay them to a redirect target; callers must make an
			// endpoint migration explicit in provider configuration.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		auth: newAuthenticator(provider),
	}
}

// Do performs an HTTP request with authentication applied.
func (c *Client) Do(req *http.Request, provider *catalogs.Provider) (*http.Response, error) {
	return c.DoWithContext(req.Context(), req, provider)
}

// DoWithContext performs an HTTP request with authentication applied and context support.
// The provided context will be used for the request, overriding any existing context in req.
func (c *Client) DoWithContext(ctx context.Context, req *http.Request, provider *catalogs.Provider) (*http.Response, error) {
	// Clone the request with the provided context to ensure context is respected
	req = req.Clone(ctx)

	// Apply authentication if provider has API key
	if provider != nil {
		apiKey, err := provider.APIKeyValue()
		if err != nil {
			return nil, &errors.AuthenticationError{
				Provider: string(provider.ID),
				Method:   "api_key",
				Message:  "failed to retrieve API key",
				Err:      err,
			}
		}
		if apiKey != "" {
			c.auth.Apply(req, apiKey)
		}

		// Apply provider-specific headers
		rb := NewRequestBuilder(provider)
		rb.AddProviderHeaders(req)
	}

	// Set common headers
	req.Header.Set("Accept", "application/json")
	if req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH" {
		req.Header.Set("Content-Type", "application/json")
	}

	response, err := c.http.Do(req) //nolint:gosec // Provider endpoints are trusted catalog configuration or caller-supplied integration points.
	if err != nil && providerUsesQueryAuthentication(provider) {
		// net/http errors include the request URL. Query-authenticated URLs
		// contain the credential, so retain cancellation semantics but never
		// expose the transport error or URL through the returned error graph.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, &errors.APIError{
			Provider: string(provider.ID),
			Message:  "query-authenticated provider request failed",
		}
	}
	return response, err
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, url string, provider *catalogs.Provider) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.WrapResource("create", "request", "GET "+url, err)
	}
	return c.DoWithContext(ctx, req, provider)
}

// newAuthenticator returns the appropriate authenticator for a provider.
func newAuthenticator(provider *catalogs.Provider) Authenticator {
	if provider == nil {
		return &NoAuth{}
	}

	// Use ProviderAuth to read authentication configuration from YAML
	return &ProviderAuth{Provider: provider}
}

func providerUsesQueryAuthentication(provider *catalogs.Provider) bool {
	return provider != nil &&
		provider.APIKey != nil &&
		provider.APIKey.QueryParam != ""
}
