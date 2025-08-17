package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/agentstation/starmap/pkg/catalogs"
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
	if rb.provider != nil && rb.provider.Catalog != nil && rb.provider.Catalog.APIURL != nil {
		return *rb.provider.Catalog.APIURL
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
func DecodeResponse(resp *http.Response, target interface{}) error {
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log warning but don't override the main error
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decoding JSON response: %w", err)
	}

	return nil
}
