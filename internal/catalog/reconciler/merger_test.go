package reconciler

import (
	"slices"
	"testing"
	"time"

	"github.com/agentstation/utc"
	"github.com/google/go-cmp/cmp"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

// Test helper functions.
func createTestModel(id, name string, contextWindow int64) *catalogs.Model {
	return &catalogs.Model{
		ID:          id,
		Name:        name,
		Description: "Test model " + name,
		Limits: &catalogs.ModelLimits{
			ContextWindow: contextWindow,
		},
	}
}

func createTestModelWithPricing(id, name string, inputCost, outputCost float64) *catalogs.Model {
	return &catalogs.Model{
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

func createTestProvider(id catalogs.ProviderID, name string) *catalogs.Provider {
	return &catalogs.Provider{
		ID:   id,
		Name: name,
		Models: map[string]*catalogs.Model{
			"model-1": createTestModel("model-1", "Test Model", 1000),
		},
	}
}

func mustSetProviderForReconcilerTest(t *testing.T, cat *catalogs.Builder, provider catalogs.Provider) {
	t.Helper()
	if err := cat.SetProvider(provider); err != nil {
		t.Fatalf("SetProvider(%s) failed: %v", provider.ID, err)
	}
}

func TestMergerConstruction(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)

	merger := newMerger(authorities, strategy, nil)
	if merger == nil {
		t.Fatal("Expected merger to be non-nil")
	}

	// Test with provenance
	tracker := provenance.NewTracker(true)
	mergerWithProv := newMergerWithProvenance(authorities, strategy, tracker, nil)
	if mergerWithProv == nil {
		t.Fatal("Expected merger with provenance to be non-nil")
	}
}

// TestMergeModelsBasic tests basic model merging.
func TestMergeModelsBasic(t *testing.T) {
	tests := []struct {
		name     string
		sources  map[sources.ID][]*catalogs.Model
		expected []*catalogs.Model
	}{
		{
			name:     "empty sources",
			sources:  map[sources.ID][]*catalogs.Model{},
			expected: []*catalogs.Model{},
		},
		{
			name: "single source single model",
			sources: map[sources.ID][]*catalogs.Model{
				sources.ProvidersID: {
					createTestModel("model-1", "API Model", 1000),
				},
			},
			expected: []*catalogs.Model{
				createTestModel("model-1", "API Model", 1000),
			},
		},
		{
			name: "multiple sources same model",
			sources: map[sources.ID][]*catalogs.Model{
				sources.ProvidersID: {
					createTestModel("model-1", "API Model", 1000),
				},
				sources.ModelsDevHTTPID: {
					createTestModelWithPricing("model-1", "ModelsDev Model", 0.5, 1.0),
				},
			},
			expected: []*catalogs.Model{
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
			sources: map[sources.ID][]*catalogs.Model{
				sources.ProvidersID: {
					createTestModel("model-1", "API Model 1", 1000),
					createTestModel("model-2", "API Model 2", 2000),
				},
				sources.ModelsDevHTTPID: {
					createTestModel("model-3", "ModelsDev Model", 3000),
				},
			},
			expected: []*catalogs.Model{
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
			merger := newMerger(authorities, strategy, nil)

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
			resultMap := make(map[string]*catalogs.Model)
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

func TestMergeComplexStructures(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	// Create models with complex structures
	sources := map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:   "model-1",
				Name: "Test Model",
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{"text"},
						Output: []catalogs.ModelModality{"text"},
					},
					Streaming:   true,
					MaxTokens:   true,
					Temperature: true,
				},
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID: "model-1",
				Metadata: &catalogs.ModelMetadata{
					ReleaseDate: utc.New(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
					OpenWeights: true,
				},
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{"image", "pdf"},
						Output: []catalogs.ModelModality{"audio"},
					},
					ToolCalls:         true,
					Tools:             true,
					ToolChoice:        true,
					WebSearch:         true,
					Attachments:       true,
					Reasoning:         true,
					ReasoningEffort:   true,
					StructuredOutputs: true,
				},
				Reasoning: &catalogs.ModelControlLevels{
					Levels: []catalogs.ModelControlLevel{
						catalogs.ModelControlLevelLow,
						catalogs.ModelControlLevelHigh,
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

	if model.Features == nil {
		t.Fatal("Expected features to be non-nil")
	}

	// Preserve provider API features.
	if !model.Features.Streaming {
		t.Error("Expected streaming to be true from Provider API")
	}

	if !model.Features.ToolCalls || !model.Features.WebSearch || !model.Features.Tools ||
		!model.Features.ToolChoice || !model.Features.Attachments || !model.Features.Reasoning ||
		!model.Features.ReasoningEffort || !model.Features.StructuredOutputs {
		t.Fatalf("models.dev did not fill missing provider capabilities: %#v", model.Features)
	}
	if !containsModality(model.Features.Modalities.Input, catalogs.ModelModalityText) ||
		!containsModality(model.Features.Modalities.Input, catalogs.ModelModalityImage) ||
		!containsModality(model.Features.Modalities.Input, catalogs.ModelModalityPDF) {
		t.Errorf("Expected provider and ModelsDev input modalities to be merged, got %#v", model.Features.Modalities.Input)
	}
	if !containsModality(model.Features.Modalities.Output, catalogs.ModelModalityText) ||
		!containsModality(model.Features.Modalities.Output, catalogs.ModelModalityAudio) {
		t.Errorf("Expected provider and ModelsDev output modalities to be merged, got %#v", model.Features.Modalities.Output)
	}
	if model.Reasoning == nil || len(model.Reasoning.Levels) != 2 {
		t.Fatalf("Expected reasoning levels from ModelsDev, got %#v", model.Reasoning)
	}

	// Check metadata from ModelsDev
	if model.Metadata == nil {
		t.Fatal("Expected metadata to be non-nil")
	}

	if !model.Metadata.OpenWeights {
		t.Error("Expected open weights to be true")
	}
}

func TestKnownFalseAndAbsentCapabilityAuthority(t *testing.T) {
	tests := []struct {
		name             string
		providerFeatures *catalogs.ModelFeatures
		wantTools        bool
		wantState        catalogs.ValuePresence
	}{
		{
			name:             "known provider false remains authoritative",
			providerFeatures: explicitlySupported(catalogs.ModelFeatureTools, false),
			wantTools:        false,
			wantState:        catalogs.ValueKnown,
		},
		{
			name:             "missing field in provider capability record permits fallback",
			providerFeatures: &catalogs.ModelFeatures{Streaming: true},
			wantTools:        true,
			wantState:        catalogs.ValueKnown,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorities := authority.New()
			merger := newMerger(authorities, NewAuthorityStrategy(authorities), nil)
			result, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
				sources.ProvidersID: {{ID: "model-1", Name: "Provider Model", Features: test.providerFeatures}},
				sources.ModelsDevHTTPID: {{
					ID: "model-1", Name: "Fallback Model", Features: &catalogs.ModelFeatures{Tools: true},
				}},
			})
			if err != nil {
				t.Fatalf("Models: %v", err)
			}
			if len(result) != 1 || result[0].Features == nil {
				t.Fatalf("features result = %#v", result)
			}
			if got := result[0].Features.Tools; got != test.wantTools {
				t.Fatalf("Tools = %t, want %t", got, test.wantTools)
			}
			if _, state := result[0].Features.Support(catalogs.ModelFeatureTools); state != test.wantState {
				t.Fatalf("Tools state = %v, want %v", state, test.wantState)
			}
		})
	}
}

func explicitlySupported(feature catalogs.ModelFeature, supported bool) *catalogs.ModelFeatures {
	features := &catalogs.ModelFeatures{}
	features.SetSupport(feature, supported)
	return features
}

func TestMergeMetadataPreservesOpenWeightsTrueFromSupplementalSources(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	releaseDate := utc.New(time.Date(2024, 2, 3, 0, 0, 0, 0, time.UTC))
	result, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.LocalCatalogID: {
			{
				ID:   "model-1",
				Name: "Curated Model",
				Metadata: &catalogs.ModelMetadata{
					OpenWeights: true,
				},
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID:   "model-1",
				Name: "models.dev Model",
				Metadata: &catalogs.ModelMetadata{
					ReleaseDate: releaseDate,
					OpenWeights: false,
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

	model := result[0]
	if model.Metadata == nil {
		t.Fatal("Metadata is nil")
	}
	if !model.Metadata.OpenWeights {
		t.Fatalf("local OpenWeights=true was overwritten by models.dev false: %#v", model.Metadata)
	}
	if model.Metadata.ReleaseDate != releaseDate {
		t.Fatalf("ReleaseDate = %v, want %v", model.Metadata.ReleaseDate, releaseDate)
	}
}

func TestMergeModelsCombinesMetadataSubfields(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	releaseDate := utc.Now()
	result, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:   "model-1",
				Name: "Provider Model",
				Metadata: &catalogs.ModelMetadata{
					Tags: []catalogs.ModelTag{catalogs.ModelTagCoding},
					Architecture: &catalogs.ModelArchitecture{
						Tokenizer: catalogs.TokenizerGPT,
					},
				},
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID:   "model-1",
				Name: "ModelsDev Model",
				Metadata: &catalogs.ModelMetadata{
					ReleaseDate: releaseDate,
					Tags:        []catalogs.ModelTag{catalogs.ModelTagChat},
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
	metadata := result[0].Metadata
	if metadata == nil || !metadata.ReleaseDate.Equal(releaseDate) {
		t.Fatalf("models.dev release date was not preserved: %#v", metadata)
	}
	for _, tag := range []catalogs.ModelTag{catalogs.ModelTagChat, catalogs.ModelTagCoding} {
		if !hasModelTag(metadata.Tags, tag) {
			t.Fatalf("tag %q missing from %#v", tag, metadata.Tags)
		}
	}
	if metadata.Architecture == nil || metadata.Architecture.Tokenizer != catalogs.TokenizerGPT {
		t.Fatalf("provider architecture was not filled: %#v", metadata.Architecture)
	}
}

func TestCopyModelPricingDeepCopiesNestedFields(t *testing.T) {
	request := 0.01
	imageInput := 0.02
	audioInput := 0.03
	videoInput := 0.04
	imageGen := 0.05
	audioGen := 0.06
	videoGen := 0.07
	webSearch := 0.08
	functionCall := 0.09
	toolUse := 0.10

	source := &catalogs.ModelPricing{
		Currency: catalogs.ModelPricingCurrencyUSD,
		Tokens: &catalogs.ModelTokenPricing{
			Input:      &catalogs.ModelTokenCost{Per1M: 1},
			Output:     &catalogs.ModelTokenCost{Per1M: 2},
			Reasoning:  &catalogs.ModelTokenCost{Per1M: 3},
			CacheRead:  &catalogs.ModelTokenCost{Per1M: 4},
			CacheWrite: &catalogs.ModelTokenCost{Per1M: 5},
		},
		Operations: &catalogs.ModelOperationPricing{
			Request:      &request,
			ImageInput:   &imageInput,
			AudioInput:   &audioInput,
			VideoInput:   &videoInput,
			ImageGen:     &imageGen,
			AudioGen:     &audioGen,
			VideoGen:     &videoGen,
			WebSearch:    &webSearch,
			FunctionCall: &functionCall,
			ToolUse:      &toolUse,
		},
		Tiers: []catalogs.ModelPricingTier{{
			Name: "context",
			Tokens: &catalogs.ModelTokenPricing{
				Input: &catalogs.ModelTokenCost{Per1M: 8},
			},
			Operations: &catalogs.ModelOperationPricing{
				Request: &request,
			},
		}},
	}

	copied := copyModelPricing(source)
	if copied == nil ||
		copied.Tokens == nil ||
		copied.Tokens.CacheRead == nil ||
		copied.Tokens.CacheWrite == nil ||
		copied.Operations == nil ||
		len(copied.Tiers) != 1 {
		t.Fatalf("pricing was not copied deeply: %#v", copied)
	}
	source.Tokens.Input.Per1M = 99
	source.Operations.Request = nil
	source.Tiers[0].Tokens.Input.Per1M = 99
	if copied.Tokens.Input.Per1M != 1 ||
		copied.Operations.Request == nil ||
		*copied.Operations.Request != request ||
		copied.Tiers[0].Tokens.Input.Per1M != 8 {
		t.Fatalf("copied pricing aliases source: %#v", copied)
	}
}

func TestMergeMetadataCopiesAndFillsNestedFields(t *testing.T) {
	knowledge := utc.Now()
	baseModel := "base-model"
	wantBaseModel := baseModel
	source := &catalogs.ModelMetadata{
		KnowledgeCutoff: &knowledge,
		Tags:            []catalogs.ModelTag{catalogs.ModelTagResearch},
		Architecture: &catalogs.ModelArchitecture{
			ParameterCount: "70B",
			Type:           catalogs.ArchitectureTypeTransformer,
			Tokenizer:      catalogs.TokenizerGPT,
			Quantization:   catalogs.QuantizationFP16,
			Quantized:      true,
			FineTuned:      true,
			BaseModel:      &baseModel,
		},
	}

	copied := mergeSupplementalMetadata(nil, source)
	if copied == nil ||
		copied.KnowledgeCutoff == nil ||
		copied.Architecture == nil ||
		copied.Architecture.BaseModel == nil {
		t.Fatalf("metadata was not copied deeply: %#v", copied)
	}
	source.Tags[0] = catalogs.ModelTagMath
	*source.Architecture.BaseModel = "changed-base"
	if copied.Tags[0] != catalogs.ModelTagResearch || *copied.Architecture.BaseModel != wantBaseModel {
		t.Fatalf("copied metadata aliases source: %#v", copied)
	}

	filled := mergeSupplementalMetadata(&catalogs.ModelMetadata{
		Tags: []catalogs.ModelTag{catalogs.ModelTagChat},
	}, copied)
	if !hasModelTag(filled.Tags, catalogs.ModelTagChat) ||
		!hasModelTag(filled.Tags, catalogs.ModelTagResearch) ||
		filled.Architecture == nil ||
		filled.Architecture.BaseModel == nil ||
		*filled.Architecture.BaseModel != wantBaseModel {
		t.Fatalf("metadata missing fields were not filled: %#v", filled)
	}
}

// TestNilHandling tests handling of nil values.
func TestNilHandling(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	// Test with nil maps in sources
	sources := map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:       "model-1",
				Name:     "Test Model",
				Limits:   nil, // nil limits
				Pricing:  nil, // nil pricing
				Features: nil, // nil features
			},
		},
		sources.ModelsDevHTTPID: {
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

func hasModelTag(tags []catalogs.ModelTag, want catalogs.ModelTag) bool {
	return slices.Contains(tags, want)
}

func TestMergeModelsUsesProviderInputTokenLimit(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	tracker := provenance.NewTracker(true)
	merger := newMergerWithProvenance(authorities, strategy, tracker, nil)

	result, prov, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:   "model-1",
				Name: "Provider Model",
				Limits: &catalogs.ModelLimits{
					ContextWindow: 128000,
					InputTokens:   64000,
					OutputTokens:  4096,
				},
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID:   "model-1",
				Name: "ModelsDev Model",
				Limits: &catalogs.ModelLimits{
					ContextWindow: 400000,
					InputTokens:   272000,
					OutputTokens:  128000,
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
	if result[0].Limits == nil || result[0].Limits.InputTokens != 64000 {
		t.Fatalf("limits = %#v, want provider input token limit", result[0].Limits)
	}
	if _, ok := prov["models.model-1.limits.input_tokens"]; !ok {
		t.Fatalf("missing input token limit provenance in %#v", prov)
	}
}

func TestMergeModelsFillsMissingProviderLimitFromModelsDev(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	result, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:   "model-1",
				Name: "Provider Model",
				Limits: &catalogs.ModelLimits{
					ContextWindow: 128000,
					OutputTokens:  4096,
				},
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID:   "model-1",
				Name: "ModelsDev Model",
				Limits: &catalogs.ModelLimits{
					ContextWindow: 200000,
					InputTokens:   96000,
					OutputTokens:  8192,
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
	got := result[0].Limits
	if got == nil {
		t.Fatal("limits missing")
	}
	if got.ContextWindow != 128000 || got.InputTokens != 96000 || got.OutputTokens != 4096 {
		t.Fatalf("provider limits with models.dev gap fill = %#v", got)
	}
}

func TestMergeModelsPreservesBaselineInputTokensWhenProviderLimitsArePartial(t *testing.T) {
	baseline := catalogs.NewEmpty()
	baselineModel := catalogs.Model{
		ID:   "model-1",
		Name: "Baseline Model",
		Limits: &catalogs.ModelLimits{
			ContextWindow: 128000,
			InputTokens:   96000,
			OutputTokens:  4096,
		},
	}
	if err := baseline.SetProvider(catalogs.Provider{
		ID:   "groq",
		Name: "Groq",
		Models: map[string]*catalogs.Model{
			baselineModel.ID: &baselineModel,
		},
	}); err != nil {
		t.Fatalf("set baseline provider: %v", err)
	}

	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, snapshotForTest(t, baseline))

	result, _, err := merger.ModelsForProvider("groq", map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:   "model-1",
				Name: "Provider Model",
				Limits: &catalogs.ModelLimits{
					ContextWindow: 131072,
					OutputTokens:  8192,
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
	got := result[0].Limits
	if got == nil {
		t.Fatal("limits missing")
	}
	if got.ContextWindow != 131072 || got.OutputTokens != 8192 {
		t.Fatalf("provider limits were not applied: %#v", got)
	}
	if got.InputTokens != baselineModel.Limits.InputTokens {
		t.Fatalf("baseline input token limit was not preserved: %#v", got)
	}
}

func TestMergeModelsUsesProviderScopedBaselineModel(t *testing.T) {
	baseline := catalogs.NewEmpty()
	sharedModelID := "gemini-embedding-001"
	studioModel := catalogs.Model{
		ID:          sharedModelID,
		Name:        "Gemini Embedding",
		Description: "AI Studio baseline description",
	}
	vertexModel := catalogs.Model{
		ID:          sharedModelID,
		Name:        "Gemini Embedding",
		Description: "Vertex baseline description",
	}
	if err := baseline.SetProvider(catalogs.Provider{
		ID:   "google-ai-studio",
		Name: "Google AI Studio",
		Models: map[string]*catalogs.Model{
			sharedModelID: &studioModel,
		},
	}); err != nil {
		t.Fatalf("set studio baseline provider: %v", err)
	}
	if err := baseline.SetProvider(catalogs.Provider{
		ID:   "google-vertex",
		Name: "Google Vertex",
		Models: map[string]*catalogs.Model{
			sharedModelID: &vertexModel,
		},
	}); err != nil {
		t.Fatalf("set vertex baseline provider: %v", err)
	}

	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, snapshotForTest(t, baseline))
	sourceModels := map[sources.ID][]*catalogs.Model{
		sources.ModelsDevHTTPID: {
			{
				ID:   sharedModelID,
				Name: "Gemini Embedding from models.dev",
			},
		},
	}

	studioResult, _, err := merger.ModelsForProvider("google-ai-studio", sourceModels)
	if err != nil {
		t.Fatalf("merge studio model: %v", err)
	}
	vertexResult, _, err := merger.ModelsForProvider("google-vertex", sourceModels)
	if err != nil {
		t.Fatalf("merge vertex model: %v", err)
	}
	if len(studioResult) != 1 || len(vertexResult) != 1 {
		t.Fatalf("result lengths = %d/%d, want 1/1", len(studioResult), len(vertexResult))
	}
	if studioResult[0].Description != studioModel.Description {
		t.Fatalf("studio baseline description = %q, want %q", studioResult[0].Description, studioModel.Description)
	}
	if vertexResult[0].Description != vertexModel.Description {
		t.Fatalf("vertex baseline description = %q, want %q", vertexResult[0].Description, vertexModel.Description)
	}
}

func TestMergeModelsPreservesSourceTimestampsForNewModel(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)
	createdAt := utc.New(time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC))
	updatedAt := utc.New(time.Date(2025, 6, 20, 0, 0, 0, 0, time.UTC))

	result, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:        "model-1",
				Name:      "Provider Model",
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID:        "model-1",
				Name:      "ModelsDev Model",
				UpdatedAt: updatedAt,
			},
		},
	})
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("models = %d, want 1", len(result))
	}
	if !result[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("CreatedAt = %v, want %v", result[0].CreatedAt, createdAt)
	}
	if !result[0].UpdatedAt.Equal(updatedAt) {
		t.Fatalf("UpdatedAt = %v, want %v", result[0].UpdatedAt, updatedAt)
	}
}

func TestMergeModelsOrdersMixedSourceTimestamps(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)
	firstKnown := utc.New(time.Date(2025, 4, 29, 0, 0, 0, 0, time.UTC))
	firstObserved := utc.New(time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC))

	result, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:        "model-1",
				Name:      "Provider Model",
				CreatedAt: firstObserved,
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID:        "model-1",
				Name:      "ModelsDev Model",
				UpdatedAt: firstKnown,
			},
		},
	})
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("models = %d, want 1", len(result))
	}
	if !result[0].CreatedAt.Equal(firstKnown) {
		t.Fatalf("CreatedAt = %v, want earliest %v", result[0].CreatedAt, firstKnown)
	}
	if !result[0].UpdatedAt.Equal(firstObserved) {
		t.Fatalf("UpdatedAt = %v, want latest %v", result[0].UpdatedAt, firstObserved)
	}
}

