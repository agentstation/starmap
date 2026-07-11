package catalogs

import (
	"testing"
	"time"

	"github.com/agentstation/utc"
)

func TestDeepCopyProviderModels(t *testing.T) {
	// Test nil input
	t.Run("nil input", func(t *testing.T) {
		result := DeepCopyProviderModels(nil)
		if result != nil {
			t.Error("Expected nil result for nil input")
		}
	})

	// Test empty map
	t.Run("empty map", func(t *testing.T) {
		input := make(map[string]*Model)
		result := DeepCopyProviderModels(input)
		if result == nil {
			t.Fatal("Expected non-nil result for empty map")
		}
		if len(result) != 0 {
			t.Error("Expected empty result map")
		}
		// Verify it's a different map instance
		if &input == &result {
			t.Error("Expected different map instances")
		}
	})

	// Test map with models
	t.Run("map with models", func(t *testing.T) {
		model1 := &Model{
			ID:        "model-1",
			Name:      "Test Model 1",
			CreatedAt: utc.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		}
		model2 := &Model{
			ID:        "model-2",
			Name:      "Test Model 2",
			CreatedAt: utc.New(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)),
		}

		input := map[string]*Model{
			"model-1": model1,
			"model-2": model2,
		}

		result := DeepCopyProviderModels(input)

		// Check length
		if len(result) != len(input) {
			t.Errorf("Expected length %d, got %d", len(input), len(result))
		}

		// Check that models are copied, not shared
		for k, v := range input {
			resultModel := result[k]
			if resultModel == nil {
				t.Errorf("Missing model for key %s", k)
				continue
			}

			// Different pointers (deep copy)
			if v == resultModel {
				t.Errorf("Expected different pointer for model %s", k)
			}

			// Same content
			if v.ID != resultModel.ID || v.Name != resultModel.Name {
				t.Errorf("Model content mismatch for key %s", k)
			}
		}

		// Verify mutation independence
		result["model-1"].Name = "Modified"
		if input["model-1"].Name == "Modified" {
			t.Error("Original model should not be affected by copy mutation")
		}
	})

	// Test map with nil model pointer
	t.Run("map with nil model", func(t *testing.T) {
		input := map[string]*Model{
			"nil-model": nil,
		}

		result := DeepCopyProviderModels(input)
		if len(result) != 1 {
			t.Error("Expected one entry in result")
		}
		if result["nil-model"] != nil {
			t.Error("Expected nil model to remain nil")
		}
	})
}

func TestDeepCopyAuthorModels(t *testing.T) {
	// Test nil input
	t.Run("nil input", func(t *testing.T) {
		result := DeepCopyAuthorModels(nil)
		if result != nil {
			t.Error("Expected nil result for nil input")
		}
	})

	// Test basic functionality (same logic as provider models)
	t.Run("basic functionality", func(t *testing.T) {
		model := &Model{
			ID:        "author-model",
			Name:      "Author Test Model",
			CreatedAt: utc.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		}

		input := map[string]*Model{
			"author-model": model,
		}

		result := DeepCopyAuthorModels(input)

		if len(result) != 1 {
			t.Error("Expected one model in result")
		}

		resultModel := result["author-model"]
		if resultModel == model {
			t.Error("Expected different pointer (deep copy)")
		}

		if resultModel.ID != model.ID {
			t.Error("Expected same content after copy")
		}
	})
}

