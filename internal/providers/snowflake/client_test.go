package snowflake

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/acquisition/testsource"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestSessionModelsMapRegionCrossRegionAndLifecycle(t *testing.T) {
	t.Setenv("SNOWFLAKE_TOKEN", "snowflake-fixture-token")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v2/cortex/models" || request.Header.Get("Authorization") != "Bearer snowflake-fixture-token" {
			t.Errorf("request = %s %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"models":[{"name":"claude-sonnet-4-6","status":"active"},{"name":"llama3.3-70b","deprecated":true}]}`))
	}))
	defer server.Close()
	provider := snowflakeTestProvider(t, server.URL, "us-east-1", "ANY_REGION")
	models, err := NewClient(testsource.Authenticated(t, provider)).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %#v", models)
	}
	claude := models[0]
	if claude.Authors[0].ID != catalogs.AuthorIDAnthropic || claude.Pricing != nil || len(claude.Modes) != 0 || len(claude.OfferingRegions) != 1 || claude.OfferingInferenceProfile == nil {
		t.Fatalf("claude = %#v", claude)
	}
	if models[1].Status != catalogs.ModelStatusDeprecated || models[1].Pricing != nil {
		t.Fatalf("deprecated model = %#v", models[1])
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

func TestCanonicalOfferingPreservesSnowflakeGeography(t *testing.T) {
	provider := snowflakeTestProvider(t, "https://account.snowflakecomputing.com", "eu-west-1", "AWS_EU")
	model := convertModel(sessionModel{Name: "mistral-7b"}, "eu-west-1", "AWS_EU")
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
	if len(offering.Regions) != 1 || offering.Regions[0].ID != "eu-west-1" || offering.InferenceProfile == nil || offering.InferenceProfile.ID != "AWS_EU" {
		t.Fatalf("offering = %#v", offering)
	}
}

func TestReviewedSnowflakePricesAreCanonicalOfferings(t *testing.T) {
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	offering, err := catalog.Offering(catalogs.ProviderIDSnowflake, "claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	if offering.Pricing == nil || offering.Pricing.Tokens.Input.Per1M != 3.3 || offering.Modes["global-routing"].Pricing.Tokens.Output.Per1M != 15 {
		t.Fatalf("canonical pricing = %#v", offering)
	}
}

func TestSessionModelsFailClosedWithoutAccountOrModels(t *testing.T) {
	provider := snowflakeTestProvider(t, "", "", "")
	if _, err := acquisition.NewResolver().Resolve(context.Background(), provider, "models"); err == nil {
		t.Fatal("expected missing-account failure")
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"models":null}`))
	}))
	defer server.Close()
	t.Setenv("SNOWFLAKE_TOKEN", "token")
	provider = snowflakeTestProvider(t, server.URL, "us-east-1", "DISABLED")
	if _, err := NewClient(testsource.Authenticated(t, provider)).ListModels(context.Background()); err == nil {
		t.Fatal("expected null-model failure")
	}
}

func snowflakeTestProvider(t *testing.T, accountURL, region, crossRegion string) *catalogs.Provider {
	t.Helper()
	t.Setenv("SNOWFLAKE_ACCOUNT_URL", accountURL)
	t.Setenv("SNOWFLAKE_REGION", region)
	t.Setenv("SNOWFLAKE_CORTEX_CROSS_REGION", crossRegion)
	return &catalogs.Provider{
		ID: catalogs.ProviderIDSnowflake, Name: "Snowflake Cortex AI",
		Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": {Env: catalogs.ProviderEnvironmentNames{"SNOWFLAKE_TOKEN"}}},
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeCredentialScoped},
			Auth: catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}},
			Scopes: map[string]catalogs.ProviderScopeBinding{
				"region": {Source: catalogs.ProviderBindingSourceEnv, Name: catalogs.ProviderEnvironmentNames{"SNOWFLAKE_REGION"}, Role: catalogs.ProviderBindingRoleRequiredInput},
			},
			Options: map[string]catalogs.ProviderOptionBinding{
				"cross_region": {Source: catalogs.ProviderBindingSourceEnv, Name: catalogs.ProviderEnvironmentNames{"SNOWFLAKE_CORTEX_CROSS_REGION"}},
			},
			Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeSnowflake, URL: "https://docs.example", BaseURLEnv: "SNOWFLAKE_ACCOUNT_URL", Path: "/api/v2/cortex/models"},
		}}},
	}
}
