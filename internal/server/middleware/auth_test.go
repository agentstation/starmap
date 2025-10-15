package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
)

// TestDefaultAuthConfig tests default configuration.
func TestDefaultAuthConfig(t *testing.T) {
	config := DefaultAuthConfig()

	if config.Enabled {
		t.Error("expected Enabled=false by default")
	}
	if config.HeaderName != "X-API-Key" {
		t.Errorf("expected HeaderName=X-API-Key, got %s", config.HeaderName)
	}
	if len(config.PublicPaths) == 0 {
		t.Error("expected default public paths to be set")
	}
	if config.BearerPrefix {
		t.Error("expected BearerPrefix=false by default")
	}
}

// TestAuth tests the Auth middleware with various scenarios.
func TestAuth(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name           string
		config         AuthConfig
		path           string
		headers        map[string]string
		expectedStatus int
		expectedPass   bool
	}{
		{
			name: "auth disabled - always pass",
			config: AuthConfig{
				Enabled:     false,
				APIKey:      "secret-key",
				HeaderName:  "X-API-Key",
				PublicPaths: []string{},
			},
			path:           "/api/v1/models",
			headers:        map[string]string{},
			expectedStatus: http.StatusOK,
			expectedPass:   true,
		},
		{
			name: "public path - always pass",
			config: AuthConfig{
				Enabled:     true,
				APIKey:      "secret-key",
				HeaderName:  "X-API-Key",
				PublicPaths: []string{"/health", "/api/v1/health"},
			},
			path:           "/health",
			headers:        map[string]string{},
			expectedStatus: http.StatusOK,
			expectedPass:   true,
		},
		{
			name: "valid API key in custom header",
			config: AuthConfig{
				Enabled:     true,
				APIKey:      "secret-key",
				HeaderName:  "X-API-Key",
				PublicPaths: []string{},
			},
			path: "/api/v1/models",
			headers: map[string]string{
				"X-API-Key": "secret-key",
			},
			expectedStatus: http.StatusOK,
			expectedPass:   true,
		},
		{
			name: "valid API key in Authorization header",
			config: AuthConfig{
				Enabled:     true,
				APIKey:      "secret-key",
				HeaderName:  "X-API-Key",
				PublicPaths: []string{},
			},
			path: "/api/v1/models",
			headers: map[string]string{
				"Authorization": "Bearer secret-key",
			},
			expectedStatus: http.StatusOK,
			expectedPass:   true,
		},
		{
			name: "valid API key without Bearer prefix",
			config: AuthConfig{
				Enabled:     true,
				APIKey:      "secret-key",
				HeaderName:  "X-API-Key",
				PublicPaths: []string{},
			},
			path: "/api/v1/models",
			headers: map[string]string{
				"Authorization": "secret-key",
			},
			expectedStatus: http.StatusOK,
			expectedPass:   true,
		},
		{
			name: "missing API key",
			config: AuthConfig{
				Enabled:     true,
				APIKey:      "secret-key",
				HeaderName:  "X-API-Key",
				PublicPaths: []string{},
			},
			path:           "/api/v1/models",
			headers:        map[string]string{},
			expectedStatus: http.StatusUnauthorized,
			expectedPass:   false,
		},
		{
			name: "invalid API key",
			config: AuthConfig{
				Enabled:     true,
				APIKey:      "secret-key",
				HeaderName:  "X-API-Key",
				PublicPaths: []string{},
			},
			path: "/api/v1/models",
			headers: map[string]string{
				"X-API-Key": "wrong-key",
			},
			expectedStatus: http.StatusUnauthorized,
			expectedPass:   false,
		},
		{
			name: "invalid Bearer token",
			config: AuthConfig{
				Enabled:     true,
				APIKey:      "secret-key",
				HeaderName:  "X-API-Key",
				PublicPaths: []string{},
			},
			path: "/api/v1/models",
			headers: map[string]string{
				"Authorization": "Bearer wrong-key",
			},
			expectedStatus: http.StatusUnauthorized,
			expectedPass:   false,
		},
		{
			name: "empty API key",
			config: AuthConfig{
				Enabled:     true,
				APIKey:      "secret-key",
				HeaderName:  "X-API-Key",
				PublicPaths: []string{},
			},
			path: "/api/v1/models",
			headers: map[string]string{
				"X-API-Key": "",
			},
			expectedStatus: http.StatusUnauthorized,
			expectedPass:   false,
		},
		{
			name: "custom header name",
			config: AuthConfig{
				Enabled:     true,
				APIKey:      "custom-key",
				HeaderName:  "X-Custom-Auth",
				PublicPaths: []string{},
			},
			path: "/api/v1/models",
			headers: map[string]string{
				"X-Custom-Auth": "custom-key",
			},
			expectedStatus: http.StatusOK,
			expectedPass:   true,
		},
		{
			name: "multiple public paths",
			config: AuthConfig{
				Enabled:     true,
				APIKey:      "secret-key",
				HeaderName:  "X-API-Key",
				PublicPaths: []string{"/health", "/ready", "/api/v1/openapi.json"},
			},
			path:           "/api/v1/openapi.json",
			headers:        map[string]string{},
			expectedStatus: http.StatusOK,
			expectedPass:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test handler that tracks if it was called
			handlerCalled := false
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with auth middleware
			middleware := Auth(tt.config, &logger)
			handler := middleware(testHandler)

			// Create request
			req := httptest.NewRequest("GET", tt.path, nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Record response
			w := httptest.NewRecorder()

			// Execute
			handler.ServeHTTP(w, req)

			// Verify status code
			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Verify handler was called (or not)
			if handlerCalled != tt.expectedPass {
				t.Errorf("expected handler called=%v, got %v", tt.expectedPass, handlerCalled)
			}

			// Verify unauthorized response format
			if !tt.expectedPass {
				contentType := w.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("expected Content-Type=application/json for error, got %s", contentType)
				}

				// Body should contain error JSON
				body := w.Body.String()
				if body == "" {
					t.Error("expected error response body")
				}
				if !contains(body, "UNAUTHORIZED") {
					t.Error("expected UNAUTHORIZED in error response")
				}
			}
		})
	}
}

