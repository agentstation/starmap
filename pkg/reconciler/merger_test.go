package reconciler

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/authority"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/utc"
	"github.com/google/go-cmp/cmp"
)

// Test helper functions
func createTestModel(id, name string, contextWindow int64) catalogs.Model {
	return catalogs.Model{
		ID:          id,
		Name:        name,
		Description: "Test model " + name,
		Limits: &catalogs.ModelLimits{
			ContextWindow: contextWindow,
		},
	}
}

func createTestModelWithPricing(id, name string, inputCost, outputCost float64) catalogs.Model {
	return catalogs.Model{
		ID:   id,
		Name: name,
		Pricing: &catalogs.ModelPricing{
			Currency: "USD",
			Tokens: &catalogs.ModelTokenPricing{
				Input: &catalogs.ModelTokenCost{
					Per1M: inputCost,
				},
				Output: &catalogs.ModelTokenCost{
					Per1M: outputCost,
				},
			},
		},
	}
}

func createTestProvider(id catalogs.ProviderID, name string) catalogs.Provider {
	return catalogs.Provider{
		ID:   id,
		Name: name,
		Models: map[string]catalogs.Model{
			"model-1": createTestModel("model-1", "Test Model", 1000),
		},
	}
}

// TestMergerInterface tests that the interface is properly implemented
func TestMergerInterface(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)

	// Ensure newStrategicMerger returns Merger interface
	var merger Merger = newMerger(authorities, strategy)
	if merger == nil {
		t.Fatal("Expected merger to be non-nil")
	}

	// Test with provenance
	tracker := provenance.NewTracker(true)
	mergerWithProv := newMergerWithProvenance(authorities, strategy, tracker)
	if mergerWithProv == nil {
		t.Fatal("Expected merger with provenance to be non-nil")
	}
}

// TestMergeModelsBasic tests basic model merging
func TestMergeModelsBasic(t *testing.T) {
	tests := []struct {
		name     string
		sources  map[sources.Type][]catalogs.Model
		expected []catalogs.Model
	}{
		{
			name:     "empty sources",
			sources:  map[sources.Type][]catalogs.Model{},
			expected: []catalogs.Model{},
		},
		{
			name: "single source single model",
			sources: map[sources.Type][]catalogs.Model{
				sources.ProviderAPI: {
					createTestModel("model-1", "API Model", 1000),
				},
			},
			expected: []catalogs.Model{
				createTestModel("model-1", "API Model", 1000),
			},
		},
		{
			name: "multiple sources same model",
			sources: map[sources.Type][]catalogs.Model{
				sources.ProviderAPI: {
					createTestModel("model-1", "API Model", 1000),
				},
				sources.ModelsDevHTTP: {
					createTestModelWithPricing("model-1", "ModelsDev Model", 0.5, 1.0),
				},
			},
			expected: []catalogs.Model{
				{
					ID:   "model-1",
					Name: "API Model", // Provider API name takes precedence
					Limits: &catalogs.ModelLimits{
						ContextWindow: 1000,
					},
					Pricing: &catalogs.ModelPricing{
						Currency: "USD",
						Tokens: &catalogs.ModelTokenPricing{
							Input: &catalogs.ModelTokenCost{
								Per1M: 0.5,
							},
							Output: &catalogs.ModelTokenCost{
								Per1M: 1.0,
							},
						},
					},
				},
			},
		},
		{
			name: "multiple models from different sources",
			sources: map[sources.Type][]catalogs.Model{
				sources.ProviderAPI: {
					createTestModel("model-1", "API Model 1", 1000),
					createTestModel("model-2", "API Model 2", 2000),
				},
				sources.ModelsDevHTTP: {
					createTestModel("model-3", "ModelsDev Model", 3000),
				},
			},
			expected: []catalogs.Model{
				createTestModel("model-1", "API Model 1", 1000),
				createTestModel("model-2", "API Model 2", 2000),
				createTestModel("model-3", "ModelsDev Model", 3000),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorities := authority.New()
			strategy := NewAuthorityStrategy(authorities)
			merger := newMerger(authorities, strategy)

			result, _, err := merger.Models(tt.sources)
			if err != nil {
				t.Fatalf("MergeModels failed: %v", err)
			}

			// Compare lengths first
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d models, got %d", len(tt.expected), len(result))
				return
			}

			// Create maps for easier comparison
			resultMap := make(map[string]catalogs.Model)
			for _, m := range result {
				resultMap[m.ID] = m
			}

			for _, expected := range tt.expected {
				actual, found := resultMap[expected.ID]
				if !found {
					t.Errorf("Expected model %s not found in result", expected.ID)
					continue
				}

				// Compare key fields
				if actual.Name != expected.Name {
					t.Errorf("Model %s: expected name %s, got %s", expected.ID, expected.Name, actual.Name)
				}

				// Compare limits if present
				if expected.Limits != nil && actual.Limits != nil {
					if actual.Limits.ContextWindow != expected.Limits.ContextWindow {
						t.Errorf("Model %s: expected context window %d, got %d",
							expected.ID, expected.Limits.ContextWindow, actual.Limits.ContextWindow)
					}
				}

				// Compare pricing if present
				if expected.Pricing != nil && actual.Pricing != nil {
					if actual.Pricing.Tokens != nil && expected.Pricing.Tokens != nil {
						if actual.Pricing.Tokens.Input != nil && expected.Pricing.Tokens.Input != nil {
							if actual.Pricing.Tokens.Input.Per1M != expected.Pricing.Tokens.Input.Per1M {
								t.Errorf("Model %s: expected input cost %f, got %f",
									expected.ID, expected.Pricing.Tokens.Input.Per1M, actual.Pricing.Tokens.Input.Per1M)
							}
						}
					}
				}
			}
		})
	}
}

