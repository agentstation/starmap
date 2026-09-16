package server

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/server/administration"
)

func administrativeServer(t *testing.T, server *Server) string {
	t.Helper()
	manager, token, err := administration.Initialize(t.Context(), administration.Config{StateDirectory: filepath.Join(t.TempDir(), "state"), Audience: "test"}, "operator")
	if err != nil {
		t.Fatal(err)
	}
	server.administration, server.audience = manager, "test"
	t.Cleanup(func() { _ = server.Shutdown(context.Background()); _ = manager.Close() })
	return token
}

func authenticatedHandler(handler http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+token)
		handler.ServeHTTP(w, r)
	})
}
