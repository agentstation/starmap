// Package clients provides provider client registry functions.
// This package is separate from the providers source to avoid circular dependencies.
package clients

import (
	"context"
	"fmt"
	"io"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"

	// Import provider implementations for clients.

	"github.com/agentstation/starmap/internal/sources/providers/anthropic"
	"github.com/agentstation/starmap/internal/sources/providers/google"
	"github.com/agentstation/starmap/internal/sources/providers/openai"
)

// ProviderClient defines the interface for provider API clients.
// Each provider implementation must satisfy this interface to fetch model information.
type ProviderClient interface {
	// ListModels retrieves all available models from the provider.
	ListModels(ctx context.Context) ([]catalogs.Model, error)

	// isAPIKeyRequired returns true if the client requires an API key.
	IsAPIKeyRequired() bool

	// HasAPIKey returns true if the client has an API key.
	HasAPIKey() bool
}

// NewProvider creates a new provider client for the given provider.
func NewProvider(provider *catalogs.Provider) (ProviderClient, error) {
	switch provider.Catalog.Endpoint.Type {
	case catalogs.EndpointTypeOpenAI:
		return openai.NewClient(provider), nil
	case catalogs.EndpointTypeAnthropic:
		return anthropic.NewClient(provider), nil
	case catalogs.EndpointTypeGoogle:
		return google.NewClient(provider), nil
	case catalogs.EndpointTypeGoogleCloud:
		return google.NewClient(provider), nil
	}
	return nil, &errors.ValidationError{
		Field:   "provider.catalog.endpoint.type",
		Value:   provider.Catalog.Endpoint.Type,
		Message: fmt.Sprintf("unsupported endpoint type: %s", provider.Catalog.Endpoint.Type),
	}
}

// FetchRaw fetches raw response data from a provider's API endpoint.
// This function is used for fetching raw API responses for testdata generation.
func FetchRaw(ctx context.Context, provider *catalogs.Provider, endpoint string) ([]byte, error) {
	// Create transport client configured for this provider
	transportClient := transport.New(provider)

	// Make the raw request
	resp, err := transportClient.Get(ctx, endpoint, provider)
	if err != nil {
		return nil, &errors.APIError{
			Provider: string(provider.ID),
			Endpoint: endpoint,
			Message:  "API request failed",
			Err:      err,
		}
	}
	defer func() {
		// Drain any remaining body to allow connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	// Read raw response body
	rawData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WrapIO("read", "response body", err)
	}

	return rawData, nil
}
