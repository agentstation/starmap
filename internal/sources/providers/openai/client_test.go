package openai

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/agentstation/starmap/internal/sources/providers/baseclient"
	"github.com/agentstation/starmap/internal/sources/providers/testhelper"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestMain handles flag parsing for the -update flag
func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

// loadTestdataResponse loads an OpenAI API response from testdata
func loadTestdataResponse(t *testing.T, filename string) baseclient.OpenAIResponse {
	t.Helper()
	var response baseclient.OpenAIResponse
	testhelper.LoadJSON(t, filename, &response)
	return response
}

// loadTestdataModel loads a single OpenAI model from testdata by finding it in the models list
func loadTestdataModel(t *testing.T, modelID string) baseclient.OpenAIModelData {
	t.Helper()
	response := loadTestdataResponse(t, "models_list.json")

	for _, model := range response.Data {
		if model.ID == modelID {
			return model
		}
	}

	t.Fatalf("Model %s not found in testdata", modelID)
	return baseclient.OpenAIModelData{}
}

// TestOpenAIModelDataParsing tests that we can properly parse OpenAI API responses.
func TestOpenAIModelDataParsing(t *testing.T) {
	// Test parsing the models list response
	response := loadTestdataResponse(t, "models_list.json")

	// Verify response structure
	if response.Object != "list" {
		t.Errorf("Expected object 'list', got '%s'", response.Object)
	}

	if len(response.Data) == 0 {
		t.Error("Expected at least some models, got 0")
	}

	// Test specific model data - use models that should exist in real OpenAI API
	// These tests verify we can parse the actual API format
	tests := []struct {
		name            string
		expectedID      string
		expectedOwnedBy string
		hasCreated      bool
	}{
		{
			name:            "GPT-4o model",
			expectedID:      "gpt-4o",
			expectedOwnedBy: "system",
			hasCreated:      true,
		},
		{
			name:            "GPT-3.5 Turbo model",
			expectedID:      "gpt-3.5-turbo",
			expectedOwnedBy: "openai",
			hasCreated:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find the model by ID
			var model baseclient.OpenAIModelData
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

			if tt.hasCreated && model.Created == 0 {
				t.Error("Expected Created to be non-zero")
			}

			if model.Object != "model" {
				t.Errorf("Expected Object 'model', got '%s'", model.Object)
			}
		})
	}
}

// TestOpenAISingleModelParsing tests parsing of a single model response.
func TestOpenAISingleModelParsing(t *testing.T) {
	model := loadTestdataModel(t, "gpt-4o")

	// Verify all fields are correctly parsed
	if model.ID != "gpt-4o" {
		t.Errorf("Expected ID 'gpt-4o', got '%s'", model.ID)
	}

	if model.Object != "model" {
		t.Errorf("Expected Object 'model', got '%s'", model.Object)
	}

	if model.Created == 0 {
		t.Error("Expected Created to be non-zero")
	}

	if model.OwnedBy != "system" {
		t.Errorf("Expected OwnedBy 'system', got '%s'", model.OwnedBy)
	}
}

// TestConvertToOpenAIModel tests the conversion from OpenAIModelData to catalogs.Model.
func TestConvertToOpenAIModel(t *testing.T) {
	// Create a mock OpenAI client
	client := &Client{
		OpenAIClient: nil, // We'll test the conversion method directly
	}

	// Test data based on actual OpenAI API response format
	openaiModel := baseclient.OpenAIModelData{
		ID:      "gpt-4o",
		Object:  "model",
		Created: 1715367049,
		OwnedBy: "system",
	}

	// Convert to starmap model
	model := client.ConvertToModel(openaiModel)

	// Verify basic fields
	if model.ID != "gpt-4o" {
		t.Errorf("Expected ID 'gpt-4o', got '%s'", model.ID)
	}

	if model.Name != "gpt-4o" { // OpenAI uses ID as display name
		t.Errorf("Expected Name 'gpt-4o', got '%s'", model.Name)
	}

	// OpenAI models should have some basic features inferred
	if model.Features == nil {
		t.Fatal("Expected Features to be set")
	}

	// GPT-4o should support tools
	if !model.Features.Tools {
		t.Error("Expected Tools to be true for GPT-4o")
	}

	// GPT-4o should have basic text modalities
	if len(model.Features.Modalities.Input) == 0 {
		t.Fatal("Expected Modalities.Input to be set")
	}

	hasTextInput := false
	for _, modality := range model.Features.Modalities.Input {
		if modality == catalogs.ModelModalityText {
			hasTextInput = true
			break
		}
	}
	if !hasTextInput {
		t.Error("Expected GPT-4o to support text input")
	}
}

