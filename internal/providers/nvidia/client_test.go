package nvidia

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestPublicCatalogIsDiscoverableWithoutInventedRoute(t *testing.T) {
	server := modelServer(t, `{"object":"list","data":[{"id":"meta/llama-3.1-8b-instruct","object":"model","created":1,"owned_by":"meta"},{"id":"nvidia/nv-embed-v1","object":"model","created":1,"owned_by":"nvidia"}]}`)
	defer server.Close()
	models, err := NewClient(nvidiaTestProvider(server.URL + "/v1/models")).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %#v", models)
	}
	for _, model := range models {
		if len(model.InvocationAPIs) != 0 || model.OfferingAccess == nil || model.OfferingAccess.Routability != catalogs.OfferingRoutabilityDiscoverable || model.OfferingDeployment.Type != "nvidia-hosted" {
			t.Fatalf("public model invented route = %#v", model)
		}
	}
	if models[0].Authors[0].ID != catalogs.AuthorIDMeta || models[1].Authors[0].ID != catalogs.AuthorIDNVIDIA {
		t.Fatalf("authors = %#v/%#v", models[0].Authors, models[1].Authors)
	}
}

func TestCustomerNIMStaysPrivateAndRequiresDefinitionMapping(t *testing.T) {
	server := modelServer(t, `{"object":"list","data":[{"id":"team-served-name","object":"model","created":1,"owned_by":"system"}]}`)
	defer server.Close()
	region := &catalogs.CloudRegion{ID: "customer-dc-1"}
	inventory, err := FetchCustomerNIM(context.Background(), NIMInventoryConfig{
		BaseURL: server.URL, AccountID: "account-private", DeploymentID: "nim-prod", Region: region, Aliases: []string{"production"},
		DefinitionByName: map[string]catalogs.ModelDefinitionID{"team-served-name": "meta/llama-3.1-8b-instruct"},
	})
	if err != nil {
		t.Fatalf("FetchCustomerNIM: %v", err)
	}
	if inventory.Scope.AccountID != "account-private" || len(inventory.Deployments) != 1 {
		t.Fatalf("inventory = %#v", inventory)
	}
	deployment := inventory.Deployments[0]
	if deployment.Endpoint != server.URL || deployment.ProviderModelID != "team-served-name" || deployment.DefinitionID != "meta/llama-3.1-8b-instruct" || deployment.Deployment.Type != "customer-hosted-nim" {
		t.Fatalf("deployment = %#v", deployment)
	}
	public, err := publicModel(model{ID: "meta/llama-3.1-8b-instruct", OwnedBy: "meta"})
	if err != nil {
		t.Fatalf("publicModel: %v", err)
	}
	if public.ID == deployment.ID || public.Extensions["nvidia"].Fields["account_id"] != nil {
		t.Fatalf("private identity leaked into public model = %#v", public)
	}
	_, err = FetchCustomerNIM(context.Background(), NIMInventoryConfig{BaseURL: server.URL, AccountID: "account-private", DeploymentID: "nim-prod", DefinitionByName: map[string]catalogs.ModelDefinitionID{"other": "definition"}})
	if err == nil {
		t.Fatal("expected missing mapping failure")
	}
}

func TestStrictEnvelopeAndPrivateConfig(t *testing.T) {
	server := modelServer(t, `{"object":"list","data":null}`)
	defer server.Close()
	if _, err := NewClient(nvidiaTestProvider(server.URL)).ListModels(context.Background()); err == nil {
		t.Fatal("expected null-data failure")
	}
	if _, err := FetchCustomerNIM(context.Background(), NIMInventoryConfig{BaseURL: server.URL}); err == nil {
		t.Fatal("expected private-config failure")
	}
}

func modelServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Date", "Sun, 12 Jul 2026 19:50:00 GMT")
		_, _ = writer.Write([]byte(body))
	}))
}

func nvidiaTestProvider(endpoint string) *catalogs.Provider {
	return &catalogs.Provider{ID: catalogs.ProviderIDNVIDIA, Name: "NVIDIA API Catalog", Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeNVIDIA, URL: endpoint}}}
}
