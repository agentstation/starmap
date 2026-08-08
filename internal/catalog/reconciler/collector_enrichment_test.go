package reconciler

import (
	"context"
	"testing"

	"github.com/agentstation/starmap/internal/catalog/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestCollectorEnrichmentDoesNotUseSameIDModelFromDifferentProvider(t *testing.T) {
	primaryCatalog := catalogs.NewEmpty()
	primaryModel := catalogs.Model{
		ID:       "shared-model",
		ModelRef: "test-author/shared-model",
		Name:     "Primary Provider Model",
	}
	author := catalogs.Author{ID: "test-author", Name: "Test Author"}
	if err := primaryCatalog.SetAuthor(author); err != nil {
		t.Fatalf("set primary author: %v", err)
	}
	if err := primaryCatalog.SetAuthorModel(author.ID, catalogs.Model{
		ID: "shared-model", Name: "Shared Model", Authors: []catalogs.Author{author},
	}); err != nil {
		t.Fatalf("set primary authored model: %v", err)
	}
	primaryProvider := catalogs.Provider{
		ID:   "target-provider",
		Name: "Target Provider",
		Models: map[string]*catalogs.Model{
			primaryModel.ID: &primaryModel,
		},
	}
	if err := primaryCatalog.SetProvider(primaryProvider); err != nil {
		t.Fatalf("set primary provider: %v", err)
	}

	enrichmentCatalog := catalogs.NewEmpty()
	if err := enrichmentCatalog.SetProvider(catalogs.Provider{
		ID:     "target-provider",
		Name:   "Target Provider from enrichment",
		Models: map[string]*catalogs.Model{},
	}); err != nil {
		t.Fatalf("set empty target enrichment provider: %v", err)
	}
	otherProviderModel := catalogs.Model{
		ID:   primaryModel.ID,
		Name: "Wrong Provider Model",
		Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
			Tokens: &catalogs.ModelTokenPricing{
				Input: &catalogs.ModelTokenCost{Per1M: 123.45},
			},
		},
	}
	if err := enrichmentCatalog.SetProvider(catalogs.Provider{
		ID:   "other-provider",
		Name: "Other Provider from enrichment",
		Models: map[string]*catalogs.Model{
			otherProviderModel.ID: &otherProviderModel,
		},
	}); err != nil {
		t.Fatalf("set other enrichment provider: %v", err)
	}

	reconcile, err := New(WithAuthorities(authority.New()))
	if err != nil {
		t.Fatalf("New reconciler: %v", err)
	}
	result, err := reconcile.Sources(context.Background(), sources.ProvidersID, ConvertCatalogsMapToSources(map[sources.ID]*catalogs.Builder{
		sources.ProvidersID:     primaryCatalog,
		sources.ModelsDevHTTPID: enrichmentCatalog,
	}))
	if err != nil {
		t.Fatalf("reconcile sources: %v", err)
	}
	provider, err := result.Catalog.Provider(primaryProvider.ID)
	if err != nil {
		t.Fatalf("target provider missing: %v", err)
	}
	got := provider.Models[primaryModel.ID]
	if got == nil {
		t.Fatal("target model missing")
	}
	if got.Pricing != nil {
		t.Fatalf("same-ID enrichment model from wrong provider affected target model: %#v", got.Pricing)
	}
}
