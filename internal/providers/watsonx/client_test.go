package watsonx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition/testsource"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestListModelsUsesDatedOpaquePaginationAndMapsTasks(t *testing.T) {
	t.Setenv("IBM_WATSONX_TOKEN", "watsonx-fixture-token")
	var requests atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Header.Get("Authorization") != "Bearer watsonx-fixture-token" || request.URL.Query().Get("version") != apiVersion || request.URL.Query().Get("limit") != "200" {
			t.Errorf("request auth/query = %q/%q", request.Header.Get("Authorization"), request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("start") == "opaque-2" {
			_, _ = writer.Write([]byte(`{"resources":[{"model_id":"ibm/granite-embedding","label":"Granite Embedding","provider":"IBM","source":"IBM","tasks":[{"id":"embedding"}],"model_limits":{"max_sequence_length":8192}}]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"resources":[{"model_id":"meta-llama/llama-3","label":"Llama","provider":"Meta","source":"Hugging Face","tasks":[{"id":"text_generation"}],"lifecycle":[{"id":"deprecated","start":"2026-07-01"}]}],"next":{"href":"` + server.URL + `/ml/v1/foundation_model_specs?version=2024-03-14&limit=200&start=opaque-2"}}`))
	}))
	defer server.Close()
	provider := watsonxTestProvider(t, server.URL, "us-south")
	models, err := NewClient(testsource.Authenticated(t, provider)).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if requests.Load() != 2 || len(models) != 2 {
		t.Fatalf("requests/models = %d/%#v", requests.Load(), models)
	}
	if models[0].InvocationAPIs[0] != catalogs.InvocationAPIEmbeddings || models[0].Limits.ContextWindow != 8192 || models[0].OfferingRegions[0].ID != "us-south" {
		t.Fatalf("embedding = %#v", models[0])
	}
	if models[1].Status != catalogs.ModelStatusDeprecated || models[1].InvocationAPIs[0] != catalogs.InvocationAPIWatsonxGenerate || models[1].OfferingDeployment.Type != "curated-multitenant" {
		t.Fatalf("generation = %#v", models[1])
	}
}

