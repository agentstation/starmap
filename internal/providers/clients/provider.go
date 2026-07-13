// Package clients provides provider client registry functions.
// This package is separate from the providers source to avoid circular dependencies.
package clients

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"

	// Import provider implementations for clients.

	"github.com/agentstation/starmap/internal/providers/anthropic"
	"github.com/agentstation/starmap/internal/providers/cloudflare"
	"github.com/agentstation/starmap/internal/providers/cohere"
	"github.com/agentstation/starmap/internal/providers/databricks"
	"github.com/agentstation/starmap/internal/providers/google"
	"github.com/agentstation/starmap/internal/providers/huggingface"
	"github.com/agentstation/starmap/internal/providers/mistral"
	"github.com/agentstation/starmap/internal/providers/novita"
	"github.com/agentstation/starmap/internal/providers/nvidia"
	"github.com/agentstation/starmap/internal/providers/openai"
	"github.com/agentstation/starmap/internal/providers/sambanova"
	"github.com/agentstation/starmap/internal/providers/snowflake"
	"github.com/agentstation/starmap/internal/providers/together"
	"github.com/agentstation/starmap/internal/providers/watsonx"
	"github.com/agentstation/starmap/internal/providers/xai"
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
	if err := provider.ValidateConfiguration(); err != nil {
		return nil, err
	}
	switch provider.Catalog.Endpoint.Type {
	case catalogs.EndpointTypeOpenAI:
		client, err := openai.NewClient(provider, openAIProviderOptions(provider.ID)...)
		if err != nil {
			return nil, err
		}
		return client, nil
	case catalogs.EndpointTypeAnthropic:
		return anthropic.NewClient(provider), nil
	case catalogs.EndpointTypeCohere:
		return cohere.NewClient(provider), nil
	case catalogs.EndpointTypeCloudflare:
		return cloudflare.NewClient(provider), nil
	case catalogs.EndpointTypeSambaNova:
		return sambanova.NewClient(provider), nil
	case catalogs.EndpointTypeApplication:
		return applicationClient{}, nil
	case catalogs.EndpointTypeTogether:
		return together.NewClient(provider), nil
	case catalogs.EndpointTypeHuggingFace:
		return huggingface.NewClient(provider), nil
	case catalogs.EndpointTypeNVIDIA:
		return nvidia.NewClient(provider), nil
	case catalogs.EndpointTypeDatabricks:
		return databricks.NewClient(provider), nil
	case catalogs.EndpointTypeSnowflake:
		return snowflake.NewClient(provider), nil
	case catalogs.EndpointTypeWatsonx:
		return watsonx.NewClient(provider), nil
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

func openAIProviderOptions(providerID catalogs.ProviderID) []openai.Option {
	switch providerID {
	case catalogs.ProviderIDMistralAI:
		return mistral.Options()
	case catalogs.ProviderIDNovita:
		return novita.Options()
	case catalogs.ProviderIDXAI:
		return xai.Options()
	default:
		return nil
	}
}

type applicationClient struct{}

func (applicationClient) ListModels(context.Context) ([]catalogs.Model, error) { return nil, nil }
func (applicationClient) IsAPIKeyRequired() bool                               { return false }
func (applicationClient) HasAPIKey() bool                                      { return false }

// FetchRawResult contains the result of a raw fetch operation.
type FetchRawResult struct {
	Data       []byte
	Response   *http.Response
	Latency    time.Duration
	RequestURL string
}

// FetchRaw fetches raw response data from a provider's API endpoint.
// This function is used for fetching raw API responses for testdata generation.
// Returns a FetchRawResult containing the data, response headers, latency, and URL.
func FetchRaw(ctx context.Context, provider *catalogs.Provider, endpoint string) (*FetchRawResult, error) {
	// Create transport client configured for this provider
	transportClient := transport.New(provider)

	// Track start time for latency calculation
	startTime := time.Now()

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
		// Bound best-effort draining so a peer cannot turn cleanup into an
		// unbounded read after the payload ceiling is reached.
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
