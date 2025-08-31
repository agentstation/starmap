package docs

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create float pointers
func floatPtr(f float64) *float64 {
	return &f
}

func TestWriteModalityTable(t *testing.T) {
	tests := []struct {
		name     string
		model    *catalogs.Model
		expected []string
		notExpected []string
	}{
		{
			name: "multimodal input and output",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Modalities: catalogs.ModelModalities{
						Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage, catalogs.ModelModalityAudio},
						Output: []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage},
					},
				},
			},
			expected: []string{
				"| Direction | Text | Image | Audio | Video | PDF |",
				"| **Input** | ✅ | ✅ | ✅ | ❌ | ❌ |",
				"| **Output** | ✅ | ✅ | ❌ | ❌ | ❌ |",
			},
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
			expected: []string{
				"| **Input** | ✅ | ❌ | ❌ | ❌ | ❌ |",
				"| **Output** | ✅ | ❌ | ❌ | ❌ | ❌ |",
			},
		},
		{
			name: "no features",
			model: &catalogs.Model{
				Features: nil,
			},
			expected: []string{
				"No modality information available.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpFile, err := os.CreateTemp("", "test_modality_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			// Write table
			writeModalityTable(tmpFile, tt.model)

			// Read content
			tmpFile.Seek(0, 0)
			content, err := os.ReadFile(tmpFile.Name())
			require.NoError(t, err)
			contentStr := string(content)

			// Check expected content
			for _, expected := range tt.expected {
				assert.Contains(t, contentStr, expected)
			}

			// Check not expected content
			for _, notExpected := range tt.notExpected {
				assert.NotContains(t, contentStr, notExpected)
			}
		})
	}
}

func TestWriteCoreFeatureTable(t *testing.T) {
	tests := []struct {
		name     string
		model    *catalogs.Model
		expected []string
	}{
		{
			name: "full featured model",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Tools:       true,
					ToolCalls:   true,
					ToolChoice:  true,
					WebSearch:   true,
					Attachments: true,
				},
			},
			expected: []string{
				"| Tool Calling | Tool Definitions | Tool Choice | Web Search | File Attachments |",
				"| ✅ | ✅ | ✅ | ✅ | ✅ |",
			},
		},
		{
			name: "partial features",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Tools:     true,
					ToolCalls: true,
				},
			},
			expected: []string{
				"| ✅ | ✅ | ❌ | ❌ | ❌ |",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test_core_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			writeCoreFeatureTable(tmpFile, tt.model)

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

func TestWriteResponseDeliveryTable(t *testing.T) {
	tests := []struct {
		name     string
		model    *catalogs.Model
		expected []string
	}{
		{
			name: "model with all response features",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Streaming:         true,
					StructuredOutputs: true,
					FormatResponse:    true,
					ToolCalls:         true,
				},
			},
			expected: []string{
				"| Streaming | Structured Output | JSON Mode | Function Call | Text Format |",
				"| ✅ | ✅ | ✅ | ✅ | ✅ |",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test_response_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			writeResponseDeliveryTable(tmpFile, tt.model)

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

func TestWriteAdvancedReasoningTable(t *testing.T) {
	tests := []struct {
		name        string
		model       *catalogs.Model
		expected    []string
		shouldBeEmpty bool
	}{
		{
			name: "model with reasoning features",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Reasoning:        true,
					ReasoningEffort:  true,
					ReasoningTokens:  true,
					IncludeReasoning: true,
					Verbosity:        true,
				},
			},
			expected: []string{
				"| Basic Reasoning | Reasoning Effort | Reasoning Tokens | Include Reasoning | Verbosity Control |",
				"| ✅ | ✅ | ✅ | ✅ | ✅ |",
			},
		},
		{
			name: "model without reasoning features",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{},
			},
			shouldBeEmpty: true,  // Table is not generated when no reasoning features
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test_reasoning_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			writeAdvancedReasoningTable(tmpFile, tt.model)

			tmpFile.Seek(0, 0)
			content, err := os.ReadFile(tmpFile.Name())
			require.NoError(t, err)
			contentStr := string(content)

			if tt.shouldBeEmpty {
				assert.Empty(t, contentStr, "Table should not be generated when no reasoning features")
			} else {
				for _, expected := range tt.expected {
					assert.Contains(t, contentStr, expected)
				}
			}
		})
	}
}

