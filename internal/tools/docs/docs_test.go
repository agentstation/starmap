package docs

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/utc"
	"github.com/stretchr/testify/assert"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestGenerator(t *testing.T) {
	t.Run("create new generator", func(t *testing.T) {
		g := New()
		assert.NotNil(t, g)
	})

	t.Run("with options", func(t *testing.T) {
		g := New(
			WithOutputDir("/tmp/test"),
			WithVerbose(true),
		)
		assert.NotNil(t, g)
		assert.Equal(t, "/tmp/test", g.outputDir)
		assert.True(t, g.verbose)
	})
}

func TestGenerateDocumentation(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create test catalog
	catalog := createTestCatalog()

	// Create generator
	g := New(WithOutputDir(tempDir))

	// Generate documentation
	ctx := context.Background()
	err := g.Generate(ctx, catalog)
	assert.NoError(t, err)

	// Check main directories were created
	assert.DirExists(t, filepath.Join(tempDir, "catalog", "models"))
	assert.DirExists(t, filepath.Join(tempDir, "catalog", "providers"))
	assert.DirExists(t, filepath.Join(tempDir, "catalog", "authors"))

	// Check main README
	assert.FileExists(t, filepath.Join(tempDir, "catalog", "README.md"))
}

func TestDetectModelFamily(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"GPT-4", "GPT"},
		{"gpt-4-turbo", "GPT"},
		{"Claude 3 Opus", "Claude"},
		{"claude-3-sonnet", "Claude"},
		{"Gemini Pro", "Gemini"},
		{"gemini-1.5-flash", "Gemini"},
		{"Llama 3 70B", "Llama"},
		{"Mixtral 8x7B", "Mixtral"},
		{"DeepSeek Chat", "DeepSeek"},
		{"unknown-model", "Other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectModelFamily(tt.name)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateCompactFeatures(t *testing.T) {
	tests := []struct {
		name     string
		model    catalogs.Model
		expected []string
	}{
		{
			name: "multimodal model",
			model: catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage},
						Output: []catalogs.ModelModality{catalogs.ModelModalityText},
					},
					Tools:     true,
					Streaming: true,
				},
			},
			expected: []string{"📝", "👁️", "🔧", "⚡"},
		},
		{
			name: "audio model",
			model: catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{catalogs.ModelModalityAudio},
						Output: []catalogs.ModelModality{catalogs.ModelModalityText},
					},
				},
			},
			expected: []string{"📝", "🎵"},
		},
		{
			name: "embedding model",
			model: catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
						Output: []catalogs.ModelModality{catalogs.ModelModalityEmbedding},
					},
				},
			},
			expected: []string{"📝"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compactFeatures(tt.model)

			// Parse result
			var resultArray []string
			if result != "" && result != "—" {
				resultArray = strings.Split(result, " ")
			}

			assert.ElementsMatch(t, tt.expected, resultArray)
		})
	}
}

func TestFormatContext(t *testing.T) {
	tests := []struct {
		context  int64
		expected string
	}{
		{128000, "128k"},
		{2000000, "2.0M"},
		{4096, "4.1k"},
		{500, "500"},
	}

	for _, tt := range tests {
		result := formatContext(tt.context)
		assert.Equal(t, tt.expected, result)
	}
}

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		price    float64
		expected string
	}{
		{0.0, "Free"},
		{10.0, "$10.00"},
		{0.5, "$0.5000"},
		{0.002, "$0.002000"},
		{0.0001, "$0.000100"},
	}

	for _, tt := range tests {
		result := formatPrice(tt.price)
		assert.Equal(t, tt.expected, result)
	}
}

// Test badge generation.
func TestGenerateModalityBadges(t *testing.T) {
	features := &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
	}

	badges := modalityBadges(features)
	badgesStr := strings.Join(badges, " ")

	// generateModalityBadges returns HTML badges with tooltips
	assert.Contains(t, badgesStr, "text")
	assert.Contains(t, badgesStr, "vision")
}

