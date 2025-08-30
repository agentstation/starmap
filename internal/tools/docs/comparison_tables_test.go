package docs

import (
	"os"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteModelsOverviewTable(t *testing.T) {
	tests := []struct {
		name      string
		models    []*catalogs.Model
		providers []*catalogs.Provider
		expected  []string
	}{
		{
			name: "basic models overview",
			models: []*catalogs.Model{
				{
					ID:   "model1",
					Name: "Model One",
					Limits: &catalogs.ModelLimits{
						ContextWindow: 128000,
						OutputTokens:  4096,
					},
					Pricing: &catalogs.ModelPricing{
						Tokens: &catalogs.TokenPricing{
							Input:  &catalogs.TokenCost{Per1M: 10.0},
							Output: &catalogs.TokenCost{Per1M: 30.0},
						},
					},
					Features: &catalogs.ModelFeatures{
						Modalities: catalogs.ModelModalities{
							Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage},
							Output: []catalogs.ModelModality{catalogs.ModelModalityText},
						},
						Tools:     true,
						Streaming: true,
					},
				},
			},
			providers: []*catalogs.Provider{
				{
					ID:   "provider1",
					Name: "Provider One",
					Models: map[string]catalogs.Model{
						"model1": {ID: "model1"},
					},
				},
			},
			expected: []string{
				"| Model | Provider | Context | Input | Output | Features |",
				"| **Model One** |",
				"128k",
				"$10.00",
				"$30.00",
				"👁️", // Vision icon
				"🔧", // Tools icon
				"⚡", // Streaming icon
			},
		},
		{
			name: "multiple models sorted",
			models: []*catalogs.Model{
				{ID: "zebra", Name: "Zebra Model"},
				{ID: "alpha", Name: "Alpha Model"},
				{ID: "beta", Name: "Beta Model"},
			},
			providers: []*catalogs.Provider{},
			expected: []string{
				"| **Alpha Model** |",
				"| **Beta Model** |",
				"| **Zebra Model** |",
			},
		},
		{
			name: "more than 20 models",
			models: func() []*catalogs.Model {
				models := make([]*catalogs.Model, 25)
				for i := 0; i < 25; i++ {
					models[i] = &catalogs.Model{
						ID:   strings.Repeat("m", i+1),
						Name: strings.Repeat("M", i+1),
					}
				}
				return models
			}(),
			providers: []*catalogs.Provider{},
			expected: []string{
				"...and 5 more",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test_models_overview_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			writeModelsOverviewTable(tmpFile, tt.models, tt.providers)

			tmpFile.Seek(0, 0)
			content, err := os.ReadFile(tmpFile.Name())
			require.NoError(t, err)
			contentStr := string(content)

			for _, expected := range tt.expected {
				assert.Contains(t, contentStr, expected)
			}
		})
	}
}

func TestWriteProviderComparisonTable(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name      string
		providers []*catalogs.Provider
		expected  []string
	}{
		{
			name: "provider with free tier",
			providers: []*catalogs.Provider{
				{
					ID:   "provider1",
					Name: "Provider One",
					Models: map[string]catalogs.Model{
						"free-model": {
							ID: "free-model",
							Pricing: &catalogs.ModelPricing{
								Tokens: &catalogs.TokenPricing{
									Input:  &catalogs.TokenCost{Per1M: 0.0},
									Output: &catalogs.TokenCost{Per1M: 0.0},
								},
							},
						},
					},
					StatusPageURL: strPtr("https://status.provider.com"),
				},
			},
			expected: []string{
				"| Provider | Models | Free Tier | API Key Required | Status Page |",
				"| **Provider One** | 1 | ✅ |",
				"[Status](https://status.provider.com)",
			},
		},
		{
			name: "provider without free tier",
			providers: []*catalogs.Provider{
				{
					ID:   "provider2",
					Name: "Provider Two",
					Models: map[string]catalogs.Model{
						"paid-model": {
							ID: "paid-model",
							Pricing: &catalogs.ModelPricing{
								Tokens: &catalogs.TokenPricing{
									Input:  &catalogs.TokenCost{Per1M: 10.0},
									Output: &catalogs.TokenCost{Per1M: 30.0},
								},
							},
						},
					},
					APIKey: &catalogs.ProviderAPIKey{Name: "API_KEY"},
				},
			},
			expected: []string{
				"| **Provider Two** | 1 | ❌ | ✅ |",
			},
		},
		{
			name: "provider with env vars",
			providers: []*catalogs.Provider{
				{
					ID:   "provider3",
					Name: "Provider Three",
					EnvVars: []catalogs.ProviderEnvVar{
						{Name: "PROJECT_ID"},
						{Name: "LOCATION"},
					},
				},
			},
			expected: []string{
				"| **Provider Three** | 0 | ❌ | ✅ |",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test_provider_comp_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			writeProviderComparisonTable(tmpFile, tt.providers)

			tmpFile.Seek(0, 0)
			content, err := os.ReadFile(tmpFile.Name())
			require.NoError(t, err)
			contentStr := string(content)

			for _, expected := range tt.expected {
				assert.Contains(t, contentStr, expected)
			}
		})
	}
}

