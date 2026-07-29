package reconciler

import (
	"testing"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
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
				ID:   "openai",
				Name: "OpenAI Local",
				APIKey: &catalogs.ProviderAPIKey{
					Name:   "LOCAL_KEY",
					Header: "Authorization",
				},
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
				ID:   "openai",
				Name: "OpenAI models.dev",
				APIKey: &catalogs.ProviderAPIKey{
					Name:   "MODELS_DEV_KEY",
					Header: "X-API-Key",
				},
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
	if provider.APIKey == nil || provider.APIKey.Name != "LOCAL_KEY" {
		t.Fatalf("Expected local API key configuration, got %#v", provider.APIKey)
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

// TestMergeComplexStructures tests merging of complex nested structures.
