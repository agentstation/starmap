package databricks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestPublicSupportMatrixProducesDiscoveryOnlyModels(t *testing.T) {
	body := strings.Join([]string{
		"databricks-gpt-5-4", "databricks-claude-sonnet-4-6", "databricks-gemini-3-5-flash",
		"databricks-meta-llama-3-3-70b-instruct", "databricks-qwen3-next-80b-a3b-instruct", "databricks-gpt-5-4",
	}, " ")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(body)) }))
	defer server.Close()
	models, err := NewClient(databricksTestProvider(server.URL)).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 5 {
		t.Fatalf("models = %#v", models)
	}
	for _, model := range models {
		if len(model.InvocationAPIs) != 0 || model.OfferingAccess == nil || model.OfferingAccess.Routability != catalogs.OfferingRoutabilityDiscoverable {
			t.Fatalf("public model invented workspace route = %#v", model)
		}
	}
}

func TestWorkspacePaginationExternalModelsAndTrafficAliasesStayPrivate(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if request.Header.Get("Authorization") != "Bearer workspace-secret" || request.URL.Path != "/api/2.0/serving-endpoints" {
			t.Errorf("request = %s %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("page_token") == "page-2" {
			_, _ = writer.Write([]byte(`{"endpoints":[{"name":"managed","config":{"served_entities":[{"name":"current","entity_name":"catalog.schema.model","entity_version":"7"}],"traffic_config":{"routes":[{"served_model_name":"current","traffic_percentage":100}]}}}]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"endpoints":[{"name":"gateway","config":{"served_entities":[{"name":"primary","external_model":{"name":"gpt-5","provider":"openai","task":"llm/v1/chat"}},{"name":"challenger","external_model":{"name":"claude-sonnet","provider":"anthropic","task":"llm/v1/chat"}}],"traffic_config":{"routes":[{"served_model_name":"primary","traffic_percentage":90},{"served_model_name":"challenger","traffic_percentage":10}]}}}],"next_page_token":"page-2"}`))
	}))
	defer server.Close()
	config := WorkspaceConfig{Host: server.URL, Token: "workspace-secret", WorkspaceID: "workspace-private", DefinitionByEntity: map[string]catalogs.ModelDefinitionID{
		"openai/gpt-5": "gpt-5", "anthropic/claude-sonnet": "claude-sonnet", "catalog.schema.model@7": "catalog.schema.model@7",
	}}
	inventory, err := FetchWorkspace(context.Background(), config)
	if err != nil {
		t.Fatalf("FetchWorkspace: %v", err)
	}
	if calls.Load() != 2 || inventory.Scope.WorkspaceID != "workspace-private" || len(inventory.Deployments) != 3 {
		t.Fatalf("calls/inventory = %d/%#v", calls.Load(), inventory)
	}
	if inventory.Deployments[0].Deployment.Type != "external-model" || inventory.Deployments[0].ProviderModelID != "gpt-5" || !strings.Contains(strings.Join(inventory.Deployments[0].Aliases, ","), "traffic=90%") {
		t.Fatalf("external deployment = %#v", inventory.Deployments[0])
	}
	if inventory.Deployments[2].Deployment.Type != "workspace-serving-endpoint" || inventory.Deployments[2].ProviderModelID != "catalog.schema.model@7" {
		t.Fatalf("managed deployment = %#v", inventory.Deployments[2])
	}
	public := publicModel("databricks-gpt-5")
	if public.Extensions["databricks"].Fields["workspace_id"] != nil || strings.Contains(public.ID, "gateway") {
		t.Fatalf("private identity leaked = %#v", public)
	}
}

func TestWorkspaceFailsClosedWithoutMappingOrSecureHost(t *testing.T) {
	if _, err := FetchWorkspace(context.Background(), WorkspaceConfig{Host: "http://workspace.example.com", Token: "token", WorkspaceID: "workspace", DefinitionByEntity: map[string]catalogs.ModelDefinitionID{"x": "x"}}); err == nil {
		t.Fatal("expected insecure-host failure")
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"endpoints":[{"name":"endpoint","config":{"served_entities":[{"name":"served","entity_name":"unknown","entity_version":"1"}]}}]}`))
	}))
	defer server.Close()
	if _, err := FetchWorkspace(context.Background(), WorkspaceConfig{Host: server.URL, Token: "token", WorkspaceID: "workspace", DefinitionByEntity: map[string]catalogs.ModelDefinitionID{"other": "other"}}); err == nil {
		t.Fatal("expected missing-mapping failure")
	}
}

func databricksTestProvider(endpoint string) *catalogs.Provider {
	return &catalogs.Provider{ID: catalogs.ProviderIDDatabricks, Name: "Databricks", Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeDatabricks, URL: endpoint}}}
}