// TestIsPublicPath tests public path matching.
func TestIsPublicPath(t *testing.T) {
	publicPaths := []string{"/health", "/ready", "/api/v1/openapi.json"}

	tests := []struct {
		path     string
		expected bool
	}{
		{"/health", true},
		{"/ready", true},
		{"/api/v1/openapi.json", true},
		{"/api/v1/models", false},
		{"/health/sub", false}, // Exact match only
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isPublicPath(tt.path, publicPaths)
			if result != tt.expected {
				t.Errorf("isPublicPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestExtractAPIKey tests API key extraction from various headers.
func TestExtractAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		config   AuthConfig
		headers  map[string]string
		expected string
	}{
		{
			name: "from custom header",
			config: AuthConfig{
				HeaderName: "X-API-Key",
			},
			headers: map[string]string{
				"X-API-Key": "test-key",
			},
			expected: "test-key",
		},
		{
			name: "from Authorization with Bearer",
			config: AuthConfig{
				HeaderName: "X-API-Key",
			},
			headers: map[string]string{
				"Authorization": "Bearer test-key",
			},
			expected: "test-key",
		},
		{
			name: "from Authorization without Bearer",
			config: AuthConfig{
				HeaderName: "X-API-Key",
			},
			headers: map[string]string{
				"Authorization": "test-key",
			},
			expected: "test-key",
		},
		{
			name: "custom header takes precedence",
			config: AuthConfig{
				HeaderName: "X-API-Key",
			},
			headers: map[string]string{
				"X-API-Key":     "custom-key",
				"Authorization": "Bearer auth-key",
			},
			expected: "custom-key",
		},
		{
			name: "no API key",
			config: AuthConfig{
				HeaderName: "X-API-Key",
			},
			headers:  map[string]string{},
			expected: "",
		},
		{
			name: "empty header value",
			config: AuthConfig{
				HeaderName: "X-API-Key",
			},
			headers: map[string]string{
				"X-API-Key": "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := extractAPIKey(req, tt.config)
			if result != tt.expected {
				t.Errorf("extractAPIKey() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestAuth_ConcurrentRequests tests auth middleware under concurrent load.
func TestAuth_ConcurrentRequests(t *testing.T) {
	logger := zerolog.Nop()
	config := AuthConfig{
		Enabled:     true,
		APIKey:      "secret-key",
		HeaderName:  "X-API-Key",
		PublicPaths: []string{"/health"},
	}

	middleware := Auth(config, &logger)
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware(testHandler)

	// Run concurrent requests
	const numRequests = 100
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			// Alternate between valid and invalid keys
			key := "secret-key"
			if id%2 == 0 {
				key = "wrong-key"
			}

			req := httptest.NewRequest("GET", "/api/v1/models", nil)
			req.Header.Set("X-API-Key", key)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Verify expected status
			if id%2 == 0 {
				if w.Code != http.StatusUnauthorized {
					t.Errorf("request %d: expected 401, got %d", id, w.Code)
				}
			} else {
				if w.Code != http.StatusOK {
					t.Errorf("request %d: expected 200, got %d", id, w.Code)
				}
			}

			done <- true
		}(i)
	}

	// Wait for all requests
	for i := 0; i < numRequests; i++ {
		<-done
	}
}

// Helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInner(s, substr)))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
