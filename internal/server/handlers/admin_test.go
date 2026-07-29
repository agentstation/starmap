package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/server/cache"
	"github.com/agentstation/starmap/internal/server/sse"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestStatsExposesCatalogAndPublicationHealthSeparately(t *testing.T) {
	t.Parallel()

	client, err := starmap.New()
	if err != nil {
		t.Fatalf("starmap.New: %v", err)
	}
	logger := zerolog.Nop()
	broadcaster, err := sse.NewBroadcaster(sse.Config{}, &logger)
	if err != nil {
		t.Fatalf("NewBroadcaster: %v", err)
	}
	handler := New(
		&testApplication{
			CatalogFunc: func() (*catalogs.Catalog, error) {
				return client.Catalog(), nil
			},
			CatalogStateFunc: func() (starmap.CatalogState, error) {
				return client.CurrentCatalogState(), nil
			},
			StarmapFunc: func(...starmap.Option) (*starmap.Client, error) {
				return client, nil
			},
		},
		cache.New(time.Minute, time.Minute),
		broadcaster,
		&logger,
		time.Now().Add(-time.Minute),
	)

	recorder := httptest.NewRecorder()
	handler.HandleStats(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil),
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body)
	}
	var responseBody map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	data := responseBody["data"].(map[string]any)
	catalog := data["catalog"].(map[string]any)
	realtime := data["realtime"].(map[string]any)
	if catalog["generation_id"] == "" ||
		catalog["generated_at"] == nil ||
		catalog["age_seconds"] == nil {
		t.Fatalf("catalog health = %#v", catalog)
	}
	if realtime["sse"] == nil || realtime["publication"] == nil {
		t.Fatalf("realtime health = %#v", realtime)
	}
}
