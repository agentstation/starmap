package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDefaultCORSConfig tests default CORS configuration.
func TestDefaultCORSConfig(t *testing.T) {
	config := DefaultCORSConfig()

	if config.AllowAll {
		t.Error("expected AllowAll=false by default")
	}
	if len(config.AllowedOrigins) == 0 {
		t.Error("expected default allowed origins")
	}
	if config.AllowedOrigins[0] != "*" {
		t.Errorf("expected first origin to be *, got %s", config.AllowedOrigins[0])
	}
}

// TestCORS tests the CORS middleware with various scenarios.
func TestCORS(t *testing.T) {
	tests := []struct {
		name           string
		config         CORSConfig
		method         string
		origin         string
		expectHeaders  map[string]string
		expectNoHeader bool
	}{
		{
			name: "allow all - wildcard",
			config: CORSConfig{
				AllowAll:       true,
				AllowedMethods: []string{"GET", "POST", "OPTIONS"},
				AllowedHeaders: []string{"Content-Type"},
			},
			method: "GET",
			origin: "https://example.com",
			expectHeaders: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
		},
		{
			name: "specific origin allowed",
			config: CORSConfig{
				AllowAll:       false,
				AllowedOrigins: []string{"https://example.com", "https://app.example.com"},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Content-Type"},
			},
			method: "GET",
			origin: "https://example.com",
			expectHeaders: map[string]string{
				"Access-Control-Allow-Origin": "https://example.com",
			},
		},
		{
			name: "origin not allowed",
			config: CORSConfig{
				AllowAll:       false,
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"GET"},
				AllowedHeaders: []string{"Content-Type"},
			},
			method:         "GET",
			origin:         "https://evil.com",
			expectNoHeader: true,
		},
		{
			name: "no origin header - allow all",
			config: CORSConfig{
				AllowAll:       true,
				AllowedMethods: []string{"GET"},
				AllowedHeaders: []string{"Content-Type"},
			},
			method: "GET",
			origin: "",
			expectHeaders: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
		},
		{
			name: "preflight request",
			config: CORSConfig{
				AllowAll:       true,
				AllowedMethods: []string{"GET", "POST", "OPTIONS"},
				AllowedHeaders: []string{"Content-Type", "Authorization"},
			},
			method: "OPTIONS",
			origin: "https://example.com",
			expectHeaders: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test handler
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with CORS middleware
			middleware := CORS(tt.config)
			handler := middleware(testHandler)

			// Create request
			req := httptest.NewRequest(tt.method, "/api/v1/models", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			// Record response
			w := httptest.NewRecorder()

			// Execute
			handler.ServeHTTP(w, req)

			// Verify headers
			if tt.expectNoHeader {
				if w.Header().Get("Access-Control-Allow-Origin") != "" {
					t.Error("expected no CORS headers, but found Access-Control-Allow-Origin")
				}
			} else {
				for header, expectedValue := range tt.expectHeaders {
					actualValue := w.Header().Get(header)
					if actualValue != expectedValue {
						t.Errorf("header %s: expected %q, got %q", header, expectedValue, actualValue)
					}
				}
			}

			// Preflight should return 200
			if tt.method == "OPTIONS" && !tt.expectNoHeader {
				if w.Code != http.StatusOK {
					t.Errorf("preflight: expected status 200, got %d", w.Code)
				}
			}
		})
	}
}

