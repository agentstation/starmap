package catalogs

import (
	"strings"
	"testing"
	"time"

	"github.com/agentstation/utc"
)

func TestModel_FormatYAML_EnhancedFormat(t *testing.T) {
	// Create a test model similar to the reference gemini-2.5-pro format
	model := Model{
		ID:          "gemini-2.5-pro",
		Name:        "Gemini 2.5 Pro",
		Description: "Google Vertex AI model: gemini-2.5-pro",

		// Add metadata section with date-only formatting
		Metadata: &ModelMetadata{
			ReleaseDate:     utc.Time{Time: time.Date(2025, 3, 20, 0, 0, 0, 0, time.UTC)},
			KnowledgeCutoff: &utc.Time{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		},

		Features: &ModelFeatures{
			Modalities: ModelModalities{
				Input:  []ModelModality{ModelModalityText, ModelModalityImage},
				Output: []ModelModality{ModelModalityText},
			},
			ToolCalls:         true,
			Tools:             true,
			ToolChoice:        false,
			WebSearch:         false,
			Attachments:       false,
			Reasoning:         true,
			ReasoningEffort:   false,
			ReasoningTokens:   false,
			IncludeReasoning:  false,
			Verbosity:         false,
			Temperature:       false,
			TopP:              false,
			TopK:              false,
			TopA:              false,
			MinP:              false,
			MaxTokens:         false,
			FrequencyPenalty:  false,
			PresencePenalty:   false,
			RepetitionPenalty: false,
			LogitBias:         false,
			Seed:              false,
			Stop:              false,
			Logprobs:          false,
			FormatResponse:    false,
			StructuredOutputs: false,
			Streaming:         false,
		},

		// Empty limits like the reference
		Limits: &ModelLimits{},

		Pricing: &ModelPricing{
			Currency: "USD",
			Tokens: &TokenPricing{
				Input:  &TokenCost{Per1M: 1.25},
				Output: &TokenCost{Per1M: 10.00},
				Cache: &TokenCachePricing{
					Read: &TokenCost{Per1M: 0.31},
				},
			},
		},

		CreatedAt: utc.Time{Time: time.Date(2025, 8, 22, 4, 9, 45, 0, time.UTC)},
		UpdatedAt: utc.Time{Time: time.Date(2025, 8, 22, 4, 9, 45, 0, time.UTC)},
	}

	yaml := model.FormatYAML()

	// Print the actual output for comparison with reference
	t.Logf("Generated YAML:\n%s", yaml)

	// Test specific formatting improvements
	expectedElements := []string{
		"# gemini-2.5-pro - Google Vertex AI model: gemini-2.5-pro",
		"id: gemini-2.5-pro",
		"name: Gemini 2.5 Pro",
		"description: |-", // Block scalar format

		"# Model metadata",
		"metadata:",
		"release_date: 2025-03-20",     // Date-only format
		"knowledge_cutoff: 2025-01-01", // Date-only format

		"# Model features",
		"features:",
		"# Core capabilities",
		"tool_calls: true",
		"# Reasoning & Verbosity",
		"reasoning: true",
		"# Generation control support flags",
		"temperature: false",
		"# Response delivery",
		"format_response: false",

		"# Model limits",
		"limits:",

		"# Model pricing",
		"pricing:",
		"currency: USD",
		"per_1m: 1.25",
		"per_1m: 10.00",
		"cache_read:", // Flat structure, not cache.read
		"per_1m: 0.31",

		"# Timestamps",
		"created_at: 2025-08-22T04:09:45Z",
		"updated_at: 2025-08-22T04:09:45Z",
	}

	for _, element := range expectedElements {
		if !strings.Contains(yaml, element) {
			t.Errorf("YAML should contain: %s", element)
		}
	}
}
