package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/server/administration"
)

func TestAdministrationAccessPermissions(t *testing.T) {
	manager, token, err := administration.Initialize(t.Context(), administration.Config{StateDirectory: filepath.Join(t.TempDir(), "state"), Audience: "catalog"}, "operator")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	actor, _ := manager.Authenticate(token, "catalog")
	subscriber, err := manager.Create(t.Context(), actor, "reader", administration.Subscriber)
	if err != nil {
		t.Fatal(err)
	}
	unauthorized := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }
	for _, test := range []struct {
		name, path, method, key, header string
		managed                         bool
		status                          int
		principal                       string
	}{
		{"anonymous read without manager", "/api/v1/models", "GET", "", "", false, 204, ""},
		{"disabled administration", "/api/v1/update", "POST", "", "", false, 403, ""},
		{"public health", "/health", "GET", "", "", true, 204, ""},
		{"missing key", "/api/v1/models", "GET", "", "", true, 401, ""},
		{"wrong key", "/api/v1/models", "GET", "inference-key", "Bearer ", true, 401, ""},
		{"wrong scheme", "/api/v1/models", "GET", subscriber, "Basic ", true, 401, ""},
		{"subscriber read", "/api/v1/models", "GET", subscriber, "Bearer ", true, 204, "reader"},
		{"subscriber search", "/api/v1/models/search", "POST", subscriber, "Bearer ", true, 204, "reader"},
		{"subscriber stream", "/api/v1/updates/stream", "GET", subscriber, "Bearer ", true, 204, "reader"},
		{"subscriber write", "/api/v1/update", "POST", subscriber, "Bearer ", true, 403, ""},
		{"subscriber report", "/admin/config/effective", "GET", subscriber, "Bearer ", true, 403, ""},
		{"subscriber metrics", "/metrics", "GET", subscriber, "Bearer ", true, 403, ""},
		{"subscriber stats", "/api/v1/stats", "GET", subscriber, "Bearer ", true, 403, ""},
		{"subscriber operation", "/api/v1/updates/example", "GET", subscriber, "Bearer ", true, 403, ""},
		{"administrator write", "/api/v1/update", "POST", token, "bEaReR ", true, 204, "operator"},
		{"administrator header", "/admin", "GET", token, "X-API-Key", true, 204, "operator"},
	} {
		t.Run(test.name, func(t *testing.T) {
			selected := manager
			if !test.managed {
				selected = nil
			}
			handler := AdministrationAccess(selected, "catalog", "X-API-Key", "/api/v1", []string{"/health"}, unauthorized)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				principal, exists := AdministrativePrincipal(r.Context())
				if exists != (test.principal != "") || principal.ID() != test.principal {
					t.Fatal("unexpected request principal")
				}
				w.WriteHeader(204)
			}))
			request := httptest.NewRequest(test.method, test.path, nil)
			if test.header == "X-API-Key" {
				request.Header.Set(test.header, test.key)
			} else if test.header != "" {
				request.Header.Set("Authorization", test.header+test.key)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d want=%d", response.Code, test.status)
			}
		})
	}
	if _, exists := AdministrativePrincipal(context.Background()); exists {
		t.Fatal("empty context supplied a principal")
	}
}
