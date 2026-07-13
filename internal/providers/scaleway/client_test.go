package scaleway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"

	"github.com/agentstation/starmap/internal/providers/openai"
	"github.com/agentstation/starmap/internal/providers/testhelper"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestScalewayLiveInventoryPreservesIdentityRegionAndDriftWithoutCuratedFacts(t *testing.T) {
	t.Setenv("SCW_SECRET_KEY", "fixture-token")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(testhelper.LoadTestdata(t, "models_list.json"))
	}))
	defer server.Close()

	provider := &catalogs.Provider{
		ID: catalogs.ProviderIDScaleway, Name: "Scaleway Generative APIs",
		APIKey: &catalogs.ProviderAPIKey{Name: "SCW_SECRET_KEY", Header: "Authorization", Scheme: catalogs.ProviderAPIKeySchemeBearer},
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
			Type: catalogs.EndpointTypeOpenAI, URL: server.URL, AuthRequired: true,
			AuthorMapping: &catalogs.AuthorMapping{Field: "id", Normalized: map[string]catalogs.AuthorID{
				"glm-*": catalogs.AuthorIDZhipuAI, "qwen*": catalogs.AuthorIDAlibabaQwen,
				"devstral-*": catalogs.AuthorIDMistralAI, "holo*": catalogs.AuthorIDHCompany,
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
	if len(models) != 4 {
		t.Fatalf("models = %#v", models)
	}

	glm := findModel(t, models, "glm-5.2")
	if glm.Authors[0].ID != catalogs.AuthorIDZhipuAI || glm.Status != "" {
		t.Fatalf("glm identity/lifecycle = %#v", glm)
	}
	if glm.Pricing != nil || len(glm.Modes) != 0 {
		t.Fatalf("live inventory invented curated pricing = %#v/%#v", glm.Pricing, glm.Modes)
	}
	if len(glm.Extensions["scaleway"].Fields["unknown_fields"].([]any)) == 0 {
		t.Fatalf("glm drift evidence = %#v", glm.Extensions)
	}

	holo := findModel(t, models, "holo2-30b-a3b")
	if holo.Authors[0].ID != catalogs.AuthorIDHCompany || holo.Pricing != nil {
		t.Fatalf("holo live identity/pricing = %#v/%#v", holo.Authors, holo.Pricing)
	}
}

func TestScalewayCuratedFactsLiveInEmbeddedCatalog(t *testing.T) {
	builder, err := catalogs.NewFromPath(filepath.Join("..", "..", "..", "internal", "embedded", "catalog"))
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	glm, err := catalog.Offering(catalogs.ProviderIDScaleway, "glm-5.2")
	if err != nil {
		t.Fatalf("glm Offering: %v", err)
	}
	if glm.Pricing == nil || glm.Pricing.Currency != catalogs.ModelPricingCurrencyEUR || glm.Pricing.Tokens.Input.Per1M != 1.8 ||
		glm.Pricing.Tokens.Output.Per1M != 5.5 || glm.Modes["batch"].Pricing.Tokens.Input.Per1M != 0.9 {
		t.Fatalf("glm catalog facts = %#v", glm)
	}
	if len(glm.Regions) != 1 || glm.Regions[0].ID != "fr-par" || glm.Regions[0].Residency.Countries[0] != "FR" ||
		glm.Endpoint.BaseURL != "https://api.scaleway.ai/v1" || glm.Deployment.Tier != "pay-per-use" {
		t.Fatalf("glm configured offering defaults = %#v", glm)
	}
	embed, err := catalog.Offering(catalogs.ProviderIDScaleway, "qwen3-embedding-8b")
	if err != nil || !slices.Equal(embed.Access.APIs, []catalogs.InvocationAPI{catalogs.InvocationAPIEmbeddings}) || embed.Pricing.Tokens.Output != nil {
		t.Fatalf("embedding catalog facts = %#v/%v", embed, err)
	}
	deprecated, err := catalog.Offering(catalogs.ProviderIDScaleway, "devstral-2-123b-instruct-2512")
	if err != nil || deprecated.Lifecycle != catalogs.OfferingLifecycleDeprecated {
		t.Fatalf("deprecated catalog facts = %#v/%v", deprecated, err)
	}
	holoDefinition, err := catalog.Definition("holo2-30b-a3b")
	if err != nil || holoDefinition.Capabilities.Features == nil || holoDefinition.Capabilities.Features.Tools ||
		!slices.Contains(holoDefinition.Capabilities.Features.Modalities.Input, catalogs.ModelModalityImage) {
		t.Fatalf("holo catalog facts = %#v/%v", holoDefinition, err)
	}
}

func findModel(t *testing.T, models []catalogs.Model, id string) *catalogs.Model {
	t.Helper()
	for i := range models {
		model := &models[i]
		if model.ID == id {
			return model
		}
	}
	t.Fatalf("model %q not found", id)
	return nil
}
