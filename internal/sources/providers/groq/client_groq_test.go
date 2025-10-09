package groq

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentstation/starmap/internal/sources/providers/openai"
	"github.com/agentstation/starmap/internal/sources/providers/testhelper"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestGroqFieldMappings tests that the unified openai client correctly handles Groq-specific field mappings.
func TestGroqFieldMappings(t *testing.T) {
	// Create Groq-configured provider
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDGroq,
		Name: "Groq",
		APIKey: &catalogs.ProviderAPIKey{
			Name:   "GROQ_API_KEY",
			Header: "Authorization",
			Scheme: "Bearer",
		},
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type:         catalogs.EndpointTypeOpenAI,
				URL:          "https://api.groq.com/openai/v1/models",
				AuthRequired: true,
				FieldMappings: []catalogs.FieldMapping{
					{
						From: "context_window",
						To:   "limits.context_window",
					},
					{
						From: "max_completion_tokens",
						To:   "limits.output_tokens",
					},
				},
			},
		},
	}

	client := openai.NewClient(provider)

	// Test with actual Groq API model data (from their API response)
	groqModel := openai.Model{
		ID:                  "llama-3.3-70b-versatile",
		Object:              "model",
		OwnedBy:             "Meta",
		Created:             1733447754,
		Active:              boolPtr(true),
		ContextWindow:       int64Ptr(131072), // Context window from Groq
		MaxCompletionTokens: int64Ptr(32768),  // Different max completion tokens
		PublicApps:          nil,
	}

	// Convert using the configured client
	starmapModel := client.ConvertToModel(groqModel)

	// Verify field mappings worked correctly
	require.NotNil(t, starmapModel.Limits, "Limits should be set")
	assert.Equal(t, int64(131072), starmapModel.Limits.ContextWindow, "Context window should be mapped")
	assert.Equal(t, int64(32768), starmapModel.Limits.OutputTokens, "Output tokens should be mapped")
	assert.Equal(t, "llama-3.3-70b-versatile", starmapModel.ID, "ID should be preserved")
	assert.Equal(t, "meta", starmapModel.Authors[0].Name, "Author should be extracted from owned_by")
}

// TestGroqTestdataParsing tests parsing of the actual Groq testdata.
func TestGroqTestdataParsing(t *testing.T) {
	// Load testdata
	var response openai.Response
	testhelper.LoadJSON(t, "models_list.json", &response)

	// Verify response structure
	assert.Equal(t, "list", response.Object)
	assert.Greater(t, len(response.Data), 0, "Should have at least some models")

	// Create Groq-configured client
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDGroq,
		Name: "Groq",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				FieldMappings: []catalogs.FieldMapping{
					{From: "context_window", To: "limits.context_window"},
					{From: "max_completion_tokens", To: "limits.output_tokens"},
				},
			},
		},
	}
	client := openai.NewClient(provider)

	// Test conversion of all models in testdata
	for _, modelData := range response.Data {
		converted := client.ConvertToModel(modelData)

		// Basic validation
		assert.NotEmpty(t, converted.ID, "Model ID should be set for model: %s", modelData.ID)

		// Verify field mappings if source fields are present
		if modelData.ContextWindow != nil && *modelData.ContextWindow > 0 {
			require.NotNil(t, converted.Limits, "Limits should be set for model: %s", modelData.ID)
			assert.Equal(t, *modelData.ContextWindow, converted.Limits.ContextWindow,
				"Context window mapping failed for model: %s", modelData.ID)
		}

		if modelData.MaxCompletionTokens != nil && *modelData.MaxCompletionTokens > 0 {
			require.NotNil(t, converted.Limits, "Limits should be set for model: %s", modelData.ID)
			assert.Equal(t, *modelData.MaxCompletionTokens, converted.Limits.OutputTokens,
				"Max completion tokens mapping failed for model: %s", modelData.ID)
		}
	}

	t.Logf("✅ Successfully tested %d Groq models from testdata", len(response.Data))
}

// TestGroqSpecificFields tests that Groq-specific fields are handled correctly.
func TestGroqSpecificFields(t *testing.T) {
	// Test model with Groq-specific fields
	groqModel := openai.Model{
		ID:                  "whisper-large-v3",
		Object:              "model",
		OwnedBy:             "OpenAI",
		Created:             1693721698,
		Active:              boolPtr(true), // Groq-specific
		ContextWindow:       int64Ptr(448), // Small context for whisper
		MaxCompletionTokens: int64Ptr(448), // Same as context for whisper
		PublicApps:          nil,           // Groq-specific
	}

	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDGroq,
		Name: "Groq",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				FieldMappings: []catalogs.FieldMapping{
					{From: "context_window", To: "limits.context_window"},
					{From: "max_completion_tokens", To: "limits.output_tokens"},
				},
			},
		},
	}

	client := openai.NewClient(provider)
	converted := client.ConvertToModel(groqModel)

	// Verify the model is processed correctly despite having Groq-specific fields
	assert.Equal(t, "whisper-large-v3", converted.ID)
	require.NotNil(t, converted.Limits)
	assert.Equal(t, int64(448), converted.Limits.ContextWindow)
	assert.Equal(t, int64(448), converted.Limits.OutputTokens)
}

// Helper functions
func int64Ptr(v int64) *int64 {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}
