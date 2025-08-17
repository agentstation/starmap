package transport

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestNoAuth tests that NoAuth applies no authentication.
func TestNoAuth(t *testing.T) {
	auth := &NoAuth{}
	req := &http.Request{
		Header: make(http.Header),
	}

	auth.Apply(req, "test-api-key")

	// Should not have any authentication headers
	if len(req.Header) != 0 {
		t.Errorf("Expected no headers, got %d", len(req.Header))
	}
}

// TestBearerAuth tests Bearer token authentication.
func TestBearerAuth(t *testing.T) {
	auth := &BearerAuth{}
	req := &http.Request{
		Header: make(http.Header),
	}

	auth.Apply(req, "test-api-key")

	authHeader := req.Header.Get("Authorization")
	expected := "Bearer test-api-key"
	if authHeader != expected {
		t.Errorf("Expected Authorization header '%s', got '%s'", expected, authHeader)
	}
}

// TestHeaderAuth tests custom header authentication.
func TestHeaderAuth(t *testing.T) {
	auth := &HeaderAuth{Header: "x-api-key"}
	req := &http.Request{
		Header: make(http.Header),
	}

	auth.Apply(req, "test-api-key")

	headerValue := req.Header.Get("x-api-key")
	if headerValue != "test-api-key" {
		t.Errorf("Expected x-api-key header 'test-api-key', got '%s'", headerValue)
	}

	// Should not have Authorization header
	if req.Header.Get("Authorization") != "" {
		t.Error("Should not have Authorization header")
	}
}

// TestQueryAuth tests query parameter authentication.
func TestQueryAuth(t *testing.T) {
	auth := &QueryAuth{Param: "key"}

	// Test with valid URL
	reqURL, _ := url.Parse("https://example.com/api/models")
	req := &http.Request{
		URL:    reqURL,
		Header: make(http.Header),
	}

	auth.Apply(req, "test-api-key")

	// Check that the query parameter was added
	if req.URL.Query().Get("key") != "test-api-key" {
		t.Errorf("Expected query param 'key=test-api-key', got '%s'", req.URL.RawQuery)
	}

	// Test with existing query parameters
	reqURL2, _ := url.Parse("https://example.com/api/models?existing=value")
	req2 := &http.Request{
		URL:    reqURL2,
		Header: make(http.Header),
	}

	auth.Apply(req2, "test-api-key")

	query := req2.URL.Query()
	if query.Get("key") != "test-api-key" {
		t.Errorf("Expected query param 'key=test-api-key', got '%s'", query.Get("key"))
	}
	if query.Get("existing") != "value" {
		t.Errorf("Expected existing param to be preserved, got '%s'", query.Get("existing"))
	}

	// Test with nil URL (should not panic)
	req3 := &http.Request{
		URL:    nil,
		Header: make(http.Header),
	}

	auth.Apply(req3, "test-api-key")
	// Should not panic and should do nothing
}

// TestProviderAuth tests provider-specific authentication from YAML configuration.
func TestProviderAuth(t *testing.T) {
	tests := []struct {
		name           string
		provider       *catalogs.Provider
		expectedHeader string
		expectedValue  string
		queryParam     string
	}{
		{
			name: "Groq Bearer Auth",
			provider: &catalogs.Provider{
				ID: catalogs.ProviderIDGroq,
				APIKey: &catalogs.ProviderAPIKey{
					Header: "Authorization",
					Scheme: catalogs.ProviderAPIKeySchemeBearer,
				},
			},
			expectedHeader: "Authorization",
			expectedValue:  "Bearer test-api-key",
		},
		{
			name: "Anthropic Direct Header Auth",
			provider: &catalogs.Provider{
				ID: catalogs.ProviderIDAnthropic,
				APIKey: &catalogs.ProviderAPIKey{
					Header: "x-api-key",
					Scheme: catalogs.ProviderAPIKeySchemeDirect,
				},
			},
			expectedHeader: "x-api-key",
			expectedValue:  "test-api-key",
		},
		{
			name: "Google AI Studio Query Auth",
			provider: &catalogs.Provider{
				ID: catalogs.ProviderIDGoogleAIStudio,
				APIKey: &catalogs.ProviderAPIKey{
					QueryParam: "key",
				},
			},
			queryParam: "key",
		},
		{
			name: "Default Authorization Header",
			provider: &catalogs.Provider{
				ID: catalogs.ProviderIDOpenAI,
				APIKey: &catalogs.ProviderAPIKey{
					Scheme: catalogs.ProviderAPIKeySchemeBearer,
				},
			},
			expectedHeader: "Authorization",
			expectedValue:  "Bearer test-api-key",
		},
		{
			name: "Basic Auth Scheme",
			provider: &catalogs.Provider{
				ID: catalogs.ProviderIDOpenAI,
				APIKey: &catalogs.ProviderAPIKey{
					Header: "Authorization",
					Scheme: catalogs.ProviderAPIKeySchemeBasic,
				},
			},
			expectedHeader: "Authorization",
			expectedValue:  "Basic test-api-key",
		},
		{
			name: "Empty Scheme (Direct)",
			provider: &catalogs.Provider{
				ID: catalogs.ProviderIDOpenAI,
				APIKey: &catalogs.ProviderAPIKey{
					Header: "Custom-Header",
					Scheme: "",
				},
			},
			expectedHeader: "Custom-Header",
			expectedValue:  "test-api-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &ProviderAuth{Provider: tt.provider}

			// Setup request
			reqURL, _ := url.Parse("https://example.com/api/models")
			req := &http.Request{
				URL:    reqURL,
				Header: make(http.Header),
			}

			auth.Apply(req, "test-api-key")

			if tt.queryParam != "" {
				// Test query parameter auth
				queryValue := req.URL.Query().Get(tt.queryParam)
				if queryValue != "test-api-key" {
					t.Errorf("Expected query param '%s=test-api-key', got '%s'", tt.queryParam, queryValue)
				}
			} else {
				// Test header auth
				headerValue := req.Header.Get(tt.expectedHeader)
				if headerValue != tt.expectedValue {
					t.Errorf("Expected header '%s: %s', got '%s'", tt.expectedHeader, tt.expectedValue, headerValue)
				}
			}
		})
	}
}

