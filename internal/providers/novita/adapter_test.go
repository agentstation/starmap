package novita

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/providers/openai"
	"github.com/agentstation/starmap/internal/providers/testhelper"
	"github.com/agentstation/starmap/pkg/catalogs"
	starmaperrors "github.com/agentstation/starmap/pkg/errors"
)

func TestNovitaPreservesFixedPointPricingLimitsAndRawEvidence(t *testing.T) {
	client, closeServer := newNovitaFixtureClient(t, string(testhelper.LoadTestdata(t, "models_list.json")))
	defer closeServer()
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	model := findModel(t, models, "meta-llama/llama-3.3-70b-instruct")
	if model.Authors[0].ID != catalogs.AuthorIDMeta || model.Name != "Llama 3.3 70B Instruct" || model.Description == "" || model.Limits.ContextWindow != 131072 {
		t.Fatalf("identity/metadata/limits = %#v", model)
	}
	if model.Pricing.Currency != catalogs.ModelPricingCurrencyUSD || model.Pricing.Tokens.Input.Per1M != 0.135 || model.Pricing.Tokens.Output.Per1M != 0.4 {
		t.Fatalf("pricing = %#v", model.Pricing)
	}
	if model.Modes["batch"].Pricing.Tokens.Input.Per1M != 0.0675 || model.Modes["batch"].Pricing.Tokens.Output.Per1M != 0.2 {
		t.Fatalf("batch pricing = %#v", model.Modes)
	}
	if len(model.InvocationAPIs) != 0 {
		t.Fatalf("live inventory invented configured contracts = %#v", model.InvocationAPIs)
	}
	fields := model.Extensions["novita"].Fields
	if fields["input_token_price_per_m_raw"] != int64(135) || fields["output_token_price_per_m_raw"] != int64(400) || len(fields["unknown_fields"].([]any)) == 0 {
		t.Fatalf("raw/drift evidence = %#v", fields)
	}
}

func TestNovitaRejectsMalformedPricesAndDuplicateIDs(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "negative price", body: `{"data":[{"id":"meta-llama/model","object":"model","input_token_price_per_m":-1,"output_token_price_per_m":1,"title":"Model","description":"Description","context_size":1}]}`, want: "must not be negative"},
		{name: "missing context", body: `{"data":[{"id":"meta-llama/model","object":"model","input_token_price_per_m":1,"output_token_price_per_m":1,"title":"Model","description":"Description"}]}`, want: "must be positive"},
		{name: "duplicate", body: `{"data":[{"id":"meta-llama/model","object":"model","input_token_price_per_m":1,"output_token_price_per_m":1,"title":"Model","description":"Description","context_size":1},{"id":"meta-llama/model","object":"model","input_token_price_per_m":1,"output_token_price_per_m":1,"title":"Model","description":"Description","context_size":1}]}`, want: "duplicate model id"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, closeServer := newNovitaFixtureClient(t, test.body)
			defer closeServer()
			_, err := client.ListModels(context.Background())
			apiErr, ok := err.(*starmaperrors.APIError)
			if !ok || apiErr.Err == nil || !strings.Contains(apiErr.Err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestNovitaOfferingDefaultsLiveInProviderConfiguration(t *testing.T) {
	builder, err := catalogs.NewFromPath(filepath.Join("..", "..", "..", "internal", "embedded", "catalog"))
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	offering, err := catalog.Offering(catalogs.ProviderIDNovita, "meta-llama/llama-3.3-70b-instruct")
	if err != nil {
		t.Fatalf("Offering: %v", err)
	}
	if offering.Endpoint.BaseURL != "https://api.novita.ai/openai/v1" || offering.Deployment.Tier != "pay-per-token" ||
		len(offering.Access.APIs) != 2 || offering.Access.APIs[1] != catalogs.InvocationAPICompletions {
		t.Fatalf("configured offering defaults = %#v", offering)
	}
}

func newNovitaFixtureClient(t *testing.T, body string) (*openai.Client, func()) {
	t.Helper()
	t.Setenv("NOVITA_API_KEY", "fixture-token")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(body))
	}))
	provider := &catalogs.Provider{
		ID: catalogs.ProviderIDNovita, Name: "Novita AI LLM API",
		APIKey: &catalogs.ProviderAPIKey{Name: "NOVITA_API_KEY", Header: "Authorization", Scheme: catalogs.ProviderAPIKeySchemeBearer},
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
			Type: catalogs.EndpointTypeOpenAI, URL: server.URL, AuthRequired: true,
			AuthorMapping: &catalogs.AuthorMapping{Field: "id", Normalized: map[string]catalogs.AuthorID{"meta-llama/*": catalogs.AuthorIDMeta}},
		}},
	}
	provider.LoadAPIKey()
	client, err := openai.NewClient(provider, Options()...)
	if err != nil {
		server.Close()
		t.Fatalf("NewClient: %v", err)
	}
	return client, server.Close
}

func findModel(t *testing.T, models []catalogs.Model, id string) *catalogs.Model {
	t.Helper()
	for i := range models {
		if models[i].ID == id {
			return &models[i]
		}
	}
	t.Fatalf("model %q not found", id)
	return nil
}
