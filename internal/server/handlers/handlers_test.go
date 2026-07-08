package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/cmd/application"
	"github.com/agentstation/starmap/internal/server/cache"
	"github.com/agentstation/starmap/internal/server/response"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestHandleListModelsUsesSharedPagination(t *testing.T) {
	cat := catalogs.NewEmpty()
	if err := cat.SetProvider(catalogs.Provider{
		ID:   "provider",
		Name: "Provider",
		Models: map[string]*catalogs.Model{
			"model-a": {ID: "model-a", Name: "Model A"},
			"model-b": {ID: "model-b", Name: "Model B"},
		},
	}); err != nil {
		t.Fatalf("Failed to seed catalog: %v", err)
	}

	h := newTestHandlers(cat)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/models?limit=1&offset=1", nil)
	rec := httptest.NewRecorder()

	h.HandleListModels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var got response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	data := got.Data.(map[string]any)
	models := data["models"].([]any)
	if len(models) != 1 {
		t.Fatalf("Expected one paginated model, got %d", len(models))
	}
	pagination := data["pagination"].(map[string]any)
	if pagination["total"].(float64) != 2 ||
		pagination["limit"].(float64) != 1 ||
		pagination["offset"].(float64) != 1 ||
		pagination["count"].(float64) != 1 {
		t.Fatalf("Unexpected pagination metadata: %#v", pagination)
	}
}

func TestHandleListModelsFiltersByProvider(t *testing.T) {
	cat := catalogs.NewEmpty()
	if err := cat.SetProvider(catalogs.Provider{
		ID:   "openai",
		Name: "OpenAI",
		Models: map[string]*catalogs.Model{
			"openai-model": {ID: "openai-model", Name: "OpenAI Model"},
		},
	}); err != nil {
		t.Fatalf("Failed to seed OpenAI provider: %v", err)
	}
	if err := cat.SetProvider(catalogs.Provider{
		ID:   "anthropic",
		Name: "Anthropic",
		Models: map[string]*catalogs.Model{
			"anthropic-model": {ID: "anthropic-model", Name: "Anthropic Model"},
		},
	}); err != nil {
		t.Fatalf("Failed to seed Anthropic provider: %v", err)
	}

	h := newTestHandlers(cat)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/models?provider=openai", nil)
	rec := httptest.NewRecorder()

	h.HandleListModels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var got response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	data := got.Data.(map[string]any)
	models := data["models"].([]any)
	if len(models) != 1 {
		t.Fatalf("Expected one OpenAI model, got %d", len(models))
	}
	first := models[0].(map[string]any)
	if first["id"] != "openai-model" {
		t.Fatalf("Expected OpenAI model, got %#v", first)
	}
}

func TestHandleListProvidersUsesSharedProviderQuery(t *testing.T) {
	cat := catalogs.NewEmpty()
	if err := cat.SetProvider(catalogs.Provider{ID: "z-provider", Name: "Z Provider"}); err != nil {
		t.Fatalf("Failed to seed z provider: %v", err)
	}
	if err := cat.SetProvider(catalogs.Provider{ID: "a-provider", Name: "A Provider"}); err != nil {
		t.Fatalf("Failed to seed a provider: %v", err)
	}

	h := newTestHandlers(cat)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	rec := httptest.NewRecorder()

	h.HandleListProviders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var got response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	data := got.Data.(map[string]any)
	providers := data["providers"].([]any)
	if len(providers) != 2 {
		t.Fatalf("Expected two providers, got %d", len(providers))
	}
	first := providers[0].(map[string]any)
	if first["id"] != "a-provider" {
		t.Fatalf("Expected providers sorted by ID, got first provider %#v", first)
	}
	if data["count"].(float64) != 2 {
		t.Fatalf("Expected count 2, got %#v", data["count"])
	}
}

func newTestHandlers(cat catalogs.Catalog) *Handlers {
	return &Handlers{
		app: &application.Mock{
			CatalogFunc: func() (catalogs.Catalog, error) {
				return cat, nil
			},
		},
		cache: cache.New(time.Minute, time.Minute),
	}
}