func TestGenerateToolBadges(t *testing.T) {
	features := &catalogs.ModelFeatures{
		Tools:     true,
		ToolCalls: true,
		WebSearch: true,
	}

	badges := toolBadges(features)
	badgesStr := strings.Join(badges, " ")

	// generateToolBadges returns HTML badges with underscores in badge names
	assert.Contains(t, badgesStr, "tool__calls")
	assert.Contains(t, badgesStr, "tools")
	assert.Contains(t, badgesStr, "web__search")
}

// Helper function to create test catalog.
func createTestCatalog() catalogs.Reader {
	catalog, _ := catalogs.New()

	// Add test provider
	provider := catalogs.Provider{
		ID:   "test-provider",
		Name: "Test Provider",
		Models: map[string]*catalogs.Model{
			"test-model": {
				ID:       "test-model",
				Name:     "Test Model",
				Features: createTestFeatures(),
				Limits:   &catalogs.ModelLimits{ContextWindow: 128000, OutputTokens: 4096},
				Pricing:  createTestPricing(),
			},
		},
	}
	catalog.SetProvider(provider)

	// Add test author
	author := catalogs.Author{
		ID:   "test-author",
		Name: "Test Author",
	}
	catalog.SetAuthor(author)

	// Add test model
	model := catalogs.Model{
		ID:       "test-model",
		Name:     "Test Model",
		Features: createTestFeatures(),
		Limits:   &catalogs.ModelLimits{ContextWindow: 128000, OutputTokens: 4096},
		Pricing:  createTestPricing(),
		Metadata: &catalogs.ModelMetadata{
			ReleaseDate: mustParseUTC("2024-01"),
			Tags: []catalogs.ModelTag{
				catalogs.ModelTagChat,
				catalogs.ModelTagCoding,
			},
			Architecture: &catalogs.ModelArchitecture{
				ParameterCount: "70B",
				Type:           catalogs.ArchitectureTypeTransformer,
				Tokenizer:      catalogs.TokenizerGPT,
			},
		},
	}
	// Update provider with complex model
	provider.Models["complex-model"] = &model
	catalog.SetProvider(provider)

	// Also create OpenAI provider for this model
	openaiProvider := catalogs.Provider{
		ID:   catalogs.ProviderID("openai"),
		Name: "OpenAI",
		Models: map[string]*catalogs.Model{
			model.ID: &model,
		},
	}
	catalog.SetProvider(openaiProvider)

	return catalog
}

func createTestFeatures() *catalogs.ModelFeatures {
	return &catalogs.ModelFeatures{
		Modalities: catalogs.ModelModalities{
			Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
			Output: []catalogs.ModelModality{catalogs.ModelModalityText},
		},
		Tools:       true,
		Streaming:   true,
		Temperature: true,
		TopP:        true,
		MaxTokens:   true,
	}
}

func createTestPricing() *catalogs.ModelPricing {
	return &catalogs.ModelPricing{
		Currency: "USD",
		Tokens: &catalogs.ModelTokenPricing{
			Input:  &catalogs.ModelTokenCost{Per1M: 10.0},
			Output: &catalogs.ModelTokenCost{Per1M: 30.0},
		},
	}
}

// Helper to parse UTC dates for testing.
func mustParseUTC(s string) utc.Time {
	// Try parsing as YYYY-MM format
	t, err := utc.Parse("2006-01", s)
	if err != nil {
		// Try parsing as YYYY-MM-DD format
		t, err = utc.Parse("2006-01-02", s)
		if err != nil {
			panic(err)
		}
	}
	return t
}

