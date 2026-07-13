package hyperbolic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/providers/openai"
	"github.com/agentstation/starmap/internal/providers/testhelper"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestHyperbolicLiveInventoryPreservesIdentityAndDriftWithoutCuratedFacts(t *testing.T) {
	t.Setenv("HYPERBOLIC_API_KEY", "fixture-token")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(testhelper.LoadTestdata(t, "models_list.json"))
	}))
	defer server.Close()

	provider := &catalogs.Provider{
		ID: catalogs.ProviderIDHyperbolic, Name: "Hyperbolic Serverless Inference",
		APIKey: &catalogs.ProviderAPIKey{Name: "HYPERBOLIC_API_KEY", Header: "Authorization", Scheme: catalogs.ProviderAPIKeySchemeBearer},
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
			Type: catalogs.EndpointTypeOpenAI, URL: server.URL, AuthRequired: true,
			AuthorMapping: &catalogs.AuthorMapping{Field: "id", Normalized: map[string]catalogs.AuthorID{
				"meta-llama/*": catalogs.AuthorIDMeta, "openai/*": catalogs.AuthorIDOpenAI,
			}},
		}},
	}
	provider.LoadAPIKey()
	client, err := openai.NewClient(provider)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}

	llama := findModel(t, models, "meta-llama/Llama-3.3-70B-Instruct")
	if llama.Authors[0].ID != catalogs.AuthorIDMeta || llama.Pricing != nil {
		t.Fatalf("llama live identity/pricing = %#v", llama)
	}
	if len(llama.Extensions["hyperbolic"].Fields["unknown_fields"].([]any)) == 0 {
		t.Fatalf("llama drift evidence = %#v", llama.Extensions)
	}

}

func TestHyperbolicCuratedFactsLiveInEmbeddedCatalog(t *testing.T) {
	builder, err := catalogs.NewFromPath(filepath.Join("..", "..", "..", "internal", "embedded", "catalog"))
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	llama, err := catalog.Offering(catalogs.ProviderIDHyperbolic, "meta-llama/Llama-3.3-70B-Instruct")
	if err != nil || llama.Pricing == nil || llama.Pricing.Tokens.Input.Per1M != 0.4 {
		t.Fatalf("llama catalog facts = %#v/%v", llama, err)
	}
	if llama.Endpoint.BaseURL != "https://api.hyperbolic.xyz/v1" || llama.Deployment.Tier != "pay-per-token" || len(llama.Access.APIs) != 1 {
		t.Fatalf("llama configured offering defaults = %#v", llama)
	}
	definition, err := catalog.Definition(llama.DefinitionID)
	if err != nil || definition.Capabilities.Features == nil || !definition.Capabilities.Features.Tools {
		t.Fatalf("llama catalog capabilities = %#v/%v", definition, err)
	}
	sunset, err := catalog.Offering(catalogs.ProviderIDHyperbolic, "openai/gpt-oss-120b")
	if err != nil || sunset.Lifecycle != catalogs.OfferingLifecycleDeprecated || sunset.Pricing.Tokens.Input.Per1M != 0.3 {
		t.Fatalf("sunset catalog facts = %#v/%v", sunset, err)
	}
	base, err := catalog.Offering(catalogs.ProviderIDHyperbolic, "meta-llama/Meta-Llama-3.1-405B")
	if err != nil || len(base.Access.APIs) != 1 || base.Access.APIs[0] != catalogs.InvocationAPICompletions || base.Lifecycle != catalogs.OfferingLifecycleDeprecated {
		t.Fatalf("base catalog contract = %#v/%v", base, err)
	}
}

func findModel(t *testing.T, models []catalogs.Model, id string) *catalogs.Model {
	t.Helper()
	for i := range models {
		if models[i].ID == id {
			return &models[i]
		}
	}
	t.Fatalf("model %q not found", id)
	return nil
}
