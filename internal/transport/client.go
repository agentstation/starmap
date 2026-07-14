package transport

import (
	"context"
	"net/http"
	"strings"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

// DefaultHTTPTimeout is the default timeout for HTTP requests.
var DefaultHTTPTimeout = constants.DefaultHTTPTimeout

// Client provides HTTP client functionality with authentication.
type Client struct {
	http *http.Client
	auth auth.ResolvedAuth
}

// New creates a source-local transport. Authentication has already been
// resolved and no environment lookup occurs in the transport layer.
func New(resolvedAuth auth.ResolvedAuth) *Client {
	return &Client{
		http: &http.Client{Timeout: DefaultHTTPTimeout, CheckRedirect: sameOriginRedirect},
		auth: resolvedAuth,
	}
}

func sameOriginRedirect(request *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	origin := via[0].URL
	target := request.URL
	if !strings.EqualFold(origin.Scheme, target.Scheme) || !strings.EqualFold(origin.Host, target.Host) {
		return &errors.ValidationError{Field: "provider.redirect.origin", Value: target.Scheme + "://" + target.Host, Message: "must match configured request origin " + origin.Scheme + "://" + origin.Host}
	}
	return nil
}

// Do performs an HTTP request with authentication applied.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.DoWithContext(req.Context(), req)
}

// DoWithContext performs an HTTP request with authentication applied and context support.
// The provided context will be used for the request, overriding any existing context in req.
func (c *Client) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	// Clone the request with the provided context to ensure context is respected
	req = req.Clone(ctx)

	if err := c.auth.Apply(req); err != nil {
		return nil, err
	}
	// Set common headers
	req.Header.Set("Accept", "application/json")
	if req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH" {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.http.Do(req) //nolint:gosec // Provider endpoints are trusted catalog configuration or caller-supplied integration points.
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.WrapResource("create", "request", "GET "+url, err)
	}
	return c.DoWithContext(ctx, req)
}
