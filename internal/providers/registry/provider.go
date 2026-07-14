// Package registry composes reusable protocol connectors with provider-specific acquisition.
package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/transport"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"

	"github.com/agentstation/starmap/internal/connectors/anthropic"
	"github.com/agentstation/starmap/internal/connectors/google"
	"github.com/agentstation/starmap/internal/connectors/openai"
	"github.com/agentstation/starmap/internal/providers/cloudflare"
	"github.com/agentstation/starmap/internal/providers/cohere"
	"github.com/agentstation/starmap/internal/providers/databricks"
	"github.com/agentstation/starmap/internal/providers/huggingface"
	"github.com/agentstation/starmap/internal/providers/novita"
	"github.com/agentstation/starmap/internal/providers/nvidia"
	"github.com/agentstation/starmap/internal/providers/snowflake"
	"github.com/agentstation/starmap/internal/providers/together"
	"github.com/agentstation/starmap/internal/providers/watsonx"
	"github.com/agentstation/starmap/internal/providers/xai"
)

// Connector retrieves model information from a configured provider endpoint.
type Connector interface {
	// ListModels retrieves all available models from the provider.
	ListModels(ctx context.Context) ([]catalogs.Model, error)
}

type fixtureDecoder interface {
	DecodeModels([]byte) ([]catalogs.Model, error)
}

type captureURLProvider interface {
	CaptureURL() (string, error)
}

// DecodeFixture replays captured response bytes through the selected connector's
// exact live schema and normalization path without transport or a second parser.
func DecodeFixture(source acquisition.Source, payload []byte) ([]catalogs.Model, error) {
	connector, err := New(source)
	if err != nil {
		return nil, err
	}
	decoder, supported := connector.(fixtureDecoder)
	if !supported {
		return nil, &errors.ValidationError{
			Field: "provider.fixture.decoder", Value: source.Config().Endpoint.Type,
			Message: "the selected connector does not support governed offline replay",
		}
	}
	return decoder.DecodeModels(payload)
}

// Supports reports whether an endpoint protocol has an executable connector.
func Supports(endpointType catalogs.EndpointType) bool {
	switch endpointType {
	case catalogs.EndpointTypeOpenAI,
		catalogs.EndpointTypeAnthropic,
		catalogs.EndpointTypeCohere,
		catalogs.EndpointTypeCloudflare,
		catalogs.EndpointTypeTogether,
		catalogs.EndpointTypeHuggingFace,
		catalogs.EndpointTypeNVIDIA,
		catalogs.EndpointTypeDatabricks,
		catalogs.EndpointTypeSnowflake,
		catalogs.EndpointTypeWatsonx,
		catalogs.EndpointTypeGoogle,
		catalogs.EndpointTypeGoogleCloud:
		return true
	default:
		return false
	}
}

// New selects and configures an outbound connector for one resolved source.
func New(source acquisition.Source) (Connector, error) {
	provider := source.Provider()
	config := source.Config()
	switch config.Endpoint.Type {
	case catalogs.EndpointTypeOpenAI:
		client, err := openai.NewClient(source, openAIProviderOptions(provider.ID)...)
		if err != nil {
			return nil, err
		}
		return client, nil
	case catalogs.EndpointTypeAnthropic:
		return anthropic.NewClient(source), nil
	case catalogs.EndpointTypeCohere:
		return cohere.NewClient(source), nil
	case catalogs.EndpointTypeCloudflare:
		return cloudflare.NewClient(source), nil
	case catalogs.EndpointTypeApplication:
		return nil, &errors.ValidationError{Field: "provider.catalog.source.endpoint.type", Value: config.Endpoint.Type, Message: "application metadata is not acquirable"}
	case catalogs.EndpointTypeTogether:
		return together.NewClient(source), nil
	case catalogs.EndpointTypeHuggingFace:
		return huggingface.NewClient(source), nil
	case catalogs.EndpointTypeNVIDIA:
		return nvidia.NewClient(source), nil
	case catalogs.EndpointTypeDatabricks:
		return databricks.NewClient(source), nil
	case catalogs.EndpointTypeSnowflake:
		return snowflake.NewClient(source), nil
	case catalogs.EndpointTypeWatsonx:
		return watsonx.NewClient(source), nil
	case catalogs.EndpointTypeGoogle:
		return google.NewClient(source), nil
	case catalogs.EndpointTypeGoogleCloud:
		return google.NewClient(source), nil
	}
	return nil, &errors.ValidationError{
		Field:   "provider.catalog.source.endpoint.type",
		Value:   config.Endpoint.Type,
		Message: fmt.Sprintf("unsupported endpoint type: %s", config.Endpoint.Type),
	}
}

func openAIProviderOptions(providerID catalogs.ProviderID) []openai.Option {
	switch providerID {
	case catalogs.ProviderIDNovita:
		return novita.Options()
	case catalogs.ProviderIDXAI:
		return xai.Options()
	default:
		return nil
	}
}

// FetchRawResult contains the result of a raw fetch operation.
type FetchRawResult struct {
	Data       []byte
	StatusCode int
	Header     http.Header
	Latency    time.Duration
}

// FetchRaw fetches raw response data from a provider's API endpoint.
// This function is used for fetching raw API responses for testdata generation.
// Returns a FetchRawResult containing the data, response headers, latency, and URL.
func FetchRaw(ctx context.Context, source acquisition.Source) (*FetchRawResult, error) {
	provider := source.Provider()
	endpoint := source.EndpointURL()
	connector, err := New(source)
	if err != nil {
		return nil, err
	}
	if capture, supported := connector.(captureURLProvider); supported {
		endpoint, err = capture.CaptureURL()
		if err != nil {
			return nil, err
		}
	}
	transportClient := transport.New(source.Auth())

	// Track start time for latency calculation
	startTime := time.Now()

	// Make the raw request
	resp, err := transportClient.Get(ctx, endpoint)
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
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Latency:    latency,
	}

	return result, nil
}