func TestMergeModelsDoesNotBumpUpdatedAtForExtensionDynamicTypeOnlyChanges(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	createdAt := utc.New(time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC))
	updatedAt := utc.New(time.Date(2025, 6, 20, 0, 0, 0, 0, time.UTC))
	baseline := catalogs.NewEmpty()
	baselineModel := catalogs.Model{
		ID:        "model-1",
		Name:      "Provider Model",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Extensions: catalogs.SourceExtensions{
			"models.dev": {Fields: map[string]any{
				"values": []any{"xhigh"},
				"limit": map[string]any{
					"min": int64(1),
				},
			}},
		},
	}
	if err := baseline.SetProvider(catalogs.Provider{
		ID: "provider",
		Models: map[string]*catalogs.Model{
			baselineModel.ID: &baselineModel,
		},
	}); err != nil {
		t.Fatalf("SetProvider baseline: %v", err)
	}

	merger := newMerger(authorities, strategy, snapshotForTest(t, baseline))
	result, _, err := merger.ModelsForProvider("provider", map[sources.ID][]*catalogs.Model{
		sources.ModelsDevHTTPID: {
			{
				ID:        baselineModel.ID,
				Name:      baselineModel.Name,
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
				Extensions: catalogs.SourceExtensions{
					"models.dev": {Fields: map[string]any{
						"values": []string{"xhigh"},
						"limit": map[string]int{
							"min": 1,
						},
					}},
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
	if !result[0].UpdatedAt.Equal(updatedAt) {
		t.Fatalf("UpdatedAt = %v, want unchanged %v", result[0].UpdatedAt, updatedAt)
	}
}

func TestMergeModelsUsesModelsDevStatus(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	result, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:     "model-1",
				Name:   "Provider Model",
				Status: catalogs.ModelStatusActive,
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID:     "model-1",
				Name:   "ModelsDev Model",
				Status: catalogs.ModelStatusDeprecated,
			},
		},
	})
	if err != nil {
		t.Fatalf("MergeModels failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("models = %d, want 1", len(result))
	}
	if result[0].Status != catalogs.ModelStatusDeprecated {
		t.Fatalf("status = %q, want %q", result[0].Status, catalogs.ModelStatusDeprecated)
	}
}

func TestMergeModelsCombinesLineageSubfields(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	tracker := provenance.NewTracker(true)
	merger := newMergerWithProvenance(authorities, strategy, tracker, nil)
	root := "provider-root"
	parent := "provider-parent"

	result, prov, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:   "model-1",
				Name: "Provider Model",
				Lineage: &catalogs.ModelLineage{
					Root:   &root,
					Parent: &parent,
				},
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID:   "model-1",
				Name: "ModelsDev Model",
				Lineage: &catalogs.ModelLineage{
					Family: "model-family",
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
	if result[0].Lineage == nil ||
		result[0].Lineage.Family != "model-family" ||
		result[0].Lineage.Root == nil ||
		*result[0].Lineage.Root != root ||
		result[0].Lineage.Parent == nil ||
		*result[0].Lineage.Parent != parent {
		t.Fatalf("lineage = %#v", result[0].Lineage)
	}
	for _, key := range []string{
		"models.model-1.lineage.family",
		"models.model-1.lineage.root",
		"models.model-1.lineage.parent",
	} {
		if _, ok := prov[key]; !ok {
			t.Fatalf("missing lineage provenance key %q in %#v", key, prov)
		}
	}
}

func TestMergeModelsUsesProviderModesWithUpstreamGapFilling(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	tracker := provenance.NewTracker(true)
	merger := newMergerWithProvenance(authorities, strategy, tracker, nil)

	result, prov, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ProvidersID: {
			{
				ID:   "model-1",
				Name: "Provider Model",
				Modes: map[string]catalogs.ModelMode{
					"fast": {
						Provider: &catalogs.ModelProviderMode{
							Body: map[string]any{"service_tier": "standard"},
						},
					},
				},
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID:   "model-1",
				Name: "ModelsDev Model",
				Modes: map[string]catalogs.ModelMode{
					"fast": {
						Pricing: &catalogs.ModelPricing{
							Currency: catalogs.ModelPricingCurrencyUSD,
							Tokens: &catalogs.ModelTokenPricing{
								Input: &catalogs.ModelTokenCost{Per1M: 3.50},
							},
						},
						Provider: &catalogs.ModelProviderMode{
							Headers: map[string]string{"anthropic-beta": "fast-mode"},
							Body:    map[string]any{"service_tier": "priority"},
						},
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
	fastMode, ok := result[0].Modes["fast"]
	if !ok {
		t.Fatalf("fast mode missing from %#v", result[0].Modes)
	}
	if fastMode.Pricing == nil ||
		fastMode.Pricing.Tokens == nil ||
		fastMode.Pricing.Tokens.Input == nil ||
		fastMode.Pricing.Tokens.Input.Per1M != 3.50 ||
		fastMode.Provider == nil ||
		fastMode.Provider.Headers["anthropic-beta"] != "fast-mode" ||
		fastMode.Provider.Body["service_tier"] != "standard" {
		t.Fatalf("fast mode = %#v, want provider mode with models.dev pricing/header gaps filled", fastMode)
	}
	if _, ok := prov["models.model-1.modes"]; !ok {
		t.Fatalf("missing modes provenance in %#v", prov)
	}
}

func TestMergeModelsCombinesSourceExtensions(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	result, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.LocalCatalogID: {
			{
				ID:   "model-1",
				Name: "Local Model",
				Extensions: catalogs.SourceExtensions{
					"models.dev": {Fields: map[string]any{"provider": "local-override"}},
				},
			},
		},
		sources.ModelsDevHTTPID: {
			{
				ID:   "model-1",
				Name: "ModelsDev Model",
				Extensions: catalogs.SourceExtensions{
					"models.dev": {Fields: map[string]any{
						"provider":    "models-dev",
						"interleaved": true,
					}},
				},
			},
		},
		sources.ProvidersID: {
			{
				ID:   "model-1",
				Name: "Provider Model",
				Extensions: catalogs.SourceExtensions{
					"groq": {Fields: map[string]any{"hugging_face_id": "meta/model"}},
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
	if result[0].Extensions["models.dev"].Fields["provider"] != "local-override" {
		t.Fatalf("local extension field was overwritten: %#v", result[0].Extensions)
	}
	if result[0].Extensions["models.dev"].Fields["interleaved"] != true {
		t.Fatalf("models.dev extension field missing: %#v", result[0].Extensions)
	}
	if result[0].Extensions["groq"].Fields["hugging_face_id"] != "meta/model" {
		t.Fatalf("provider extension field missing: %#v", result[0].Extensions)
	}
}

func TestMergeModelsUpdatesBaselineSourceExtensionsFromFreshSource(t *testing.T) {
	baseline := catalogs.NewEmpty()
	baselineModel := catalogs.Model{
		ID:   "model-1",
		Name: "Baseline Model",
		Extensions: catalogs.SourceExtensions{
			"models.dev": {Fields: map[string]any{
				"provider":       "chat",
				"persisted_only": "keep",
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

	result, _, err := merger.Models(map[sources.ID][]*catalogs.Model{
		sources.ModelsDevHTTPID: {
			{
				ID:   "model-1",
				Name: "ModelsDev Model",
				Extensions: catalogs.SourceExtensions{
					"models.dev": {Fields: map[string]any{
						"provider": "responses",
					}},
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
	fields := result[0].Extensions["models.dev"].Fields
	if fields["provider"] != "responses" {
		t.Fatalf("fresh source extension did not update baseline value: %#v", fields)
	}
	if fields["persisted_only"] != "keep" {
		t.Fatalf("baseline-only extension field was not preserved: %#v", fields)
	}
}

// TestEmptySources tests merging with empty source maps.
func TestEmptySources(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	// Test with completely empty sources
	result, prov, err := merger.Models(map[sources.ID][]*catalogs.Model{})
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

// TestConcurrentMerging tests thread safety of merger.
func TestConcurrentMerging(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	// Run multiple merges concurrently
	done := make(chan bool, 10)
	for i := range 10 {
		go func(id int) {
			sources := map[sources.ID][]*catalogs.Model{
				sources.ProvidersID: {
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
	for range 10 {
		<-done
	}
}

func containsModality(modalities []catalogs.ModelModality, want catalogs.ModelModality) bool {
	return slices.Contains(modalities, want)
}

// BenchmarkMergeModels benchmarks model merging performance.
func BenchmarkMergeModels(b *testing.B) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	// Create test data
	testSources := map[sources.ID][]*catalogs.Model{
		sources.ProvidersID:     make([]*catalogs.Model, 100),
		sources.ModelsDevHTTPID: make([]*catalogs.Model, 100),
	}

	for i := range 100 {
		id := string(rune('a'+i%26)) + string(rune('0'+i/26))
		testSources[sources.ProvidersID][i] = createTestModel(id, "API Model", int64(i*1000))
		testSources[sources.ModelsDevHTTPID][i] = createTestModelWithPricing(id, "ModelsDev Model", float64(i)*0.1, float64(i)*0.2)
	}

	b.ResetTimer()
	for b.Loop() {
		_, _, err := merger.Models(testSources)
		if err != nil {
			b.Fatalf("MergeModels failed: %v", err)
		}
	}
}

func snapshotForTest(t *testing.T, reader catalogs.Reader) *catalogs.Catalog {
	t.Helper()
	snapshot, err := catalogs.NewObservationCatalog(reader)
	if err != nil {
		t.Fatalf("NewObservationCatalog: %v", err)
	}
	return snapshot
}

// TestMergeFieldReflection tests the reflection-based field merging.
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
	name := sm.modelFieldValue(&model, "Name")
	if name != "Test Model" {
		t.Errorf("Expected name 'Test Model', got %v", name)
	}

	// Test nested field access - this will not work with current implementation
	// as it expects capitalized field names in the path
	contextWindow := sm.modelFieldValue(&model, "Limits")
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

// TestEdgeCases tests various edge cases.
func TestEdgeCases(t *testing.T) {
	authorities := authority.New()
	strategy := NewAuthorityStrategy(authorities)
	merger := newMerger(authorities, strategy, nil)

	t.Run("duplicate models in same source", func(t *testing.T) {
		sources := map[sources.ID][]*catalogs.Model{
			sources.ProvidersID: {
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
		// Test that deep nesting does not cause issues
		sources := map[sources.ID][]*catalogs.Model{
			sources.ProvidersID: {
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

// stringPtr is a helper to create a string pointer.
func stringPtr(s string) *string {
	return &s
}

// TestComparisonHelpers tests the comparison helper functions.
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
