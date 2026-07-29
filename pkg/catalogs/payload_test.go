package catalogs

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestCatalogPayloadPersistsOnlyProviderModels(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetProvider(Provider{
		ID: "provider", Name: "Provider",
		Models: map[string]*Model{
			"model": {
				ID:      "model",
				Name:    "Model",
				Authors: []Author{{ID: "author", Name: "Author"}},
				Pricing: &ModelPricing{
					Currency: ModelPricingCurrencyUSD,
					Tokens: &ModelTokenPricing{
						CacheRead: &ModelTokenCost{Per1M: 0.25},
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}

	payload, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatalf("Unmarshal payload: %v", err)
	}

	keys := make([]string, 0, len(document))
	for key := range document {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	want := []string{
		"authors",
		"endpoints",
		"provenance",
		"provider_models",
		"providers",
		"schema_version",
	}
	if !slices.Equal(keys, want) {
		t.Fatalf("top-level payload keys = %v, want %v", keys, want)
	}

	var providerModels map[string][]Model
	if err := json.Unmarshal(document["provider_models"], &providerModels); err != nil {
		t.Fatalf("Unmarshal provider models: %v", err)
	}
	if models := providerModels["provider"]; len(models) != 1 || models[0].ID != "model" {
		t.Fatalf("provider models = %#v, want one canonical provider model", providerModels)
	}
	model := providerModels["provider"][0]
	if model.Pricing == nil || model.Pricing.Tokens == nil ||
		model.Pricing.Tokens.CacheRead == nil ||
		model.Pricing.Tokens.CacheRead.Per1M != 0.25 {
		t.Fatalf("flat cache pricing = %#v, want cache_read 0.25", model.Pricing)
	}
}
