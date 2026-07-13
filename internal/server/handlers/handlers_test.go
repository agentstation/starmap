package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/utc"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/application"
	"github.com/agentstation/starmap/internal/server/cache"
	"github.com/agentstation/starmap/internal/server/response"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestHandleUpdateRequiresWritableStoreBeforeSync(t *testing.T) {
	client, err := starmap.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h := &Handlers{
		app: &application.Mock{StarmapFunc: func(...starmap.Option) (*starmap.Client, error) {
			return client, nil
		}},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/update", nil)
	rec := httptest.NewRecorder()

	h.HandleUpdate(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	var got response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Decode response: %v", err)
	}
	if got.Error == nil || got.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("response error = %#v, want INTERNAL_ERROR", got.Error)
	}
}

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

func TestHandleListModelsRejectsInvalidSortAndFilterParameters(t *testing.T) {
	h := newTestHandlers(catalogs.NewEmpty())
	for _, query := range []string{"sort=price", "limit=invalid", "feature=invented", "min_context=100&max_context=10"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/models?"+query, nil)
		recorder := httptest.NewRecorder()
		h.HandleListModels(recorder, req)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("query %q status = %d, want %d: %s", query, recorder.Code, http.StatusBadRequest, recorder.Body.String())
		}
	}
}

func TestHandleListModelsFiltersByProvider(t *testing.T) {
	cat := catalogs.NewEmpty()
	if err := cat.SetProvider(catalogs.Provider{
		ID:   "openai",
		Name: "OpenAI",
		Models: map[string]*catalogs.Model{
			"shared-model": {ID: "shared-model", Name: "OpenAI Offering"},
		},
	}); err != nil {
		t.Fatalf("Failed to seed OpenAI provider: %v", err)
	}
	if err := cat.SetProvider(catalogs.Provider{
		ID:   "anthropic",
		Name: "Anthropic",
		Models: map[string]*catalogs.Model{
			"shared-model": {ID: "shared-model", Name: "Anthropic Offering"},
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
	if first["id"] != "shared-model" || first["name"] != "Anthropic Offering" {
		t.Fatalf("Expected OpenAI model, got %#v", first)
	}
}

func TestHandleSearchModelsAppliesReleaseDateRange(t *testing.T) {
	cat := catalogs.NewEmpty()
	if err := cat.SetProvider(catalogs.Provider{
		ID:   "provider",
		Name: "Provider",
		Models: map[string]*catalogs.Model{
			"old-model": {
				ID:   "old-model",
				Name: "Old Model",
				Metadata: &catalogs.ModelMetadata{
					ReleaseDate: utc.New(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
			},
			"new-model": {
				ID:   "new-model",
				Name: "New Model",
				Metadata: &catalogs.ModelMetadata{
					ReleaseDate: utc.New(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
				},
			},
		},
	}); err != nil {
		t.Fatalf("Failed to seed provider: %v", err)
	}

	h := newTestHandlers(cat)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/models/search",
		strings.NewReader(`{"release_date":{"after":"2024-01-01T00:00:00Z"}}`),
	)
	rec := httptest.NewRecorder()

	h.HandleSearchModels(rec, req)

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
		t.Fatalf("Expected one model, got %d", len(models))
	}
	first := models[0].(map[string]any)
	if first["id"] != "new-model" {
		t.Fatalf("Expected new-model, got %#v", first)
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

func newTestHandlers(cat *catalogs.Builder) *Handlers {
	return &Handlers{
		app: &application.Mock{
			CatalogFunc: func() (*catalogs.Catalog, error) {
				return cat.Build()
			},
		},
		cache: cache.New(time.Minute, time.Minute),
	}
}
