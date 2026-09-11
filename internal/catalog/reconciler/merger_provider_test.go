package reconciler

import (
	"reflect"
	"testing"

	"github.com/agentstation/starmap/internal/sources/modelsdev"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestMergeProviders(t *testing.T) {
	tests := []struct {
		name     string
		sources  map[sources.ID][]*catalogs.Provider
		expected []*catalogs.Provider
	}{
		{
			name:     "empty sources",
			sources:  map[sources.ID][]*catalogs.Provider{},
			expected: []*catalogs.Provider{},
		},
		{
			name: "single source single provider",
			sources: map[sources.ID][]*catalogs.Provider{
				sources.ProvidersID: {
					createTestProvider("openai", "OpenAI"),
				},
			},
			expected: []*catalogs.Provider{
				createTestProvider("openai", "OpenAI"),
			},
		},
		{
			name: "multiple sources same provider",
			sources: map[sources.ID][]*catalogs.Provider{
				sources.ProvidersID: {
					createTestProvider("openai", "OpenAI API"),
				},
				sources.LocalCatalogID: {
					{
						ID:           "openai",
						Name:         "OpenAI Embedded",
						Headquarters: stringPtr("San Francisco, USA"),
					},
				},
			},
			expected: []*catalogs.Provider{
				{
					ID:           "openai",
					Name:         "OpenAI API", // Current provider identity beats local fallback.
					Headquarters: stringPtr("San Francisco, USA"),
					Models: map[string]*catalogs.Model{
						"model-1": createTestModel("model-1", "Test Model", 1000),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorities := authority.New()
			strategy := NewAuthorityStrategy(authorities)
			merger := newMerger(authorities, strategy, nil)

			result, err := merger.Providers(tt.sources)
			if err != nil {
				t.Fatalf("MergeProviders failed: %v", err)
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d providers, got %d", len(tt.expected), len(result))
				return
			}

			// Create maps for easier comparison
			resultMap := make(map[catalogs.ProviderID]*catalogs.Provider)
			for _, p := range result {
				resultMap[p.ID] = p
			}

			for _, expected := range tt.expected {
				actual, found := resultMap[expected.ID]
				if !found {
					t.Errorf("Expected provider %s not found in result", expected.ID)
					continue
				}

				if actual.Name != expected.Name {
					t.Errorf("Provider %s: expected name %s, got %s", expected.ID, expected.Name, actual.Name)
				}

				// Check headquarters if present
				if expected.Headquarters != nil && actual.Headquarters != nil {
					if *actual.Headquarters != *expected.Headquarters {
						t.Errorf("Provider %s: expected headquarters %s, got %s",
							expected.ID, *expected.Headquarters, *actual.Headquarters)
					}
				}
			}
		})
	}
}

func TestMergeProvidersUsesProviderAuthorities(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	localURL := "https://local.example.com/models"
	modelsDevURL := "https://models-dev.example.com/models"

	result, err := merger.Providers(map[sources.ID][]*catalogs.Provider{
		sources.LocalCatalogID: {
			{
				ID: "openai", Name: "OpenAI Local",
				Credentials: testcatalog.APIKeyCredentials(
					"LOCAL_KEY", "Authorization", catalogs.ProviderCredentialSchemeBearer,
				),
				Catalog: &catalogs.ProviderCatalog{
					Endpoint: catalogs.ProviderEndpoint{
						Type: catalogs.EndpointTypeOpenAI,
						URL:  localURL,
					},
				},
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID: "openai", Name: "OpenAI models.dev",
				Credentials: testcatalog.APIKeyCredentials(
					"MODELS_DEV_KEY", "X-API-Key", catalogs.ProviderCredentialSchemeDirect,
				),
				Catalog: &catalogs.ProviderCatalog{
					Endpoint: catalogs.ProviderEndpoint{
						Type: catalogs.EndpointTypeOpenAI,
						URL:  modelsDevURL,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("MergeProviders failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(result))
	}

	provider := result[0]
	if provider.Name != "OpenAI models.dev" {
		t.Fatalf("Expected observed provider name, got %q", provider.Name)
	}
	if provider.Credentials == nil ||
		provider.Credentials.Fields[0].Environment[0] != "LOCAL_KEY" {
		t.Fatalf("Expected local credential configuration, got %#v", provider.Credentials)
	}
	if provider.Catalog == nil || provider.Catalog.Endpoint.URL != localURL {
		t.Fatalf("Expected local catalog endpoint, got %#v", provider.Catalog)
	}
}

func TestMergeProvidersCombinesSourceExtensions(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	result, err := merger.Providers(map[sources.ID][]*catalogs.Provider{
		sources.LocalCatalogID: {
			{
				ID:   "openai",
				Name: "OpenAI Local",
				Extensions: catalogs.SourceExtensions{
					"models.dev": {Fields: map[string]any{"npm": "local-package"}},
				},
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID:   "openai",
				Name: "OpenAI models.dev",
				Extensions: catalogs.SourceExtensions{
					"models.dev": {Fields: map[string]any{
						"npm": "models-dev-package",
						"doc": "https://models.dev/openai",
					}},
				},
			},
		},
		sources.ProvidersID: {
			{
				ID:   "openai",
				Name: "OpenAI Provider",
				Extensions: catalogs.SourceExtensions{
					"openai": {Fields: map[string]any{"status": "live"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("MergeProviders failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("providers = %d, want 1", len(result))
	}
	if result[0].Extensions["models.dev"].Fields["npm"] != "local-package" {
		t.Fatalf("local provider extension field was overwritten: %#v", result[0].Extensions)
	}
	if result[0].Extensions["models.dev"].Fields["doc"] != "https://models.dev/openai" {
		t.Fatalf("models.dev provider extension field missing: %#v", result[0].Extensions)
	}
	if result[0].Extensions["openai"].Fields["status"] != "live" {
		t.Fatalf("provider extension field missing: %#v", result[0].Extensions)
	}
}

func TestMergeModelsDevDocsPreservesProviderContracts(t *testing.T) {
	baseline, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}
	for _, providerID := range []catalogs.ProviderID{"cohere", "openai"} {
		for _, sourceID := range []sources.ID{sources.ModelsDevHTTPID, sources.ModelsDevGitID} {
			t.Run(string(providerID)+"/"+string(sourceID), func(t *testing.T) {
				original, err := baseline.Provider(providerID)
				if err != nil {
					t.Fatalf("baseline provider: %v", err)
				}
				if err := original.ValidateContract(); err != nil {
					t.Fatalf("baseline contract: %v", err)
				}
				observed, err := (&modelsdev.Provider{
					ID: string(providerID), Name: original.Name,
					Doc: "https://example.test/provider-docs",
				}).ToStarmapProvider()
				if err != nil {
					t.Fatalf("convert models.dev provider: %v", err)
				}
				authorities := authority.New()
				merger := newMerger(authorities, NewAuthorityStrategy(authorities), nil)
				providers, err := merger.Providers(map[sources.ID][]*catalogs.Provider{
					sources.LocalCatalogID: {&original},
					sourceID:               {observed},
				})
				if err != nil {
					t.Fatalf("merge providers: %v", err)
				}
				if len(providers) != 1 {
					t.Fatalf("provider count = %d, want 1", len(providers))
				}
				got := providers[0]
				if err := got.ValidateContract(); err != nil {
					t.Fatalf("merged provider contract: %v", err)
				}
				if !reflect.DeepEqual(got.Catalog, original.Catalog) {
					t.Errorf("catalog acquisition changed: got %#v, want %#v", got.Catalog, original.Catalog)
				}
				if !reflect.DeepEqual(got.Inference, original.Inference) {
					t.Errorf("inference endpoints changed: got %#v, want %#v", got.Inference, original.Inference)
				}
				if !reflect.DeepEqual(got.Credentials, original.Credentials) {
					t.Errorf("provider credential contract changed")
				}
				if !reflect.DeepEqual(got.DocsURL, original.DocsURL) {
					t.Errorf("curated provider documentation changed")
				}
			})
		}
	}
}
