package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition"
	"github.com/agentstation/starmap/internal/acquisition/testsource"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestListModelsUsesAccountSearchPaginationAndOpenRouterPricing(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "fixture-token")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Header.Get("Authorization") != "Bearer fixture-token" || request.URL.Query().Get("format") != "openrouter" || request.URL.Query().Get("hide_experimental") != "true" || request.URL.Query().Get("include_deprecated") != "true" || request.URL.Query().Get("per_page") != "50" {
			t.Errorf("auth/query = %q/%q", request.Header.Get("Authorization"), request.URL.RawQuery)
		}
		if request.URL.Path != "/accounts/account-123/ai/models/search" {
			t.Errorf("path = %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		page := request.URL.Query().Get("page")
		models := make([]map[string]any, 0)
		if page == "1" {
			for index := 0; index < perPage; index++ {
				models = append(models, map[string]any{"id": fmt.Sprintf("@cf/meta/fixture-%02d", index), "name": "Fixture", "context_length": 8192, "pricing": map[string]string{"prompt": "0.000000027", "completion": "0.000000201"}, "architecture": map[string]any{"modality": "text->text"}, "supported_parameters": []string{"tools"}})
			}
		} else {
			models = append(models, map[string]any{"id": "@cf/baai/bge-small-en-v1.5", "name": "BGE", "context_length": 512, "pricing": map[string]string{"prompt": "0.000000020"}, "architecture": map[string]any{"input_modalities": []string{"text"}, "output_modalities": []string{"embedding"}}, "new_upstream_field": true})
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"data": models})
	}))
	defer server.Close()
	provider := testProvider(t, server.URL, "account-123")
	models, err := NewClient(testsource.Authenticated(t, provider)).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if requests.Load() != 2 || len(models) != 51 {
		t.Fatalf("requests/models = %d/%d", requests.Load(), len(models))
	}
	chat := models[1]
	if chat.Pricing.Tokens.Input.Per1M != 0.027 || chat.Pricing.Tokens.Output.Per1M != 0.201 || !slices.Contains(chat.InvocationAPIs, catalogs.InvocationAPIChatCompletions) || chat.Authors[0].ID != catalogs.AuthorIDMeta {
		t.Fatalf("chat model = %#v", chat)
	}
	embedding := models[0]
	if !slices.Contains(embedding.InvocationAPIs, catalogs.InvocationAPIEmbeddings) || embedding.Authors[0].ID != "baai" || len(embedding.Extensions["cloudflare"].Fields["unknown_fields"].([]any)) == 0 {
		t.Fatalf("embedding model = %#v", embedding)
	}
	payload, err := json.Marshal(models)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, private := range []string{"fixture-token", "account-123", server.URL} {
		if strings.Contains(string(payload), private) {
			t.Fatalf("private account state leaked: %s", payload)
		}
	}
}

func TestListModelsRejectsMissingAccountNullDataAndInvalidPricing(t *testing.T) {
	provider := testProvider(t, "https://api.cloudflare.test", "")
	if _, err := acquisition.NewResolver().Resolve(context.Background(), provider, "models"); err == nil {
		t.Fatal("expected missing-account failure")
	}
	if _, err := convertModel(model{ID: "@cf/meta/model", Pricing: pricing{Prompt: "-0.1"}}); err == nil {
		t.Fatal("expected negative-price failure")
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":null}`))
	}))
	defer server.Close()
	provider = testProvider(t, server.URL, "account")
	if _, err := NewClient(testsource.Unauthenticated(t, provider)).ListModels(context.Background()); err == nil {
		t.Fatal("expected null-data failure")
	}
}

func TestUnknownModalityRemainsDiscoverableOnly(t *testing.T) {
	converted, err := convertModel(model{ID: "@cf/vendor/special", Architecture: architecture{OutputModalities: []string{"special"}}})
	if err != nil {
		t.Fatalf("convertModel: %v", err)
	}
	if len(converted.InvocationAPIs) != 0 || converted.OfferingAccess == nil || converted.OfferingAccess.Routability != catalogs.OfferingRoutabilityDiscoverable {
		t.Fatalf("unknown modality invented route: %#v", converted)
	}
}

func testProvider(t *testing.T, baseURL, accountID string) *catalogs.Provider {
	t.Helper()
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", accountID)
	t.Setenv("CLOUDFLARE_API_BASE_URL", baseURL)
	return &catalogs.Provider{
		ID: catalogs.ProviderIDCloudflare, Name: "Cloudflare Workers AI",
		Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": {Env: catalogs.ProviderEnvironmentNames{"CLOUDFLARE_API_TOKEN"}}},
		Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
			ID: "models", ObservationScope: catalogs.ProviderObservationPolicy{Invariant: catalogs.ProviderObservationScopeCredentialScoped},
			Auth: catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"api_key"}},
			Scopes: map[string]catalogs.ProviderScopeBinding{
				"account": {Source: catalogs.ProviderBindingSourceEnv, Name: catalogs.ProviderEnvironmentNames{"CLOUDFLARE_ACCOUNT_ID"}, Role: catalogs.ProviderBindingRoleRequiredInput},
			},
			Endpoint: catalogs.ProviderSourceEndpoint{Type: catalogs.EndpointTypeCloudflare, URL: "https://api.cloudflare.com/client/v4", BaseURLEnv: "CLOUDFLARE_API_BASE_URL"},
		}}},
	}
}
