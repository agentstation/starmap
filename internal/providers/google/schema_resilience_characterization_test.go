package google

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestF009CharacterizationMalformedGooglePageDropsEarlierPages pins the
// pagination failure boundary. P4.8 must preserve accepted records from
// completed pages and quarantine a malformed later-page record with
// observation degradation evidence.
func TestF009CharacterizationMalformedGooglePageDropsEarlierPages(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = w.Write([]byte(`{
				"models":[{"name":"models/page-one","displayName":"Page One"}],
				"nextPageToken":"page-two"
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"models":[
				{"name":"models/page-two-valid","displayName":"Page Two Valid"},
				{"name":"models/page-two-invalid","displayName":"Page Two Invalid","inputTokenLimit":"schema-drift"}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient(&catalogs.Provider{
		ID:   catalogs.ProviderIDGoogleAIStudio,
		Name: "Google AI Studio",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeGoogle,
				URL:  server.URL,
			},
		},
	})
	models, err := client.listModelsAIStudioREST(context.Background())
	if err == nil {
		t.Fatal("F-009 characterization changed: malformed later page did not fail")
	}
	if len(models) != 0 {
		t.Fatalf("F-009 characterization changed: partial pages returned %d models, want 0", len(models))
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2 pages", got)
	}
}
