package xai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/internal/providers/openai"
	"github.com/agentstation/starmap/internal/providers/testhelper"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestXAILanguageModelMapsExactInventoryContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(testhelper.LoadTestdata(t, "models_list.json"))
	}))
	defer server.Close()
	provider := xaiTestProvider(server.URL)
	provider.Catalog.Endpoint.AuthRequired = false
	client, err := openai.NewClient(provider, Options()...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	models, err := client.ListModels(context.Background())
	if err != nil || len(models) != 1 {
		t.Fatalf("response shape = %#v err=%v", models, err)
	}
	model := &models[0]
	if model.Status != catalogs.ModelStatusActive || model.Limits == nil || model.Limits.ContextWindow != 256000 {
		t.Fatalf("lifecycle/limits = %q/%#v", model.Status, model.Limits)
	}
	if len(model.Authors) != 1 || model.Authors[0].ID != catalogs.AuthorIDXAI || model.Features == nil ||
		!slices.Contains(model.Features.Modalities.Input, catalogs.ModelModalityImage) {
		t.Fatalf("authorship/features = %#v/%#v", model.Authors, model.Features)
	}
	pricing := model.Pricing
	if pricing == nil || pricing.Tokens == nil || pricing.Tokens.Input == nil || pricing.Tokens.Input.Per1M != 2 ||
		pricing.Tokens.Output == nil || pricing.Tokens.Output.Per1M != 8 || pricing.Tokens.Cache == nil ||
		pricing.Tokens.Cache.Read == nil || pricing.Tokens.Cache.Read.Per1M != 0.2 || pricing.Operations == nil ||
		pricing.Operations.WebSearch == nil || *pricing.Operations.WebSearch != 1 || len(pricing.Tiers) != 1 {
		t.Fatalf("pricing = %#v", pricing)
	}
	tier := pricing.Tiers[0]
	if tier.Size != 128000 || tier.Type != catalogs.ModelPricingTierTypeContext || tier.Tokens.Input.Per1M != 4 ||
		tier.Tokens.Output.Per1M != 16 || tier.Tokens.Cache.Read.Per1M != 0.2 {
		t.Fatalf("long-context tier = %#v", tier)
	}
	extension := model.Extensions[string(catalogs.ProviderIDXAI)].Fields
	aliases, ok := extension["aliases"].([]any)
	if !ok || len(aliases) != 1 || aliases[0] != "grok-latest" || extension["fingerprint"] != "fp_fixture" ||
		extension["version"] != "1.0" || extension["prompt_image_token_price_cents_per_100m"] != int64(30000) {
		t.Fatalf("extension = %#v", extension)
	}
}

func TestXAIListModelsUsesLanguageModelsBearerContractAndCanonicalSeparation(t *testing.T) {
	t.Setenv("XAI_API_KEY", "xai-fixture-key")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Method != http.MethodGet || request.URL.Path != "/v1/language-models" ||
			request.Header.Get("Authorization") != "Bearer xai-fixture-key" {
			t.Errorf("request = %s %s auth %q", request.Method, request.URL.Path, request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"models":[{"id":"grok-4.5","owned_by":"xai","input_modalities":["text","image"],"output_modalities":["text"],"prompt_text_token_price":20000,"completion_text_token_price":60000}]}`))
	}))
	defer server.Close()
	provider := xaiTestProvider(server.URL + "/v1/language-models")
	provider.LoadAPIKey()
	client, err := openai.NewClient(provider, Options()...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if requests.Load() != 1 || len(models) != 1 {
		t.Fatalf("requests/models = %d/%#v", requests.Load(), models)
	}
	provider.Models = map[string]*catalogs.Model{models[0].ID: &models[0]}
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	offering, err := catalog.Offering(catalogs.ProviderIDXAI, "grok-4.5")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	definition, err := catalog.Definition(offering.DefinitionID)
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if offering.ProviderID != catalogs.ProviderIDXAI || len(definition.AuthorIDs) != 1 || definition.AuthorIDs[0] != catalogs.AuthorIDXAI {
		t.Fatalf("provider/author separation = %#v/%#v", offering, definition)
	}
}

func TestXAILanguageModelsMissingModelsFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()
	provider := xaiTestProvider(server.URL)
	provider.Catalog.Endpoint.AuthRequired = false
	client, newErr := openai.NewClient(provider, Options()...)
	if newErr != nil {
		t.Fatalf("NewClient: %v", newErr)
	}
	_, err := client.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected schema-drift failure")
	}
}

func TestXAILanguageModelsRejectsNegativeSourcePricing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"models":[{"id":"grok-invalid","owned_by":"xai","prompt_text_token_price":-1}]}`))
	}))
	defer server.Close()
	provider := xaiTestProvider(server.URL)
	provider.Catalog.Endpoint.AuthRequired = false
	client, newErr := openai.NewClient(provider, Options()...)
	if newErr != nil {
		t.Fatalf("NewClient: %v", newErr)
	}
	_, err := client.ListModels(context.Background())
	if err == nil {
		t.Fatal("expected invalid-pricing failure")
	}
}

func xaiTestProvider(endpoint string) *catalogs.Provider {
	return &catalogs.Provider{
		ID: catalogs.ProviderIDXAI, Name: "xAI",
		APIKey: &catalogs.ProviderAPIKey{Name: "XAI_API_KEY", Header: "Authorization", Scheme: catalogs.ProviderAPIKeySchemeBearer},
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
			Type: catalogs.EndpointTypeOpenAI, URL: endpoint, ResponseCollection: "models", AuthRequired: endpoint != "",
			AuthorMapping: &catalogs.AuthorMapping{Field: "owned_by", Normalized: map[string]catalogs.AuthorID{
				"xai": catalogs.AuthorIDXAI,
			}},
		}},
	}
}