func TestDeepCopyProvider(t *testing.T) {
	t.Run("provider with models", func(t *testing.T) {
		model := &Model{
			ID:   "test-model",
			Name: "Test Model",
		}

		original := Provider{
			ID:   "test-provider",
			Name: "Test Provider",
			Models: map[string]*Model{
				"test-model": model,
			},
		}

		copy := DeepCopyProvider(original)

		// Basic field copy
		if copy.ID != original.ID || copy.Name != original.Name {
			t.Error("Provider fields not copied correctly")
		}

		// Models map should be deep copied
		if &copy.Models == &original.Models {
			t.Error("Models map should be different instance")
		}

		if copy.Models["test-model"] == original.Models["test-model"] {
			t.Error("Model pointers should be different (deep copy)")
		}

		if copy.Models["test-model"].ID != original.Models["test-model"].ID {
			t.Error("Model content should be the same")
		}

		// Test mutation independence
		copy.Models["test-model"].Name = "Modified"
		if original.Models["test-model"].Name == "Modified" {
			t.Error("Original should not be affected by copy mutation")
		}
	})

	t.Run("provider without models", func(t *testing.T) {
		original := Provider{
			ID:     "test-provider",
			Name:   "Test Provider",
			Models: nil,
		}

		copy := DeepCopyProvider(original)

		if copy.Models != nil {
			t.Error("Models should remain nil")
		}
		if copy.ID != original.ID {
			t.Error("Provider fields should be copied")
		}
	})
}

func TestDeepCopyAuthor(t *testing.T) {
	t.Run("author with models", func(t *testing.T) {
		model := &Model{
			ID:   "test-model",
			Name: "Test Model",
		}

		original := Author{
			ID:          "test-author",
			Name:        "Test Author",
			Description: stringPtr("Test description"),
			Models: map[string]*Model{
				"test-model": model,
			},
		}

		copy := DeepCopyAuthor(original)

		// Basic field copy
		if copy.ID != original.ID || copy.Name != original.Name {
			t.Error("Author fields not copied correctly")
		}

		// Models map should be deep copied
		if &copy.Models == &original.Models {
			t.Error("Models map should be different instance")
		}

		if copy.Models["test-model"] == original.Models["test-model"] {
			t.Error("Model pointers should be different (deep copy)")
		}

		if copy.Models["test-model"].ID != original.Models["test-model"].ID {
			t.Error("Model content should be the same")
		}

		// Test mutation independence
		copy.Models["test-model"].Name = "Modified"
		if original.Models["test-model"].Name == "Modified" {
			t.Error("Original should not be affected by copy mutation")
		}
	})
}

func TestShallowCopyProviderModels(t *testing.T) {
	t.Run("shallow copy behavior", func(t *testing.T) {
		model := &Model{
			ID:   "test-model",
			Name: "Test Model",
		}

		input := map[string]*Model{
			"test-model": model,
		}

		result := ShallowCopyProviderModels(input)

		// Different map instances
		if &input == &result {
			t.Error("Expected different map instances")
		}

		// Same model pointers (shallow copy)
		if result["test-model"] != input["test-model"] {
			t.Error("Expected same model pointers (shallow copy)")
		}

		// Verify shared mutation
		result["test-model"].Name = "Modified"
		if input["test-model"].Name != "Modified" {
			t.Error("Both maps should share model instances")
		}
	})
}

func TestShallowCopyAuthorModels(t *testing.T) {
	t.Run("shallow copy behavior", func(t *testing.T) {
		model := &Model{
			ID:   "test-model",
			Name: "Test Model",
		}

		input := map[string]*Model{
			"test-model": model,
		}

		result := ShallowCopyAuthorModels(input)

		// Different map instances
		if &input == &result {
			t.Error("Expected different map instances")
		}

		// Same model pointers (shallow copy)
		if result["test-model"] != input["test-model"] {
			t.Error("Expected same model pointers (shallow copy)")
		}
	})
}

