package groq

import (
	"context"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/agentstation/starmap/internal/sources/providers/testhelper"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestMain handles flag parsing for the -update flag.
func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

// loadTestdataResponse loads a Groq API response from testdata.
func loadTestdataResponse(t *testing.T, filename string) Response {
	t.Helper()
	var response Response
	testhelper.LoadJSON(t, filename, &response)
	return response
}

// loadTestdataModel loads a single Groq model from testdata by finding it in the models list.
func loadTestdataModel(t *testing.T, modelID string) ModelData {
	t.Helper()
	response := loadTestdataResponse(t, "models_list.json")

	for _, model := range response.Data {
		if model.ID == modelID {
			return model
		}
	}

	t.Fatalf("Model %s not found in testdata", modelID)
	return ModelData{}
}

// TestModelDataParsing tests that we can properly parse Groq API responses with provider-specific fields.
func TestModelDataParsing(t *testing.T) {
	// Test parsing the models list response
	response := loadTestdataResponse(t, "models_list.json")

	// Verify response structure
	if response.Object != "list" {
		t.Errorf("Expected object 'list', got '%s'", response.Object)
	}

	if len(response.Data) == 0 {
		t.Error("Expected at least some models, got 0")
	}

	// Test specific model data - use models that actually exist in testdata
	// The Groq API now includes max_completion_tokens in the list response
	tests := []struct {
		name             string
		expectedID       string
		expectedOwnedBy  string
		expectedActive   bool
		hasContextWindow bool
		hasMaxCompTokens bool
	}{
		{
			name:             "Llama instant model",
			expectedID:       "llama-3.1-8b-instant",
			expectedOwnedBy:  "Meta",
			expectedActive:   true,
			hasContextWindow: true,
			hasMaxCompTokens: true,
		},
		{
			name:             "Whisper audio model",
			expectedID:       "whisper-large-v3",
			expectedOwnedBy:  "OpenAI",
			expectedActive:   true,
			hasContextWindow: true,
			hasMaxCompTokens: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find the model by ID
			var model ModelData
			found := false
			for _, m := range response.Data {
				if m.ID == tt.expectedID {
					model = m
					found = true
					break
				}
			}

			if !found {
				t.Fatalf("Model '%s' not found in response", tt.expectedID)
			}

			if model.ID != tt.expectedID {
				t.Errorf("Expected ID '%s', got '%s'", tt.expectedID, model.ID)
			}

			if model.OwnedBy != tt.expectedOwnedBy {
				t.Errorf("Expected OwnedBy '%s', got '%s'", tt.expectedOwnedBy, model.OwnedBy)
			}

			if model.Active != tt.expectedActive {
				t.Errorf("Expected Active %v, got %v", tt.expectedActive, model.Active)
			}

			if tt.hasContextWindow && model.ContextWindow == 0 {
				t.Error("Expected ContextWindow to be non-zero")
			}

			if tt.hasMaxCompTokens && model.MaxCompletionTokens == 0 {
				t.Error("Expected MaxCompletionTokens to be non-zero")
			}
		})
	}
}

// TestGroqSingleStepCompleteness tests that the models list endpoint now provides complete data.
// This verifies that we no longer need the two-step process since Groq API evolved.
func TestGroqSingleStepCompleteness(t *testing.T) {
	// Load the models list response
	response := loadTestdataResponse(t, "models_list.json")

	// Test key models to ensure they have complete data in the list response
	testModels := []string{
		"llama-3.1-8b-instant",
		"meta-llama/llama-guard-4-12b",
		"whisper-large-v3",
	}

	foundModels := make(map[string]ModelData)
	for _, model := range response.Data {
		foundModels[model.ID] = model
	}

	for _, modelID := range testModels {
		t.Run(modelID, func(t *testing.T) {
			model, found := foundModels[modelID]
			if !found {
				// If model not found in current testdata, skip test
				t.Skipf("Model %s not found in current testdata", modelID)
				return
			}

			// Verify core fields are present
			if model.ID == "" {
				t.Error("ID should not be empty")
			}

			if model.Object != "model" {
				t.Errorf("Expected Object 'model', got '%s'", model.Object)
			}

			if model.Created == 0 {
				t.Error("Created should not be zero")
			}

			if model.OwnedBy == "" {
				t.Error("OwnedBy should not be empty")
			}

			// Verify Groq-specific fields are present in list response
			if model.ContextWindow == 0 {
				t.Error("ContextWindow should be non-zero in list response")
			}

			if model.MaxCompletionTokens == 0 {
				t.Error("MaxCompletionTokens should be non-zero in list response")
			}

			t.Logf("✅ Model %s has complete data: context=%d, max_tokens=%d",
				model.ID, model.ContextWindow, model.MaxCompletionTokens)
		})
	}
}

