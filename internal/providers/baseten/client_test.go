package baseten

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/internal/providers/openai"
	"github.com/agentstation/starmap/internal/providers/testhelper"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestBasetenModelAPIsPreservePricingLimitsFeaturesAndDualContracts(t *testing.T) {
	t.Setenv("BASETEN_API_KEY", "fixture-token")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(testhelper.LoadTestdata(t, "models_list.json"))
	}))
	defer server.Close()
	provider := &catalogs.Provider{
		ID: catalogs.ProviderIDBaseten, Name: "Baseten Model APIs",
		APIKey:  &catalogs.ProviderAPIKey{Name: "BASETEN_API_KEY", Header: "Authorization", Scheme: catalogs.ProviderAPIKeySchemeBearer},
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: server.URL, AuthRequired: true, AuthorMapping: &catalogs.AuthorMapping{Field: "id", Normalized: map[string]catalogs.AuthorID{"zai-org/*": "zhipu-ai"}}}},
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
	if len(models) != 1 {
		t.Fatalf("models = %#v", models)
	}
	model := models[0]
	if model.Authors[0].ID != "zhipu-ai" || model.Limits.ContextWindow != 262144 || model.Limits.OutputTokens != 65536 {
		t.Fatalf("identity/limits = %#v", model)
	}
	if model.Pricing.Tokens.Input.Per1M != 1.40 || model.Pricing.Tokens.Output.Per1M != 4.40 || model.Pricing.Tokens.Cache.Read.Per1M != 0.26 {
		t.Fatalf("pricing = %#v", model.Pricing)
	}
	if model.Features.Tools || !model.Features.Reasoning || !model.Features.StructuredOutputs || len(model.InvocationAPIs) != 0 {
		t.Fatalf("features/live contracts = %#v/%#v", model.Features, model.InvocationAPIs)
	}
	if len(model.Extensions["baseten"].Fields["unknown_fields"].([]any)) == 0 {
		t.Fatalf("drift evidence = %#v", model.Extensions)
	}
}

func TestBasetenOfferingDefaultsAndMutableFactsLiveInEmbeddedCatalog(t *testing.T) {
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	offering, err := catalog.Offering(catalogs.ProviderIDBaseten, "zai-org/GLM-5.2")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	if offering.Endpoint.BaseURL != "https://inference.baseten.co/v1" || offering.Deployment.Tier != "model-api" ||
		len(offering.Access.APIs) != 2 || offering.Access.APIs[1] != catalogs.InvocationAPIMessages {
		t.Fatalf("configured offering defaults = %#v", offering)
	}
	definition, err := catalog.Definition(offering.DefinitionID)
	if err != nil || definition.Capabilities.Features == nil || !definition.Capabilities.Features.Tools {
		t.Fatalf("catalog capability baseline = %#v/%v", definition, err)
	}
}
