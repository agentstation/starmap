package transport

import (
	"context"
	"net/http"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

// DefaultHTTPTimeout is the default timeout for HTTP requests.
var DefaultHTTPTimeout = constants.DefaultHTTPTimeout

// Client provides HTTP client functionality with authentication.
type Client struct {
	http *http.Client
	auth Authenticator
}

// New creates a new transport client with the specified authenticator.
func New(auth Authenticator) *Client {
	return &Client{
		http: &http.Client{Timeout: DefaultHTTPTimeout},
		auth: auth,
	}
}

// NewForProvider creates a transport client configured for a specific provider.
func NewForProvider(provider *catalogs.Provider) *Client {
	auth := getAuthenticatorForProvider(provider)
	return New(auth)
}

// Do performs an HTTP request with authentication applied.
func (c *Client) Do(req *http.Request, provider *catalogs.Provider) (*http.Response, error) {
	return c.DoWithContext(context.Background(), req, provider)
}

// DoWithContext performs an HTTP request with authentication applied and context support.
func (c *Client) DoWithContext(ctx context.Context, req *http.Request, provider *catalogs.Provider) (*http.Response, error) {
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

	return c.http.Do(req)
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, url string, provider *catalogs.Provider) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.WrapResource("create", "request", "GET "+url, err)
	}
	return c.DoWithContext(ctx, req, provider)
}

// getAuthenticatorForProvider returns the appropriate authenticator for a provider.
func getAuthenticatorForProvider(provider *catalogs.Provider) Authenticator {
	if provider == nil {
		return &NoAuth{}
	}

	// Use ProviderAuth to read authentication configuration from YAML
	return &ProviderAuth{Provider: provider}
}