func TestDeepCopyModelCopiesNestedMutableFields(t *testing.T) {
	precision := "fp16"
	searchPrompt := "find sources"
	topLogprobs := 5
	root := "root-model"
	parent := "parent-model"
	tierInput := 2.5
	modeInput := 3.5

	original := Model{
		ID:   "nested-model",
		Name: "Nested Model",
		Lineage: &ModelLineage{
			Family: "nested",
			Root:   &root,
			Parent: &parent,
		},
		Metadata: &ModelMetadata{
			Tags: []ModelTag{ModelTagCoding},
			Architecture: &ModelArchitecture{
				Precision: &precision,
			},
		},
		Features: &ModelFeatures{
			Modalities: ModelModalities{
				Input: []ModelModality{ModelModalityText},
			},
		},
		Generation: &ModelGeneration{
			TopLogprobs: &topLogprobs,
		},
		Tools: &ModelTools{
			ToolChoices: []ToolChoice{ToolChoiceAuto},
			WebSearch: &ModelWebSearch{
				SearchPrompt:       &searchPrompt,
				SearchContextSizes: []ModelControlLevel{ModelControlLevelLow},
			},
		},
		Pricing: &ModelPricing{
			Tokens: &ModelTokenPricing{
				Input: &ModelTokenCost{Per1M: 1.25},
			},
			Tiers: []ModelPricingTier{{
				Type: ModelPricingTierTypeContext,
				Size: 200000,
				Tokens: &ModelTokenPricing{
					Input: &ModelTokenCost{Per1M: tierInput},
				},
			}},
		},
		Modes: map[string]ModelMode{
			"fast": {
				Pricing: &ModelPricing{
					Tokens: &ModelTokenPricing{
						Input: &ModelTokenCost{Per1M: modeInput},
					},
				},
				Provider: &ModelProviderMode{
					Headers: map[string]string{"anthropic-beta": "fast-mode"},
					Body: map[string]any{
						"service_tier": "priority",
						"reasoning":    map[string]any{"mode": "pro"},
					},
				},
			},
		},
		Extensions: SourceExtensions{
			"models.dev": {
				Fields: map[string]any{
					"provider_shape": "chat",
					"flags":          []any{"experimental"},
				},
			},
		},
	}

	copied := DeepCopyModel(original)
	*copied.Lineage.Root = "changed-root"
	*copied.Lineage.Parent = "changed-parent"
	copied.Metadata.Tags[0] = ModelTagMath
	*copied.Metadata.Architecture.Precision = "fp8"
	copied.Features.Modalities.Input[0] = ModelModalityImage
	*copied.Generation.TopLogprobs = 10
	*copied.Tools.WebSearch.SearchPrompt = "changed"
	copied.Tools.WebSearch.SearchContextSizes[0] = ModelControlLevelHigh
	copied.Pricing.Tokens.Input.Per1M = 9.99
	copied.Pricing.Tiers[0].Tokens.Input.Per1M = 7.77
	copied.Modes["fast"].Pricing.Tokens.Input.Per1M = 8.88
	copied.Modes["fast"].Provider.Headers["anthropic-beta"] = "changed-mode"
	copied.Modes["fast"].Provider.Body["service_tier"] = "changed-tier"
	copied.Modes["fast"].Provider.Body["reasoning"].(map[string]any)["mode"] = "changed-mode"
	copied.Extensions["models.dev"].Fields["provider_shape"] = "responses"
	copied.Extensions["models.dev"].Fields["flags"].([]any)[0] = "changed"

	if original.Metadata.Tags[0] != ModelTagCoding {
		t.Fatal("metadata tags were shared between original and copy")
	}
	if *original.Lineage.Root != "root-model" || *original.Lineage.Parent != "parent-model" {
		t.Fatal("lineage pointers were shared between original and copy")
	}
	if *original.Metadata.Architecture.Precision != "fp16" {
		t.Fatal("architecture precision pointer was shared between original and copy")
	}
	if original.Features.Modalities.Input[0] != ModelModalityText {
		t.Fatal("feature modality slice was shared between original and copy")
	}
	if *original.Generation.TopLogprobs != 5 {
		t.Fatal("generation pointer was shared between original and copy")
	}
	if *original.Tools.WebSearch.SearchPrompt != "find sources" {
		t.Fatal("web search prompt pointer was shared between original and copy")
	}
	if original.Tools.WebSearch.SearchContextSizes[0] != ModelControlLevelLow {
		t.Fatal("web search context size slice was shared between original and copy")
	}
	if original.Pricing.Tokens.Input.Per1M != 1.25 {
		t.Fatal("pricing token cost pointer was shared between original and copy")
	}
	if original.Pricing.Tiers[0].Tokens.Input.Per1M != tierInput {
		t.Fatal("pricing tier token cost pointer was shared between original and copy")
	}
	if original.Modes["fast"].Pricing.Tokens.Input.Per1M != modeInput {
		t.Fatal("mode pricing pointer was shared between original and copy")
	}
	if original.Modes["fast"].Provider.Headers["anthropic-beta"] != "fast-mode" {
		t.Fatal("mode provider headers map was shared between original and copy")
	}
	if original.Modes["fast"].Provider.Body["service_tier"] != "priority" {
		t.Fatal("mode provider body map was shared between original and copy")
	}
	if original.Modes["fast"].Provider.Body["reasoning"].(map[string]any)["mode"] != "pro" {
		t.Fatal("nested mode provider body map was shared between original and copy")
	}
	if original.Extensions["models.dev"].Fields["provider_shape"] != "chat" {
		t.Fatal("model extension fields map was shared between original and copy")
	}
	if original.Extensions["models.dev"].Fields["flags"].([]any)[0] != "experimental" {
		t.Fatal("model extension fields slice was shared between original and copy")
	}
}

