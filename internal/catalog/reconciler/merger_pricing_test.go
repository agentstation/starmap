package reconciler

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/sources"
)

// TestFieldPriorities tests that field priorities are respected.
func TestFieldPriorities(t *testing.T) {
	// Set up authorities with default priorities
	// The default authorities already have pricing from models.dev
	// and name from provider API
	authorities := authority.New()

	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	sources := map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:   "model-1",
				Name: "Provider API Name",
				Pricing: &catalogs.ModelPricing{
					Currency: "EUR", // Wrong currency from Provider API
				},
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID:   "model-1",
				Name: "ModelsDev Name", // This should be ignored
				Pricing: &catalogs.ModelPricing{
					Currency: "USD", // Correct currency from ModelsDev
					Tokens: &catalogs.ModelTokenPricing{
						Input: &catalogs.ModelTokenCost{
							Per1M: 0.5,
						},
					},
				},
			},
		},
	}

	result, _, err := merger.Models(sources)
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 model, got %d", len(result))
	}

	model := result[0]

	// Name should come from Provider API (has priority)
	if model.Name != "Provider API Name" {
		t.Errorf("Expected name from Provider API, got %s", model.Name)
	}

	// Pricing should come from ModelsDev (has authority)
	if model.Pricing == nil || model.Pricing.Currency != "USD" {
		t.Error("Expected pricing from ModelsDev with USD currency")
	}
}

