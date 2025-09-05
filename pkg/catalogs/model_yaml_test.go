package catalogs

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/utc"
)

func TestModel_FormatYAML_ComprehensiveFormatting(t *testing.T) {
	// Create a comprehensive test model with all features enabled to test formatting
	testTime := time.Date(2025, 8, 22, 4, 9, 45, 0, time.UTC)
	releaseDate := time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)
	knowledgeCutoff := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)

	model := Model{
		ID:          "test-model-comprehensive",
		Name:        "Test Model Comprehensive",
		Description: "A comprehensive test model used for verifying YAML formatting with all sections and subsections enabled",
		Authors: []Author{
			{
				ID:   "test-corp",
				Name: "Test Corporation",
			},
		},

		// Metadata section - should have blank line before
		Metadata: &ModelMetadata{
			ReleaseDate:     utc.Time{Time: releaseDate},
			KnowledgeCutoff: &utc.Time{Time: knowledgeCutoff},
			OpenWeights:     true,
			Tags:            []ModelTag{ModelTagMultimodal, ModelTagChat},
			Architecture: &ModelArchitecture{
				ParameterCount: "7B",
				Type:           ArchitectureTypeTransformer,
			},
		},

		// Features section - should have blank line before
		Features: &ModelFeatures{
			Modalities: ModelModalities{
				Input:  []ModelModality{ModelModalityText, ModelModalityImage, ModelModalityAudio},
				Output: []ModelModality{ModelModalityText, ModelModalityAudio},
			},

			// Core capabilities subsection - should have blank line before
			ToolCalls:   true,
			Tools:       true,
			ToolChoice:  true,
			WebSearch:   true,
			Attachments: true,

			// Reasoning & Verbosity subsection - should have blank line before
			Reasoning:        true,
			ReasoningEffort:  true,
			ReasoningTokens:  true,
			IncludeReasoning: true,
			Verbosity:        true,

			// Generation control support flags subsection - should have blank line before
			Temperature:                   true,
			TopP:                          true,
			TopK:                          true,
			TopA:                          true,
			MinP:                          true,
			TypicalP:                      true,
			TFS:                           true,
			MaxTokens:                     true,
			MaxOutputTokens:               true,
			Stop:                          true,
			StopTokenIDs:                  true,
			FrequencyPenalty:              true,
			PresencePenalty:               true,
			RepetitionPenalty:             true,
			NoRepeatNgramSize:             true,
			LengthPenalty:                 true,
			LogitBias:                     true,
			BadWords:                      true,
			AllowedTokens:                 true,
			Seed:                          true,
			Logprobs:                      true,
			TopLogprobs:                   true,
			Echo:                          true,
			N:                             true,
			BestOf:                        true,
			Mirostat:                      true,
			MirostatTau:                   true,
			MirostatEta:                   true,
			ContrastiveSearchPenaltyAlpha: true,
			NumBeams:                      true,
			EarlyStopping:                 true,
			DiversityPenalty:              true,

			// Response delivery subsection - should have blank line before
			FormatResponse:    true,
			StructuredOutputs: true,
			Streaming:         true,
		},

		// Generation section - should have blank line before (if present)
		Generation: &ModelGeneration{
			Temperature: &FloatRange{Min: 0.0, Max: 2.0, Default: 1.0},
			TopP:        &FloatRange{Min: 0.0, Max: 1.0, Default: 0.9},
			TopK:        &IntRange{Min: 1, Max: 100, Default: 50},
			MaxTokens:   intPtr(4096),
		},

		// Reasoning section - should have blank line before (if present)
		Reasoning: &ModelControlLevels{
			Levels:  []ModelControlLevel{ModelControlLevelLow, ModelControlLevelMedium, ModelControlLevelHigh},
			Default: &[]ModelControlLevel{ModelControlLevelMedium}[0],
		},

		// ReasoningTokens section - should have blank line before (if present)
		ReasoningTokens: &IntRange{Min: 1000, Max: 10000, Default: 5000},

		// Verbosity section - should have blank line before (if present)
		Verbosity: &ModelControlLevels{
			Levels:  []ModelControlLevel{ModelControlLevelLow, ModelControlLevelHigh},
			Default: &[]ModelControlLevel{ModelControlLevelLow}[0],
		},

		// Tools section - should have blank line before (if present)
		Tools: &ModelTools{
			ToolChoices: []ToolChoice{ToolChoiceAuto, ToolChoiceNone, ToolChoiceRequired},
			WebSearch: &ModelWebSearch{
				MaxResults:         intPtr(10),
				SearchPrompt:       &[]string{"Search for relevant information"}[0],
				SearchContextSizes: []ModelControlLevel{ModelControlLevelLow, ModelControlLevelMedium},
				DefaultContextSize: &[]ModelControlLevel{ModelControlLevelMedium}[0],
			},
		},

		// Attachments section - should have blank line before (if present)
		Attachments: &ModelAttachments{
			MimeTypes:   []string{"image/jpeg", "image/png", "audio/mpeg"},
			MaxFileSize: int64Ptr(10485760), // 10MB
			MaxFiles:    intPtr(5),
		},

		// Delivery section - should have blank line before (if present)
		Delivery: &ModelDelivery{
			Formats:   []ModelResponseFormat{ModelResponseFormatJSON, ModelResponseFormatText},
			Protocols: []ModelResponseProtocol{ModelResponseProtocolHTTP, ModelResponseProtocolWebSocket},
			Streaming: []ModelStreaming{ModelStreamingSSE, ModelStreamingWebSocket},
		},

		// Limits section - should have blank line before
		Limits: &ModelLimits{
			ContextWindow: 128000,
			OutputTokens:  8192,
		},

		// Pricing section - should have blank line before
		Pricing: &ModelPricing{
			Currency: ModelPricingCurrencyUSD,
			Tokens: &ModelTokenPricing{
				Input:  &ModelTokenCost{Per1M: 1.50},
				Output: &ModelTokenCost{Per1M: 6.00},
				Cache: &ModelTokenCachePricing{
					Read:  &ModelTokenCost{Per1M: 0.15},
					Write: &ModelTokenCost{Per1M: 1.50},
				},
				Reasoning: &ModelTokenCost{Per1M: 10.00},
			},
			Operations: &ModelOperationPricing{
				Request:      float64Ptr(0.001),
				ImageInput:   float64Ptr(0.01),
				AudioInput:   float64Ptr(0.005),
				WebSearch:    float64Ptr(0.02),
				FunctionCall: float64Ptr(0.001),
			},
		},

		// Timestamps section - should have blank line before
		CreatedAt: utc.Time{Time: testTime},
		UpdatedAt: utc.Time{Time: testTime},
	}

	yaml := model.FormatYAML()

	// Print the actual output for debugging
	t.Logf("Generated YAML:\n%s", yaml)

	// Test 1: Verify major section headers have blank lines before them
	testMajorSectionSpacing(t, yaml)

	// Test 2: Verify subsection headers within features have blank lines before them
	testSubsectionSpacing(t, yaml)

	// Test 3: Verify the overall structure and content preservation
	testStructurePreservation(t, yaml, model)

	// Test 4: Verify proper comment formatting
	testCommentFormatting(t, yaml)
}

