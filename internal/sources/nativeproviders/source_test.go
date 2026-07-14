package nativeproviders

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestConfiguredDatabricksWorkspaceSourceProducesCanonicalContextualObservation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/2.0/serving-endpoints" || request.Header.Get("Authorization") != "Bearer workspace-token" {
			t.Errorf("request = %s auth=%q", request.URL.Path, request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"endpoints":[{"name":"private-endpoint","config":{"served_entities":[{"name":"served","entity_name":"catalog.schema.private","entity_version":"1"}]}}]}`))
	}))
	defer server.Close()
	t.Setenv("DATABRICKS_TOKEN_TEST", "workspace-token")
	t.Setenv("DATABRICKS_HOST_TEST", server.URL)
	t.Setenv("DATABRICKS_WORKSPACE_TEST", "workspace-a")
	provider := catalogs.Provider{
		ID: catalogs.ProviderIDDatabricks, Name: "Databricks",
		Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": {Env: catalogs.ProviderEnvironmentNames{"DATABRICKS_TOKEN_TEST"}}},
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "workspace", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeCredentialScoped},
			Auth: catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}}, Optional: true,
			Topology: catalogs.ProviderSourceTopologyPaginated,
			Scopes:   map[string]catalogs.ProviderScopeBinding{"workspace_id": {Source: catalogs.ProviderBindingSourceEnv, Name: catalogs.ProviderEnvironmentNames{"DATABRICKS_WORKSPACE_TEST"}, Role: catalogs.ProviderBindingRoleRequiredInput}},
			Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeDatabricksWorkspace, BaseURLEnv: "DATABRICKS_HOST_TEST", Path: "/api/2.0/serving-endpoints"},
		}}},
	}
	observation := observeConfiguredTestSource(t, provider)
	if observation.Metrics.Scope != catalogmeta.ObservationScopeCredentialScoped || len(observation.Catalog.Offerings()) != 1 || len(observation.Catalog.Definitions()) != 1 {
		t.Fatalf("Databricks contextual observation = %#v", observation)
	}
	if len(observation.Metrics.Acquisitions) != 1 || observation.Metrics.Acquisitions[0].SourceID != "workspace" || observation.Metrics.Acquisitions[0].AuthMethod != "api_key" || observation.Metrics.Acquisitions[0].Topology != catalogmeta.AcquisitionTopologyPaginated {
		t.Fatalf("Databricks provenance = %#v", observation.Metrics.Acquisitions)
	}
}

func TestConfiguredWatsonxDeploymentSourceProducesCanonicalContextualObservation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/ml/v4/deployments" || request.URL.Query().Get("project_id") != "project-a" || request.Header.Get("Authorization") != "Bearer watsonx-token" {
			t.Errorf("request = %s?%s auth=%q", request.URL.Path, request.URL.RawQuery, request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"resources":[{"metadata":{"id":"deployment-a"},"entity":{"name":"Private Granite","deployed_asset_type":"custom_foundation_model","asset":{"id":"asset-a"}}}]}`))
	}))
	defer server.Close()
	t.Setenv("WATSONX_TOKEN_TEST", "watsonx-token")
	t.Setenv("WATSONX_BASE_TEST", server.URL)
	t.Setenv("WATSONX_PROJECT_TEST", "project-a")
	t.Setenv("WATSONX_REGION_TEST", "us-south")
	provider := catalogs.Provider{
		ID: catalogs.ProviderIDWatsonx, Name: "watsonx",
		Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": {Env: catalogs.ProviderEnvironmentNames{"WATSONX_TOKEN_TEST"}}},
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "deployments", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeCredentialScoped},
			Auth: catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}}, Optional: true,
			Topology: catalogs.ProviderSourceTopologyPaginated,
			Scopes:   map[string]catalogs.ProviderScopeBinding{"region": {Source: catalogs.ProviderBindingSourceEnv, Name: catalogs.ProviderEnvironmentNames{"WATSONX_REGION_TEST"}, Role: catalogs.ProviderBindingRoleRequiredInput}},
			Options:  map[string]catalogs.ProviderOptionBinding{"project_id": {Source: catalogs.ProviderBindingSourceEnv, Name: catalogs.ProviderEnvironmentNames{"WATSONX_PROJECT_TEST"}}},
			Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeWatsonxDeployments, BaseURLEnv: "WATSONX_BASE_TEST", Path: "/ml/v4/deployments"},
		}}},
	}
	observation := observeConfiguredTestSource(t, provider)
	if observation.Metrics.Scope != catalogmeta.ObservationScopeCredentialScoped || len(observation.Catalog.Offerings()) != 1 || len(observation.Catalog.Definitions()) != 1 {
		t.Fatalf("watsonx contextual observation = %#v", observation)
	}
}

func observeConfiguredTestSource(t testing.TB, provider catalogs.Provider) sources.Observation {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	configured, err := New(builder.Providers())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(configured) != 1 {
		t.Fatalf("configured sources = %#v", configured)
	}
	observation, err := configured[0].Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	return observation
}

func TestMissingOptionalNativeCredentialReturnsSecretSafeEmptyObservation(t *testing.T) {
	const secret = "native-secret-must-not-leak"
	t.Setenv("STARMAP_TEST_NATIVE_KEY", secret)
	builder := catalogs.NewEmpty()
	provider := catalogs.Provider{
		ID: catalogs.ProviderIDAmazonBedrock, Name: "Amazon Bedrock",
		Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{
			"api_key": {Env: catalogs.ProviderEnvironmentNames{"STARMAP_TEST_NATIVE_KEY"}},
		},
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "account", Optional: true,
			ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeCredentialScoped},
			Auth:             catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}},
			Endpoint:         catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeBedrock, URL: "https://bedrock.us-east-1.amazonaws.com"},
		}}},
	}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	configured, err := New(builder.Providers())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(configured) != 1 || configured[0].ID() != sources.AmazonBedrockID || !configured[0].IsOptional() {
		t.Fatalf("configured native sources = %#v", configured)
	}
	observation, err := configured[0].Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if observation.Status != sources.ObservationStatusDegraded || observation.Metrics.Scope != catalogmeta.ObservationScopeGlobalPublic || observation.Catalog == nil || len(observation.Catalog.Offerings()) != 0 {
		t.Fatalf("missing credential observation = %#v", observation)
	}
	if len(observation.Metrics.Acquisitions) != 1 || observation.Metrics.Acquisitions[0].ProviderID != string(catalogs.ProviderIDAmazonBedrock) || observation.Metrics.Acquisitions[0].SourceID != "account" || observation.Metrics.Acquisitions[0].AuthMethod != "" {
		t.Fatalf("safe unavailable provenance = %#v", observation.Metrics.Acquisitions)
	}
	payload, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), secret) || strings.Contains(string(payload), "STARMAP_TEST_NATIVE_KEY=") {
		t.Fatalf("secret leaked into observation: %s", payload)
	}
}
