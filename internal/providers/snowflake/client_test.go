package snowflake

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/internal/providerdata"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestSessionModelsMapRegionCrossRegionLifecycleAndPricing(t *testing.T) {
	t.Setenv("SNOWFLAKE_TOKEN", "snowflake-fixture-token")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v2/cortex/models" || request.Header.Get("Authorization") != "Bearer snowflake-fixture-token" {
			t.Errorf("request = %s %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"models":[{"name":"claude-sonnet-4-6","status":"active"},{"name":"llama3.3-70b","deprecated":true}]}`))
	}))
	defer server.Close()
	provider := snowflakeTestProvider(server.URL, "us-east-1", "ANY_REGION")
	provider.LoadAPIKey()
	models, err := NewClient(provider).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %#v", models)
	}
	claude := models[0]
	if claude.Authors[0].ID != catalogs.AuthorIDAnthropic || claude.Pricing.Tokens.Input.Per1M != 3 || claude.Pricing.Tokens.Output.Per1M != 15 ||
		math.Abs(claude.Modes["regional-routing"].Pricing.Tokens.Input.Per1M-3.3) > 1e-9 || len(claude.OfferingRegions) != 1 || claude.OfferingInferenceProfile == nil {
		t.Fatalf("claude = %#v", claude)
	}
	if models[1].Status != catalogs.ModelStatusDeprecated || models[1].Pricing.Tokens.Input.Per1M != 0.72 {
		t.Fatalf("deprecated/pricing = %#v", models[1])
	}
	copied := catalogs.DeepCopyModel(claude)
	copied.OfferingRegions[0].ID = "mutated"
	copied.OfferingInferenceProfile.DestinationRegions[0] = "mutated"
	if claude.OfferingRegions[0].ID != "us-east-1" || claude.OfferingInferenceProfile.DestinationRegions[0] != "any_region" {
		t.Fatal("DeepCopyModel aliased Snowflake geography")
	}
}

func TestModelEnvelopeAcceptsDocumentedSessionShapes(t *testing.T) {
	for _, fixture := range []string{`["mistral-7b"]`, `{"data":[{"id":"deepseek-r1"}]}`} {
		var envelope modelEnvelope
		if err := envelope.UnmarshalJSON([]byte(fixture)); err != nil || len(envelope.Models) != 1 {
			t.Fatalf("UnmarshalJSON(%s) = %#v/%v", fixture, envelope, err)
		}
	}
}

func TestCanonicalOfferingPreservesSnowflakeGeographyAndModes(t *testing.T) {
	provider := snowflakeTestProvider("https://account.snowflakecomputing.com", "eu-west-1", "AWS_EU")
	pricingCatalog, err := providerdata.LoadPricingCatalog(catalogs.ProviderIDSnowflake)
	if err != nil {
		t.Fatalf("LoadPricingCatalog: %v", err)
	}
	model := convertModel(sessionModel{Name: "mistral-7b"}, provider, pricingCatalog)
	provider.Models = map[string]*catalogs.Model{model.ID: &model}
	builder := catalogs.NewEmpty()
	if err := builder.SetAuthor(catalogs.Author{ID: catalogs.AuthorIDMistralAI, Name: "Mistral AI"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	offering, err := catalog.Offering(catalogs.ProviderIDSnowflake, "mistral-7b")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	if len(offering.Regions) != 1 || offering.Regions[0].ID != "eu-west-1" || offering.InferenceProfile == nil || offering.InferenceProfile.ID != "AWS_EU" || len(offering.Modes) != 2 {
		t.Fatalf("offering = %#v", offering)
	}
}

func TestSessionModelsFailClosedWithoutAccountOrModels(t *testing.T) {
	provider := snowflakeTestProvider("", "", "")
	if _, err := NewClient(provider).ListModels(context.Background()); err == nil {
		t.Fatal("expected missing-account failure")
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"models":null}`))
	}))
	defer server.Close()
	t.Setenv("SNOWFLAKE_TOKEN", "token")
	provider = snowflakeTestProvider(server.URL, "", "DISABLED")
	provider.LoadAPIKey()
	if _, err := NewClient(provider).ListModels(context.Background()); err == nil {
		t.Fatal("expected null-model failure")
	}
}

func snowflakeTestProvider(accountURL, region, crossRegion string) *catalogs.Provider {
	provider := &catalogs.Provider{
		ID: catalogs.ProviderIDSnowflake, Name: "Snowflake Cortex AI",
		APIKey:  &catalogs.ProviderAPIKey{Name: "SNOWFLAKE_TOKEN", Header: "Authorization", Scheme: catalogs.ProviderAPIKeySchemeBearer},
		EnvVars: []catalogs.ProviderEnvVar{{Name: "SNOWFLAKE_ACCOUNT_URL"}, {Name: "SNOWFLAKE_REGION"}, {Name: "SNOWFLAKE_CORTEX_CROSS_REGION"}},
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeSnowflake, URL: "https://docs.example", BaseURLEnvVar: "SNOWFLAKE_ACCOUNT_URL", Path: "/api/v2/cortex/models", AuthRequired: true}},
	}
	provider.EnvVarValues = map[string]string{"SNOWFLAKE_ACCOUNT_URL": accountURL, "SNOWFLAKE_REGION": region, "SNOWFLAKE_CORTEX_CROSS_REGION": crossRegion}
	return provider
}