func TestWritePricingComparisonTable(t *testing.T) {
	tests := []struct {
		name     string
		models   []*catalogs.Model
		expected []string
	}{
		{
			name: "sorted by price",
			models: []*catalogs.Model{
				{
					Name: "Expensive Model",
					Pricing: &catalogs.ModelPricing{
						Tokens: &catalogs.TokenPricing{
							Input:  &catalogs.TokenCost{Per1M: 100.0},
							Output: &catalogs.TokenCost{Per1M: 300.0},
						},
					},
				},
				{
					Name: "Cheap Model",
					Pricing: &catalogs.ModelPricing{
						Tokens: &catalogs.TokenPricing{
							Input:  &catalogs.TokenCost{Per1M: 1.0},
							Output: &catalogs.TokenCost{Per1M: 3.0},
						},
					},
				},
				{
					Name: "Medium Model",
					Pricing: &catalogs.ModelPricing{
						Tokens: &catalogs.TokenPricing{
							Input:  &catalogs.TokenCost{Per1M: 10.0},
							Output: &catalogs.TokenCost{Per1M: 30.0},
						},
					},
				},
			},
			expected: []string{
				"| Model | Input (per 1M) | Output (per 1M) | Cache Read | Cache Write |",
				// Should be sorted by input price: Cheap, Medium, Expensive
				"| **Cheap Model** | $1.00",
				"| **Medium Model** | $10.00",
				"| **Expensive Model** | $100.00",
			},
		},
		{
			name: "with cache pricing",
			models: []*catalogs.Model{
				{
					Name: "Cache Model",
					Pricing: &catalogs.ModelPricing{
						Tokens: &catalogs.TokenPricing{
							Input:      &catalogs.TokenCost{Per1M: 10.0},
							Output:     &catalogs.TokenCost{Per1M: 30.0},
							CacheRead:  &catalogs.TokenCost{Per1M: 1.0},
							CacheWrite: &catalogs.TokenCost{Per1M: 15.0},
						},
					},
				},
			},
			expected: []string{
				"| **Cache Model** | $10.00 | $30.00 | $1.00 | $15.00 |",
			},
		},
		{
			name: "more than 15 models",
			models: func() []*catalogs.Model {
				models := make([]*catalogs.Model, 20)
				for i := 0; i < 20; i++ {
					models[i] = &catalogs.Model{
						Name: "Model",
						Pricing: &catalogs.ModelPricing{
							Tokens: &catalogs.TokenPricing{
								Input:  &catalogs.TokenCost{Per1M: float64(i)},
								Output: &catalogs.TokenCost{Per1M: float64(i * 3)},
							},
						},
					}
				}
				return models
			}(),
			expected: []string{
				"| Model | Input (per 1M) | Output (per 1M) |",
				// Should only show 15 models
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test_pricing_comp_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			writePricingComparisonTable(tmpFile, tt.models)

			tmpFile.Seek(0, 0)
			content, err := os.ReadFile(tmpFile.Name())
			require.NoError(t, err)
			contentStr := string(content)

			for _, expected := range tt.expected {
				assert.Contains(t, contentStr, expected)
			}

			// Count number of model rows (excluding header)
			lines := strings.Split(contentStr, "\n")
			modelRows := 0
			for _, line := range lines {
				if strings.HasPrefix(line, "| **") && !strings.Contains(line, "Model |") {
					modelRows++
				}
			}
			
			// Should not exceed 15 models
			if len(tt.models) > 15 {
				assert.LessOrEqual(t, modelRows, 15)
			}
		})
	}
}

