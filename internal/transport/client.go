package transport

import (
	"context"
	"net/http"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// Client provides HTTP client functionality with authentication.
type Client struct {
	http   *http.Client
	policy remote.TransferPolicy
}

// providerTransferPolicy returns the transfer bounds of one provider request.
// It keeps the shared connection, header, inactivity, and duration bounds, and
// it lowers the size bound to the source payload limit. A body within that
// limit still reaches the decoder, which reports the limit it already owns.
func providerTransferPolicy() remote.TransferPolicy {
	policy := remote.DefaultTransferPolicy()
	policy.MaxCompressedBytes = constants.MaxSourcePayloadBytes + 1
	return policy
}

// New creates a provider HTTP client. The client applies the transfer bounds
// through its transport and its body reads, and it sets no client-wide
// timeout. A client-wide timeout also covers body reads, so it cannot bound a
// progress-aware transfer.
func New() *Client {
	return newClient(providerTransferPolicy())
}

// newClient builds a provider client that applies policy to every request.
func newClient(policy remote.TransferPolicy) *Client {
	client := remote.DefaultTransferClient()
	// An unusable policy keeps the default connection bounds. Every transfer
	// validates the policy again and reports a typed error.
	if bounded, err := remote.NewTransport(policy); err == nil {
		client.Transport = bounded
	}
	// Scope provider credentials to the configured endpoint.
	// Never replay them to a redirect target. Callers must make an
	// endpoint migration explicit in provider configuration.
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Client{http: client, policy: policy}
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

	// Read the body under the transfer bounds. A provider that stalls or drips
	// then stops at the inactivity bound or the per-transfer maximum, and the
	// decoders that follow read from memory.
	transfer := remote.Transfer{Client: c.http, Policy: c.policy}
	response, err := transfer.Response(ctx, req, transferResource(provider))
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

// transferResource returns a safe label for progress and error reporting. It
// carries no URL, no token, and no host name.
func transferResource(provider *catalogs.Provider) string {
	if provider == nil || provider.ID == "" {
		return "provider request"
	}
	return "provider " + string(provider.ID)
}

func materialUsesQueryAuthentication(material sources.ProviderCredentialMaterial) bool {
	for _, placement := range material.Profile().Placements {
		if placement.Kind == catalogs.ProviderCredentialPlacementQuery {
			return true
		}
	}
	return false
}
