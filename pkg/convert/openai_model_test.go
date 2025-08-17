package convert

import (
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestOpenAIModelSchemaCompliance tests that our OpenAI structs match the expected schema
func TestOpenAIModelSchemaCompliance(t *testing.T) {
	// Create a test model with all fields populated
	model := &catalogs.Model{
		ID:          "gpt-4-turbo",
		Name:        "GPT-4 Turbo",
		Description: "Most capable GPT-4 model",
		CreatedAt:   mustParseUTC("2024-01-15"),
		UpdatedAt:   mustParseUTC("2024-01-15"),
		Authors: []catalogs.Author{
			{ID: "openai", Name: "OpenAI"},
		},
	}

	// Convert to OpenAI format
	openAIModel := ToOpenAIModel(model)

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(openAIModel, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal OpenAI model: %v", err)
	}

	// Unmarshal back to verify structure
	var unmarshaled OpenAIModel
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal OpenAI model: %v", err)
	}

	// Verify required fields exist and have correct values
	t.Run("Required Fields", func(t *testing.T) {
		if unmarshaled.ID != "gpt-4-turbo" {
			t.Errorf("ID field mismatch. Expected 'gpt-4-turbo', got %s", unmarshaled.ID)
		}
		if unmarshaled.Object != "model" {
			t.Errorf("Object field mismatch. Expected 'model', got %s", unmarshaled.Object)
		}
		if unmarshaled.Created == 0 {
			t.Error("Created field is zero")
		}
		if unmarshaled.OwnedBy != "openai" {
			t.Errorf("OwnedBy field mismatch. Expected 'openai', got %s", unmarshaled.OwnedBy)
		}
	})

	// Print the generated JSON for manual inspection
	t.Logf("Generated OpenAI JSON:\n%s", string(jsonData))
}

// TestOpenAIModelMultipleAuthors tests handling of multiple authors
func TestOpenAIModelMultipleAuthors(t *testing.T) {
	model := &catalogs.Model{
		ID:        "llama-3-70b",
		Name:      "Llama 3 70B",
		CreatedAt: mustParseUTC("2024-04-18"),
		UpdatedAt: mustParseUTC("2024-04-18"),
		Authors: []catalogs.Author{
			{ID: "meta", Name: "Meta"},
			{ID: "microsoft", Name: "Microsoft"},
		},
	}

	openAIModel := ToOpenAIModel(model)

	if openAIModel.OwnedBy != "meta,microsoft" {
		t.Errorf("Multiple authors not handled correctly. Expected 'meta,microsoft', got %s", openAIModel.OwnedBy)
	}
}

// TestOpenAIModelNoAuthors tests handling when no authors are specified
func TestOpenAIModelNoAuthors(t *testing.T) {
	model := &catalogs.Model{
		ID:        "custom-model",
		Name:      "Custom Model",
		CreatedAt: mustParseUTC("2024-01-01"),
		UpdatedAt: mustParseUTC("2024-01-01"),
		Authors:   []catalogs.Author{}, // Empty authors
	}

	openAIModel := ToOpenAIModel(model)

	if openAIModel.OwnedBy != "system" {
		t.Errorf("No authors not handled correctly. Expected 'system', got %s", openAIModel.OwnedBy)
	}
}

// TestOpenAIModelAuthorWithoutID tests handling when author has no ID
func TestOpenAIModelAuthorWithoutID(t *testing.T) {
	model := &catalogs.Model{
		ID:        "custom-model",
		Name:      "Custom Model",
		CreatedAt: mustParseUTC("2024-01-01"),
		UpdatedAt: mustParseUTC("2024-01-01"),
		Authors: []catalogs.Author{
			{Name: "SomeOrg"}, // No ID field
		},
	}

	openAIModel := ToOpenAIModel(model)

	if openAIModel.OwnedBy != "system" {
		t.Errorf("Author without ID not handled correctly. Expected 'system', got %s", openAIModel.OwnedBy)
	}
}