// TestGroqAPIEvolution tests whether the Groq API has evolved to include all necessary data in the list endpoint.
// This test helps us understand if the two-step process is still necessary.
func TestGroqAPIEvolution(t *testing.T) {
	response := loadTestdataResponse(t, "models_list.json")

	// Check if the models list now includes max_completion_tokens for all models
	hasAllMaxTokens := true
	modelsWithoutMaxTokens := []string{}

	for _, model := range response.Data {
		if model.MaxCompletionTokens == 0 {
			hasAllMaxTokens = false
			modelsWithoutMaxTokens = append(modelsWithoutMaxTokens, model.ID)
		}
	}

	if hasAllMaxTokens {
		t.Logf("✅ Groq API evolution detected: All %d models in list response have max_completion_tokens", len(response.Data))
		t.Logf("💡 The two-step process may no longer be necessary for getting complete model data")
	} else {
		t.Logf("⚠️  Two-step process still needed: %d models missing max_completion_tokens in list response", len(modelsWithoutMaxTokens))
		if len(modelsWithoutMaxTokens) < 10 { // Only log if not too many
			t.Logf("Models without max_completion_tokens: %v", modelsWithoutMaxTokens)
		}
	}

	// Verify that we can get complete information from both endpoints
	t.Run("List endpoint completeness", func(t *testing.T) {
		for _, model := range response.Data {
			// Essential fields that should always be present
			if model.ID == "" {
				t.Errorf("Model missing ID")
			}
			if model.ContextWindow == 0 {
				t.Errorf("Model %s missing ContextWindow in list response", model.ID)
			}
			// Note: We don't fail on missing MaxCompletionTokens since API might be evolving
		}
	})

	// Since we no longer use the two-step process, we just verify the list endpoint provides complete data
	t.Run("List endpoint provides complete data", func(t *testing.T) {
		// Verify that all models in the list have the essential fields
		for _, model := range response.Data {
			if model.ID == "" {
				t.Errorf("Model missing ID")
			}
			if model.ContextWindow == 0 {
				t.Errorf("Model %s missing ContextWindow in list response", model.ID)
			}
			if model.MaxCompletionTokens == 0 {
				t.Errorf("Model %s missing MaxCompletionTokens in list response", model.ID)
			}
		}
	})
}

// TestGroqSingleModelParsing tests parsing of a single model response.
func TestGroqSingleModelParsing(t *testing.T) {
	model := loadTestdataModel(t, "llama-3.1-8b-instant")

	// Verify all fields are correctly parsed
	if model.ID != "llama-3.1-8b-instant" {
		t.Errorf("Expected ID 'llama-3.1-8b-instant', got '%s'", model.ID)
	}

	if model.Object != "model" {
		t.Errorf("Expected Object 'model', got '%s'", model.Object)
	}

	if model.Created == 0 {
		t.Error("Expected Created to be non-zero")
	}

	if model.OwnedBy != "Meta" {
		t.Errorf("Expected OwnedBy 'Meta', got '%s'", model.OwnedBy)
	}

	if !model.Active {
		t.Error("Expected Active to be true")
	}

	if model.ContextWindow == 0 {
		t.Error("Expected ContextWindow to be non-zero")
	}

	if model.MaxCompletionTokens == 0 {
		t.Error("Expected MaxCompletionTokens to be non-zero")
	}
}

