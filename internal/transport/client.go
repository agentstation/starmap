package transport

import (
	"context"
	"net/http"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// Client provides HTTP client functionality with authentication.
type Client struct {
	http *http.Client
}

// New creates a provider HTTP client.
func New() *Client {
	return &Client{
		http: &http.Client{
			Timeout: constants.DefaultHTTPTimeout,
			// Scope provider credentials to the configured endpoint.
			// Never replay them to a redirect target. Callers must make an
			// endpoint migration explicit in provider configuration.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Do sends an authenticated HTTP request.
func (c *Client) Do(
	req *http.Request,
	provider *catalogs.Provider,
	material sources.ProviderCredentialMaterial,
) (*http.Response, error) {
	return c.DoWithContext(req.Context(), req, provider, material)
}

// DoWithContext sends an authenticated HTTP request. It replaces the request's
// existing context with ctx.
func (c *Client) DoWithContext(
	ctx context.Context,
	req *http.Request,
	provider *catalogs.Provider,
	material sources.ProviderCredentialMaterial,
) (*http.Response, error) {
	req = req.Clone(ctx)

	if provider != nil {
		applyCredentialMaterial(req, material)
		rb := NewRequestBuilder(provider)
		rb.AddCatalogProtocolHeaders(req)
	}

	// Set common headers
	req.Header.Set("Accept", "application/json")
	if req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH" {
		req.Header.Set("Content-Type", "application/json")
	}

	response, err := c.http.Do(req) //nolint:gosec // Provider endpoints are trusted catalog configuration or caller-supplied integration points.
	if err != nil && materialUsesQueryAuthentication(material) {
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

// Get sends a GET request.
func (c *Client) Get(
	ctx context.Context,
	url string,
	provider *catalogs.Provider,
	material sources.ProviderCredentialMaterial,
) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.WrapResource("create", "request", "GET "+url, err)
	}
	return c.DoWithContext(ctx, req, provider, material)
}

func materialUsesQueryAuthentication(material sources.ProviderCredentialMaterial) bool {
	for _, placement := range material.Profile().Placements {
		if placement.Kind == catalogs.ProviderCredentialPlacementQuery {
			return true
		}
	}
	return false
}