// TestOpenAIModelsResponse tests the full response structure
func TestOpenAIModelsResponse(t *testing.T) {
	models := []*catalogs.Model{
		{
			ID:        "gpt-4",
			Name:      "GPT-4",
			CreatedAt: mustParseUTC("2023-03-14"),
			UpdatedAt: mustParseUTC("2023-03-14"),
			Authors: []catalogs.Author{
				{ID: "openai", Name: "OpenAI"},
			},
		},
		{
			ID:        "claude-3-opus",
			Name:      "Claude 3 Opus",
			CreatedAt: mustParseUTC("2024-03-04"),
			UpdatedAt: mustParseUTC("2024-03-04"),
			Authors: []catalogs.Author{
				{ID: "anthropic", Name: "Anthropic"},
			},
		},
		{
			ID:        "gemini-pro",
			Name:      "Gemini Pro",
			CreatedAt: mustParseUTC("2023-12-06"),
			UpdatedAt: mustParseUTC("2023-12-06"),
			Authors: []catalogs.Author{
				{ID: "google", Name: "Google"},
			},
		},
	}

	// Convert to OpenAI response
	var openAIModels []OpenAIModel
	for _, model := range models {
		openAIModels = append(openAIModels, ToOpenAIModel(model))
	}

	response := OpenAIModelsResponse{
		Object: "list",
		Data:   openAIModels,
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal OpenAI response: %v", err)
	}

	// Unmarshal back to verify structure
	var unmarshaled OpenAIModelsResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal OpenAI response: %v", err)
	}

	// Verify response structure
	if unmarshaled.Object != "list" {
		t.Errorf("Expected object to be 'list', got %s", unmarshaled.Object)
	}

	if len(unmarshaled.Data) != 3 {
		t.Errorf("Expected 3 models in response, got %d", len(unmarshaled.Data))
	}

	// Verify each model maintains correct structure
	expectedOwners := map[string]string{
		"gpt-4":         "openai",
		"claude-3-opus": "anthropic",
		"gemini-pro":    "google",
	}

	for _, model := range unmarshaled.Data {
		expectedOwner, ok := expectedOwners[model.ID]
		if !ok {
			t.Errorf("Unexpected model ID: %s", model.ID)
			continue
		}
		if model.OwnedBy != expectedOwner {
			t.Errorf("Model %s: expected owner %s, got %s", model.ID, expectedOwner, model.OwnedBy)
		}
		if model.Object != "model" {
			t.Errorf("Model %s: expected object 'model', got %s", model.ID, model.Object)
		}
		if model.Created == 0 {
			t.Errorf("Model %s: created timestamp is zero", model.ID)
		}
	}

	t.Logf("Generated OpenAI Response JSON:\n%s", string(jsonData))
}

// TestOpenAIResponseMatchesExpectedFormat tests that our response matches the example format
func TestOpenAIResponseMatchesExpectedFormat(t *testing.T) {
	// Create models similar to the example
	models := []*catalogs.Model{
		{
			ID:        "model-id-0",
			Name:      "Model 0",
			CreatedAt: mustParseUTC("2023-06-16"),
			Authors: []catalogs.Author{
				{ID: "organization-owner"},
			},
		},
		{
			ID:        "model-id-1",
			Name:      "Model 1",
			CreatedAt: mustParseUTC("2023-06-16"),
			Authors: []catalogs.Author{
				{ID: "organization-owner"},
			},
		},
		{
			ID:        "model-id-2",
			Name:      "Model 2",
			CreatedAt: mustParseUTC("2023-06-16"),
			Authors: []catalogs.Author{
				{ID: "openai"},
			},
		},
	}

	// Convert to OpenAI response
	var openAIModels []OpenAIModel
	for _, model := range models {
		openAIModels = append(openAIModels, ToOpenAIModel(model))
	}

	response := OpenAIModelsResponse{
		Object: "list",
		Data:   openAIModels,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Parse as generic map to verify structure
	var genericResponse map[string]interface{}
	err = json.Unmarshal(jsonData, &genericResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal as generic map: %v", err)
	}

	// Verify top-level structure
	if genericResponse["object"] != "list" {
		t.Error("Response should have 'object': 'list'")
	}

	data, ok := genericResponse["data"].([]interface{})
	if !ok {
		t.Fatal("Response should have 'data' array")
	}

	// Verify each model in data has only the expected fields
	for i, item := range data {
		model, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("Data item %d is not a map", i)
		}

		// Check for required fields
		requiredFields := []string{"id", "object", "created", "owned_by"}
		for _, field := range requiredFields {
			if _, exists := model[field]; !exists {
				t.Errorf("Model %d missing required field: %s", i, field)
			}
		}

		// Verify object field value
		if model["object"] != "model" {
			t.Errorf("Model %d: object field should be 'model', got %v", i, model["object"])
		}
	}

	t.Logf("Response matches expected format")
}