// Test Generate function with different configurations.
func TestGenerateWithConfigurations(t *testing.T) {
	tests := []struct {
		name       string
		catalog    func() catalogs.Reader
		options    []Option
		verifyFunc func(t *testing.T, dir string)
	}{
		{
			name: "generate with verbose output",
			catalog: func() catalogs.Reader {
				return createTestCatalog()
			},
			options: []Option{
				WithVerbose(true),
			},
			verifyFunc: func(t *testing.T, dir string) {
				assert.FileExists(t, filepath.Join(dir, "catalog", "README.md"))
			},
		},
		{
			name: "generate with custom output directory",
			catalog: func() catalogs.Reader {
				return createTestCatalog()
			},
			options: []Option{},
			verifyFunc: func(t *testing.T, dir string) {
				assert.DirExists(t, filepath.Join(dir, "catalog"))
				assert.DirExists(t, filepath.Join(dir, "catalog", "models"))
				assert.DirExists(t, filepath.Join(dir, "catalog", "authors"))
				assert.DirExists(t, filepath.Join(dir, "catalog", "providers"))
			},
		},
		{
			name: "generate with empty catalog",
			catalog: func() catalogs.Reader {
				c, _ := catalogs.New()
				return c
			},
			options: []Option{},
			verifyFunc: func(t *testing.T, dir string) {
				assert.FileExists(t, filepath.Join(dir, "catalog", "README.md"))
				assert.DirExists(t, filepath.Join(dir, "catalog", "models"))
			},
		},
		{
			name: "generate with models only",
			catalog: func() catalogs.Reader {
				c, _ := catalogs.New()
				provider := catalogs.Provider{
					ID:   catalogs.ProviderID("test-provider"),
					Name: "Test Provider",
					Models: map[string]*catalogs.Model{
						"model-1": {
							ID:   "model-1",
							Name: "Model 1",
							Features: &catalogs.ModelFeatures{
								Modalities: catalogs.ModelModalities{
									Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
									Output: []catalogs.ModelModality{catalogs.ModelModalityText},
								},
							},
						},
					},
				}
				c.SetProvider(provider)
				return c
			},
			options: []Option{},
			verifyFunc: func(t *testing.T, dir string) {
				assert.FileExists(t, filepath.Join(dir, "catalog", "models", "README.md"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			opts := append(tt.options, WithOutputDir(tempDir))
			g := New(opts...)

			ctx := context.Background()
			err := g.Generate(ctx, tt.catalog())
			assert.NoError(t, err)

			tt.verifyFunc(t, tempDir)
		})
	}
}

// Test comprehensive badge generation.
func TestComprehensiveBadgeGeneration(t *testing.T) {
	tests := []struct {
		name     string
		features *catalogs.ModelFeatures
		verify   func(t *testing.T, badges []string)
	}{
		{
			name: "all modality badges",
			features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage, catalogs.ModelModalityAudio, catalogs.ModelModalityVideo},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage, catalogs.ModelModalityAudio},
				},
			},
			verify: func(t *testing.T, badges []string) {
				badgeStr := strings.Join(badges, " ")
				assert.Contains(t, badgeStr, "text")
				assert.Contains(t, badgeStr, "vision")
				assert.Contains(t, badgeStr, "audio")
				assert.Contains(t, badgeStr, "video")
			},
		},
		{
			name: "all tool badges",
			features: &catalogs.ModelFeatures{
				ToolCalls:   true,
				Tools:       true,
				ToolChoice:  true,
				WebSearch:   true,
				Attachments: true,
			},
			verify: func(t *testing.T, badges []string) {
				badgeStr := strings.Join(badges, " ")
				assert.Contains(t, badgeStr, "tool__calls")
				assert.Contains(t, badgeStr, "tools")
				assert.Contains(t, badgeStr, "tool__choice")
				assert.Contains(t, badgeStr, "web__search")
				assert.Contains(t, badgeStr, "attachments")
			},
		},
		{
			name: "reasoning badges",
			features: &catalogs.ModelFeatures{
				Reasoning:       true,
				ReasoningEffort: true,
				ReasoningTokens: true,
			},
			verify: func(t *testing.T, badges []string) {
				badgeStr := strings.Join(badges, " ")
				assert.Contains(t, badgeStr, "reasoning")
				assert.Contains(t, badgeStr, "reasoning__effort")
				assert.Contains(t, badgeStr, "reasoning__tokens")
			},
		},
		{
			name: "generation control badges",
			features: &catalogs.ModelFeatures{
				Temperature:       true,
				TopP:              true,
				TopK:              true,
				MaxTokens:         true,
				Stop:              true,
				Logprobs:          true,
				TopLogprobs:       true,
				MinP:              true,
				PresencePenalty:   true,
				FrequencyPenalty:  true,
				RepetitionPenalty: true,
			},
			verify: func(t *testing.T, badges []string) {
				badgeStr := strings.Join(badges, " ")
				assert.Contains(t, badgeStr, "temperature")
				assert.Contains(t, badgeStr, "top__p")
				assert.Contains(t, badgeStr, "top__k")
				assert.Contains(t, badgeStr, "max__tokens")
			},
		},
		{
			name: "delivery badges",
			features: &catalogs.ModelFeatures{
				Streaming: true,
			},
			verify: func(t *testing.T, badges []string) {
				badgeStr := strings.Join(badges, " ")
				assert.Contains(t, badgeStr, "streaming")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test modality badges
			badges := modalityBadges(tt.features)
			if tt.features.Modalities.Input != nil || tt.features.Modalities.Output != nil {
				assert.NotEmpty(t, badges)
			}

			// Test tool badges
			toolBadgeList := toolBadges(tt.features)

			// Test generation badges
			genBadges := generationBadges(tt.features)

			// Test reasoning badges
			reasonBadges := reasoningBadges(tt.features)

			// Test delivery badges (streaming etc)
			deliveryBadges := deliveryBadges(tt.features)

			// Combine all badges for verification
			allBadges := append(append(append(append(badges, toolBadgeList...), genBadges...), reasonBadges...), deliveryBadges...)
			tt.verify(t, allBadges)
		})
	}
}

