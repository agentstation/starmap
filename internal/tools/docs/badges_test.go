package docs

import (
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/stretchr/testify/assert"
)

func TestFeatureBadges(t *testing.T) {
	tests := []struct {
		name     string
		model    *catalogs.Model
		contains []string // Badge strings that should be present
		notEmpty bool     // Whether output should be non-empty
	}{
		{
			name: "nil features",
			model: &catalogs.Model{
				Features: nil,
			},
			notEmpty: false,
		},
		{
			name: "text only model",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
						Output: []catalogs.ModelModality{catalogs.ModelModalityText},
					},
				},
			},
			contains: []string{"text", "input", "output"},
			notEmpty: true,
		},
		{
			name: "multimodal model",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input: []catalogs.ModelModality{
							catalogs.ModelModalityText,
							catalogs.ModelModalityImage,
							catalogs.ModelModalityAudio,
						},
						Output: []catalogs.ModelModality{catalogs.ModelModalityText},
					},
				},
			},
			contains: []string{"text", "vision", "audio", "input", "output"},
			notEmpty: true,
		},
		{
			name: "model with tools",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
						Output: []catalogs.ModelModality{catalogs.ModelModalityText},
					},
					Tools:      true,
					ToolCalls:  true,
					ToolChoice: true,
					WebSearch:  true,
				},
			},
			contains: []string{"tools", "tool__calls", "tool__choice", "web__search"},
			notEmpty: true,
		},
		{
			name: "model with reasoning",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
						Output: []catalogs.ModelModality{catalogs.ModelModalityText},
					},
					Reasoning:        true,
					ReasoningEffort:  true,
					ReasoningTokens:  true,
					IncludeReasoning: true,
				},
			},
			contains: []string{"reasoning", "reasoning__effort", "reasoning__tokens", "include__reasoning"},
			notEmpty: true,
		},
		{
			name: "model with generation controls",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
						Output: []catalogs.ModelModality{catalogs.ModelModalityText},
					},
					Temperature:       true,
					TopP:              true,
					TopK:              true,
					FrequencyPenalty:  true,
					PresencePenalty:   true,
					RepetitionPenalty: true,
					Seed:              true,
					Logprobs:          true,
				},
			},
			contains: []string{
				"temperature", "top__p", "top__k",
				"frequency__penalty", "presence__penalty", "repetition__penalty",
				"seed", "logprobs",
			},
			notEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := featureBadges(tt.model)
			
			if tt.notEmpty {
				assert.NotEmpty(t, result)
			} else {
				assert.Empty(t, result)
			}
			
			// Check that expected strings are present
			for _, expected := range tt.contains {
				assert.Contains(t, result, expected, "Should contain badge for %s", expected)
			}
			
			// Verify it generates valid markdown badges (shields.io format)
			if result != "" {
				assert.Contains(t, result, "![")
				assert.Contains(t, result, "](https://img.shields.io/badge/")
			}
		})
	}
}

func TestTechnicalSpecBadges(t *testing.T) {
	tests := []struct {
		name     string
		model    *catalogs.Model
		sections []string // Section headers that should be present
		notEmpty bool
	}{
		{
			name: "nil features",
			model: &catalogs.Model{
				Features: nil,
			},
			notEmpty: false,
		},
		{
			name: "model with sampling controls",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Temperature: true,
					TopP:        true,
					TopK:        true,
					TopA:        true,
				},
			},
			sections: []string{"Sampling Controls:"},
			notEmpty: true,
		},
		{
			name: "model with repetition controls",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					FrequencyPenalty:  true,
					PresencePenalty:   true,
					RepetitionPenalty: true,
					NoRepeatNgramSize: true,
				},
			},
			sections: []string{"Repetition Controls:"},
			notEmpty: true,
		},
		{
			name: "model with observability",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Logprobs:    true,
					TopLogprobs: true,
					Echo:        true,
				},
			},
			sections: []string{"Observability:"},
			notEmpty: true,
		},
		{
			name: "model with advanced features",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Seed:   true,
					N:      true,
					BestOf: true,
				},
			},
			sections: []string{"Advanced Features:"},
			notEmpty: true,
		},
		{
			name: "comprehensive model",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Temperature:       true,
					TopP:              true,
					FrequencyPenalty:  true,
					PresencePenalty:   true,
					Logprobs:          true,
					Seed:              true,
				},
			},
			sections: []string{
				"Sampling Controls:",
				"Repetition Controls:",
				"Observability:",
				"Advanced Features:",
			},
			notEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := technicalSpecBadges(tt.model)
			
			if tt.notEmpty {
				assert.NotEmpty(t, result)
			} else {
				assert.Empty(t, result)
			}
			
			// Check that expected sections are present
			for _, section := range tt.sections {
				assert.Contains(t, result, section, "Should contain section %s", section)
			}
			
			// Verify it generates valid markdown badges
			if result != "" {
				assert.Contains(t, result, "![")
				assert.Contains(t, result, "](https://img.shields.io/badge/")
				
				// Check for proper shields.io URL encoding
				if strings.Contains(result, "top_p") {
					assert.Contains(t, result, "top__p", "Underscores should be doubled for shields.io")
				}
			}
		})
	}
}

