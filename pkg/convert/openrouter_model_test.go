package convert

import (
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/utc"
)

// TestOpenRouterModelSchemaCompliance tests that our OpenRouter structs match the expected schema
func TestOpenRouterModelSchemaCompliance(t *testing.T) {
	// Create a test model with all fields populated
	model := &catalogs.Model{
		ID:          "test/model-1",
		Name:        "Test Model 1",
		Description: "A test model for schema compliance",
		CreatedAt:   mustParseUTC("2024-01-15"),
		UpdatedAt:   mustParseUTC("2024-01-15"),
		Metadata: &catalogs.ModelMetadata{
			ReleaseDate: mustParseUTC("2024-01-15"),
			OpenWeights: true,
			Architecture: &catalogs.ModelArchitecture{
				ParameterCount: "7B",
				Type:           catalogs.ArchitectureTypeTransformer,
				Tokenizer:      catalogs.TokenizerGPT,
				Quantization:   catalogs.QuantizationFP16,
			},
		},
		Features: &catalogs.ModelFeatures{
			Modalities: catalogs.ModelModalities{
				Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage},
				Output: []catalogs.ModelModality{catalogs.ModelModalityText},
			},
			Temperature:       true,
			TopP:              true,
			MaxTokens:         true,
			Tools:             true,
			ToolChoice:        true,
			Reasoning:         true,
			IncludeReasoning:  true,
			StructuredOutputs: true,
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 128000,
			OutputTokens:  16384,
		},
		Pricing: &catalogs.ModelPricing{
			Currency: "USD",
			Tokens: &catalogs.TokenPricing{
				Input: &catalogs.TokenCost{
					PerToken: 0.0000007,
					Per1M:    0.7,
				},
				Output: &catalogs.TokenCost{
					PerToken: 0.0000007,
					Per1M:    0.7,
				},
			},
			Operations: &catalogs.OperationPricing{
				Request: ptrFloat64(0),
			},
		},
	}

	// Convert to OpenRouter format
	openRouterModel := ToOpenRouterModel(model)

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(openRouterModel, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal OpenRouter model: %v", err)
	}

	// Unmarshal back to verify structure
	var unmarshaled OpenRouterModel
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal OpenRouter model: %v", err)
	}

	// Verify required fields exist and have correct types
	t.Run("Required Fields", func(t *testing.T) {
		if unmarshaled.ID == "" {
			t.Error("ID field is empty")
		}
		if unmarshaled.Name == "" {
			t.Error("Name field is empty")
		}
		if unmarshaled.Created == 0 {
			t.Error("Created field is zero")
		}
		if unmarshaled.ContextLength == 0 {
			t.Error("ContextLength field is zero")
		}
	})

	t.Run("Architecture Fields", func(t *testing.T) {
		arch := unmarshaled.Architecture
		if len(arch.InputModalities) == 0 {
			t.Error("InputModalities is empty")
		}
		if len(arch.OutputModalities) == 0 {
			t.Error("OutputModalities is empty")
		}
		if arch.Tokenizer == "" {
			t.Error("Tokenizer is empty")
		}

		// Verify modalities contain expected values
		expectedInput := []string{"text", "image"}
		expectedOutput := []string{"text"}

		if !equalStringSlices(arch.InputModalities, expectedInput) {
			t.Errorf("InputModalities mismatch. Expected %v, got %v", expectedInput, arch.InputModalities)
		}
		if !equalStringSlices(arch.OutputModalities, expectedOutput) {
			t.Errorf("OutputModalities mismatch. Expected %v, got %v", expectedOutput, arch.OutputModalities)
		}
	})

	t.Run("TopProvider Fields", func(t *testing.T) {
		tp := unmarshaled.TopProvider
		if tp.ContextLength == 0 {
			t.Error("TopProvider.ContextLength is zero")
		}
		if tp.MaxCompletionTokens == 0 {
			t.Error("TopProvider.MaxCompletionTokens is zero")
		}
		// IsModerated can be false, so we don't check it
	})

	t.Run("Pricing Fields", func(t *testing.T) {
		pricing := unmarshaled.Pricing
		if pricing.Prompt == "" {
			t.Error("Pricing.Prompt is empty")
		}
		if pricing.Completion == "" {
			t.Error("Pricing.Completion is empty")
		}
		if pricing.Request == "" {
			t.Error("Pricing.Request is empty")
		}
		if pricing.Image == "" {
			t.Error("Pricing.Image is empty")
		}
		if pricing.WebSearch == "" {
			t.Error("Pricing.WebSearch is empty")
		}
		if pricing.InternalReasoning == "" {
			t.Error("Pricing.InternalReasoning is empty")
		}
		if pricing.InputCacheRead == "" {
			t.Error("Pricing.InputCacheRead is empty")
		}
		if pricing.InputCacheWrite == "" {
			t.Error("Pricing.InputCacheWrite is empty")
		}
	})

	t.Run("SupportedParameters", func(t *testing.T) {
		if len(unmarshaled.SupportedParameters) == 0 {
			t.Error("SupportedParameters is empty")
		}

		// Verify some expected parameters are present
		expectedParams := []string{"temperature", "top_p", "max_tokens", "tools", "tool_choice", "reasoning"}
		for _, param := range expectedParams {
			if !containsString(unmarshaled.SupportedParameters, param) {
				t.Errorf("Expected parameter %s not found in SupportedParameters", param)
			}
		}
	})

	// Print the generated JSON for manual inspection
	t.Logf("Generated OpenRouter JSON:\n%s", string(jsonData))
}