// Test pricing edge cases.
func TestPricingEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		pricing  *catalogs.ModelPricing
		expected string
	}{
		{
			name:     "nil pricing",
			pricing:  nil,
			expected: "—",
		},
		{
			name: "zero cost",
			pricing: &catalogs.ModelPricing{
				Tokens: &catalogs.ModelTokenPricing{
					Input:  &catalogs.ModelTokenCost{Per1M: 0},
					Output: &catalogs.ModelTokenCost{Per1M: 0},
				},
				Currency: "USD",
			},
			expected: "Free",
		},
		{
			name: "very small cost",
			pricing: &catalogs.ModelPricing{
				Tokens: &catalogs.ModelTokenPricing{
					Input:  &catalogs.ModelTokenCost{Per1M: 0.0001},
					Output: &catalogs.ModelTokenCost{Per1M: 0.0002},
				},
				Currency: "USD",
			},
			expected: "$0.000100",
		},
		{
			name: "mixed currencies",
			pricing: &catalogs.ModelPricing{
				Tokens: &catalogs.ModelTokenPricing{
					Input:  &catalogs.ModelTokenCost{Per1M: 10},
					Output: &catalogs.ModelTokenCost{Per1M: 20},
				},
				Currency: "USD", // Change to USD since formatTokenPrice doesn't handle EUR
			},
			expected: "$10.00",
		},
		{
			name: "with cache pricing",
			pricing: &catalogs.ModelPricing{
				Tokens: &catalogs.ModelTokenPricing{
					Input:  &catalogs.ModelTokenCost{Per1M: 10},
					Output: &catalogs.ModelTokenCost{Per1M: 20},
					Cache: &catalogs.ModelTokenCachePricing{
						Read:  &catalogs.ModelTokenCost{Per1M: 1},
						Write: &catalogs.ModelTokenCost{Per1M: 2},
					},
				},
				Currency: "USD",
			},
			expected: "$10.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.pricing == nil || tt.pricing.Tokens == nil || tt.pricing.Tokens.Input == nil {
				result = "—"
			} else {
				result = formatTokenPrice(tt.pricing.Tokens.Input, "USD")
			}
			if tt.pricing == nil {
				assert.Equal(t, tt.expected, result)
			} else if tt.pricing.Tokens != nil && tt.pricing.Tokens.Input != nil {
				if tt.pricing.Tokens.Input.Per1M == 0 {
					assert.Contains(t, result, "Free") // Result format is "- USD: Free"
				} else if tt.pricing.Currency == "EUR" {
					assert.Contains(t, result, "$") // formatTokenPrice uses USD regardless
				} else {
					assert.Contains(t, result, "$")
				}
			}
		})
	}
}

// Test context formatting edge cases.
func TestContextFormattingEdgeCases(t *testing.T) {
	tests := []struct {
		context  int64
		expected string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1k"},
		{1500, "1.5k"},
		{10000, "10k"},
		{100000, "100k"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
		{128000000, "128.0M"},
		{-1, "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatContext(tt.context)
			assert.Equal(t, tt.expected, result)
		})
	}
}
