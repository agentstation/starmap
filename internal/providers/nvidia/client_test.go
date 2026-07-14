package nvidia

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition/testsource"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestInventoryDoesNotInventInvocationPolicy(t *testing.T) {
	server := modelServer(t, `{"object":"list","data":[{"id":"meta/llama-3.1-8b-instruct","object":"model","created":1,"owned_by":"meta"},{"id":"nvidia/nv-embed-v1","object":"model","created":1,"owned_by":"nvidia"}]}`)
	defer server.Close()
	models, err := NewClient(testsource.Unauthenticated(t, nvidiaTestProvider(server.URL+"/v1/models"))).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %#v", models)
	}
	for _, model := range models {
		if model.InvocationAPIs != nil || model.OfferingAccess != nil {
			t.Fatalf("wire inventory invented offering policy = %#v", model)
		}
	}
	if models[0].Authors[0].ID != catalogs.AuthorIDMeta || models[1].Authors[0].ID != catalogs.AuthorIDNVIDIA {
		t.Fatalf("authors = %#v/%#v", models[0].Authors, models[1].Authors)
	}
}

func TestStrictEnvelope(t *testing.T) {
	server := modelServer(t, `{"object":"list","data":null}`)
	defer server.Close()
	if _, err := NewClient(testsource.Unauthenticated(t, nvidiaTestProvider(server.URL))).ListModels(context.Background()); err == nil {
		t.Fatal("expected null-data failure")
	}
}

func TestDecodeModelsRejectsDuplicateIdentity(t *testing.T) {
	client := NewClient(testsource.Unauthenticated(t, nvidiaTestProvider("https://example.test/v1/models")))
	payload := []byte(`{"object":"list","data":[{"id":"nvidia/model","object":"model","owned_by":"nvidia"},{"id":"nvidia/model","object":"model","owned_by":"nvidia"}]}`)
	if _, err := client.DecodeModels(payload); err == nil {
		t.Fatal("DecodeModels accepted duplicate model identity")
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
	return &catalogs.Provider{ID: catalogs.ProviderIDNVIDIA, Name: "NVIDIA API Catalog", Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
		ID: "models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
		Auth: catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone}, Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeNVIDIA, URL: endpoint},
	}}}}
}
