package sambanova

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestListModelsMapsExactLimitsPricingAndAuthors(t *testing.T) {
	t.Setenv("SAMBANOVA_API_KEY", "fixture-token")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"object":"list","data":[{"id":"DeepSeek-R1","object":"model","owned_by":"SambaNova","context_length":16384,"max_completion_tokens":8192,"pricing":{"prompt":"0.00000500","completion":"0.00000700"},"new_field":true},{"id":"Meta-Llama-3.3-70B-Instruct","object":"model","owned_by":"Meta","context_length":131072,"max_completion_tokens":4096,"pricing":{"prompt":"0.00000080","completion":"0.00000120"}}]}`))
	}))
	defer server.Close()
	provider := testProvider(server.URL)
	provider.LoadAPIKey()
	models, err := NewClient(provider).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 2 || models[0].Authors[0].ID != catalogs.AuthorIDDeepSeek || models[1].Authors[0].ID != catalogs.AuthorIDMeta {
		t.Fatalf("models = %#v", models)
	}
	deepseek := models[0]
	if deepseek.Limits.ContextWindow != 16384 || deepseek.Limits.OutputTokens != 8192 || deepseek.Pricing.Tokens.Input.Per1M != 5 || deepseek.Pricing.Tokens.Output.Per1M != 7 || deepseek.OfferingDeployment.Type != "serverless" {
		t.Fatalf("DeepSeek = %#v", deepseek)
	}
	if len(deepseek.Extensions["sambanova"].Fields["unknown_fields"].([]any)) == 0 {
		t.Fatalf("unknown-field evidence = %#v", deepseek.Extensions)
	}
}

func TestListModelsRejectsMalformedEnvelopeDuplicateAndPricing(t *testing.T) {
	tests := []string{
		`{"object":"list","data":null}`,
		`{"object":"wrong","data":[]}`,
		`{"object":"list","data":[{"id":"same","object":"model"},{"id":"same","object":"model"}]}`,
		`{"object":"list","data":[{"id":"bad","object":"model","pricing":{"prompt":"NaN"}}]}`,
	}
	for _, body := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(body))
		}))
		provider := testProvider(server.URL)
		if _, err := NewClient(provider).ListModels(context.Background()); err == nil {
			server.Close()
			t.Fatalf("accepted malformed response %s", body)
		}
		server.Close()
	}
}

func testProvider(endpoint string) *catalogs.Provider {
	return &catalogs.Provider{
		ID: catalogs.ProviderIDSambaNova, Name: "SambaNova Cloud",
		APIKey:  &catalogs.ProviderAPIKey{Name: "SAMBANOVA_API_KEY", Header: "Authorization", Scheme: catalogs.ProviderAPIKeySchemeBearer},
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeSambaNova, URL: endpoint, AuthRequired: true}},
	}
}
