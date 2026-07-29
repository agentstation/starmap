package google

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/internal/sourcepayload"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestF009MalformedGooglePageRetainsValidRecords proves accepted records from
// completed and current pages survive a malformed later-page sibling.
func TestF009MalformedGooglePageRetainsValidRecords(t *testing.T) {
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
	var quarantineErr *sourcepayload.QuarantineError
	if !errors.As(err, &quarantineErr) {
		t.Fatalf("error = %T: %v, want *sourcepayload.QuarantineError", err, err)
	}
	if len(models) != 2 || models[0].ID != "page-one" || models[1].ID != "page-two-valid" {
		t.Fatalf("models = %#v, want valid records from both pages", models)
	}
	if quarantineErr.Report.Rejected != 1 || len(quarantineErr.Report.Issues) != 1 {
		t.Fatalf("quarantine report = %#v, want one rejected record", quarantineErr.Report)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2 pages", got)
	}
}
