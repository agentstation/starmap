package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/server/middleware"
	"github.com/agentstation/starmap/server/administration"
)

func administratorRequests(t *testing.T, handler *Handlers) func(*http.Request) *http.Request {
	t.Helper()
	manager, token, err := administration.Initialize(t.Context(), administration.Config{StateDirectory: filepath.Join(t.TempDir(), "state"), Audience: "test"}, "operator")
	if err != nil {
		t.Fatal(err)
	}
	handler.administration = manager
	t.Cleanup(func() {
		if handler.operations != nil {
			_ = handler.operations.Close(context.Background())
		}
		_ = manager.Close()
	})
	return func(request *http.Request) *http.Request {
		request.Header.Set("Authorization", "Bearer "+token)
		var authenticated *http.Request
		middleware.AdministrationAccess(manager, "test", "X-API-Key", "/api/v1", nil, func(http.ResponseWriter, *http.Request) { t.Fatal("test administrator was rejected") })(
			http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { authenticated = r }),
		).ServeHTTP(httptest.NewRecorder(), request)
		return authenticated
	}
}
