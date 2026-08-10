package deepseek

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/agentstation/starmap/internal/providers/openai"
	"github.com/agentstation/starmap/internal/providers/testhelper"
	"github.com/agentstation/starmap/internal/testcatalog"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestDeepSeekWithUnifiedClient tests the unified OpenAI client with DeepSeek configuration.
func TestDeepSeekWithUnifiedClient(t *testing.T) {
	// Load testdata
	var response openai.Response
	testhelper.LoadJSON(t, "models_list.json", &response)

	// Verify testdata structure
	assert.Equal(t, "list", response.Object)
	assert.Greater(t, len(response.Data), 0, "Should have at least some models")

	// Create DeepSeek-configured provider (uses OpenAI-compatible API)
	provider := &catalogs.Provider{
		ID: catalogs.ProviderIDDeepSeek, Name: "DeepSeek",
		Credentials: testcatalog.APIKeyCredentials(
			"DEEPSEEK_API_KEY", "Authorization", catalogs.ProviderCredentialSchemeBearer,
		),
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type:            catalogs.EndpointTypeOpenAI,
				URL:             "https://api.deepseek.com/v1/models",
				ProtocolOptions: testcatalog.OpenAIProtocolOptions(),
				// DeepSeek might have specific field mappings
			},
		},
	}

	client, err := openai.NewClient(provider)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// Test conversion of all models in testdata
	for _, modelData := range response.Data {
		converted := client.ConvertToModel(modelData)
		assert.NotEmpty(t, converted.ID, "Model ID should be set for model: %s", modelData.ID)
		assert.NotEmpty(t, converted.Name, "Model Name should be set for model: %s", modelData.ID)
	}

	t.Logf("✅ Successfully tested %d DeepSeek models from testdata", len(response.Data))
}
