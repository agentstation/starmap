package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestHierarchicalIDRoutePreservesCompleteOpaqueModelID(t *testing.T) {
	builder := catalogs.NewEmpty()
	author := catalogs.Author{ID: "test-author", Name: "Test Author"}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	for _, model := range []catalogs.Model{
		{ID: "org", Name: "Wrong Prefix Model", Authors: []catalogs.Author{author}},
		{ID: "org--model", Name: "Hierarchical Model", Authors: []catalogs.Author{author}},
	} {
		if err := builder.SetAuthorModel(author.ID, model); err != nil {
			t.Fatalf("SetAuthorModel(%s): %v", model.ID, err)
		}
	}
	if err := builder.SetProvider(catalogs.Provider{
		ID: "provider", Name: "Provider", Models: map[string]*catalogs.Model{
			"org":       {ID: "org", ModelRef: "test-author/org", Name: "Wrong Prefix Model"},
			"org/model": {ID: "org/model", ModelRef: "test-author/org--model", Name: "Hierarchical Model"},
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
	state := starmap.CatalogState{
		Catalog: catalog, GenerationID: "hierarchical-generation", Sequence: 1,
	}
	app := &mockApplication{
		catalog:      catalog,
		catalogState: &state,
		sm:           client,
		logger:       &logger,
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
		if recorder.Code != http.StatusOK ||
			!strings.Contains(recorder.Body.String(), `"id":"test-author/org--model"`) ||
			!strings.Contains(recorder.Body.String(), `"name":"Hierarchical Model"`) ||
			strings.Contains(recorder.Body.String(), "Wrong Prefix Model") {
			t.Fatalf("GET %s = %d %s", target, recorder.Code, recorder.Body.String())
		}
	}
}