// TestOpenAIClientGetModel tests the GetModel functionality with a mock server.
func TestOpenAIClientGetModel(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		expectedPath := "/v1/models/gpt-4o"
		if r.URL.Path != expectedPath && r.URL.Path != "/gpt-4o" {
			t.Errorf("Expected path '%s' or '/gpt-4o', got '%s'", expectedPath, r.URL.Path)
		}

		// Check for authorization header
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Error("Expected Authorization header")
		}

		// Return a mock single model response based on testdata
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Load from raw API response and extract the specific model
		response := loadTestdataResponse(t, "models_list.json")
		for _, model := range response.Data {
			if model.ID == "gpt-4o" {
				fmt.Fprintf(w, `{
					"id": "%s",
					"object": "%s",
					"created": %d,
					"owned_by": "%s"
				}`, model.ID, model.Object, model.Created, model.OwnedBy)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Create an OpenAI client with mock provider
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDOpenAI,
		Name: "OpenAI",
		APIKey: &catalogs.ProviderAPIKey{
			Name:   "OPENAI_API_KEY",
			Header: "Authorization",
			Scheme: catalogs.ProviderAPIKeySchemeBearer,
		},
		Catalog: &catalogs.ProviderCatalog{
			APIURL: &server.URL,
		},
		APIKeyValue: "test-api-key",
	}

	client := &Client{}
	client.Configure(provider)

	// Test GetModel
	ctx := context.Background()
	model, err := client.GetModel(ctx, "gpt-4o")
	if err != nil {
		t.Fatalf("GetModel failed: %v", err)
	}

	// Verify we got the expected model data
	if model.ID != "gpt-4o" {
		t.Errorf("Expected model ID 'gpt-4o', got '%s'", model.ID)
	}

	if model.Name != "gpt-4o" {
		t.Errorf("Expected model Name 'gpt-4o', got '%s'", model.Name)
	}
}

// TestOpenAIClientListModels tests the ListModels functionality.
func TestOpenAIClientListModels(t *testing.T) {
	requestCount := 0
	// Create a mock HTTP server that handles list requests
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

		// Handle different endpoints
		switch {
		case r.URL.Path == "/v1/models" || r.URL.Path == "/":
			// Return list response
			w.Write(testhelper.LoadTestdata(t, "models_list.json"))
			return

		default:
			t.Errorf("Unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}))
	defer server.Close()

	// Create an OpenAI client with mock provider
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDOpenAI,
		Name: "OpenAI",
		APIKey: &catalogs.ProviderAPIKey{
			Name:   "OPENAI_API_KEY",
			Header: "Authorization",
			Scheme: catalogs.ProviderAPIKeySchemeBearer,
		},
		Catalog: &catalogs.ProviderCatalog{
			APIURL: &server.URL,
		},
		APIKeyValue: "test-api-key",
	}

	client := &Client{}
	client.Configure(provider)

	// Test ListModels
	ctx := context.Background()
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	// Verify we got a reasonable number of models
	if len(models) == 0 {
		t.Error("Expected at least some models, got 0")
	}

	// Verify we made exactly 1 request (OpenAI doesn't do individual model fetching)
	if requestCount != 1 {
		t.Errorf("Expected 1 request, got %d", requestCount)
	}

	// Find a specific model to test - use gpt-4o
	var testModel *catalogs.Model
	for _, model := range models {
		if model.ID == "gpt-4o" {
			testModel = &model
			break
		}
	}

	if testModel == nil {
		t.Fatal("Could not find gpt-4o model in response")
	}

	if testModel.Name != "gpt-4o" {
		t.Errorf("Expected test model Name to be 'gpt-4o', got '%s'", testModel.Name)
	}

	// Test that we have proper features for GPT-4o
	if testModel.Features == nil {
		t.Fatal("Expected test model to have Features")
	}

	if !testModel.Features.Tools {
		t.Error("Expected gpt-4o to support tools")
	}
}

// TestAPIFormatChanges tests that our parsing would catch provider API format changes.
// This is the kind of meaningful test the user requested.
func TestAPIFormatChanges(t *testing.T) {
	response := loadTestdataResponse(t, "models_list.json")

	// These tests ensure we catch API changes that could break our parsing
	t.Run("Required fields present", func(t *testing.T) {
		if len(response.Data) == 0 {
			t.Fatal("No models in response - API might have changed")
		}

		// Check that all models have required fields
		for i, model := range response.Data {
			if model.ID == "" {
				t.Errorf("Model %d missing required 'id' field", i)
			}
			if model.Object == "" {
				t.Errorf("Model %d missing required 'object' field", i)
			}
			if model.OwnedBy == "" {
				t.Errorf("Model %d missing required 'owned_by' field", i)
			}
			if model.Created == 0 {
				t.Errorf("Model %d missing required 'created' field", i)
			}
		}
	})

	t.Run("Known models still present", func(t *testing.T) {
		// These models should exist in OpenAI's API - if they're missing,
		// the API has changed significantly
		expectedModels := []string{"gpt-3.5-turbo", "gpt-4o"}

		for _, expectedID := range expectedModels {
			found := false
			for _, model := range response.Data {
				if model.ID == expectedID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected model '%s' not found in API response - API may have changed", expectedID)
			}
		}
	})

	t.Run("Response structure unchanged", func(t *testing.T) {
		if response.Object != "list" {
			t.Errorf("Expected response.object to be 'list', got '%s' - API structure may have changed", response.Object)
		}
	})
}