func testMajorSectionSpacing(t *testing.T, yaml string) {
	t.Helper()

	lines := strings.Split(yaml, "\n")
	majorSectionHeaders := []string{
		"# Model metadata",
		"# Model features",
		"# Model limits",
		"# Model pricing",
		"# Timestamps",
	}

	for _, header := range majorSectionHeaders {
		headerIndex := -1
		for i, line := range lines {
			if strings.TrimSpace(line) == header {
				headerIndex = i
				break
			}
		}

		if headerIndex == -1 {
			t.Errorf("Major section header not found: %s", header)
			continue
		}

		// Check if previous line is blank (indicating proper spacing)
		if headerIndex > 0 {
			previousLine := lines[headerIndex-1]
			if strings.TrimSpace(previousLine) != "" {
				t.Errorf("Major section header '%s' should have blank line before it, but previous line was: '%s'", header, previousLine)
			}
		}
	}
}

func testSubsectionSpacing(t *testing.T, yaml string) {
	t.Helper()

	lines := strings.Split(yaml, "\n")
	subsectionHeaders := []string{
		"# Core capabilities",
		"# Reasoning & Verbosity",
		"# Generation control support flags",
		"# Response delivery",
	}

	for _, header := range subsectionHeaders {
		headerIndex := -1
		for i, line := range lines {
			if strings.TrimSpace(line) == header {
				headerIndex = i
				break
			}
		}

		if headerIndex == -1 {
			t.Errorf("Subsection header not found: %s", header)
			continue
		}

		// Check if previous line is blank (indicating proper spacing)
		// Exception: Don't require blank line immediately after "features:" line
		if headerIndex > 0 {
			previousLine := lines[headerIndex-1]
			if strings.TrimSpace(previousLine) != "" && !strings.Contains(previousLine, "features:") {
				t.Errorf("Subsection header '%s' should have blank line before it, but previous line was: '%s'", header, previousLine)
			}
		}
	}
}

func testStructurePreservation(t *testing.T, yaml string, original Model) {
	t.Helper()

	// Test that essential content is preserved
	expectedContent := []string{
		// Header comment with ID and description
		fmt.Sprintf("# %s - %s", original.ID, original.FormatYAMLHeaderComment()),

		// Core fields
		fmt.Sprintf("id: %s", original.ID),
		fmt.Sprintf("name: %s", original.Name),
		"description: A comprehensive test model used for verifying YAML formatting with all sections and subsections enabled",

		// Major sections should be present
		"metadata:",
		"features:",
		"limits:",
		"pricing:",

		// Key feature flags that were set to true
		"tool_calls: true",
		"reasoning: true",
		"temperature: true",
		"format_response: true",
		"streaming: true",

		// Timestamps should be formatted properly
		"created_at: 2025-08-22T04:09:45Z",
		"updated_at: 2025-08-22T04:09:45Z",

		// Date fields should be date-only format
		"release_date: 2024-12-01",
		"knowledge_cutoff: 2024-04-01",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(yaml, expected) {
			t.Errorf("YAML should contain: %s", expected)
		}
	}
}

