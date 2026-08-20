package embedded_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/providers/clients"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestEmbeddedProviderContracts(t *testing.T) {
	t.Parallel()

	builder, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}
	providers := builder.Providers().List()
	for _, provider := range providers {
		if err := provider.ValidateContract(); err != nil {
			t.Fatalf("provider %s contract: %v", provider.ID, err)
		}
		if provider.Catalog != nil {
			if _, err := clients.NewProvider(&provider); err != nil {
				t.Fatalf("provider %s acquisition transport contract: %v", provider.ID, err)
			}
		}
	}

	for _, providerID := range []catalogs.ProviderID{
		catalogs.ProviderIDMistralAI,
		catalogs.ProviderIDAzureOpenAI,
		catalogs.ProviderIDOllama,
	} {
		provider, found := builder.Providers().Get(providerID)
		if !found {
			t.Fatalf("provider %s is missing", providerID)
		}
		if providerID != catalogs.ProviderIDMistralAI && len(provider.Models) != 0 {
			t.Fatalf("provider %s contains fabricated tenant or local models", providerID)
		}
	}

	vertex, found := builder.Providers().Get(catalogs.ProviderIDGoogleVertex)
	if !found || vertex.Catalog == nil ||
		!hasCatalogPrimitive(vertex, catalogs.ProviderAuthenticationGoogleDefault) {
		t.Fatalf("Vertex catalog auth = %#v", vertex)
	}
	azure, found := builder.Providers().Get(catalogs.ProviderIDAzureOpenAI)
	if !found || azure.Catalog == nil ||
		!hasCatalogPrimitive(azure, catalogs.ProviderAuthenticationAzureDefault) {
		t.Fatalf("Azure catalog auth = %#v", azure)
	}

	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("build embedded catalog: %v", err)
	}
	gemini, err := catalog.Offering(catalogs.ProviderIDGoogleVertex, "gemini-omni-flash-preview")
	if err != nil {
		t.Fatalf("Vertex Gemini offering: %v", err)
	}
	geminiEndpoint, found := gemini.Endpoint(catalogs.ProviderOperationChatCompletions)
	if !found || geminiEndpoint.Type != catalogs.EndpointTypeGoogleCloud ||
		!strings.HasSuffix(geminiEndpoint.URL, ":generateContent") ||
		!strings.HasSuffix(geminiEndpoint.StreamURL, ":streamGenerateContent") {
		t.Fatalf("Vertex Gemini endpoint = %#v", geminiEndpoint)
	}
	claude, err := catalog.Offering(catalogs.ProviderIDGoogleVertex, "claude-3-opus@20240229")
	if err != nil {
		t.Fatalf("Vertex Anthropic offering: %v", err)
	}
	claudeEndpoint, found := claude.Endpoint(catalogs.ProviderOperationChatCompletions)
	if !found || claudeEndpoint.Type != catalogs.EndpointTypeAnthropic ||
		!strings.HasSuffix(claudeEndpoint.URL, ":rawPredict") ||
		!strings.HasSuffix(claudeEndpoint.StreamURL, ":streamRawPredict") {
		t.Fatalf("Vertex Anthropic endpoint = %#v", claudeEndpoint)
	}
}

