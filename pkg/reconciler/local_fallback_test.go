package reconciler

import (
	"testing"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestDynamicModelFactsLeadLocalFallback(t *testing.T) {
	authorities := authority.New()
	merger := newMerger(authorities, NewAuthorityStrategy(authorities), nil)

	models, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {{
			ID:   "model-1",
			Name: "Live Provider Name",
			Limits: &catalogs.ModelLimits{
				ContextWindow: 128_000,
			},
		}},
		sources.ModelsDevHTTPID: {{
			ID:          "model-1",
			Name:        "Upstream Name",
			Description: "Current upstream description",
			Limits: &catalogs.ModelLimits{
				InputTokens: 96_000,
			},
		}},
		sources.LocalCatalogID: {{
			ID:          "model-1",
			Name:        "Stale local name",
			Description: "Stale local description",
			Limits: &catalogs.ModelLimits{
				ContextWindow: 32_000,
				InputTokens:   16_000,
				OutputTokens:  8_192,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("models = %d, want 1", len(models))
	}
	model := models[0]
	if model.Name != "Live Provider Name" {
		t.Fatalf("name = %q, want live provider fact", model.Name)
	}
	if model.Description != "Current upstream description" {
		t.Fatalf("description = %q, want maintained upstream fact", model.Description)
	}
	if model.Limits == nil ||
		model.Limits.ContextWindow != 128_000 ||
		model.Limits.InputTokens != 96_000 ||
		model.Limits.OutputTokens != 8_192 {
		t.Fatalf("limits = %#v, want provider/upstream facts with local gap fill", model.Limits)
	}
}

func TestLocalModelFactSurvivesWhenDynamicSourcesDoNotSupplyIt(t *testing.T) {
	authorities := authority.New()
	merger := newMerger(authorities, NewAuthorityStrategy(authorities), nil)

	models, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {{
			ID:   "model-1",
			Name: "Live Provider Name",
		}},
		sources.LocalCatalogID: {{
			ID:          "model-1",
			Name:        "Local Name",
			Description: "Human-supplied missing description",
		}},
	})
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("models = %d, want 1", len(models))
	}
	if got := models[0].Description; got != "Human-supplied missing description" {
		t.Fatalf("description = %q, want local missing-data fallback", got)
	}
}

func TestLocalOperatorConfigurationLeadsDiscoveredProviderMetadata(t *testing.T) {
	authorities := authority.New()
	merger := newMerger(authorities, NewAuthorityStrategy(authorities), nil)
	localURL := "https://operator.example/models"

	providers, err := merger.Providers(map[sources.ID][]*catalogs.Provider{
		sources.ModelsDevHTTPID: {{
			ID:   "openai",
			Name: "Observed Provider",
			APIKey: &catalogs.ProviderAPIKey{
				Name:   "DISCOVERED_KEY",
				Header: "X-API-Key",
			},
			Catalog: &catalogs.ProviderCatalog{
				Endpoint: catalogs.ProviderEndpoint{URL: "https://discovered.example/models"},
			},
		}},
		sources.LocalCatalogID: {{
			ID:   "openai",
			Name: "Manual Provider Name",
			APIKey: &catalogs.ProviderAPIKey{
				Name:   "OPERATOR_KEY",
				Header: "Authorization",
			},
			Catalog: &catalogs.ProviderCatalog{
				Endpoint: catalogs.ProviderEndpoint{URL: localURL},
			},
		}},
	})
	if err != nil {
		t.Fatalf("Providers: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("providers = %d, want 1", len(providers))
	}
	provider := providers[0]
	if provider.Name != "Observed Provider" {
		t.Fatalf("name = %q, want dynamic discovery fact", provider.Name)
	}
	if provider.APIKey == nil || provider.APIKey.Name != "OPERATOR_KEY" {
		t.Fatalf("api key configuration = %#v, want operator value", provider.APIKey)
	}
	if provider.Catalog == nil || provider.Catalog.Endpoint.URL != localURL {
		t.Fatalf("catalog configuration = %#v, want operator endpoint", provider.Catalog)
	}
}
