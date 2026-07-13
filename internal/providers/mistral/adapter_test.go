package mistral

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/internal/providers/openai"
	"github.com/agentstation/starmap/internal/providers/testhelper"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestMistralModelCardMapsLiveCapabilitiesAndLifecycleWithoutCuratedPricing(t *testing.T) {
	provider := &catalogs.Provider{
		ID: catalogs.ProviderIDMistralAI, Name: "Mistral AI",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				AuthorMapping: &catalogs.AuthorMapping{Field: "owned_by", Normalized: map[string]catalogs.AuthorID{
					"mistralai": catalogs.AuthorIDMistralAI,
				}},
			},
		},
	}
	client, err := openai.NewClient(provider, Options()...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	var response openai.Response
	testhelper.LoadJSON(t, "models_list.json", &response)
	model := client.ConvertToModel(response.Data[0])
	if model.ID != "mistral-small-latest" || len(model.Authors) != 1 || model.Authors[0].ID != catalogs.AuthorIDMistralAI {
		t.Fatalf("identity/authorship = %#v", model)
	}
	if model.Status != catalogs.ModelStatusActive || model.Limits == nil || model.Limits.ContextWindow != 262144 {
		t.Fatalf("lifecycle/limits = %#v/%#v", model.Status, model.Limits)
	}
	if model.Features == nil || !model.Features.Tools || !model.Features.ToolCalls || !model.Features.Streaming ||
		!slices.Contains(model.Features.Modalities.Input, catalogs.ModelModalityImage) {
		t.Fatalf("capabilities = %#v", model.Features)
	}
	if model.Metadata == nil || !slices.Contains(model.Metadata.Tags, catalogs.ModelTagChat) ||
		!slices.Contains(model.Metadata.Tags, catalogs.ModelTagCoding) ||
		!slices.Contains(model.Metadata.Tags, catalogs.ModelTagFunctionCalling) ||
		!slices.Contains(model.Metadata.Tags, catalogs.ModelTagVision) {
		t.Fatalf("categories = %#v", model.Metadata)
	}
	if model.Pricing != nil {
		t.Fatalf("live inventory invented curated pricing = %#v", model.Pricing)
	}
	extension := model.Extensions[string(catalogs.ProviderIDMistralAI)].Fields
	aliases, ok := extension["aliases"].([]any)
	if !ok || len(aliases) != 1 || aliases[0] != "mistral-small-2603" || extension["completion_fim"] != true {
		t.Fatalf("provider extension = %#v", extension)
	}
}

func TestMistralCuratedPricingLivesInEmbeddedCatalog(t *testing.T) {
	builder, err := catalogs.NewFromPath(filepath.Join("..", "..", "..", "internal", "embedded", "catalog"))
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	offering, err := catalog.Offering(catalogs.ProviderIDMistralAI, "mistral-small-latest")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	if offering.Pricing == nil || offering.Pricing.Currency != catalogs.ModelPricingCurrencyUSD ||
		offering.Pricing.Tokens == nil || offering.Pricing.Tokens.Input == nil || offering.Pricing.Tokens.Input.Per1M != 0.15 ||
		offering.Pricing.Tokens.Output == nil || offering.Pricing.Tokens.Output.Per1M != 0.6 {
		t.Fatalf("catalog pricing = %#v", offering.Pricing)
	}
}

func TestMistralListModelsUsesOfficialSinglePageBearerContract(t *testing.T) {
	t.Setenv("MISTRAL_API_KEY", "mistral-fixture-key")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Method != http.MethodGet || request.Header.Get("Authorization") != "Bearer mistral-fixture-key" {
			t.Errorf("request = %s auth %q", request.Method, request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"object":"list","data":[{"id":"mistral-large-latest","owned_by":"mistralai","max_context_length":262144,"capabilities":{"completion_chat":true}}]}`))
	}))
	defer server.Close()
	provider := &catalogs.Provider{
		ID: catalogs.ProviderIDMistralAI, Name: "Mistral AI",
		APIKey: &catalogs.ProviderAPIKey{Name: "MISTRAL_API_KEY", Header: "Authorization", Scheme: catalogs.ProviderAPIKeySchemeBearer},
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
			Type: catalogs.EndpointTypeOpenAI, URL: server.URL, AuthRequired: true,
			AuthorMapping: &catalogs.AuthorMapping{Field: "owned_by", Normalized: map[string]catalogs.AuthorID{
				"mistralai": catalogs.AuthorIDMistralAI,
			}},
		}},
	}
	provider.LoadAPIKey()
	client, err := openai.NewClient(provider, Options()...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if requests.Load() != 1 || len(models) != 1 || models[0].ID != "mistral-large-latest" {
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
	offering, err := catalog.Offering(catalogs.ProviderIDMistralAI, "mistral-large-latest")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	definition, err := catalog.Definition(offering.DefinitionID)
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if offering.ProviderID != catalogs.ProviderIDMistralAI || len(definition.AuthorIDs) != 1 || definition.AuthorIDs[0] != catalogs.AuthorIDMistralAI {
		t.Fatalf("provider/author separation = %#v/%#v", offering, definition)
	}
}

func TestMistralArchivedModelIsDeprecatedAndUnpricedUnknownRemainsUnpriced(t *testing.T) {
	archived := true
	client, err := openai.NewClient(&catalogs.Provider{
		ID: catalogs.ProviderIDMistralAI, Name: "Mistral AI",
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeOpenAI}},
	}, Options()...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	model := client.ConvertToModel(openai.Model{ID: "retired-private-model", Archived: &archived})
	if model.Status != catalogs.ModelStatusDeprecated {
		t.Fatalf("status = %q", model.Status)
	}
	if model.Pricing != nil {
		t.Fatalf("invented pricing = %#v", model.Pricing)
	}
}
