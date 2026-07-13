package huggingface

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestListModelsPreservesProviderOfferingsAndProbeTime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/models" {
			t.Errorf("path = %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Date", "Sun, 12 Jul 2026 19:40:00 GMT")
		_, _ = writer.Write([]byte(`{"object":"list","data":[{"id":"deepseek-ai/DeepSeek-R1","object":"model","created":1738368000,"owned_by":"deepseek-ai","architecture":{"input_modalities":["text"],"output_modalities":["text"]},"providers":[{"provider":"together","status":"live","context_length":131072,"pricing":{"input":3,"output":7},"is_free":false,"supports_tools":true,"supports_structured_output":true,"first_token_latency_ms":220.5,"throughput":48.25,"is_model_author":false},{"provider":"deepinfra","status":"error","is_free":false,"is_model_author":false}]}]}`))
	}))
	defer server.Close()

	models, err := NewClient(huggingFaceTestProvider(server.URL + "/v1/models")).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %#v", models)
	}
	available := models[1]
	if available.ID != "deepseek-ai/DeepSeek-R1:together" || available.DefinitionID != "deepseek-ai/DeepSeek-R1" ||
		available.AggregatorUpstream == nil || available.AggregatorUpstream.ProviderID != catalogs.ProviderIDTogetherAI ||
		available.Pricing == nil || available.Pricing.Tokens.Input.Per1M != 3 || available.Pricing.Tokens.Output.Per1M != 7 ||
		available.Extensions["huggingface"].Fields["metrics_observed_at"] != "2026-07-12T19:40:00Z" {
		t.Fatalf("available offering = %#v", available)
	}
	copied := catalogs.DeepCopyModel(available)
	copied.AggregatorUpstream.ProviderID = "mutated"
	if available.AggregatorUpstream.ProviderID != catalogs.ProviderIDTogetherAI {
		t.Fatal("DeepCopyModel aliased aggregator upstream")
	}
	unavailable := models[0]
	if unavailable.ID != "deepseek-ai/DeepSeek-R1:deepinfra" || unavailable.OfferingAvailability != catalogs.OfferingAvailabilityUnavailable || len(unavailable.InvocationAPIs) != 0 {
		t.Fatalf("unavailable offering = %#v", unavailable)
	}
}

func TestCanonicalProjectionSharesDefinitionWithoutCollapsingProviders(t *testing.T) {
	source := model{ID: "openai/gpt-oss-120b", OwnedBy: "openai", Architecture: architecture{InputModalities: []string{"text"}, OutputModalities: []string{"text"}}, Providers: []provider{
		{Provider: "groq", Status: "live", ContextLength: int64Pointer(131072), Pricing: &pricing{Input: float64Pointer(0.15), Output: float64Pointer(0.75)}},
		{Provider: "together", Status: "live", ContextLength: int64Pointer(262144), Pricing: &pricing{Input: float64Pointer(0.15), Output: float64Pointer(0.6)}},
	}}
	models, err := convertModel(source, "2026-07-12T19:40:00Z")
	if err != nil {
		t.Fatalf("convertModel: %v", err)
	}
	configured := huggingFaceTestProvider(defaultModelsURL)
	configured.Models = map[string]*catalogs.Model{}
	for index := range models {
		configured.Models[models[index].ID] = &models[index]
	}
	builder := catalogs.NewEmpty()
	if err := builder.SetAuthor(catalogs.Author{ID: catalogs.AuthorIDOpenAI, Name: "OpenAI"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetProvider(*configured); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(catalog.Definitions()) != 1 || len(catalog.Offerings()) != 2 {
		t.Fatalf("definitions/offerings = %d/%d", len(catalog.Definitions()), len(catalog.Offerings()))
	}
	for _, route := range []catalogs.ProviderModelID{"openai/gpt-oss-120b:groq", "openai/gpt-oss-120b:together"} {
		offering, offeringErr := catalog.Offering(catalogs.ProviderIDHuggingFace, route)
		if offeringErr != nil || offering.DefinitionID != "openai/gpt-oss-120b" || offering.AggregatorUpstream == nil {
			t.Fatalf("offering %q = %#v/%v", route, offering, offeringErr)
		}
	}
}

func TestRejectsPolicyLikeStatusAndNegativeMetrics(t *testing.T) {
	_, err := convertModel(model{ID: "model", OwnedBy: "author", Providers: []provider{{Provider: "fastest", Status: "live"}}}, "")
	if err == nil {
		t.Fatal("expected unsupported-status failure")
	}
	negative := -1.0
	_, err = convertModel(model{ID: "model", OwnedBy: "author", Providers: []provider{{Provider: "real", Status: "live", Throughput: &negative}}}, "")
	if err == nil {
		t.Fatal("expected negative-metric failure")
	}
}

func huggingFaceTestProvider(endpoint string) *catalogs.Provider {
	return &catalogs.Provider{
		ID: catalogs.ProviderIDHuggingFace, Name: "Hugging Face Inference Providers",
		APIKey:  &catalogs.ProviderAPIKey{Name: "HF_TOKEN", Header: "Authorization", Scheme: catalogs.ProviderAPIKeySchemeBearer},
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeHuggingFace, URL: endpoint}},
	}
}

func float64Pointer(value float64) *float64 { return &value }
func int64Pointer(value int64) *int64       { return &value }