func TestEmbeddedHetznerProviderContract(t *testing.T) {
	t.Parallel()

	builder, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}
	provider, found := builder.Providers().Get(catalogs.ProviderIDHetzner)
	if !found {
		t.Fatal("Hetzner provider is missing")
	}
	if provider.Catalog == nil ||
		provider.Catalog.Endpoint.URL != "https://inference.hetzner.com/api/v1/models" {
		t.Fatalf("Hetzner catalog endpoint = %#v", provider.Catalog)
	}
	if provider.Inference == nil ||
		provider.Inference.BaseURL != "https://inference.hetzner.com/api/v1" {
		t.Fatalf("Hetzner inference = %#v", provider.Inference)
	}
	chat, found := provider.Inference.Endpoint(catalogs.ProviderOperationChatCompletions)
	if !found || chat.Type != catalogs.EndpointTypeOpenAI || chat.Path != "/chat/completions" {
		t.Fatalf("Hetzner chat endpoint = %#v", chat)
	}
	if provider.Credentials == nil || len(provider.Credentials.Fields) != 1 ||
		!reflect.DeepEqual(provider.Credentials.Fields[0].Environment, []string{"HETZNER_API_KEY"}) {
		t.Fatalf("Hetzner credential fields = %#v", provider.Credentials)
	}

	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("build embedded catalog: %v", err)
	}
	wants := map[catalogs.ProviderModelID]catalogs.ModelDefinitionID{
		"Qwen/Qwen3.6-35B-A3B-FP8": "qwen/qwen3.6-35b-a3b",
		"Qwen3.8-27B":              "qwen/qwen3.8-27b",
	}
	for providerModelID, definitionID := range wants {
		offering, err := catalog.Offering(catalogs.ProviderIDHetzner, providerModelID)
		if err != nil {
			t.Fatalf("Hetzner offering %q: %v", providerModelID, err)
		}
		if offering.DefinitionID != definitionID {
			t.Errorf("Hetzner offering %q definition = %q, want %q",
				providerModelID, offering.DefinitionID, definitionID)
		}
		if offering.Pricing != nil {
			t.Errorf("Hetzner offering %q has unpublished pricing: %#v",
				providerModelID, offering.Pricing)
		}
		if offering.Limits == nil || offering.Limits.ContextWindow != 262144 {
			t.Errorf("Hetzner offering %q limits = %#v", providerModelID, offering.Limits)
		}
		if !offering.Supports(catalogs.ProviderOperationChatCompletions) {
			t.Errorf("Hetzner offering %q does not support chat completions", providerModelID)
		}
		endpoint, found := offering.Endpoint(catalogs.ProviderOperationChatCompletions)
		if !found || endpoint.URL != "https://inference.hetzner.com/api/v1/chat/completions" {
			t.Errorf("Hetzner offering %q endpoint = %#v", providerModelID, endpoint)
		}
		definition, err := catalog.Definition(definitionID)
		if err != nil {
			t.Fatalf("Hetzner definition %q: %v", definitionID, err)
		}
		if definition.Capabilities.Features == nil ||
			!containsModalities(definition.Capabilities.Features.Modalities.Input,
				catalogs.ModelModalityText, catalogs.ModelModalityImage) {
			t.Errorf("Hetzner definition %q input modalities = %v",
				definitionID, definition.Capabilities.Features)
		}
	}
}

func containsModalities(have []catalogs.ModelModality, wants ...catalogs.ModelModality) bool {
	for _, want := range wants {
		found := false
		for _, modality := range have {
			if modality == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestEmbeddedProviderCredentialSchemaContract(t *testing.T) {
	t.Parallel()

	builder, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}
	providers := builder.Providers().List()
	if len(providers) == 0 {
		t.Fatal("embedded catalog has no providers")
	}
	for _, provider := range providers {
		if provider.Credentials == nil {
			t.Fatalf("provider %s has no credential schema", provider.ID)
		}
		if provider.Catalog != nil && len(provider.Credentials.CatalogAcquisition.Alternatives) == 0 {
			t.Fatalf("provider %s has no catalog-acquisition alternative", provider.ID)
		}
		if len(provider.Credentials.Inference.Alternatives) == 0 {
			t.Fatalf("provider %s has no inference alternative", provider.ID)
		}
		if err := provider.ValidateContract(); err != nil {
			t.Fatalf("provider %s credential contract: %v", provider.ID, err)
		}
		for _, profile := range provider.Credentials.Profiles {
			for _, binding := range profile.EndpointBindings {
				if binding.Format != catalogs.ProviderCredentialEndpointBindingURL &&
					binding.Format != catalogs.ProviderCredentialEndpointBindingPathSegment {
					t.Fatalf("provider %s profile %s has untyped endpoint binding %#v", provider.ID, profile.ID, binding)
				}
			}
		}
	}
	ollama, found := builder.Providers().Get(catalogs.ProviderIDOllama)
	if !found || ollama.Catalog != nil {
		t.Fatalf("Ollama catalog acquisition = %#v, want no compiled acquisition contract", ollama)
	}
}

func hasCatalogPrimitive(
	provider *catalogs.Provider,
	primitive catalogs.ProviderAuthenticationPrimitive,
) bool {
	if provider == nil || provider.Credentials == nil {
		return false
	}
	allowed := make(map[catalogs.ProviderCredentialProfileID]struct{},
		len(provider.Credentials.CatalogAcquisition.Alternatives))
	for _, profileID := range provider.Credentials.CatalogAcquisition.Alternatives {
		allowed[profileID] = struct{}{}
	}
	for _, profile := range provider.Credentials.Profiles {
		if _, exists := allowed[profile.ID]; exists && profile.Primitive == primitive {
			return true
		}
	}
	return false
}