// TestProviderAuthNilProvider tests ProviderAuth with nil provider.
func TestProviderAuthNilProvider(t *testing.T) {
	auth := &ProviderAuth{Provider: nil}
	req := &http.Request{
		Header: make(http.Header),
	}

	auth.Apply(req, "test-api-key")

	// Should not panic and should do nothing
	if len(req.Header) != 0 {
		t.Errorf("Expected no headers with nil provider, got %d", len(req.Header))
	}
}

// TestProviderAuthNilAPIKey tests ProviderAuth with nil APIKey.
func TestProviderAuthNilAPIKey(t *testing.T) {
	provider := &catalogs.Provider{
		ID:     catalogs.ProviderIDGroq,
		APIKey: nil,
	}

	auth := &ProviderAuth{Provider: provider}
	req := &http.Request{
		Header: make(http.Header),
	}

	auth.Apply(req, "test-api-key")

	// Should not panic and should do nothing
	if len(req.Header) != 0 {
		t.Errorf("Expected no headers with nil APIKey, got %d", len(req.Header))
	}
}

// TestProviderAuthUnknownScheme tests ProviderAuth with unknown authentication scheme.
func TestProviderAuthUnknownScheme(t *testing.T) {
	provider := &catalogs.Provider{
		ID: catalogs.ProviderIDGroq,
		APIKey: &catalogs.ProviderAPIKey{
			Header: "Authorization",
			Scheme: "Unknown",
		},
	}

	auth := &ProviderAuth{Provider: provider}
	req := &http.Request{
		Header: make(http.Header),
	}

	auth.Apply(req, "test-api-key")

	// Unknown scheme should be treated as direct
	authHeader := req.Header.Get("Authorization")
	if authHeader != "test-api-key" {
		t.Errorf("Expected Authorization header 'test-api-key' for unknown scheme, got '%s'", authHeader)
	}
}

// TestProviderAuthBothQueryAndHeader tests behavior when both query param and header are specified.
func TestProviderAuthBothQueryAndHeader(t *testing.T) {
	provider := &catalogs.Provider{
		ID: catalogs.ProviderIDGoogleAIStudio,
		APIKey: &catalogs.ProviderAPIKey{
			Header:     "Authorization",
			Scheme:     catalogs.ProviderAPIKeySchemeBearer,
			QueryParam: "key",
		},
	}

	auth := &ProviderAuth{Provider: provider}

	reqURL, _ := url.Parse("https://example.com/api/models")
	req := &http.Request{
		URL:    reqURL,
		Header: make(http.Header),
	}

	auth.Apply(req, "test-api-key")

	// Query param should take precedence
	queryValue := req.URL.Query().Get("key")
	if queryValue != "test-api-key" {
		t.Errorf("Expected query param 'key=test-api-key', got '%s'", queryValue)
	}

	// Should not have Authorization header when query param is used
	authHeader := req.Header.Get("Authorization")
	if authHeader != "" {
		t.Errorf("Expected no Authorization header when using query param, got '%s'", authHeader)
	}
}
