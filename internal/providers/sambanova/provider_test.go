package sambanova_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition/testsource"
	"github.com/agentstation/starmap/internal/providers/fixtures"
	"github.com/agentstation/starmap/internal/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestListModelsMapsExactLimitsPricingAndAuthors(t *testing.T) {
	t.Setenv("SAMBANOVA_API_KEY", "fixture-token")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(fixtures.Load(t, "models_list.json"))
	}))
	defer server.Close()
	client := testClient(t, server.URL)
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 2 || models[0].Authors[0].ID != catalogs.AuthorIDDeepSeek || models[1].Authors[0].ID != catalogs.AuthorIDMeta {
		t.Fatalf("models = %#v", models)
	}
	deepseek := models[0]
	if deepseek.Limits.ContextWindow != 16384 || deepseek.Limits.OutputTokens != 8192 || deepseek.Pricing.Tokens.Input.Per1M != 5 || deepseek.Pricing.Tokens.Output.Per1M != 7 {
		t.Fatalf("DeepSeek = %#v", deepseek)
	}
	if len(deepseek.Extensions["sambanova"].Fields["unknown_fields"].([]any)) == 0 {
		t.Fatalf("unknown-field evidence = %#v", deepseek.Extensions)
	}
}

func TestListModelsRejectsMalformedEnvelopeDuplicateAndPricing(t *testing.T) {
	t.Setenv("SAMBANOVA_API_KEY", "fixture-token")
	tests := []string{
		`{"object":"list","data":null}`,
		`{"object":"wrong","data":[]}`,
		`{"object":"list","data":[{"id":"same","object":"model","owned_by":"one"},{"id":"same","object":"model","owned_by":"two"}]}`,
		`{"object":"list","data":[{"id":"bad","object":"model","pricing":{"prompt":"NaN"}}]}`,
	}
	for _, body := range tests {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(body))
		}))
		client := testClient(t, server.URL)
		if _, err := client.ListModels(context.Background()); err == nil {
			server.Close()
			t.Fatalf("accepted malformed response %s", body)
		}
		server.Close()
	}
}

func testClient(t *testing.T, endpoint string) registry.Connector {
	t.Helper()
	provider := fixtures.EmbeddedProvider(t, catalogs.ProviderIDSambaNova)
	provider.Catalog.Sources[0].Endpoint.URL = endpoint
	provider.Catalog.Sources[0].Endpoint.Path = ""
	client, err := registry.New(testsource.Authenticated(t, &provider))
	if err != nil {
		t.Fatalf("registry.New: %v", err)
	}
	return client
}
