package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/internal/embedded/openapi"
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
		config.PathPrefix + "/openapi.yaml",
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
		"/api/v1/openapi.yaml",
		config.PathPrefix + "/models",
		config.PathPrefix + "/updates/stream",
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

func TestAuthenticationOpenAPISecurityMatchesRuntime(t *testing.T) {
	const openAPIGet = "get"
	var spec struct {
		Paths map[string]map[string]struct {
			Security []map[string][]string `json:"security"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(openapi.SpecJSON, &spec); err != nil {
		t.Fatalf("decode OpenAPI JSON: %v", err)
	}

	for _, path := range []string{"/api/v1/openapi.json", "/api/v1/openapi.yaml"} {
		operation, found := spec.Paths[path][openAPIGet]
		if !found {
			t.Fatalf("%s GET operation is absent from OpenAPI", path)
		}
		if security := operation.Security; len(security) != 0 {
			t.Fatalf("%s security = %#v, want public", path, security)
		}
	}
	streamOperation, found := spec.Paths["/api/v1/updates/stream"][openAPIGet]
	if !found {
		t.Fatal("stream GET operation is absent from OpenAPI")
	}
	streamSecurity := streamOperation.Security
	if len(streamSecurity) != 1 {
		t.Fatalf("stream security = %#v, want one API-key requirement", streamSecurity)
	}
	if _, found := streamSecurity[0]["ApiKeyAuth"]; !found {
		t.Fatalf("stream security = %#v, want ApiKeyAuth", streamSecurity)
	}
}
