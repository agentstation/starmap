package fireworksai

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

func TestFireworksDeclarativeDeltaUsesEmbeddedMappings(t *testing.T) {
	provider := fixtures.EmbeddedProvider(t, catalogs.ProviderIDFireworksAI)
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
	if model.Authors[0].ID != catalogs.AuthorIDMeta || !model.Features.Tools || !model.Features.ToolCalls || !model.Features.ToolChoice {
		t.Fatalf("identity/tools = %#v", model)
	}
	if len(model.Features.Modalities.Input) != 2 || model.Features.Modalities.Input[1] != catalogs.ModelModalityImage {
		t.Fatalf("modalities = %#v", model.Features.Modalities)
	}
	extension := model.Extensions["fireworks"]
	if extension.Fields["kind"] != "HF_BASE_MODEL" || extension.Fields["supports_chat"] != true {
		t.Fatalf("extensions = %#v", extension)
	}
}