func TestListModelsRejectsCrossOriginCursorAndMalformedLimits(t *testing.T) {
	t.Setenv("IBM_WATSONX_TOKEN", "token")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"resources":[],"next":{"href":"https://attacker.example/models?start=secret"}}`))
	}))
	defer server.Close()
	provider := watsonxTestProvider(t, server.URL, "us-south")
	if _, err := NewClient(testsource.Authenticated(t, provider)).ListModels(context.Background()); err == nil {
		t.Fatal("expected cross-origin cursor failure")
	}
	bad := model{ModelID: "ibm/model", Provider: "IBM", ModelLimits: &struct {
		MaxSequenceLength int64 `json:"max_sequence_length"`
	}{MaxSequenceLength: -1}}
	if _, err := convertModel(bad, "us-south"); err == nil {
		t.Fatal("expected negative-limit failure")
	}
}

func TestUnknownTasksRemainDiscoverable(t *testing.T) {
	model, err := convertModel(model{ModelID: "ibm/special", Provider: "IBM", Tasks: []task{{ID: "unknown"}}}, "eu-de")
	if err != nil {
		t.Fatalf("convertModel: %v", err)
	}
	if len(model.InvocationAPIs) != 0 || model.OfferingAccess == nil || model.OfferingAccess.Routability != catalogs.OfferingRoutabilityDiscoverable {
		t.Fatalf("unknown task invented route = %#v", model)
	}
	if strings.Contains(model.Extensions["watsonx"].Fields["source"].(string), "project") {
		t.Fatalf("private scope leaked = %#v", model.Extensions)
	}
}

func TestFetchDeploymentsIsolatesProjectInventoryAndDeploymentKinds(t *testing.T) {
	var requests atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Header.Get("Authorization") != "Bearer private-token" || request.URL.Query().Get("version") != apiVersion || request.URL.Query().Get("project_id") != "project-1" {
			t.Errorf("request auth/query = %q/%q", request.Header.Get("Authorization"), request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("start") == "second" {
			_, _ = writer.Write([]byte(`{"resources":[{"metadata":{"id":"custom-1"},"entity":{"name":"Private Granite","deployed_asset_type":"custom_foundation_model","asset":{"id":"asset-1"},"online":{"parameters":{"serving_name":"private-granite"}}}}]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"resources":[{"metadata":{"id":"dedicated-1"},"entity":{"name":"Dedicated Granite","deployed_asset_type":"curated_foundation_model","asset":{"id":"asset-ignored"},"foundation_model":{"model_id":"ibm/granite-3-3-8b-instruct"},"online":{"parameters":{"serving_name":"dedicated-granite"}}}}],"next":{"href":"` + server.URL + `/ml/v4/deployments?version=2024-03-14&project_id=project-1&start=second"}}`))
	}))
	defer server.Close()
	region := catalogs.CloudRegion{ID: "us-south"}
	result, err := FetchDeployments(context.Background(), DeploymentConfig{
		BaseURL: server.URL, Token: "private-token", ProjectID: "project-1", Region: &region,
		DefinitionByAsset: map[string]catalogs.ModelDefinitionID{"ibm/granite-3-3-8b-instruct": "ibm/granite-3-3-8b-instruct", "asset-1": "customer/private-granite"},
	})
	if err != nil {
		t.Fatalf("FetchDeployments: %v", err)
	}
	offerings := result.Offerings
	if requests.Load() != 2 || len(offerings) != 2 || len(result.Definitions) != 2 {
		t.Fatalf("requests/result = %d/%#v", requests.Load(), result)
	}
	dedicated := offerings[slices.IndexFunc(offerings, func(offering catalogs.ProviderOffering) bool { return offering.DeploymentID == "dedicated-1" })]
	custom := offerings[slices.IndexFunc(offerings, func(offering catalogs.ProviderOffering) bool { return offering.DeploymentID == "custom-1" })]
	if dedicated.Deployment.Type != "on-demand-dedicated" || dedicated.Aliases[1] != "dedicated-granite" {
		t.Fatalf("on-demand deployment = %#v", dedicated)
	}
	if custom.Deployment.Type != "custom-dedicated" || custom.ProviderModelID != "asset-1" {
		t.Fatalf("custom deployment = %#v", custom)
	}
	payload, err := json.Marshal(offerings)
	if err != nil {
		t.Fatalf("marshal inventory: %v", err)
	}
	for _, secret := range []string{"private-token", "project-1"} {
		if secret == "private-token" && strings.Contains(string(payload), secret) {
			t.Fatalf("credential leaked into inventory: %s", payload)
		}
	}
}

func TestFetchDeploymentsRequiresOneScopeAndExplicitMapping(t *testing.T) {
	tests := []DeploymentConfig{
		{BaseURL: "https://us-south.ml.cloud.ibm.com", Token: "token", DefinitionByAsset: map[string]catalogs.ModelDefinitionID{"model": "definition"}},
		{BaseURL: "https://us-south.ml.cloud.ibm.com", Token: "token", ProjectID: "project", SpaceID: "space", DefinitionByAsset: map[string]catalogs.ModelDefinitionID{"model": "definition"}},
		{BaseURL: "https://us-south.ml.cloud.ibm.com", Token: "token", ProjectID: "project"},
	}
	for _, config := range tests {
		if _, err := FetchDeployments(context.Background(), config); err == nil {
			t.Fatalf("FetchDeployments accepted invalid config %#v", config)
		}
	}
}

func TestFetchDeploymentsInfersUnmappedDefinitionAndRejectsCrossOriginResources(t *testing.T) {
	t.Run("unmapped definition is canonicalized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"resources":[{"metadata":{"id":"deployment"},"entity":{"deployed_asset_type":"curated_foundation_model","foundation_model":{"model_id":"unknown/model"}}}]}`))
		}))
		defer server.Close()
		result, err := FetchDeployments(context.Background(), DeploymentConfig{BaseURL: server.URL, Token: "token", SpaceID: "space"})
		if err != nil || len(result.Definitions) != 1 || result.Definitions[0].ID != "unknown/model" || result.Offerings[0].DefinitionID != "unknown/model" {
			t.Fatalf("inferred canonical records = %#v err=%v", result, err)
		}
	})
	t.Run("cross origin", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"resources":[],"next":{"href":"https://attacker.example/deployments?start=secret"}}`))
		}))
		defer server.Close()
		_, err := FetchDeployments(context.Background(), DeploymentConfig{BaseURL: server.URL, Token: "token", SpaceID: "space", DefinitionByAsset: map[string]catalogs.ModelDefinitionID{"known/model": "known/model"}})
		if err == nil {
			t.Fatal("expected cross-origin cursor failure")
		}
	})
}

func watsonxTestProvider(t *testing.T, baseURL, region string) *catalogs.Provider {
	t.Helper()
	t.Setenv("IBM_WATSONX_BASE_URL", baseURL)
	t.Setenv("IBM_WATSONX_REGION", region)
	return &catalogs.Provider{
		ID: catalogs.ProviderIDWatsonx, Name: "IBM watsonx.ai",
		Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": {Env: catalogs.ProviderEnvironmentNames{"IBM_WATSONX_TOKEN"}}},
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeCredentialScoped},
			Auth: catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}},
			Scopes: map[string]catalogs.ProviderScopeBinding{
				"region": {Source: catalogs.ProviderBindingSourceEnv, Name: catalogs.ProviderEnvironmentNames{"IBM_WATSONX_REGION"}, Role: catalogs.ProviderBindingRoleRequiredInput},
			},
			Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeWatsonx, URL: "https://docs.example", BaseURLEnv: "IBM_WATSONX_BASE_URL", Path: "/ml/v1/foundation_model_specs"},
		}}},
	}
}