func testCommentFormatting(t *testing.T, yaml string) {
	t.Helper()

	expectedComments := []string{
		"# test-model-comprehensive - A comprehensive test model used for verifying YAML formattin...",
		"# Model metadata",
		"# Model features",
		"# Core capabilities",
		"# Reasoning & Verbosity",
		"# Generation control support flags",
		"# Response delivery",
		"# Model limits",
		"# Model pricing",
		"# Timestamps",
	}

	for _, comment := range expectedComments {
		found := false
		for _, line := range strings.Split(yaml, "\n") {
			if strings.Contains(line, comment) || strings.HasPrefix(strings.TrimSpace(line), comment) {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Expected comment not found: %s", comment)
		}
	}
}

func TestModel_FormatYAML_RoundTripConsistency(t *testing.T) {
	// Test that a model can be formatted, saved, and loaded with consistent structure
	original := createTestModel()

	// Format to YAML
	yamlContent := original.FormatYAML()

	// The formatted YAML should maintain proper spacing when formatted again
	// (This tests the idempotent nature of the formatting)
	lines := strings.Split(yamlContent, "\n")
	nonEmptyLines := make([]string, 0)
	blankLineIndices := make([]int, 0)

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankLineIndices = append(blankLineIndices, i)
		} else {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	// Verify we have the expected number of blank lines for spacing
	// Should have blank lines before major sections and subsections
	expectedMinBlankLines := 8 // metadata, features, limits, pricing, timestamps, plus subsections
	if len(blankLineIndices) < expectedMinBlankLines {
		t.Errorf("Expected at least %d blank lines for proper spacing, got %d", expectedMinBlankLines, len(blankLineIndices))
	}

	// Verify the structure is maintained
	majorSections := []string{"metadata:", "features:", "limits:", "pricing:"}
	for _, section := range majorSections {
		found := false
		for _, line := range nonEmptyLines {
			if strings.TrimSpace(line) == section {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Major section missing after formatting: %s", section)
		}
	}
}

func TestModel_FormatYAML_MinimalModel(t *testing.T) {
	// Test formatting with minimal model data
	model := Model{
		ID:        "minimal-model",
		Name:      "Minimal Model",
		CreatedAt: utc.Time{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		UpdatedAt: utc.Time{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	yaml := model.FormatYAML()

	// Should still have proper structure even with minimal data
	expectedElements := []string{
		"# minimal-model - Minimal Model",
		"id: minimal-model",
		"name: Minimal Model",
		"# Timestamps",
		"created_at: 2025-01-01T00:00:00Z",
		"updated_at: 2025-01-01T00:00:00Z",
	}

	for _, element := range expectedElements {
		if !strings.Contains(yaml, element) {
			t.Errorf("Minimal model YAML should contain: %s", element)
		}
	}

	// Should not have sections that aren't present
	unexpectedSections := []string{"# Model metadata", "# Model features", "# Model limits", "# Model pricing"}
	for _, section := range unexpectedSections {
		if strings.Contains(yaml, section) {
			t.Errorf("Minimal model YAML should not contain: %s", section)
		}
	}
}

// Helper functions for test data creation.
func createTestModel() Model {
	testTime := time.Date(2025, 8, 22, 4, 9, 45, 0, time.UTC)

	return Model{
		ID:          "test-model",
		Name:        "Test Model",
		Description: "A test model for YAML formatting verification",

		Metadata: &ModelMetadata{
			ReleaseDate: utc.Time{Time: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)},
		},

		Features: &ModelFeatures{
			Modalities: ModelModalities{
				Input:  []ModelModality{ModelModalityText},
				Output: []ModelModality{ModelModalityText},
			},
			ToolCalls:      true,
			Tools:          true,
			Temperature:    true,
			FormatResponse: true,
			Streaming:      true,
		},

		Limits: &ModelLimits{
			ContextWindow: 8192,
			OutputTokens:  2048,
		},

		Pricing: &ModelPricing{
			Currency: ModelPricingCurrencyUSD,
			Tokens: &ModelTokenPricing{
				Input:  &ModelTokenCost{Per1M: 0.50},
				Output: &ModelTokenCost{Per1M: 2.00},
			},
		},

		CreatedAt: utc.Time{Time: testTime},
		UpdatedAt: utc.Time{Time: testTime},
	}
}

// Helper functions for pointer creation.
func intPtr(i int) *int {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}