// TestIsOriginAllowed tests origin matching logic.
func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		origin         string
		expected       bool
	}{
		{
			name:           "exact match",
			allowedOrigins: []string{"https://example.com"},
			origin:         "https://example.com",
			expected:       true,
		},
		{
			name:           "no match",
			allowedOrigins: []string{"https://example.com"},
			origin:         "https://evil.com",
			expected:       false,
		},
		{
			name:           "multiple origins - matches first",
			allowedOrigins: []string{"https://example.com", "https://app.example.com"},
			origin:         "https://example.com",
			expected:       true,
		},
		{
			name:           "multiple origins - matches second",
			allowedOrigins: []string{"https://example.com", "https://app.example.com"},
			origin:         "https://app.example.com",
			expected:       true,
		},
		{
			name:           "empty allowed list",
			allowedOrigins: []string{},
			origin:         "https://example.com",
			expected:       false,
		},
		{
			name:           "empty origin",
			allowedOrigins: []string{"https://example.com"},
			origin:         "",
			expected:       false,
		},
		{
			name:           "case sensitive",
			allowedOrigins: []string{"https://example.com"},
			origin:         "https://Example.com",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isOriginAllowed(tt.origin, tt.allowedOrigins)
			if result != tt.expected {
				t.Errorf("isOriginAllowed(%q, %v) = %v, want %v", tt.origin, tt.allowedOrigins, result, tt.expected)
			}
		})
	}
}

// TestCORS_PreflightShortCircuit tests that preflight requests don't call the next handler.
func TestCORS_PreflightShortCircuit(t *testing.T) {
	config := CORSConfig{
		AllowAll:       true,
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type"},
	}

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := CORS(config)
	handler := middleware(testHandler)

	// OPTIONS request (preflight)
	req := httptest.NewRequest("OPTIONS", "/api/v1/models", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Handler should NOT be called for preflight
	if handlerCalled {
		t.Error("expected handler to not be called for preflight request")
	}

	// Should return 200
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestCORS_ActualRequestPassthrough tests that actual requests pass through to handler.
func TestCORS_ActualRequestPassthrough(t *testing.T) {
	config := CORSConfig{
		AllowAll:       true,
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"Content-Type"},
	}

	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := CORS(config)
	handler := middleware(testHandler)

	// GET request (actual request)
	req := httptest.NewRequest("GET", "/api/v1/models", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Handler SHOULD be called for actual request
	if !handlerCalled {
		t.Error("expected handler to be called for actual request")
	}

	// Should have CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("expected Access-Control-Allow-Origin header")
	}
}

// TestCORS_MultipleOrigins tests handling multiple allowed origins.
func TestCORS_MultipleOrigins(t *testing.T) {
	config := CORSConfig{
		AllowAll: false,
		AllowedOrigins: []string{
			"https://example.com",
			"https://app.example.com",
			"https://admin.example.com",
		},
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"Content-Type"},
	}

	middleware := CORS(config)
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware(testHandler)

	// Test each allowed origin
	for _, origin := range config.AllowedOrigins {
		t.Run("origin_"+origin, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/models", nil)
			req.Header.Set("Origin", origin)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			allowedOrigin := w.Header().Get("Access-Control-Allow-Origin")
			if allowedOrigin != origin {
				t.Errorf("expected Access-Control-Allow-Origin=%s, got %s", origin, allowedOrigin)
			}
		})
	}

	// Test disallowed origin
	t.Run("disallowed_origin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/models", nil)
		req.Header.Set("Origin", "https://evil.com")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("expected no Access-Control-Allow-Origin for disallowed origin")
		}
	})
}

// TestCORS_ConcurrentRequests tests CORS middleware under concurrent load.
func TestCORS_ConcurrentRequests(t *testing.T) {
	config := CORSConfig{
		AllowAll:       false,
		AllowedOrigins: []string{"https://example.com", "https://app.example.com"},
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"Content-Type"},
	}

	middleware := CORS(config)
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware(testHandler)

	// Run concurrent requests with different origins
	const numRequests = 100
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			origin := "https://example.com"
			if id%2 == 0 {
				origin = "https://app.example.com"
			}

			req := httptest.NewRequest("GET", "/api/v1/models", nil)
			req.Header.Set("Origin", origin)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Verify CORS header is set correctly
			allowedOrigin := w.Header().Get("Access-Control-Allow-Origin")
			if allowedOrigin != origin {
				t.Errorf("request %d: expected origin %s, got %s", id, origin, allowedOrigin)
			}

			done <- true
		}(i)
	}

	// Wait for all requests
	for i := 0; i < numRequests; i++ {
		<-done
	}
}