func TestCreateBadge(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		value    string
		color    string
		tooltip  string
		expected string
	}{
		{
			name:     "simple badge",
			label:    "test",
			value:    "✓",
			color:    "green",
			tooltip:  "Test badge",
			expected: "![Test badge](https://img.shields.io/badge/test-✓-green)",
		},
		{
			name:     "badge with underscores",
			label:    "top_p",
			value:    "supported",
			color:    "red",
			tooltip:  "Top-P sampling",
			expected: "![Top-P sampling](https://img.shields.io/badge/top__p-supported-red)",
		},
		{
			name:     "badge with spaces",
			label:    "tool calls",
			value:    "yes",
			color:    "blue",
			tooltip:  "Tool calling support",
			expected: "![Tool calling support](https://img.shields.io/badge/tool_calls-yes-blue)",
		},
		{
			name:     "badge with dashes",
			label:    "top-k",
			value:    "enabled",
			color:    "orange",
			tooltip:  "Top-K sampling",
			expected: "![Top-K sampling](https://img.shields.io/badge/top--k-enabled-orange)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createBadge(tt.label, tt.value, tt.color, tt.tooltip)
			assert.Equal(t, tt.expected, result)
			
			// Verify it's pure markdown (no HTML)
			assert.NotContains(t, result, "<span")
			assert.NotContains(t, result, "</span>")
			
			// Verify proper markdown badge format
			assert.True(t, strings.HasPrefix(result, "!["))
			assert.Contains(t, result, "](https://img.shields.io/badge/")
		})
	}
}

func TestModalityHelpers(t *testing.T) {
	tests := []struct {
		name     string
		features *catalogs.ModelFeatures
		hasText  bool
		hasVision bool
		hasAudio bool
		hasVideo bool
		hasPDF   bool
	}{
		{
			name:     "nil features",
			features: nil,
			hasText:  false,
			hasVision: false,
			hasAudio: false,
			hasVideo: false,
			hasPDF:   false,
		},
		{
			name: "text only",
			features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
			},
			hasText:  true,
			hasVision: false,
			hasAudio: false,
			hasVideo: false,
			hasPDF:   false,
		},
		{
			name: "multimodal input",
			features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input: []catalogs.ModelModality{
						catalogs.ModelModalityText,
						catalogs.ModelModalityImage,
						catalogs.ModelModalityAudio,
						catalogs.ModelModalityVideo,
						catalogs.ModelModalityPDF,
					},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
			},
			hasText:  true,
			hasVision: true,
			hasAudio: true,
			hasVideo: true,
			hasPDF:   true,
		},
		{
			name: "multimodal output",
			features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input: []catalogs.ModelModality{catalogs.ModelModalityText},
					Output: []catalogs.ModelModality{
						catalogs.ModelModalityText,
						catalogs.ModelModalityImage,
					},
				},
			},
			hasText:  true,
			hasVision: true,
			hasAudio: false,
			hasVideo: false,
			hasPDF:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.hasText, hasText(tt.features), "hasText")
			assert.Equal(t, tt.hasVision, hasVision(tt.features), "hasVision")
			assert.Equal(t, tt.hasAudio, hasAudio(tt.features), "hasAudio")
			assert.Equal(t, tt.hasVideo, hasVideo(tt.features), "hasVideo")
			assert.Equal(t, tt.hasPDF, hasPDF(tt.features), "hasPDF")
		})
	}
}

