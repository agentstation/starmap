package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

func TestSchemaDriftMutationMatrix(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		wantErr     bool
		wantModels  int
		wantUnknown int
	}{
		{name: "valid", payload: `{"data":[{"id":"model-a","display_name":"Model A"}]}`, wantModels: 1},
		{name: "missing", payload: `{}`, wantErr: true},
		{name: "renamed", payload: `{"models":[]}`, wantErr: true},
		{name: "null", payload: `{"data":null}`, wantErr: true},
		{name: "wrong type", payload: `{"data":{}}`, wantErr: true},
		{name: "unknown additive", payload: `{"data":[{"id":"model-a","display_name":"Model A","new_capability":true}],"new_page":1}`, wantModels: 1, wantUnknown: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(test.payload))
			}))
			defer server.Close()
			client := NewClient(&catalogs.Provider{
				ID: catalogs.ProviderIDAnthropic, Name: "Anthropic",
				Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeAnthropic, URL: server.URL}},
			})
			models, err := client.ListModels(context.Background())
			if test.wantErr && err == nil {
				t.Fatal("ListModels returned nil error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("ListModels: %v", err)
			}
			if len(models) != test.wantModels {
				t.Fatalf("models = %d, want %d", len(models), test.wantModels)
			}
			if test.wantUnknown > 0 {
				items := models[0].Extensions["anthropic"].Fields["unknown_fields"].([]sourcepayload.UnknownJSONField)
				if len(items) != test.wantUnknown {
					t.Fatalf("unknown evidence = %#v", items)
				}
			}
		})
	}
}

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

func TestAnthropicModelConversionPreservesCapabilityFields(t *testing.T) {
	client := &Client{
		provider: &catalogs.Provider{ID: catalogs.ProviderIDAnthropic, Name: "Anthropic"},
	}
	createdAt := mustParseAnthropicTime(t, "2026-01-15T00:00:00Z")

	model := client.convertToModel(modelResponse{
		Type:           "model",
		ID:             "claude-sonnet-4-5",
		DisplayName:    "Claude Sonnet 4.5",
		CreatedAt:      createdAt,
		MaxTokens:      64000,
		MaxInputTokens: 200000,
		Capabilities: &modelCapabilities{
			Batch:         supportedCapability{Supported: true},
			Citations:     supportedCapability{Supported: true},
			CodeExecution: supportedCapability{Supported: true},
			ContextManagement: contextManagementCapability{
				Supported:             true,
				ClearToolUses20250919: supportedCapability{Supported: true},
				ClearThinking20251015: supportedCapability{Supported: true},
				Compact20260112:       supportedCapability{Supported: true},
			},
			Effort: effortCapability{
				Supported: true,
				Low:       supportedCapability{Supported: true},
				Medium:    supportedCapability{Supported: true},
				High:      supportedCapability{Supported: true},
				Max:       supportedCapability{Supported: true},
			},
			ImageInput:        supportedCapability{Supported: true},
			PDFInput:          supportedCapability{Supported: true},
			StructuredOutputs: supportedCapability{Supported: true},
			Thinking: thinkingCapability{
				Supported: true,
				Types: thinkingTypeCapabilities{
					Adaptive: supportedCapability{Supported: true},
					Enabled:  supportedCapability{Supported: true},
				},
			},
		},
	})

	if model.Limits == nil ||
		model.Limits.ContextWindow != 200000 ||
		model.Limits.InputTokens != 200000 ||
		model.Limits.OutputTokens != 64000 {
		t.Fatalf("limits = %#v", model.Limits)
	}
	if model.Features == nil ||
		!containsAnthropicTestModality(model.Features.Modalities.Input, catalogs.ModelModalityImage) ||
		!containsAnthropicTestModality(model.Features.Modalities.Input, catalogs.ModelModalityPDF) ||
		!model.Features.Attachments ||
		!model.Features.StructuredOutputs ||
		!model.Features.FormatResponse ||
		!model.Features.Reasoning ||
		!model.Features.IncludeReasoning ||
		!model.Features.ReasoningEffort {
		t.Fatalf("features = %#v", model.Features)
	}
	if model.Reasoning == nil || len(model.Reasoning.Levels) != 4 {
		t.Fatalf("reasoning levels = %#v", model.Reasoning)
	}
	extension := model.Extensions["anthropic"].Fields
	if extension["batch"] != true ||
		extension["citations"] != true ||
		extension["code_execution"] != true {
		t.Fatalf("extension = %#v", extension)
	}
	contextManagement := extension["context_management"].(map[string]any)
	if contextManagement["clear_tool_uses_20250919"] != true ||
		contextManagement["clear_thinking_20251015"] != true ||
		contextManagement["compact_20260112"] != true {
		t.Fatalf("context management extension = %#v", contextManagement)
	}
	thinkingTypes := extension["thinking_types"].(map[string]any)
	if thinkingTypes["adaptive"] != true || thinkingTypes["enabled"] != true {
		t.Fatalf("thinking types extension = %#v", thinkingTypes)
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

func containsAnthropicTestModality(modalities []catalogs.ModelModality, want catalogs.ModelModality) bool {
	for _, modality := range modalities {
		if modality == want {
			return true
		}
	}
	return false
}

func mustParseAnthropicTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed
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