// TestMergeModelsWithProvenance tests provenance tracking
func TestMergeModelsWithProvenance(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	tracker := provenance.NewTracker(true)

	merger := newMergerWithProvenance(authorities, strategy, tracker)

	sources := map[sources.Type][]catalogs.Model{
		sources.ProviderAPI: {
			createTestModel("model-1", "API Model", 1000),
		},
		sources.ModelsDevHTTP: {
			createTestModelWithPricing("model-1", "ModelsDev Model", 0.5, 1.0),
		},
	}

	merged, prov, err := merger.Models(sources)
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}

	if len(merged) != 1 {
		t.Errorf("Expected 1 merged model, got %d", len(merged))
	}

	if len(prov) == 0 {
		t.Error("Expected provenance to be tracked")
	}

	// Check specific provenance entries
	if _, exists := prov["models.model-1.Name"]; !exists {
		t.Error("Expected provenance for model name")
	}

	if _, exists := prov["models.model-1.pricing"]; !exists {
		t.Error("Expected provenance for model pricing")
	}
}

// TestMergeProviders tests provider merging
func TestMergeProviders(t *testing.T) {
	tests := []struct {
		name     string
		sources  map[sources.Type][]catalogs.Provider
		expected []catalogs.Provider
	}{
		{
			name:     "empty sources",
			sources:  map[sources.Type][]catalogs.Provider{},
			expected: []catalogs.Provider{},
		},
		{
			name: "single source single provider",
			sources: map[sources.Type][]catalogs.Provider{
				sources.ProviderAPI: {
					createTestProvider("openai", "OpenAI"),
				},
			},
			expected: []catalogs.Provider{
				createTestProvider("openai", "OpenAI"),
			},
		},
		{
			name: "multiple sources same provider",
			sources: map[sources.Type][]catalogs.Provider{
				sources.ProviderAPI: {
					createTestProvider("openai", "OpenAI API"),
				},
				sources.LocalCatalog: {
					{
						ID:           "openai",
						Name:         "OpenAI Embedded",
						Headquarters: stringPtr("San Francisco, USA"),
					},
				},
			},
			expected: []catalogs.Provider{
				{
					ID:           "openai",
					Name:         "OpenAI API", // ProviderAPI has higher authority for Name field
					Headquarters: stringPtr("San Francisco, USA"),
					Models: map[string]catalogs.Model{
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
			merger := newMerger(authorities, strategy)

			result, _, err := merger.Providers(tt.sources)
			if err != nil {
				t.Fatalf("MergeProviders failed: %v", err)
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d providers, got %d", len(tt.expected), len(result))
				return
			}

			// Create maps for easier comparison
			resultMap := make(map[catalogs.ProviderID]catalogs.Provider)
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

// TestMergeComplexStructures tests merging of complex nested structures
func TestMergeComplexStructures(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy)

	// Create models with complex structures
	sources := map[sources.Type][]catalogs.Model{
		sources.ProviderAPI: {
			{
				ID:   "model-1",
				Name: "Test Model",
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{"text"},
						Output: []catalogs.ModelModality{"text"},
					},
					Streaming: true,
				},
			},
		},
		sources.ModelsDevHTTP: {
			{
				ID: "model-1",
				Metadata: &catalogs.ModelMetadata{
					ReleaseDate: utc.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
					OpenWeights: true,
				},
				Features: &catalogs.ModelFeatures{
					ToolCalls: true,
					WebSearch: true,
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

	// Check features were merged correctly
	if model.Features == nil {
		t.Fatal("Expected features to be non-nil")
	}

	// Provider API features should be preserved
	if !model.Features.Streaming {
		t.Error("Expected streaming to be true from Provider API")
	}

	// ModelsDev features should be added
	if !model.Features.ToolCalls {
		t.Error("Expected tool calls to be true from ModelsDev")
	}

	if !model.Features.WebSearch {
		t.Error("Expected web search to be true from ModelsDev")
	}

	// Check metadata from ModelsDev
	if model.Metadata == nil {
		t.Fatal("Expected metadata to be non-nil")
	}

	if !model.Metadata.OpenWeights {
		t.Error("Expected open weights to be true")
	}
}

// TestFieldPriorities tests that field priorities are respected
func TestFieldPriorities(t *testing.T) {
	// Set up authorities with default priorities
	// The default authorities already have pricing from models.dev
	// and name from provider API
	authorities := authority.New()

	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy)

	sources := map[sources.Type][]catalogs.Model{
		sources.ProviderAPI: {
			{
				ID:   "model-1",
				Name: "Provider API Name",
				Pricing: &catalogs.ModelPricing{
					Currency: "EUR", // Wrong currency from Provider API
				},
			},
		},
		sources.ModelsDevHTTP: {
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

// TestNilHandling tests handling of nil values
func TestNilHandling(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy)

	// Test with nil maps in sources
	sources := map[sources.Type][]catalogs.Model{
		sources.ProviderAPI: {
			{
				ID:       "model-1",
				Name:     "Test Model",
				Limits:   nil, // nil limits
				Pricing:  nil, // nil pricing
				Features: nil, // nil features
			},
		},
		sources.ModelsDevHTTP: {
			{
				ID: "model-1",
				Limits: &catalogs.ModelLimits{
					ContextWindow: 1000,
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

	// Should have limits from ModelsDev
	if model.Limits == nil || model.Limits.ContextWindow != 1000 {
		t.Error("Expected limits to be merged from ModelsDev")
	}
}

// TestEmptySources tests merging with empty source maps
func TestEmptySources(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy)

	// Test with completely empty sources
	result, prov, err := merger.Models(map[sources.Type][]catalogs.Model{})
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected 0 models for empty sources, got %d", len(result))
	}

	if len(prov) != 0 {
		t.Errorf("Expected no provenance for empty sources, got %d entries", len(prov))
	}

	// Test with nil source maps
	result, prov, err = merger.Models(nil)
	if err != nil {
		t.Fatalf("MergeModels failed with nil: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected 0 models for nil sources, got %d", len(result))
	}
}

// TestConcurrentMerging tests thread safety of merger
func TestConcurrentMerging(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy)

	// Run multiple merges concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			sources := map[sources.Type][]catalogs.Model{
				sources.ProviderAPI: {
					createTestModel(string(rune('0'+id)), "Model", 1000),
				},
			}

			_, _, err := merger.Models(sources)
			if err != nil {
				t.Errorf("Concurrent merge %d failed: %v", id, err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// BenchmarkMergeModels benchmarks model merging performance
func BenchmarkMergeModels(b *testing.B) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy)

	// Create test data
	testSources := map[sources.Type][]catalogs.Model{
		sources.ProviderAPI:   make([]catalogs.Model, 100),
		sources.ModelsDevHTTP: make([]catalogs.Model, 100),
	}

	for i := 0; i < 100; i++ {
		id := string(rune('a'+i%26)) + string(rune('0'+i/26))
		testSources[sources.ProviderAPI][i] = createTestModel(id, "API Model", int64(i*1000))
		testSources[sources.ModelsDevHTTP][i] = createTestModelWithPricing(id, "ModelsDev Model", float64(i)*0.1, float64(i)*0.2)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := merger.Models(testSources)
		if err != nil {
			b.Fatalf("MergeModels failed: %v", err)
		}
	}
}

// TestMergeFieldReflection tests the reflection-based field merging
func TestMergeFieldReflection(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)

	// Access internal merger to test field methods
	sm := &merger{
		authorities: authorities,
		strategy:    strategy,
	}

	// Test getFieldValue
	model := catalogs.Model{
		ID:   "test",
		Name: "Test Model",
		Limits: &catalogs.ModelLimits{
			ContextWindow: 1000,
		},
	}

	// Test simple field access
	name := sm.modelFieldValue(model, "Name")
	if name != "Test Model" {
		t.Errorf("Expected name 'Test Model', got %v", name)
	}

	// Test nested field access - this won't work with current implementation
	// as it expects capitalized field names in the path
	contextWindow := sm.modelFieldValue(model, "Limits")
	if contextWindow == nil {
		t.Error("Expected Limits to be non-nil")
	}

	// Test setFieldValue
	var targetModel catalogs.Model
	sm.setModelFieldValue(&targetModel, "Name", "New Name")
	if targetModel.Name != "New Name" {
		t.Errorf("Expected name to be set to 'New Name', got %s", targetModel.Name)
	}
}

// TestEdgeCases tests various edge cases
func TestEdgeCases(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy)

	t.Run("duplicate models in same source", func(t *testing.T) {
		sources := map[sources.Type][]catalogs.Model{
			sources.ProviderAPI: {
				createTestModel("model-1", "First", 1000),
				createTestModel("model-1", "Second", 2000), // Duplicate ID
			},
		}

		result, _, err := merger.Models(sources)
		if err != nil {
			t.Fatalf("MergeModels failed: %v", err)
		}

		// Should handle duplicates gracefully (last one wins in current implementation)
		if len(result) != 1 {
			t.Errorf("Expected 1 model after merging duplicates, got %d", len(result))
		}
	})

	t.Run("very long field paths", func(t *testing.T) {
		// Test that deep nesting doesn't cause issues
		sources := map[sources.Type][]catalogs.Model{
			sources.ProviderAPI: {
				{
					ID: "model-1",
					Metadata: &catalogs.ModelMetadata{
						ReleaseDate:     utc.Now(),
						KnowledgeCutoff: func() *utc.Time { t := utc.Now(); return &t }(),
					},
				},
			},
		}

		_, _, err := merger.Models(sources)
		if err != nil {
			t.Fatalf("MergeModels with deep nesting failed: %v", err)
		}
	})
}

// stringPtr is a helper to create a string pointer
func stringPtr(s string) *string {
	return &s
}

// TestComparisonHelpers tests the comparison helper functions
func TestComparisonHelpers(t *testing.T) {
	model1 := createTestModel("test", "Model 1", 1000)
	model2 := createTestModel("test", "Model 1", 1000)
	model3 := createTestModel("test", "Model 2", 2000)

	// Test equality
	if diff := cmp.Diff(model1, model2); diff != "" {
		t.Errorf("Expected models to be equal, but got diff: %s", diff)
	}

	// Test inequality
	if diff := cmp.Diff(model1, model3); diff == "" {
		t.Error("Expected models to be different, but they compared equal")
	}
}