func TestMergeModelsUsesCompleteProviderPricingWithoutSubfieldMixing(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	providerInput := 99.0
	modelsDevInput := 0.5
	modelsDevOutput := 1.0
	providerCacheRead := 0.25
	providerRequest := 0.01

	result, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:   "model-1",
				Name: "Provider Model",
				Pricing: &catalogs.ModelPricing{
					Currency: catalogs.ModelPricingCurrencyEUR,
					Tokens: &catalogs.ModelTokenPricing{
						Input:     &catalogs.ModelTokenCost{Per1M: providerInput},
						CacheRead: &catalogs.ModelTokenCost{Per1M: providerCacheRead},
					},
					Operations: &catalogs.ModelOperationPricing{
						Request: &providerRequest,
					},
				},
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID:   "model-1",
				Name: "ModelsDev Model",
				Pricing: &catalogs.ModelPricing{
					Currency: catalogs.ModelPricingCurrencyUSD,
					Tokens: &catalogs.ModelTokenPricing{
						Input:  &catalogs.ModelTokenCost{Per1M: modelsDevInput},
						Output: &catalogs.ModelTokenCost{Per1M: modelsDevOutput},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("models = %d, want 1", len(result))
	}
	pricing := result[0].Pricing
	if pricing == nil || pricing.Currency != catalogs.ModelPricingCurrencyEUR {
		t.Fatalf("pricing = %#v, want provider currency", pricing)
	}
	if pricing.Tokens == nil ||
		pricing.Tokens.Input == nil ||
		pricing.Tokens.Input.Per1M != providerInput {
		t.Fatalf("provider token pricing was not preserved: %#v", pricing.Tokens)
	}
	if pricing.Tokens.Output != nil {
		t.Fatalf("models.dev output price leaked into atomic provider pricing: %#v", pricing.Tokens)
	}
	if pricing.Tokens.CacheRead == nil || pricing.Tokens.CacheRead.Per1M != providerCacheRead {
		t.Fatalf("provider cache pricing was not filled: %#v", pricing.Tokens)
	}
	if pricing.Operations == nil || pricing.Operations.Request == nil || *pricing.Operations.Request != providerRequest {
		t.Fatalf("provider operation pricing was not filled: %#v", pricing.Operations)
	}
}

func TestPricingAuthorityValidProviderOfferingWinsAtomically(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	providerInput := 2.0
	modelsDevInput := 0.5
	modelsDevOutput := 1.0
	model, history := merger.model("openai", "model-1", map[sources.ID]*catalogs.Model{
		sources.ProvidersID: {
			ID:   "model-1",
			Name: "Provider Model",
			Pricing: &catalogs.ModelPricing{
				Currency: catalogs.ModelPricingCurrencyEUR,
				Tokens: &catalogs.ModelTokenPricing{
					Input: &catalogs.ModelTokenCost{Per1M: providerInput},
				},
			},
		},
		sources.ModelsDevHTTPID: {
			ID:   "model-1",
			Name: "ModelsDev Model",
			Pricing: &catalogs.ModelPricing{
				Currency: catalogs.ModelPricingCurrencyUSD,
				Tokens: &catalogs.ModelTokenPricing{
					Input:  &catalogs.ModelTokenCost{Per1M: modelsDevInput},
					Output: &catalogs.ModelTokenCost{Per1M: modelsDevOutput},
				},
			},
		},
	})

	if model.Pricing == nil {
		t.Fatal("pricing is nil")
	}
	if got := model.Pricing.Currency; got != catalogs.ModelPricingCurrencyEUR {
		t.Fatalf("currency = %q, want provider currency %q", got, catalogs.ModelPricingCurrencyEUR)
	}
	if model.Pricing.Tokens == nil || model.Pricing.Tokens.Input == nil || model.Pricing.Tokens.Input.Per1M != providerInput {
		t.Fatalf("input pricing = %#v, want provider price %v", model.Pricing.Tokens, providerInput)
	}
	if model.Pricing.Tokens.Output != nil {
		t.Fatalf("output pricing = %#v, want nil; pricing must not mix source subfields", model.Pricing.Tokens.Output)
	}
	if got := history[modelProvenancePricing].Current.Source; got != sources.ProvidersID {
		t.Fatalf("pricing provenance source = %q, want %q", got, sources.ProvidersID)
	}
}

func TestMergeModelsProviderPricingBeatsLocalWhenModelsDevAbsent(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	localInput := 1.0
	providerInput := 2.0
	result, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.LocalCatalogID: {
			{
				ID:   "model-1",
				Name: "Local Model",
				Pricing: &catalogs.ModelPricing{
					Currency: catalogs.ModelPricingCurrencyUSD,
					Tokens: &catalogs.ModelTokenPricing{
						Input: &catalogs.ModelTokenCost{Per1M: localInput},
					},
				},
			},
		},
		sources.ProvidersID: {
			{
				ID:   "model-1",
				Name: "Provider Model",
				Pricing: &catalogs.ModelPricing{
					Currency: catalogs.ModelPricingCurrencyUSD,
					Tokens: &catalogs.ModelTokenPricing{
						Input: &catalogs.ModelTokenCost{Per1M: providerInput},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("models = %d, want 1", len(result))
	}
	if result[0].Pricing == nil ||
		result[0].Pricing.Tokens == nil ||
		result[0].Pricing.Tokens.Input == nil ||
		result[0].Pricing.Tokens.Input.Per1M != providerInput {
		t.Fatalf("provider pricing did not win without models.dev: %#v", result[0].Pricing)
	}
}

func TestMergeModelsProviderPricingAtomicallyReplacesBaseline(t *testing.T) {
	baseline := catalogs.NewEmpty()
	baselineInput := 1.0
	baselineReasoning := 3.0
	baselineTierInput := 0.5
	baselineModel := catalogs.Model{
		ID:   "model-1",
		Name: "Baseline Model",
		Pricing: &catalogs.ModelPricing{
			Currency: catalogs.ModelPricingCurrencyUSD,
			Tokens: &catalogs.ModelTokenPricing{
				Input:     &catalogs.ModelTokenCost{Per1M: baselineInput},
				Reasoning: &catalogs.ModelTokenCost{Per1M: baselineReasoning},
			},
			Tiers: []catalogs.ModelPricingTier{{
				Name: "baseline-tier",
				Tokens: &catalogs.ModelTokenPricing{
					Input: &catalogs.ModelTokenCost{Per1M: baselineTierInput},
				},
			}},
		},
	}
	if err := baseline.SetProvider(catalogs.Provider{
		ID:   "openai",
		Name: "OpenAI",
		Models: map[string]*catalogs.Model{
			baselineModel.ID: &baselineModel,
		},
	}); err != nil {
		t.Fatalf("set baseline provider: %v", err)
	}

	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, snapshotForTest(t, baseline))
	providerInput := 2.0
	providerOutput := 4.0
	result, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:   "model-1",
				Name: "Provider Model",
				Pricing: &catalogs.ModelPricing{
					Currency: catalogs.ModelPricingCurrencyUSD,
					Tokens: &catalogs.ModelTokenPricing{
						Input:  &catalogs.ModelTokenCost{Per1M: providerInput},
						Output: &catalogs.ModelTokenCost{Per1M: providerOutput},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("models = %d, want 1", len(result))
	}
	pricing := result[0].Pricing
	if pricing == nil ||
		pricing.Tokens == nil ||
		pricing.Tokens.Input == nil ||
		pricing.Tokens.Input.Per1M != providerInput ||
		pricing.Tokens.Output == nil ||
		pricing.Tokens.Output.Per1M != providerOutput {
		t.Fatalf("provider pricing did not replace baseline: %#v", pricing)
	}
	if pricing.Tokens.Reasoning != nil {
		t.Fatalf("baseline reasoning price leaked into provider pricing: %#v", pricing.Tokens)
	}
	if len(pricing.Tiers) != 0 {
		t.Fatalf("baseline pricing tiers leaked into provider pricing: %#v", pricing.Tiers)
	}
}
