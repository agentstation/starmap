package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/application"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestHierarchicalIDRoutePreservesCompleteOpaqueModelID(t *testing.T) {
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{
		ID: "provider", Name: "Provider", Models: map[string]*catalogs.Model{
			"org":       {ID: "org", Name: "Wrong Prefix Model"},
			"org/model": {ID: "org/model", Name: "Hierarchical Model"},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	client, err := starmap.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger := zerolog.Nop()
	app := &application.Mock{
		CatalogFunc: func() (*catalogs.Catalog, error) { return catalog, nil },
		CatalogStateFunc: func() (starmap.CatalogState, error) {
			return starmap.CatalogState{Catalog: catalog, GenerationID: "hierarchical-generation", Sequence: 1}, nil
		},
		StarmapFunc: func(...starmap.Option) (*starmap.Client, error) { return client, nil },
		LoggerFunc:  func() *zerolog.Logger { return &logger },
	}
	server, err := New(app, Config{PathPrefix: "/api/v1", CacheTTL: time.Minute})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	for _, target := range []string{
		"/api/v1/models/org%2Fmodel",
		"/api/v1/models/org/model",
	} {
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"id":"org/model"`) ||
			strings.Contains(recorder.Body.String(), "Wrong Prefix Model") {
			t.Fatalf("GET %s = %d %s", target, recorder.Code, recorder.Body.String())
		}
	}
}
