package transport

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

// RequestBuilder helps build HTTP requests with provider-specific configurations.
type RequestBuilder struct {
	provider *catalogs.Provider
}

// NewRequestBuilder creates a new request builder for a provider.
func NewRequestBuilder(provider *catalogs.Provider) *RequestBuilder {
	return &RequestBuilder{provider: provider}
}

// GetBaseURL returns the base URL for API requests.
func (rb *RequestBuilder) GetBaseURL() string {
	if rb.provider != nil && rb.provider.Catalog != nil {
		return rb.provider.CatalogEndpointURL()
	}
	return ""
}

// GetModelsURL returns the URL for listing models.
func (rb *RequestBuilder) GetModelsURL(defaultURL string) string {
	if baseURL := rb.GetBaseURL(); baseURL != "" {
		return baseURL
	}
	return defaultURL
}

// AddProviderHeaders adds provider-specific headers to a request.
func (rb *RequestBuilder) AddProviderHeaders(req *http.Request) {
	if rb.provider == nil {
		return
	}

	switch rb.provider.ID {
	case catalogs.ProviderIDAnthropic:
		req.Header.Set("anthropic-version", "2023-06-01")
	}
}

// DecodeResponse decodes a JSON response into the target structure.
func DecodeResponse(resp *http.Response, target any) error {
	if resp == nil || resp.Body == nil {
		return &errors.ValidationError{Field: "response", Message: "response and body are required"}
	}
	if target == nil {
		return &errors.ValidationError{Field: "target", Message: "decode target is required"}
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logging.Warn().Err(err).Msg("Failed to close provider response body")
		}
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, constants.MaxSourcePayloadBytes+1))
	if err != nil {
		return errors.WrapIO("read", "response body", err)
	}
	if len(body) > constants.MaxSourcePayloadBytes {
		return &errors.ValidationError{
			Field: "response.body", Value: len(body),
			Message: "exceeds maximum source payload size",
		}
	}
	if resp.StatusCode != http.StatusOK {
		return &errors.APIError{
			Provider:   "unknown", // Provider not available in this context
			StatusCode: resp.StatusCode,
			Message:    "unexpected provider response status",
		}
	}
	if err := sourcepayload.ValidateJSON(body); err != nil {
		return err
	}

	if err := json.Unmarshal(body, target); err != nil {
		return errors.WrapParse("json", "response", err)
	}

	return nil
}