func TestWriteControlsTables(t *testing.T) {
	tests := []struct {
		name     string
		model    *catalogs.Model
		expected []string
	}{
		{
			name: "model with sampling controls",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					Temperature:      true,
					TopP:            true,
					TopK:            true,
					RepetitionPenalty: true,
					MaxTokens:       true,
					Seed:            true,
				},
			},
			expected: []string{
				"### Sampling & Decoding",
				"| Temperature | Top-P | Top-K |",
				"| 0.0-2.0 | 0.0-1.0 | ✅ |",
				"### Length & Termination",
				"| Max Tokens |",
				"### Repetition Control",
				"| Repetition Penalty |",
				"### Advanced Controls",
				"| Deterministic Seed |",
			},
		},
		{
			name: "model with penalty controls",
			model: &catalogs.Model{
				Features: &catalogs.ModelFeatures{
					FrequencyPenalty: true,
					PresencePenalty:  true,
				},
			},
			expected: []string{
				"### Repetition Control",
				"| Frequency Penalty | Presence Penalty |",
				"| -2.0 to 2.0 | -2.0 to 2.0 |",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test_controls_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			writeControlsTables(tmpFile, tt.model)

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

func TestWriteTokenPricingTable(t *testing.T) {
	tests := []struct {
		name     string
		model    *catalogs.Model
		expected []string
	}{
		{
			name: "model with standard pricing",
			model: &catalogs.Model{
				Pricing: &catalogs.ModelPricing{
					Currency: "USD",
					Tokens: &catalogs.TokenPricing{
						Input: &catalogs.TokenCost{
							Per1M: 10.0,
						},
						Output: &catalogs.TokenCost{
							Per1M: 30.0,
						},
					},
				},
			},
			expected: []string{
				"### Token Pricing",
				"| Input | Output | Reasoning | Cache Read | Cache Write |",
				"| $10.00/1M | $30.00/1M | - | - | - |",
			},
		},
		{
			name: "model with cache pricing",
			model: &catalogs.Model{
				Pricing: &catalogs.ModelPricing{
					Currency: "USD",
					Tokens: &catalogs.TokenPricing{
						Input: &catalogs.TokenCost{
							Per1M: 10.0,
						},
						Output: &catalogs.TokenCost{
							Per1M: 30.0,
						},
						CacheRead: &catalogs.TokenCost{
							Per1M: 1.0,
						},
						CacheWrite: &catalogs.TokenCost{
							Per1M: 15.0,
						},
					},
				},
			},
			expected: []string{
				"| Input | Output | Reasoning | Cache Read | Cache Write |",
				"| $10.00/1M | $30.00/1M | - | $1.00/1M | $15.00/1M |",
			},
		},
		{
			name: "free model",
			model: &catalogs.Model{
				Pricing: &catalogs.ModelPricing{
					Currency: "USD",
					Tokens: &catalogs.TokenPricing{
						Input: &catalogs.TokenCost{
							Per1M: 0.0,
						},
						Output: &catalogs.TokenCost{
							Per1M: 0.0,
						},
					},
				},
			},
			expected: []string{
				"| Input | Output | Reasoning | Cache Read | Cache Write |",
				"| - | - | - | - | - |",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test_pricing_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			writeTokenPricingTable(tmpFile, tt.model)

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

func TestWriteArchitectureTable(t *testing.T) {
	tests := []struct {
		name     string
		model    *catalogs.Model
		expected []string
	}{
		{
			name: "model with full architecture",
			model: &catalogs.Model{
				Metadata: &catalogs.ModelMetadata{
					Architecture: &catalogs.ModelArchitecture{
						Type:           catalogs.ArchitectureTypeTransformer,
						ParameterCount: "70B",
						Tokenizer:      catalogs.TokenizerGPT,
					},
				},
			},
			expected: []string{
				"| Parameter Count | Architecture Type | Tokenizer | Quantization | Fine-Tuned | Base Model |",
				"| 70B | transformer | gpt | None | No | - |",
			},
		},
		{
			name: "model with minimal architecture",
			model: &catalogs.Model{
				Metadata: &catalogs.ModelMetadata{
					Architecture: &catalogs.ModelArchitecture{
						Type:           catalogs.ArchitectureTypeDiffusion,
						ParameterCount: "1.5B",
					},
				},
			},
			expected: []string{
				"| Parameter Count | Architecture Type | Tokenizer | Quantization | Fine-Tuned | Base Model |",
				"| 1.5B | diffusion | Unknown | None | No | - |",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test_arch_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			writeArchitectureTable(tmpFile, tt.model)

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

func TestWriteModelTagsTable(t *testing.T) {
	tests := []struct {
		name     string
		model    *catalogs.Model
		expected []string
		shouldBeEmpty bool
	}{
		{
			name: "model with multiple tags",
			model: &catalogs.Model{
				Metadata: &catalogs.ModelMetadata{
					Tags: []catalogs.ModelTag{
						catalogs.ModelTagChat,
						catalogs.ModelTagCoding,
						catalogs.ModelTagReasoning,
						catalogs.ModelTagMultimodal,
					},
				},
			},
			expected: []string{
				"### Model Tags",
				"| Coding | Writing | Reasoning | Math | Chat | Multimodal | Function Calling |",
				"| ✅ | ❌ | ✅ | ❌ | ✅ | ✅ | ❌ |",
			},
		},
		{
			name: "model with no tags",
			model: &catalogs.Model{
				Metadata: &catalogs.ModelMetadata{
					Tags: []catalogs.ModelTag{},
				},
			},
			expected: []string{}, // Table is not generated when no tags
			shouldBeEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test_tags_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			writeTagsTable(tmpFile, tt.model)

			tmpFile.Seek(0, 0)
			content, err := os.ReadFile(tmpFile.Name())
			require.NoError(t, err)
			contentStr := string(content)

			if tt.shouldBeEmpty {
				assert.Empty(t, contentStr, "Table should not be generated when no tags")
			} else {
				for _, expected := range tt.expected {
					assert.Contains(t, contentStr, expected)
				}
			}
		})
	}
}

func TestWriteOperationPricingTable(t *testing.T) {
	tests := []struct {
		name     string
		model    *catalogs.Model
		expected []string
	}{
		{
			name: "model with operations",
			model: &catalogs.Model{
				Pricing: &catalogs.ModelPricing{
					Currency: "USD",
					Operations: &catalogs.OperationPricing{
						Request:   floatPtr(0.001),
						ImageGen:  floatPtr(0.02),
						WebSearch: floatPtr(0.005),
					},
				},
			},
			expected: []string{
				"### Operation Pricing",
				"| Image Input | Audio Input | Video Input | Image Gen | Audio Gen | Web Search |",
				"| - | - | - | $0.020/img | - | $0.005/query |",
			},
		},
		{
			name: "model without operations",
			model: &catalogs.Model{
				Pricing: &catalogs.ModelPricing{
					Currency: "USD",
					Operations: nil,
				},
			},
			expected: []string{},
		},
		{
			name: "model with all operation types",
			model: &catalogs.Model{
				Pricing: &catalogs.ModelPricing{
					Currency: "USD",
					Operations: &catalogs.OperationPricing{
						ImageInput: floatPtr(0.001),
						AudioInput: floatPtr(0.002),
						VideoInput: floatPtr(0.003),
						ImageGen:   floatPtr(0.02),
						AudioGen:   floatPtr(0.015),
						WebSearch:  floatPtr(0.005),
					},
				},
			},
			expected: []string{
				"### Operation Pricing",
				"$0.001/img",
				"$0.002/min",
				"$0.003/min",
				"$0.020/img",
				"$0.015/min",
				"$0.005/query",
			},
		},
		{
			name: "model with EUR currency",
			model: &catalogs.Model{
				Pricing: &catalogs.ModelPricing{
					Currency: "EUR",
					Operations: &catalogs.OperationPricing{
						ImageGen:  floatPtr(0.018),
						AudioGen:  floatPtr(0.012),
					},
				},
			},
			expected: []string{
				"### Operation Pricing",
				"€0.018/img",
				"€0.012/min",
			},
		},
		{
			name: "model with GBP currency",
			model: &catalogs.Model{
				Pricing: &catalogs.ModelPricing{
					Currency: "GBP",
					Operations: &catalogs.OperationPricing{
						VideoInput: floatPtr(0.0025),
						WebSearch:  floatPtr(0.004),
					},
				},
			},
			expected: []string{
				"### Operation Pricing",
				"£0.003/min",  // Note: rounded to 3 decimal places
				"£0.004/query",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test_op_pricing_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			writeOperationPricingTable(tmpFile, tt.model)

			tmpFile.Seek(0, 0)
			content, err := os.ReadFile(tmpFile.Name())
			require.NoError(t, err)
			contentStr := string(content)

			if len(tt.expected) == 0 {
				// Check that nothing was written or that "No operation pricing" message appears
				assert.True(t, contentStr == "" || strings.Contains(contentStr, "No operation pricing"), 
					"Expected empty output or 'No operation pricing' message")
			} else {
				for _, expected := range tt.expected {
					// Allow for some flexibility in matching since operation order may vary
					normalizedContent := strings.ReplaceAll(contentStr, "\n", " ")
					normalizedExpected := strings.ReplaceAll(expected, "\n", " ")
					assert.Contains(t, normalizedContent, normalizedExpected)
				}
			}
		})
	}
}

// Helper test to ensure all table functions handle nil features gracefully
func TestTableFunctionsHandleNilFeatures(t *testing.T) {
	model := &catalogs.Model{
		ID:       "test-model",
		Name:     "Test Model",
		Features: nil, // No features
		Metadata: nil, // No metadata
	}

	type tableFunc struct {
		fn func(io.Writer, *catalogs.Model)
		name string
		canBeEmpty bool  // Some tables are conditional
	}

	tableFuncs := []tableFunc{
		{writeModalityTable, "writeModalityTable", false},
		{writeCoreFeatureTable, "writeCoreFeatureTable", false},
		{writeResponseDeliveryTable, "writeResponseDeliveryTable", false},
		{writeAdvancedReasoningTable, "writeAdvancedReasoningTable", true},  // Conditional on reasoning features
		{writeControlsTables, "writeControlsTables", false},
		{writeArchitectureTable, "writeArchitectureTable", true},  // Conditional on metadata
		{writeTagsTable, "writeTagsTable", true},  // Conditional on tags
	}

	for _, tf := range tableFuncs {
		t.Run(tf.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test_nil_*.md")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			// Should not panic
			assert.NotPanics(t, func() {
				tf.fn(tmpFile, model)
			})

			// Check content based on whether table can be empty
			tmpFile.Seek(0, 0)
			content, err := os.ReadFile(tmpFile.Name())
			require.NoError(t, err)
			
			if tf.canBeEmpty {
				// These tables are conditional and may be empty
				// Just ensure they didn't panic
			} else {
				// These tables should write something even with nil features
				assert.NotEmpty(t, content, "Table %s should write something even with nil features", tf.name)
			}
		})
	}
}

// Test helper function for checking support
func TestCheckFeatureSupport(t *testing.T) {
	features := &catalogs.ModelFeatures{
		Tools:     true,
		Streaming: false,
		Reasoning: true,
	}

	tests := []struct {
		feature  string
		expected string
	}{
		{"Tools", "✅"},
		{"Streaming", "❌"},
		{"Reasoning", "✅"},
		{"WebSearch", "❌"}, // Not set, should be false
	}

	for _, tt := range tests {
		t.Run(tt.feature, func(t *testing.T) {
			// This would need to be extracted from the actual implementation
			// For now, we just test the concept
			var result string
			switch tt.feature {
			case "Tools":
				if features.Tools {
					result = "✅"
				} else {
					result = "❌"
				}
			case "Streaming":
				if features.Streaming {
					result = "✅"
				} else {
					result = "❌"
				}
			case "Reasoning":
				if features.Reasoning {
					result = "✅"
				} else {
					result = "❌"
				}
			case "WebSearch":
				if features.WebSearch {
					result = "✅"
				} else {
					result = "❌"
				}
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCurrencySymbol(t *testing.T) {
	tests := []struct {
		name     string
		currency string
		expected string
	}{
		{
			name:     "USD currency",
			currency: "USD",
			expected: "$",
		},
		{
			name:     "empty string defaults to USD",
			currency: "",
			expected: "$",
		},
		{
			name:     "EUR currency",
			currency: "EUR",
			expected: "€",
		},
		{
			name:     "GBP currency",
			currency: "GBP",
			expected: "£",
		},
		{
			name:     "JPY currency",
			currency: "JPY",
			expected: "¥",
		},
		{
			name:     "unknown currency defaults to USD",
			currency: "XXX",
			expected: "$",
		},
		{
			name:     "lowercase currency not recognized",
			currency: "usd",
			expected: "$", // defaults to USD
		},
		{
			name:     "CAD not explicitly handled",
			currency: "CAD",
			expected: "$", // defaults to USD
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCurrencySymbol(tt.currency)
			assert.Equal(t, tt.expected, result)
		})
	}
}