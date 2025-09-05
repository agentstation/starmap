package anthropic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestAnthropicParsing(t *testing.T) {
	// Load testdata response
	response := loadTestdataResponse(t, "models_list.json")

	// Verify we can parse the raw response
	if len(response.Data) == 0 {
		t.Fatal("Expected models in testdata response, got none")
	}

	t.Logf("✅ Successfully parsed %d models from Anthropic API testdata", len(response.Data))

	// Check that all expected fields are present for each model
	for i, model := range response.Data {
		if model.ID == "" {
			t.Errorf("Model %d: missing ID", i)
		}
		if model.DisplayName == "" {
			t.Errorf("Model %d (%s): missing display_name", i, model.ID)
		}
		if model.Type == "" {
			t.Errorf("Model %d (%s): missing type", i, model.ID)
		}
		if model.CreatedAt.IsZero() {
			t.Errorf("Model %d (%s): missing or invalid created_at", i, model.ID)
		}

		t.Logf("  • %s: %s (%s, created: %s)", model.ID, model.DisplayName, model.Type, model.CreatedAt.Format("2006-01-02"))
	}
}

func TestAnthropicModelConversion(t *testing.T) {
	// Load testdata and convert to starmap models
	response := loadTestdataResponse(t, "models_list.json")

	// Create a client to test conversion
	client := &Client{}

	// Convert each model and verify the conversion
	for _, apiModel := range response.Data {
		starmapModel := client.convertToModel(apiModel)

		// Verify basic mapping
		if starmapModel.ID != apiModel.ID {
			t.Errorf("Model %s: ID mismatch, got %s", apiModel.ID, starmapModel.ID)
		}
		if starmapModel.Name != apiModel.DisplayName {
			t.Errorf("Model %s: Name mismatch, expected %s, got %s", apiModel.ID, apiModel.DisplayName, starmapModel.Name)
		}

		// Verify author is set to Anthropic
		if len(starmapModel.Authors) == 0 {
			t.Errorf("Model %s: missing authors", apiModel.ID)
		} else if starmapModel.Authors[0].ID != catalogs.AuthorIDAnthropic {
			t.Errorf("Model %s: expected Anthropic author, got %s", apiModel.ID, starmapModel.Authors[0].ID)
		}

		// Verify features are inferred
		if starmapModel.Features == nil {
			t.Errorf("Model %s: missing features", apiModel.ID)
		} else {
			// All Claude models should support basic text input/output
			if len(starmapModel.Features.Modalities.Input) == 0 {
				t.Errorf("Model %s: missing input modalities", apiModel.ID)
			}
			if len(starmapModel.Features.Modalities.Output) == 0 {
				t.Errorf("Model %s: missing output modalities", apiModel.ID)
			}
		}

		t.Logf("✅ Model %s converted successfully: %s", starmapModel.ID, starmapModel.Name)
	}
}

func TestAnthropicAPIFormatChanges(t *testing.T) {
	// This test helps detect if Anthropic changes their API format
	response := loadTestdataResponse(t, "models_list.json")

	// Check for expected structure
	if response.Data == nil {
		t.Fatal("Expected 'data' field in response")
	}

	// Verify we have recent Claude models
	foundOpus4 := false
	foundSonnet3_7 := false

	for _, model := range response.Data {
		switch {
		case model.ID == "claude-opus-4-1-20250805":
			foundOpus4 = true
			if model.DisplayName != "Claude Opus 4.1" {
				t.Errorf("Unexpected display name for Opus 4.1: %s", model.DisplayName)
			}
		case model.ID == "claude-3-7-sonnet-20250219":
			foundSonnet3_7 = true
			if model.DisplayName != "Claude Sonnet 3.7" {
				t.Errorf("Unexpected display name for Sonnet 3.7: %s", model.DisplayName)
			}
		}
	}

	if !foundOpus4 {
		t.Error("Expected to find Claude Opus 4.1 in API response")
	}
	if !foundSonnet3_7 {
		t.Error("Expected to find Claude Sonnet 3.7 in API response")
	}

	t.Logf("✅ Found expected Claude models: Opus 4.1=%v, Sonnet 3.7=%v", foundOpus4, foundSonnet3_7)
}

// Helper function to load testdata response.
func loadTestdataResponse(t *testing.T, filename string) modelsResponse {
	testdataPath := filepath.Join("testdata", filename)
	data, err := os.ReadFile(testdataPath)
	if err != nil {
		t.Fatalf("Failed to read testdata file %s: %v", testdataPath, err)
	}

	var response modelsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("Failed to parse testdata JSON from %s: %v", testdataPath, err)
	}

	return response
}