func TestWriteContextLimitsTable(t *testing.T) {
	tests := []struct {
		name     string
		models   []*catalogs.Model
		expected []string
	}{
		{
			name: "sorted by context window",
			models: []*catalogs.Model{
				{
					Name: "Small Context",
					Limits: &catalogs.ModelLimits{
						ContextWindow: 4096,
						OutputTokens:  1024,
					},
				},
				{
					Name: "Large Context",
					Limits: &catalogs.ModelLimits{
						ContextWindow: 2000000,
						OutputTokens:  8192,
					},
				},
				{
					Name: "Medium Context",
					Limits: &catalogs.ModelLimits{
						ContextWindow: 128000,
						OutputTokens:  4096,
					},
				},
			},
			expected: []string{
				"| Model | Context Window | Max Output | Modalities |",
				// Should be sorted by context window: Large, Medium, Small
				"| **Large Context** | 2.0M",
				"| **Medium Context** | 128k",
				"| **Small Context** | 4.1k",
			},
		},
		{
			name: "with modalities",
			models: []*catalogs.Model{
				{
					Name: "Multimodal Model",
					Limits: &catalogs.ModelLimits{
						ContextWindow: 128000,
					},
					Features: &catalogs.ModelFeatures{
						Modalities: catalogs.ModelModalities{
							Input: []catalogs.ModelModality{
								catalogs.ModelModalityText,
								catalogs.ModelModalityImage,
								catalogs.ModelModalityAudio,
								catalogs.ModelModalityVideo,
							},
							Output: []catalogs.ModelModality{
								catalogs.ModelModalityText,
							},
						},
					},
				},
			},
			expected: []string{
				"| **Multimodal Model** | 128k | — | Text, Image, Audio, Video |",
			},
		},
		{
			name: "model without limits",
			models: []*catalogs.Model{
				{
					Name:   "No Limits Model",
					Limits: nil,
				},
			},
			expected: []string{
				// Should skip models without limits
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test_context_limits_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			writeContextLimitsTable(tmpFile, tt.models)

			tmpFile.Seek(0, 0)
			content, err := os.ReadFile(tmpFile.Name())
			require.NoError(t, err)
			contentStr := string(content)

			for _, expected := range tt.expected {
				assert.Contains(t, contentStr, expected)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{5242880, "5.0 MB"},
		{1073741824, "1.0 GB"},
		{2147483648, "2.0 GB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		assert.Equal(t, tt.expected, result)
	}
}

func TestComparisonTableHelpers(t *testing.T) {
	t.Run("hasText", func(t *testing.T) {
		features := &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input: []catalogs.ModelModality{catalogs.ModelModalityText},
			},
		}
		assert.True(t, hasText(features))

		features.Modalities.Input = []catalogs.ModelModality{catalogs.ModelModalityImage}
		assert.False(t, hasText(features))
	})

	t.Run("hasVision", func(t *testing.T) {
		features := &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input: []catalogs.ModelModality{catalogs.ModelModalityImage},
			},
		}
		assert.True(t, hasVision(features))

		features.Modalities.Input = []catalogs.ModelModality{catalogs.ModelModalityText}
		assert.False(t, hasVision(features))
	})

	t.Run("hasAudio", func(t *testing.T) {
		features := &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input: []catalogs.ModelModality{catalogs.ModelModalityAudio},
			},
		}
		assert.True(t, hasAudio(features))

		features.Modalities.Input = []catalogs.ModelModality{catalogs.ModelModalityText}
		assert.False(t, hasAudio(features))
	})

	t.Run("hasVideo", func(t *testing.T) {
		features := &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input: []catalogs.ModelModality{catalogs.ModelModalityVideo},
			},
		}
		assert.True(t, hasVideo(features))

		features.Modalities.Input = []catalogs.ModelModality{catalogs.ModelModalityText}
		assert.False(t, hasVideo(features))
	})
}

func TestComparisonTableEdgeCases(t *testing.T) {
	t.Run("empty models slice", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test_empty_*.md")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		writeModelsOverviewTable(tmpFile, []*catalogs.Model{}, []*catalogs.Provider{})

		tmpFile.Seek(0, 0)
		content, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		contentStr := string(content)

		// Should still have headers
		assert.Contains(t, contentStr, "| Model | Provider | Context | Input | Output | Features |")
	})

	t.Run("nil models slice", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test_nil_*.md")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		// Should not panic
		assert.NotPanics(t, func() {
			writeModelsOverviewTable(tmpFile, nil, nil)
		})
	})

	t.Run("model with nil pricing", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test_nil_pricing_*.md")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		models := []*catalogs.Model{
			{
				Name:    "No Pricing Model",
				Pricing: nil,
			},
		}

		writePricingComparisonTable(tmpFile, models)

		tmpFile.Seek(0, 0)
		content, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		contentStr := string(content)

		// Should handle nil pricing gracefully
		assert.Contains(t, contentStr, "| Model | Input (per 1M) | Output (per 1M) |")
	})
}