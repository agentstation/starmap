package moonshot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agentstation/starmap/internal/sources/providers/openai"
	"github.com/agentstation/starmap/internal/sources/providers/testhelper"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestMoonshotAuthorMapping tests that the unified openai client correctly handles Moonshot-specific author mapping.
func TestMoonshotAuthorMapping(t *testing.T) {
	// Create Moonshot-configured provider
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDMoonshotAI,
		Name: "Moonshot AI",
		APIKey: &catalogs.ProviderAPIKey{
			Name:   "MOONSHOT_API_KEY",
			Header: "Authorization",
			Scheme: "Bearer",
		},
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type:         catalogs.EndpointTypeOpenAI,
				URL:          "https://api.moonshot.ai/v1/models",
				AuthRequired: true,
				AuthorMapping: &catalogs.AuthorMapping{
					Field: "owned_by",
					Normalized: map[string]catalogs.AuthorID{
						"Moonshot": "moonshot-ai",
					},
				},
			},
		},
	}

	client := openai.NewClient(provider)

	// Test with actual Moonshot AI model data
	moonshotModel := openai.Model{
		ID:      "kimi-latest",
		Object:  "model",
		OwnedBy: "Moonshot",
		Created: 1733447754,
	}

	// Convert using the configured client
	starmapModel := client.ConvertToModel(moonshotModel)

	// Verify author mapping worked correctly
	require.Len(t, starmapModel.Authors, 1, "Should have exactly one author")
	assert.Equal(t, "moonshot-ai", starmapModel.Authors[0].Name, "Author should be normalized to moonshot-ai")
	assert.Equal(t, "kimi-latest", starmapModel.ID, "ID should be preserved")
}

// TestMoonshotTestdataParsing tests parsing of the actual Moonshot testdata.
func TestMoonshotTestdataParsing(t *testing.T) {
	// Load testdata
	var response openai.Response
	testhelper.LoadJSON(t, "models_list.json", &response)

	// Verify response structure
	assert.Equal(t, "list", response.Object)
	assert.Greater(t, len(response.Data), 0, "Should have at least some models")

	// Create Moonshot-configured client
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDMoonshotAI,
		Name: "Moonshot AI",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				AuthorMapping: &catalogs.AuthorMapping{
					Field: "owned_by",
					Normalized: map[string]catalogs.AuthorID{
						"Moonshot": "moonshot-ai",
					},
				},
			},
		},
	}
	client := openai.NewClient(provider)

	// Test conversion of all models in testdata
	expectedModels := map[string]bool{
		"moonshot-v1-8k":                true,
		"moonshot-v1-32k":               true,
		"moonshot-v1-128k":              true,
		"kimi-latest":                   true,
		"kimi-thinking-preview":         true,
		"kimi-k2-turbo-preview":         true,
		"moonshot-v1-8k-vision-preview": true,
	}

	foundModels := make(map[string]bool)
	for _, modelData := range response.Data {
		converted := client.ConvertToModel(modelData)

		// Basic validation
		assert.NotEmpty(t, converted.ID, "Model ID should be set for model: %s", modelData.ID)
		assert.Equal(t, modelData.ID, converted.ID, "Converted ID should match source ID")

		// Track which models we found
		foundModels[converted.ID] = true

		// Verify author normalization for all models
		require.Len(t, converted.Authors, 1, "Should have exactly one author for model: %s", modelData.ID)
		assert.Equal(t, "moonshot-ai", converted.Authors[0].Name,
			"Author should be normalized to moonshot-ai for model: %s", modelData.ID)
	}

	// Verify we found expected models
	for expectedModel := range expectedModels {
		assert.True(t, foundModels[expectedModel],
			"Expected to find model %s in testdata", expectedModel)
	}

	t.Logf("✅ Successfully tested %d Moonshot AI models from testdata", len(response.Data))
}

// TestMoonshotModelVariants tests that different Moonshot model variants are handled correctly.
func TestMoonshotModelVariants(t *testing.T) {
	provider := &catalogs.Provider{
		ID:   catalogs.ProviderIDMoonshotAI,
		Name: "Moonshot AI",
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				AuthorMapping: &catalogs.AuthorMapping{
					Field: "owned_by",
					Normalized: map[string]catalogs.AuthorID{
						"Moonshot": "moonshot-ai",
					},
				},
			},
		},
	}

	client := openai.NewClient(provider)

	testCases := []struct {
		name       string
		model      openai.Model
		expectedID string
	}{
		{
			name: "moonshot v1 8k",
			model: openai.Model{
				ID:      "moonshot-v1-8k",
				Object:  "model",
				OwnedBy: "Moonshot",
				Created: 1710000000,
			},
			expectedID: "moonshot-v1-8k",
		},
		{
			name: "kimi latest",
			model: openai.Model{
				ID:      "kimi-latest",
				Object:  "model",
				OwnedBy: "Moonshot",
				Created: 1733447754,
			},
			expectedID: "kimi-latest",
		},
		{
			name: "kimi thinking preview",
			model: openai.Model{
				ID:      "kimi-thinking-preview",
				Object:  "model",
				OwnedBy: "Moonshot",
				Created: 1733447754,
			},
			expectedID: "kimi-thinking-preview",
		},
		{
			name: "vision preview model",
			model: openai.Model{
				ID:      "moonshot-v1-128k-vision-preview",
				Object:  "model",
				OwnedBy: "Moonshot",
				Created: 1733447754,
			},
			expectedID: "moonshot-v1-128k-vision-preview",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			converted := client.ConvertToModel(tc.model)

			assert.Equal(t, tc.expectedID, converted.ID, "ID should match")
			require.Len(t, converted.Authors, 1, "Should have exactly one author")
			assert.Equal(t, "moonshot-ai", converted.Authors[0].Name, "Author should be normalized")
		})
	}
}
