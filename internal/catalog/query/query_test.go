package query

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestPaginate(t *testing.T) {
	items := []int{1, 2, 3, 4}

	page := Paginate(items, 2, 1)
	if page.Total != 4 || page.Limit != 2 || page.Offset != 1 || page.Count != 2 {
		t.Fatalf("Unexpected pagination metadata: %#v", page)
	}
	if got := page.Items; len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("Unexpected page items: %#v", got)
	}

	empty := Paginate(items, 2, 10)
	if empty.Total != 4 || empty.Count != 0 || len(empty.Items) != 0 {
		t.Fatalf("Expected empty out-of-range page, got %#v", empty)
	}
}

func TestModelsFiltersSortsAndLimits(t *testing.T) {
	cheap := 0.25
	expensive := 1.25
	models := []catalogs.Model{
		{
			ID:   "z-model",
			Name: "Z Model",
			Authors: []catalogs.Author{{
				ID:   "openai",
				Name: "OpenAI",
			}},
			Features: &catalogs.ModelFeatures{Reasoning: true},
			Limits:   &catalogs.ModelLimits{ContextWindow: 128000},
			Pricing: &catalogs.ModelPricing{Tokens: &catalogs.ModelTokenPricing{
				Input: &catalogs.ModelTokenCost{Per1M: cheap},
			}},
		},
		{
			ID:   "a-model",
			Name: "A Model",
			Authors: []catalogs.Author{{
				ID:   "anthropic",
				Name: "Anthropic",
			}},
			Features: &catalogs.ModelFeatures{Streaming: true},
			Limits:   &catalogs.ModelLimits{ContextWindow: 200000},
			Pricing: &catalogs.ModelPricing{Tokens: &catalogs.ModelTokenPricing{
				Input: &catalogs.ModelTokenCost{Per1M: expensive},
			}},
		},
	}

	filtered := Models(models, ModelOptions{
		Author:     "openai",
		Capability: "reasoning",
		MinContext: 100000,
		MaxPrice:   0.50,
		Search:     "z",
		Limit:      1,
	})
	if len(filtered) != 1 || filtered[0].ID != "z-model" {
		t.Fatalf("Unexpected filtered models: %#v", filtered)
	}

	sorted := Models(models, ModelOptions{})
	if len(sorted) != 2 || sorted[0].ID != "a-model" || sorted[1].ID != "z-model" {
		t.Fatalf("Expected models sorted by ID, got %#v", sorted)
	}
}

func TestModelsFiltersByProvider(t *testing.T) {
	catalog := catalogs.NewEmpty()
	providers := []catalogs.Provider{
		{
			ID:      "openai",
			Aliases: []catalogs.ProviderID{"openai-alias"},
			Models: map[string]*catalogs.Model{
				"openai-model": {ID: "openai-model", Name: "OpenAI Model"},
			},
		},
		{
			ID: "anthropic",
			Models: map[string]*catalogs.Model{
				"anthropic-model": {ID: "anthropic-model", Name: "Anthropic Model"},
			},
		},
	}
	for _, provider := range providers {
		if err := catalog.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider(%s): %v", provider.ID, err)
		}
	}
	snapshot, err := catalog.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	filtered, err := CatalogModels(snapshot, "openai")
	if err != nil {
		t.Fatalf("CatalogModels(openai): %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != "openai-model" {
		t.Fatalf("Expected only OpenAI model, got %#v", filtered)
	}

	aliasFiltered, err := CatalogModels(snapshot, "openai-alias")
	if err != nil {
		t.Fatalf("CatalogModels(openai-alias): %v", err)
	}
	if len(aliasFiltered) != 1 || aliasFiltered[0].ID != "openai-model" {
		t.Fatalf("Expected OpenAI alias to resolve provider models, got %#v", aliasFiltered)
	}

	unknown, err := CatalogModels(snapshot, "missing-provider")
	if err != nil {
		t.Fatalf("CatalogModels(missing-provider): %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("Expected unknown provider to return no models, got %#v", unknown)
	}
}

func TestProviderFilterPreservesDuplicateModelOfferings(t *testing.T) {
	catalog := catalogs.NewEmpty()
	for _, provider := range []catalogs.Provider{
		{
			ID:   "a-provider",
			Name: "Provider A",
			Models: map[string]*catalogs.Model{
				"shared-model": {ID: "shared-model", Name: "Provider A Offering"},
			},
		},
		{
			ID:   "b-provider",
			Name: "Provider B",
			Models: map[string]*catalogs.Model{
				"shared-model": {ID: "shared-model", Name: "Provider B Offering"},
			},
		},
	} {
		if err := catalog.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider(%s): %v", provider.ID, err)
		}
	}

	snapshot, err := catalog.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	filtered, err := CatalogModels(snapshot, "a-provider")
	if err != nil {
		t.Fatalf("CatalogModels(a-provider): %v", err)
	}
	if len(filtered) != 1 || filtered[0].Name != "Provider A Offering" {
		t.Fatalf("Provider A lookup returned wrong canonical definition: %#v", filtered)
	}
	bOffering, err := snapshot.Offering("b-provider", "shared-model")
	if err != nil {
		t.Fatalf("Offering(b-provider/shared-model): %v", err)
	}
	if bOffering.ProviderModelID != "shared-model" {
		t.Fatalf("Provider B offering = %#v", bOffering)
	}
}

func TestProvidersFiltersSortsAndLimits(t *testing.T) {
	hq := "San Francisco"
	providers := []catalogs.Provider{
		{ID: "z-provider", Name: "Z Provider"},
		{ID: "a-provider", Name: "A Provider", Headquarters: &hq},
	}

	filtered := Providers(providers, ProviderOptions{Search: "francisco", Limit: 1})
	if len(filtered) != 1 || filtered[0].ID != "a-provider" {
		t.Fatalf("Unexpected filtered providers: %#v", filtered)
	}

	sorted := Providers(providers, ProviderOptions{})
	if len(sorted) != 2 || sorted[0].ID != "a-provider" || sorted[1].ID != "z-provider" {
		t.Fatalf("Expected providers sorted by ID, got %#v", sorted)
	}
}

func TestAuthorsFiltersSortsAndLimits(t *testing.T) {
	description := "Frontier research lab"
	authors := []catalogs.Author{
		{ID: "z-author", Name: "Z Author"},
		{ID: "a-author", Name: "A Author", Description: &description},
	}

	filtered := Authors(authors, AuthorOptions{Search: "research", Limit: 1})
	if len(filtered) != 1 || filtered[0].ID != "a-author" {
		t.Fatalf("Unexpected filtered authors: %#v", filtered)
	}

	sorted := Authors(authors, AuthorOptions{})
	if len(sorted) != 2 || sorted[0].ID != "a-author" || sorted[1].ID != "z-author" {
		t.Fatalf("Expected authors sorted by ID, got %#v", sorted)
	}
}
