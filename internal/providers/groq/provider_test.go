package groq

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition/testsource"
	"github.com/agentstation/starmap/internal/providers/fixtures"
	"github.com/agentstation/starmap/internal/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestGroqDeclarativeDeltaUsesEmbeddedMappings(t *testing.T) {
	provider := fixtures.EmbeddedProvider(t, catalogs.ProviderIDGroq)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(fixtures.Load(t, "models_list.json"))
	}))
	t.Cleanup(server.Close)
	provider.Credentials = nil
	provider.Catalog.Sources[0].Endpoint.URL = server.URL
	provider.Catalog.Sources[0].Auth = catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone}
	client, err := registry.New(testsource.Unauthenticated(t, &provider))
	if err != nil {
		t.Fatalf("registry.New: %v", err)
	}
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	var model catalogs.Model
	found := false
	for _, candidate := range models {
		if candidate.ID == "llama-3.3-70b-versatile" {
			model = candidate
			found = true
			break
		}
	}
	if !found || model.Limits == nil || model.Limits.ContextWindow != 131072 || model.Limits.OutputTokens != 32768 ||
		model.Status != catalogs.ModelStatusActive || len(model.Authors) != 1 || model.Authors[0].ID != catalogs.AuthorIDMeta {
		t.Fatalf("mapped Groq model = %#v", model)
	}
}