// TestConvertToGroqModel tests the conversion from ModelData to catalogs.Model.
func TestConvertToGroqModel(t *testing.T) {
	// Create a mock Groq client
	client := &Client{
		OpenAIClient: nil, // We'll test the conversion method directly
	}

	// Test data
	groqModel := ModelData{
		OpenAIModelData: struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		}{
			ID:      "llama-3.1-8b-instant",
			Object:  "model",
			Created: 1693721698,
			OwnedBy: "Meta",
		},
		Active:              true,
		ContextWindow:       131072,
		MaxCompletionTokens: 8192,
		PublicApps:          nil,
	}

	// Convert to starmap model
	model := client.ConvertToGroqModel(groqModel)

	// Verify basic fields
	if model.ID != "llama-3.1-8b-instant" {
		t.Errorf("Expected ID 'llama-3.1-8b-instant', got '%s'", model.ID)
	}

	if model.Name != "llama-3.1-8b-instant" { // Should be formatted by formatModelName
		t.Errorf("Expected Name 'llama-3.1-8b-instant', got '%s'", model.Name)
	}

	// Verify limits were properly set
	if model.Limits == nil {
		t.Fatal("Expected Limits to be set")
	}

	if model.Limits.ContextWindow != 131072 {
		t.Errorf("Expected ContextWindow 131072, got %d", model.Limits.ContextWindow)
	}

	if model.Limits.OutputTokens != 8192 {
		t.Errorf("Expected OutputTokens 8192, got %d", model.Limits.OutputTokens)
	}

	// Verify features were set
	if model.Features == nil {
		t.Fatal("Expected Features to be set")
	}

	// Should have basic chat features for Llama models
	if !model.Features.Tools {
		t.Error("Expected Tools to be true for Llama model")
	}

	if !model.Features.ToolChoice {
		t.Error("Expected ToolChoice to be true for Llama model")
	}
}

// TestGroqClientListModels tests the single-step ListModels functionality.
func TestGroqClientListModels(t *testing.T) {
	// Set a test API key to ensure the test works without environment variables
	t.Setenv("GROQ_API_KEY", "test-api-key")
	
	requestCount := 0
	// Create a mock HTTP server that only handles the models list endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Verify the request
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// Check for authorization header
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Error("Expected Authorization header")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Should only hit the models list endpoint
		switch {
		case r.URL.Path == "/openai/v1/models" || r.URL.Path == "/":
			// Return list response with all necessary fields including max_completion_tokens
			w.Write(testhelper.LoadTestdata(t, "models_list.json"))
			return

		default:
			t.Errorf("Unexpected path: %s - ListModels should only call the models list endpoint", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}))
	defer server.Close()

	// Create a Groq client with mock provider
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDGroq,
		Name: "Groq",
		APIKey: &catalogs.ProviderAPIKey{
			Name:   "GROQ_API_KEY",
			Header: "Authorization",
			Scheme: catalogs.ProviderAPIKeySchemeBearer,
		},
		Catalog: &catalogs.ProviderCatalog{
			APIURL: &server.URL,
		},
	}

	client := &Client{}
	client.Configure(provider)

	// Test single-step ListModels
	ctx := context.Background()
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	// Verify we got a reasonable number of models
	if len(models) == 0 {
		t.Error("Expected at least some models, got 0")
	}

	// Verify we made exactly 1 request (single-step process)
	expectedRequests := 1 // Only 1 list request needed now
	if requestCount != expectedRequests {
		t.Errorf("Expected %d total request (single list call), got %d", expectedRequests, requestCount)
	}

	// Find a specific model to test - use llama-3.1-8b-instant
	var testModel *catalogs.Model
	for _, model := range models {
		if model.ID == "llama-3.1-8b-instant" {
			testModel = &model
			break
		}
	}

	if testModel == nil {
		t.Fatal("Could not find llama-3.1-8b-instant model in response")
	}

	if testModel.Limits == nil {
		t.Fatal("Expected test model to have Limits")
	}

	if testModel.Limits.ContextWindow == 0 {
		t.Error("Expected test model ContextWindow to be non-zero")
	}

	if testModel.Limits.OutputTokens == 0 {
		t.Error("Expected test model OutputTokens to be non-zero")
	}

	// Test Guard model - should have lower token count
	var guardModel *catalogs.Model
	for _, model := range models {
		if model.ID == "meta-llama/llama-guard-4-12b" {
			guardModel = &model
			break
		}
	}

	if guardModel != nil && guardModel.Limits != nil {
		// Guard models typically have lower output limits
		if guardModel.Limits.OutputTokens == 0 {
			t.Error("Expected guard model OutputTokens to be non-zero")
		}
	}
}

