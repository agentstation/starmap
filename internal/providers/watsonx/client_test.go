package watsonx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

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
	provider := watsonxTestProvider(server.URL, "us-south")
	provider.LoadAPIKey()
	models, err := NewClient(provider).ListModels(context.Background())
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
	provider := watsonxTestProvider(server.URL, "")
	provider.LoadAPIKey()
	if _, err := NewClient(provider).ListModels(context.Background()); err == nil {
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
	inventory, err := FetchDeployments(context.Background(), DeploymentConfig{
		BaseURL: server.URL, Token: "private-token", ProjectID: "project-1", Region: &region,
		DefinitionByAsset: map[string]catalogs.ModelDefinitionID{"ibm/granite-3-3-8b-instruct": "ibm/granite-3-3-8b-instruct", "asset-1": "customer/private-granite"},
	})
	if err != nil {
		t.Fatalf("FetchDeployments: %v", err)
	}
	if requests.Load() != 2 || inventory.Scope.ProjectID != "project-1" || len(inventory.Deployments) != 2 {
		t.Fatalf("requests/inventory = %d/%#v", requests.Load(), inventory)
	}
	if inventory.Deployments[0].Deployment.Type != "on-demand-dedicated" || inventory.Deployments[0].Aliases[1] != "dedicated-granite" {
		t.Fatalf("on-demand deployment = %#v", inventory.Deployments[0])
	}
	if inventory.Deployments[1].Deployment.Type != "custom-dedicated" || inventory.Deployments[1].ProviderModelID != "asset-1" {
		t.Fatalf("custom deployment = %#v", inventory.Deployments[1])
	}
	payload, err := json.Marshal(inventory)
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

func TestFetchDeploymentsRejectsUnmappedAndCrossOriginResources(t *testing.T) {
	t.Run("unmapped", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"resources":[{"metadata":{"id":"deployment"},"entity":{"deployed_asset_type":"curated_foundation_model","foundation_model":{"model_id":"unknown/model"}}}]}`))
		}))
		defer server.Close()
		_, err := FetchDeployments(context.Background(), DeploymentConfig{BaseURL: server.URL, Token: "token", SpaceID: "space", DefinitionByAsset: map[string]catalogs.ModelDefinitionID{"known/model": "known/model"}})
		if err == nil {
			t.Fatal("expected unmapped deployment failure")
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

func watsonxTestProvider(baseURL, region string) *catalogs.Provider {
	provider := &catalogs.Provider{
		ID: catalogs.ProviderIDWatsonx, Name: "IBM watsonx.ai",
		APIKey:  &catalogs.ProviderAPIKey{Name: "IBM_WATSONX_TOKEN", Header: "Authorization", Scheme: catalogs.ProviderAPIKeySchemeBearer},
		EnvVars: []catalogs.ProviderEnvVar{{Name: "IBM_WATSONX_BASE_URL"}, {Name: "IBM_WATSONX_REGION"}},
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeWatsonx, URL: "https://docs.example", BaseURLEnvVar: "IBM_WATSONX_BASE_URL", Path: "/ml/v1/foundation_model_specs", AuthRequired: true}},
	}
	provider.EnvVarValues = map[string]string{"IBM_WATSONX_BASE_URL": baseURL, "IBM_WATSONX_REGION": region}
	return provider
}
