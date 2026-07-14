package cerebras

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition/testsource"
	"github.com/agentstation/starmap/internal/providers/fixtures"
	"github.com/agentstation/starmap/internal/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestCerebrasEmbeddedConfigurationProjectsRichPublicInventory(t *testing.T) {
	provider := fixtures.EmbeddedProvider(t, catalogs.ProviderIDCerebras)
	configuredURL, err := url.Parse(provider.Catalog.Sources[0].Endpoint.URL)
	if err != nil {
		t.Fatalf("Parse configured URL: %v", err)
	}
	if configuredURL.Host != "api.cerebras.ai" || configuredURL.Path != "/public/v1/models" || configuredURL.Query().Get("format") != "openrouter" {
		t.Fatalf("catalog endpoint = %q", provider.Catalog.Sources[0].Endpoint.URL)
	}
	if provider.Catalog.Sources[0].Auth.Mode != catalogs.ProviderAuthModeNone {
		t.Fatal("public Cerebras catalog endpoint unexpectedly requires credentials")
	}
	if provider.Catalog.Sources[0].Offering == nil || provider.Catalog.Sources[0].Offering.Endpoint.BaseURL != "https://api.cerebras.ai/v1" {
		t.Fatalf("offering endpoint = %#v", provider.Catalog.Sources[0].Offering)
	}

	payload := fixtures.Load(t, "models_list.json")
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		if request.URL.Query().Get("format") != "openrouter" {
			t.Errorf("format query = %q", request.URL.RawQuery)
		}
		if authorization := request.Header.Get("Authorization"); authorization != "" {
			t.Errorf("public catalog authorization = %q", authorization)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(payload)
	}))
	t.Cleanup(server.Close)

	provider.Credentials = nil
	provider.Catalog.Sources[0].Endpoint.URL = server.URL + "?format=openrouter"
	client, err := registry.New(testsource.Unauthenticated(t, &provider))
	if err != nil {
		t.Fatalf("registry.New: %v", err)
	}
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("models = %#v", models)
	}
	model := models[0]
	if model.ID != "gpt-oss-120b" || model.Name != "GPT OSS 120B" || model.Description == "" {
		t.Fatalf("identity = %#v", model)
	}
	if len(model.Authors) != 1 || model.Authors[0].ID != catalogs.AuthorIDOpenAI {
		t.Fatalf("authors = %#v", model.Authors)
	}
	if model.Limits == nil || model.Limits.ContextWindow != 131072 || model.Limits.OutputTokens != 32768 {
		t.Fatalf("limits = %#v", model.Limits)
	}
	if model.Pricing == nil || model.Pricing.Tokens == nil || model.Pricing.Tokens.Input == nil || model.Pricing.Tokens.Output == nil ||
		math.Abs(model.Pricing.Tokens.Input.Per1M-0.85) > 1e-12 || math.Abs(model.Pricing.Tokens.Output.Per1M-1.2) > 1e-12 ||
		model.Pricing.Tokens.CacheRead == nil || math.Abs(model.Pricing.Tokens.CacheRead.Per1M-0.1) > 1e-12 {
		t.Fatalf("pricing = %#v tokens=%#v input=%#v output=%#v cache=%#v", model.Pricing, model.Pricing.Tokens, model.Pricing.Tokens.Input, model.Pricing.Tokens.Output, model.Pricing.Tokens.Cache)
	}
	if model.Status != catalogs.ModelStatusActive || model.Features == nil || !model.Features.Tools || !model.Features.ToolChoice ||
		!model.Features.Reasoning || !model.Features.StructuredOutputs || !model.Features.FormatResponse {
		t.Fatalf("lifecycle/features = %q/%#v", model.Status, model.Features)
	}
	if len(model.Features.Modalities.Input) != 1 || model.Features.Modalities.Input[0] != catalogs.ModelModalityText ||
		len(model.Features.Modalities.Output) != 1 || model.Features.Modalities.Output[0] != catalogs.ModelModalityText {
		t.Fatalf("modalities = %#v", model.Features.Modalities)
	}
	extension := model.Extensions[catalogs.ProviderIDCerebras.String()]
	unknown, ok := extension.Fields["unknown_fields"].([]any)
	if !ok || len(unknown) == 0 {
		t.Fatalf("unknown-field drift evidence = %#v", model.Extensions)
	}

	// A second acquisition must not share mutable slices or extension maps with the caller.
	model.Features.Modalities.Input[0] = catalogs.ModelModalityImage
	extension.Fields["fixture_mutation"] = true
	modelsAgain, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels again: %v", err)
	}
	if modelsAgain[0].Features.Modalities.Input[0] != catalogs.ModelModalityText || modelsAgain[0].Extensions[catalogs.ProviderIDCerebras.String()].Fields["fixture_mutation"] != nil {
		t.Fatalf("caller mutation leaked into later acquisition: %#v", modelsAgain[0])
	}
	encoded, err := json.Marshal(modelsAgain[0])
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, forbidden := range []string{"api_key", "authorization", "account_id", "customer_id"} {
		if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
			t.Fatalf("public model serialized customer/credential field %q: %s", forbidden, encoded)
		}
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d", requestCount)
	}
}

func TestCerebrasRejectsMissingOpenAICollection(t *testing.T) {
	provider := fixtures.EmbeddedProvider(t, catalogs.ProviderIDCerebras)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"object":"list","models":[]}`))
	}))
	t.Cleanup(server.Close)
	provider.Catalog.Sources[0].Endpoint.URL = server.URL
	provider.Catalog.Sources[0].Auth = catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone}
	client, err := registry.New(testsource.Unauthenticated(t, &provider))
	if err != nil {
		t.Fatalf("registry.New: %v", err)
	}
	if _, err := client.ListModels(context.Background()); err == nil {
		t.Fatal("missing OpenAI data collection was accepted")
	}
}