func TestHasToolSupport(t *testing.T) {
	tests := []struct {
		name     string
		features *catalogs.ModelFeatures
		expected bool
	}{
		{
			name:     "nil features",
			features: nil,
			expected: false,
		},
		{
			name: "no tool support",
			features: &catalogs.ModelFeatures{
				Tools:     false,
				ToolCalls: false,
			},
			expected: false,
		},
		{
			name: "has tools",
			features: &catalogs.ModelFeatures{
				Tools:     true,
				ToolCalls: false,
			},
			expected: true,
		},
		{
			name: "has tool calls",
			features: &catalogs.ModelFeatures{
				Tools:     false,
				ToolCalls: true,
			},
			expected: true,
		},
		{
			name: "has both",
			features: &catalogs.ModelFeatures{
				Tools:     true,
				ToolCalls: true,
			},
			expected: true,
		},
		{
			name: "has tool choice",
			features: &catalogs.ModelFeatures{
				Tools:      false,
				ToolCalls:  false,
				ToolChoice: true,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasToolSupport(tt.features)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestModalitySliceToStrings(t *testing.T) {
	tests := []struct {
		name       string
		modalities []catalogs.ModelModality
		expected   []string
	}{
		{
			name:       "empty slice",
			modalities: []catalogs.ModelModality{},
			expected:   []string{},
		},
		{
			name: "single modality",
			modalities: []catalogs.ModelModality{
				catalogs.ModelModalityText,
			},
			expected: []string{"text"},
		},
		{
			name: "multiple modalities",
			modalities: []catalogs.ModelModality{
				catalogs.ModelModalityText,
				catalogs.ModelModalityImage,
				catalogs.ModelModalityAudio,
			},
			expected: []string{"text", "image", "audio"},
		},
		{
			name: "all modalities",
			modalities: []catalogs.ModelModality{
				catalogs.ModelModalityText,
				catalogs.ModelModalityImage,
				catalogs.ModelModalityAudio,
				catalogs.ModelModalityVideo,
				catalogs.ModelModalityPDF,
				catalogs.ModelModalityEmbedding,
			},
			expected: []string{"text", "image", "audio", "video", "pdf", "embedding"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := modalitySliceToStrings(tt.modalities)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeliveryBadges(t *testing.T) {
	tests := []struct {
		name     string
		features *catalogs.ModelFeatures
		contains []string
		notEmpty bool
	}{
		{
			name: "no delivery features",
			features: &catalogs.ModelFeatures{
				FormatResponse:    false,
				StructuredOutputs: false,
				Streaming:         false,
			},
			notEmpty: false,
		},
		{
			name: "has format response",
			features: &catalogs.ModelFeatures{
				FormatResponse: true,
			},
			contains: []string{"format__response", "Alternative response formats"},
			notEmpty: true,
		},
		{
			name: "has structured outputs",
			features: &catalogs.ModelFeatures{
				StructuredOutputs: true,
			},
			contains: []string{"structured__outputs", "JSON schema validation"},
			notEmpty: true,
		},
		{
			name: "has streaming",
			features: &catalogs.ModelFeatures{
				Streaming: true,
			},
			contains: []string{"streaming", "Response streaming"},
			notEmpty: true,
		},
		{
			name: "all delivery features",
			features: &catalogs.ModelFeatures{
				FormatResponse:    true,
				StructuredOutputs: true,
				Streaming:         true,
			},
			contains: []string{"format__response", "structured__outputs", "streaming"},
			notEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			badges := deliveryBadges(tt.features)
			result := strings.Join(badges, " ")
			
			if tt.notEmpty {
				assert.NotEmpty(t, result)
			} else {
				assert.Empty(t, result)
			}
			
			for _, expected := range tt.contains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

// TestGenerationBadgesSimple tests generationBadges to improve coverage
func TestGenerationBadgesSimple(t *testing.T) {
	// Test with empty features
	features1 := &catalogs.ModelFeatures{}
	badges1 := generationBadges(features1)
	assert.Empty(t, badges1)

	// Test with temperature and top_p
	features2 := &catalogs.ModelFeatures{
		Temperature: true,
		TopP:        true,
	}
	badges2 := generationBadges(features2)
	assert.Len(t, badges2, 2)

	// Test with all core sampling
	features3 := &catalogs.ModelFeatures{
		Temperature: true,
		TopP:        true,
		TopK:        true,
		TopA:        true,
		MinP:        true,
		TypicalP:    true,
		TFS:         true,
	}
	badges3 := generationBadges(features3)
	assert.Greater(t, len(badges3), 5)

	// Test with length controls
	features4 := &catalogs.ModelFeatures{
		MaxTokens:       true,
		MaxOutputTokens: true,
		Stop:            true,
		StopTokenIds:    true,
	}
	badges4 := generationBadges(features4)
	assert.Greater(t, len(badges4), 2)

	// Test with penalties
	features5 := &catalogs.ModelFeatures{
		FrequencyPenalty:  true,
		PresencePenalty:   true,
		RepetitionPenalty: true,
		NoRepeatNgramSize: true,
		LengthPenalty:     true,
	}
	badges5 := generationBadges(features5)
	assert.Greater(t, len(badges5), 3)

	// Test with advanced features
	features6 := &catalogs.ModelFeatures{
		Seed:          true,
		BestOf:        true,
		Logprobs:      true,
		TopLogprobs:   true,
		LogitBias:     true,
		BadWords:      true,
		AllowedTokens: true,
		Mirostat:      true,
		Echo:          true,
	}
	badges6 := generationBadges(features6)
	assert.Greater(t, len(badges6), 5)
}

// TestReasoningBadgesSimple tests reasoningBadges to improve coverage
func TestReasoningBadgesSimple(t *testing.T) {
	// Test with no reasoning features
	features1 := &catalogs.ModelFeatures{}
	badges1 := reasoningBadges(features1)
	assert.Empty(t, badges1)

	// Test with basic reasoning
	features2 := &catalogs.ModelFeatures{
		Reasoning: true,
	}
	badges2 := reasoningBadges(features2)
	assert.NotEmpty(t, badges2)

	// Test with all reasoning features
	features3 := &catalogs.ModelFeatures{
		Reasoning:        true,
		ReasoningEffort:  true,
		ReasoningTokens:  true,
		IncludeReasoning: true,
		Verbosity:        true,
	}
	badges3 := reasoningBadges(features3)
	assert.Greater(t, len(badges3), 3)
}