// TestGroqPerformanceImprovement verifies that ListModels is now single-step.
func TestGroqPerformanceImprovement(t *testing.T) {
	callLog := []string{}

	// Create a mock HTTP server that logs all requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callLog = append(callLog, r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Only respond to the models list endpoint
		if r.URL.Path == "/openai/v1/models" || r.URL.Path == "/" {
			w.Write(testhelper.LoadTestdata(t, "models_list.json"))
		} else {
			t.Errorf("Unexpected API call to %s - single-step should only call models list", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Configure client
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDGroq,
		Name: "Groq",
		APIKey: &catalogs.ProviderAPIKey{
			Name:   "GROQ_API_KEY",
			Header: "Authorization",
			Scheme: catalogs.ProviderAPIKeySchemeBearer,
		},
		Catalog: &catalogs.ProviderCatalog{
			APIURL: &server.URL,
		},
	}

	client := &Client{}
	client.Configure(provider)

	// Measure performance improvement
	ctx := context.Background()
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	// Verify we got models
	if len(models) == 0 {
		t.Error("Expected at least some models")
	}

	// Performance verification: exactly 1 API call for any number of models
	if len(callLog) != 1 {
		t.Errorf("Expected exactly 1 API call, got %d calls: %v", len(callLog), callLog)
	}

	if len(callLog) > 0 && callLog[0] != "/openai/v1/models" && callLog[0] != "/" {
		t.Errorf("Expected call to /openai/v1/models or /, got %s", callLog[0])
	}

	// Verify we still get complete model data (context_window + max_completion_tokens)
	for _, model := range models {
		if model.Limits != nil {
			if model.Limits.ContextWindow == 0 {
				t.Errorf("Model %s missing ContextWindow despite single-step fetch", model.ID)
			}
			if model.Limits.OutputTokens == 0 {
				t.Errorf("Model %s missing OutputTokens despite single-step fetch", model.ID)
			}
		}
	}

	t.Logf("✅ Performance improvement verified: 1 API call for %d models (previously would have been %d calls)",
		len(models), len(models)+1)
}

// TestFormatModelName tests the model name formatting logic.
func TestFormatModelName(t *testing.T) {
	client := &Client{}

	tests := []struct {
		input    string
		expected string
	}{
		{"llama-3.1-8b-instant", "llama-3.1-8b-instant"},
		{"meta-llama/llama-guard-4-12b", "llama-guard-4-12b"},
		{"openai/gpt-oss-120b", "gpt-oss-120b"},
		{"qwen/qwen3-32b", "qwen3-32b"},
		{"compound-beta", "compound-beta"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := client.formatModelName(tt.input)
			if result != tt.expected {
				t.Errorf("formatModelName(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestInferFeatures tests the Groq-specific feature inference.
func TestInferFeatures(t *testing.T) {
	client := &Client{}

	tests := []struct {
		modelID      string
		expectTools  bool
		expectVision bool
		expectAudio  bool
	}{
		{"llama-3.1-8b-instant", true, false, false},
		{"meta-llama/llama-guard-4-12b", false, false, false}, // Guard models don't have tools
		{"mixtral-8x7b-32768", true, false, false},
		{"gemma2-9b-it", true, false, false},
		{"llava-v1.5-7b-4096-preview", false, true, false},
		{"whisper-large-v3", false, false, true},
		{"compound-beta", false, false, false}, // Unknown model gets defaults
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			features := client.inferFeatures(tt.modelID)

			if features.Tools != tt.expectTools {
				t.Errorf("Model %s: expected Tools=%v, got %v", tt.modelID, tt.expectTools, features.Tools)
			}

			if tt.expectVision {
				hasImageInput := false
				for _, modality := range features.Modalities.Input {
					if modality == catalogs.ModelModalityImage {
						hasImageInput = true
						break
					}
				}
				if !hasImageInput {
					t.Errorf("Model %s: expected image input modality", tt.modelID)
				}
			}

			if tt.expectAudio {
				hasAudioInput := false
				for _, modality := range features.Modalities.Input {
					if modality == catalogs.ModelModalityAudio {
						hasAudioInput = true
						break
					}
				}
				if !hasAudioInput {
					t.Errorf("Model %s: expected audio input modality", tt.modelID)
				}
			}

			// All Groq models should support seed parameter
			if !features.Seed {
				t.Errorf("Model %s: expected Seed support", tt.modelID)
			}
		})
	}
}
