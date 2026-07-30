package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestModelSearchUsesDocumentedRouteOnly(t *testing.T) {
	srv, err := New(newMockApplication(), Config{
		PathPrefix: "/api/v1",
		CacheTTL:   time.Minute,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	body := `{"name_contains":"kimi","provider":"moonshot-ai","max_results":3}`
	canonical := httptest.NewRecorder()
	srv.Handler().ServeHTTP(
		canonical,
		httptest.NewRequest(
			http.MethodPost,
			"/api/v1/models/search",
			strings.NewReader(body),
		),
	)
	if canonical.Code != http.StatusOK {
		t.Fatalf(
			"POST /api/v1/models/search = %d %s",
			canonical.Code,
			canonical.Body.String(),
		)
	}

	undocumented := httptest.NewRecorder()
	srv.Handler().ServeHTTP(
		undocumented,
		httptest.NewRequest(
			http.MethodPost,
			"/api/v1/models",
			strings.NewReader(body),
		),
	)
	if undocumented.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"POST /api/v1/models = %d, want %d",
			undocumented.Code,
			http.StatusMethodNotAllowed,
		)
	}
}
