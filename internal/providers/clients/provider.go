// Package clients provides provider client registry functions.
// This package is separate from the providers source to avoid circular dependencies.
package clients

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"

	// Import provider implementations for clients.

	"github.com/agentstation/starmap/internal/providers/anthropic"
	"github.com/agentstation/starmap/internal/providers/google"
	"github.com/agentstation/starmap/internal/providers/openai"
)

// ProviderClient defines the interface for provider API clients.
// Each provider implementation must satisfy this interface to fetch model information.
type ProviderClient interface {
	// ListModels retrieves all available models from the provider.
	ListModels(context.Context, sources.ProviderCredentialMaterial) ([]catalogs.Model, error)
}

// NewProvider creates a new provider client for the given provider.
func NewProvider(provider *catalogs.Provider) (ProviderClient, error) {
	if provider == nil {
		return nil, &errors.ValidationError{Field: "provider", Message: "is required"}
	}
	if provider.Catalog == nil {
		return nil, &errors.ValidationError{
			Field: "provider.catalog", Value: provider.ID, Message: "catalog acquisition is not configured",
		}
	}
	if err := provider.ValidateContract(); err != nil {
		return nil, err
	}
	switch provider.Catalog.Endpoint.Type {
	case catalogs.EndpointTypeOpenAI:
		client, err := openai.NewClient(provider)
		if err != nil {
			return nil, err
		}
		return client, nil
	case catalogs.EndpointTypeAnthropic:
		if err := anthropic.ValidateCatalogEndpoint(provider); err != nil {
			return nil, err
		}
		return anthropic.NewClient(provider), nil
	case catalogs.EndpointTypeGoogle:
		if err := google.ValidateCatalogEndpoint(provider); err != nil {
			return nil, err
		}
		return google.NewClient(provider), nil
	case catalogs.EndpointTypeGoogleCloud:
		if err := google.ValidateCatalogEndpoint(provider); err != nil {
			return nil, err
		}
		return google.NewClient(provider), nil
	}
	return nil, &errors.ValidationError{
		Field:   "provider.catalog.endpoint.type",
		Value:   provider.Catalog.Endpoint.Type,
		Message: fmt.Sprintf("unsupported endpoint type: %s", provider.Catalog.Endpoint.Type),
	}
}

// FetchRawResult contains the result of a raw fetch operation.
type FetchRawResult struct {
	Data       []byte
	Response   *http.Response
	Latency    time.Duration
	RequestURL string
}

// FetchRaw gets an unparsed provider API response for test data generation. Its
// result contains the data, response headers, latency, and URL.
func FetchRaw(
	ctx context.Context,
	provider *catalogs.Provider,
	material sources.ProviderCredentialMaterial,
	endpoint string,
) (*FetchRawResult, error) {
	// Create transport client configured for this provider
	transportClient := transport.New()
	if endpoint == provider.CatalogEndpointURL() {
		resolved, err := provider.BindCatalogEndpoint(material.EndpointBindings())
		if err != nil {
			return nil, err
		}
		endpoint = resolved
	}

	// Track start time for latency calculation
	startTime := time.Now()

	// Make the raw request
	resp, err := transportClient.Get(ctx, endpoint, provider, material)
	if err != nil {
		return nil, &errors.APIError{
			Provider: string(provider.ID),
			Endpoint: endpoint,
			Message:  "API request failed",
			Err:      err,
		}
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
	}()

	// Calculate latency
	latency := time.Since(startTime)

	// Read raw response body
	rawData, err := io.ReadAll(io.LimitReader(resp.Body, constants.MaxSourcePayloadBytes+1))
	if err != nil {
		return nil, errors.WrapIO("read", "response body", err)
	}
	if len(rawData) > constants.MaxSourcePayloadBytes {
		return nil, &errors.ValidationError{
			Field: "response.body", Value: len(rawData),
			Message: "exceeds maximum source payload size",
		}
	}

	result := &FetchRawResult{
		Data:       rawData,
		Response:   resp,
		Latency:    latency,
		RequestURL: endpoint,
	}

	return result, nil
}
