package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticationPublicPathsFollowConfiguredPrefix(t *testing.T) {
	t.Setenv("API_KEY", "test-server-key")
	config := DefaultConfig()
	config.PathPrefix = "/catalog/v2"
	config.AuthEnabled = true

	server, err := New(newMockApplication(), config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	handler := server.Handler()

	for _, path := range []string{
		"/health",
		config.PathPrefix + "/health",
		config.PathPrefix + "/ready",
		config.PathPrefix + "/openapi.json",
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(
			recorder,
			httptest.NewRequest(http.MethodGet, path, nil),
		)
		if recorder.Code == http.StatusUnauthorized {
			t.Fatalf("public path %q required authentication", path)
		}
	}

	for _, path := range []string{
		"/api/v1/health",
		"/api/v1/ready",
		"/api/v1/openapi.json",
		config.PathPrefix + "/models",
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(
			recorder,
			httptest.NewRequest(http.MethodGet, path, nil),
		)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf(
				"protected path %q status = %d, want %d",
				path,
				recorder.Code,
				http.StatusUnauthorized,
			)
		}
	}
}