func TestDeepCopyProviderCopiesNestedMutableFields(t *testing.T) {
	docs := "https://example.com/docs"
	privacyURL := "https://example.com/privacy"

	original := Provider{
		ID:      "provider",
		Name:    "Provider",
		Aliases: []ProviderID{"provider-alias"},
		Catalog: &ProviderCatalog{
			Docs: &docs,
			Endpoint: ProviderEndpoint{
				FeatureRules: []FeatureRule{{
					Field:    "id",
					Contains: []string{"reasoning"},
					Feature:  "reasoning",
					Value:    true,
				}},
				AuthorMapping: &AuthorMapping{
					Field: "owned_by",
					Normalized: map[string]AuthorID{
						"openai": AuthorIDOpenAI,
					},
				},
			},
			Authors: []AuthorID{AuthorIDOpenAI},
		},
		PrivacyPolicy: &ProviderPrivacyPolicy{
			PrivacyPolicyURL: &privacyURL,
		},
		EnvVarValues: map[string]string{
			"API_KEY": "secret",
		},
		Extensions: SourceExtensions{
			"models.dev": {
				Fields: map[string]any{
					"npm": "@ai-sdk/anthropic",
				},
			},
		},
	}

	copied := DeepCopyProvider(original)
	copied.Aliases[0] = "changed"
	*copied.Catalog.Docs = "changed"
	copied.Catalog.Endpoint.FeatureRules[0].Contains[0] = "changed"
	copied.Catalog.Endpoint.AuthorMapping.Normalized["openai"] = AuthorIDGoogle
	copied.Catalog.Authors[0] = AuthorIDGoogle
	*copied.PrivacyPolicy.PrivacyPolicyURL = "changed"
	copied.EnvVarValues["API_KEY"] = "changed"
	copied.Extensions["models.dev"].Fields["npm"] = "@changed/provider"

	if original.Aliases[0] != "provider-alias" {
		t.Fatal("provider aliases were shared between original and copy")
	}
	if *original.Catalog.Docs != "https://example.com/docs" {
		t.Fatal("provider catalog docs pointer was shared between original and copy")
	}
	if original.Catalog.Endpoint.FeatureRules[0].Contains[0] != "reasoning" {
		t.Fatal("feature rule contains slice was shared between original and copy")
	}
	if original.Catalog.Endpoint.AuthorMapping.Normalized["openai"] != AuthorIDOpenAI {
		t.Fatal("author mapping map was shared between original and copy")
	}
	if original.Catalog.Authors[0] != AuthorIDOpenAI {
		t.Fatal("provider catalog authors slice was shared between original and copy")
	}
	if *original.PrivacyPolicy.PrivacyPolicyURL != "https://example.com/privacy" {
		t.Fatal("provider privacy pointer was shared between original and copy")
	}
	if original.EnvVarValues["API_KEY"] != "secret" {
		t.Fatal("provider environment values map was shared between original and copy")
	}
	if original.Extensions["models.dev"].Fields["npm"] != "@ai-sdk/anthropic" {
		t.Fatal("provider extension fields map was shared between original and copy")
	}
}