// TestOpenRouterModelsResponse tests the full response structure
func TestOpenRouterModelsResponse(t *testing.T) {
	models := []*catalogs.Model{
		{
			ID:          "test/model-1",
			Name:        "Test Model 1",
			Description: "First test model",
			CreatedAt:   mustParseUTC("2024-01-15"),
			UpdatedAt:   mustParseUTC("2024-01-15"),
			Features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityText},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
				Temperature: true,
				MaxTokens:   true,
			},
			Limits: &catalogs.ModelLimits{
				ContextWindow: 128000,
				OutputTokens:  4096,
			},
		},
		{
			ID:          "test/model-2",
			Name:        "Test Model 2",
			Description: "Second test model",
			CreatedAt:   mustParseUTC("2024-01-16"),
			UpdatedAt:   mustParseUTC("2024-01-16"),
			Features: &catalogs.ModelFeatures{
				Modalities: catalogs.ModelModalities{
					Input:  []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage},
					Output: []catalogs.ModelModality{catalogs.ModelModalityText},
				},
				Temperature:       true,
				TopP:              true,
				MaxTokens:         true,
				Tools:             true,
				StructuredOutputs: true,
			},
			Limits: &catalogs.ModelLimits{
				ContextWindow: 256000,
				OutputTokens:  8192,
			},
		},
	}

	// Convert to OpenRouter response
	var openRouterModels []OpenRouterModel
	for _, model := range models {
		openRouterModels = append(openRouterModels, ToOpenRouterModel(model))
	}

	response := OpenRouterModelsResponse{
		Data: openRouterModels,
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal OpenRouter response: %v", err)
	}

	// Unmarshal back to verify structure
	var unmarshaled OpenRouterModelsResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal OpenRouter response: %v", err)
	}

	// Verify response structure
	if len(unmarshaled.Data) != 2 {
		t.Errorf("Expected 2 models in response, got %d", len(unmarshaled.Data))
	}

	if len(unmarshaled.Data) > 0 {
		firstModel := unmarshaled.Data[0]
		if firstModel.ID != "test/model-1" {
			t.Errorf("Expected first model ID to be 'test/model-1', got %s", firstModel.ID)
		}
	}

	t.Logf("Generated OpenRouter Response JSON:\n%s", string(jsonData))
}

// Helper functions
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func ptrFloat64(f float64) *float64 {
	return &f
}

// mustParseUTC is a helper that parses a date string and panics on error (for tests only)
func mustParseUTC(s string) utc.Time {
	t, err := utc.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
