package embedded_test

import (
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestEmbeddedProviderContracts(t *testing.T) {
	t.Parallel()

	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}
	providers := builder.Providers().List()
	for _, provider := range providers {
		if err := provider.ValidateContract(); err != nil {
			t.Fatalf("provider %s contract: %v", provider.ID, err)
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

func TestEmbeddedProviderCredentialSchemaContract(t *testing.T) {
	t.Parallel()

	builder, err := catalogs.NewEmbedded()
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
		if len(provider.Credentials.CatalogAcquisition.Alternatives) == 0 {
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
