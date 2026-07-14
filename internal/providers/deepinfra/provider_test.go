package deepinfra

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

func TestDeepInfraDeclarativeDeltaUsesEmbeddedMappings(t *testing.T) {
	provider := fixtures.EmbeddedProvider(t, catalogs.ProviderIDDeepInfra)
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
	if len(models) != 1 {
		t.Fatalf("models = %#v", models)
	}
	model := models[0]
	if model.Description == "" || model.Limits.ContextWindow != 131072 || !model.Features.Reasoning || model.Authors[0].ID != catalogs.AuthorIDDeepSeek {
		t.Fatalf("identity/capabilities = %#v", model)
	}
	if model.Pricing.Tokens.Input.Per1M != 0.4 || model.Pricing.Tokens.Output.Per1M != 1.2 || model.Pricing.Tokens.CacheRead.Per1M != 0.1 || *model.Pricing.Operations.ImageGen != 0.02 {
		t.Fatalf("pricing = %#v", model.Pricing)
	}
	unknown, ok := model.Extensions[catalogs.ProviderIDDeepInfra.String()].Fields["unknown_fields"].([]any)
	if !ok || len(unknown) == 0 {
		t.Fatal("unmapped precision drift was not retained")
	}
}
