package together

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition/testsource"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestListModelsExecutesServerlessAndDedicatedAsIndependentLogicalSources(t *testing.T) {
	t.Setenv("TOGETHER_API_KEY", "together-fixture-key")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Header.Get("Authorization") != "Bearer together-fixture-key" {
			t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("dedicated") == "true" {
			_, _ = writer.Write([]byte(`[{"id":"meta-llama/Llama-3.3-70B-Instruct-Turbo","object":"model","created":1692896905,"type":"chat","display_name":"Llama 3.3 70B","organization":"Meta","license":"llama","context_length":131072,"pricing":{"hourly":4.5}}]`))
			return
		}
		_, _ = writer.Write([]byte(`[{"id":"meta-llama/Llama-3.3-70B-Instruct-Turbo","object":"model","created":1692896905,"type":"chat","display_name":"Llama 3.3 70B","organization":"Meta","license":"llama","context_length":131072,"pricing":{"input":1.04,"output":1.04,"cached_input":0.2}},{"id":"customer/private-model","object":"model","created":1692896905,"type":"chat","organization":"customer","context_length":2048,"pricing":{"input":1,"output":1}}]`))
	}))
	defer server.Close()
	serverlessProvider := togetherTestProvider(server.URL, "serverless")
	serverless, err := NewClient(testsource.Authenticated(t, serverlessProvider)).ListModels(context.Background())
	if err != nil {
		t.Fatalf("serverless ListModels: %v", err)
	}
	dedicatedProvider := togetherTestProvider(server.URL, "dedicated")
	dedicated, err := NewClient(testsource.Authenticated(t, dedicatedProvider)).ListModels(context.Background())
	if err != nil {
		t.Fatalf("dedicated ListModels: %v", err)
	}
	if requests.Load() != 2 || len(serverless) != 1 || len(dedicated) != 1 {
		t.Fatalf("requests/inventories = %d/%#v/%#v", requests.Load(), serverless, dedicated)
	}
	model := serverless[0]
	if len(model.Authors) != 1 || model.Authors[0].ID != catalogs.AuthorIDMeta || model.OfferingDeployment.Type != "serverless" ||
		!slices.Equal(model.InvocationAPIs, []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}) {
		t.Fatalf("identity/deployment = %#v", model)
	}
	if model.Pricing == nil || model.Pricing.Tokens.Input.Per1M != 1.04 || model.Pricing.Tokens.Output.Per1M != 1.04 ||
		model.Pricing.Tokens.Cache.Read.Per1M != 0.2 {
		t.Fatalf("pricing = %#v", model.Pricing)
	}
	if dedicated[0].OfferingDeployment.Type != "dedicated" || dedicated[0].Modes["dedicated"].Deployment == nil || dedicated[0].Modes["dedicated"].Deployment.Type != "dedicated" || dedicated[0].Extensions["together"].Fields["hourly_price"] != 4.5 {
		t.Fatalf("dedicated inventory = %#v", dedicated[0])
	}
}

func TestTogetherCanonicalOfferingPreservesUnderlyingAuthorAndDeploymentModes(t *testing.T) {
	source := model{ID: "Qwen/Qwen3.7-Plus", Type: "chat", DisplayName: "Qwen3.7 Plus", Organization: "Qwen", ContextLength: 262144, Pricing: pricing{Input: floatPointer(0.32), Output: floatPointer(1.28)}}
	converted, ok, err := convertModel(source, "serverless")
	if err != nil || !ok {
		t.Fatalf("convertModel = %#v/%v/%v", converted, ok, err)
	}
	provider := togetherTestProvider("https://api.together.ai/v1/models", "serverless")
	provider.Models = map[string]*catalogs.Model{converted.ID: &converted}
	builder := catalogs.NewEmpty()
	if err := builder.SetAuthor(catalogs.Author{ID: catalogs.AuthorIDAlibabaQwen, Name: "Alibaba Cloud"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	offering, err := catalog.Offering(catalogs.ProviderIDTogetherAI, catalogs.ProviderModelID(converted.ID))
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	definition, err := catalog.Definition(offering.DefinitionID)
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if offering.ProviderID != catalogs.ProviderIDTogetherAI || offering.Deployment.Type != "serverless" ||
		len(definition.AuthorIDs) != 1 || definition.AuthorIDs[0] != catalogs.AuthorIDAlibabaQwen {
		t.Fatalf("provider/author/deployment = %#v/%#v", offering, definition)
	}
}

func TestTogetherRejectsNegativePricing(t *testing.T) {
	_, _, err := convertModel(model{ID: "meta/model", Type: "chat", Organization: "Meta", Pricing: pricing{Input: floatPointer(-1)}}, "serverless")
	if err == nil {
		t.Fatal("expected negative-pricing failure")
	}
}

func TestDecodeModelsRejectsDuplicateIdentity(t *testing.T) {
	t.Setenv("TOGETHER_API_KEY", "fixture-key")
	client := NewClient(testsource.Authenticated(t, togetherTestProvider("https://example.test/models", "serverless")))
	payload := []byte(`[{"id":"meta-llama/model","object":"model","type":"chat","organization":"Meta"},{"id":"meta-llama/model","object":"model","type":"chat","organization":"Meta"}]`)
	if _, err := client.DecodeModels(payload); err == nil {
		t.Fatal("DecodeModels accepted duplicate model identity")
	}
}

func togetherTestProvider(endpoint, inventory string) *catalogs.Provider {
	return &catalogs.Provider{
		ID: catalogs.ProviderIDTogetherAI, Name: "Together AI",
		Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": {Env: catalogs.ProviderEnvironmentNames{"TOGETHER_API_KEY"}}},
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeGlobalPublic},
			Auth:     catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}},
			Options:  map[string]catalogs.ProviderOptionBinding{"inventory": {Source: catalogs.ProviderBindingSourceStatic, Value: inventory}},
			Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeTogether, URL: endpoint},
		}}},
	}
}

func floatPointer(value float64) *float64 { return &value }
