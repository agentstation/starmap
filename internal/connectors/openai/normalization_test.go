package openai

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/internal/acquisition/testsource"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientAppliesDeclarativePricingNormalization(t *testing.T) {
	client := normalizationTestClient(t, `{"data":[{"id":"priced","archived":false,"supports_tools":true,"pricing":{"prompt":0,"completion":2.5,"image":0.01}}]}`, []catalogs.FieldMapping{
		{From: "pricing.prompt", To: "pricing.tokens.input", Unit: catalogs.ProviderNormalizationUnitPerToken, Currency: catalogs.ModelPricingCurrencyUSD},
		{From: "pricing.completion", To: "pricing.tokens.output", Unit: catalogs.ProviderNormalizationUnitPerMillionTokens, Currency: catalogs.ModelPricingCurrencyUSD},
		{From: "pricing.completion", To: "pricing.tokens.output", Unit: catalogs.ProviderNormalizationUnitMilliCurrencyPerMillionTokens, Currency: catalogs.ModelPricingCurrencyUSD, Tier: &catalogs.ProviderPricingTier{Name: "long", Type: catalogs.ModelPricingTierTypeContext, Size: 200_000}},
		{From: "pricing.image", To: "pricing.operations.image_gen", Unit: catalogs.ProviderNormalizationUnitPerOperation, Currency: catalogs.ModelPricingCurrencyUSD, Mode: "batch"},
		{From: "archived", To: "lifecycle", Values: map[string]string{"false": "active", "true": "deprecated"}},
		{From: "supports_tools", To: "features.tools"},
	})

	models, err := client.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 1)
	model := models[0]
	require.NotNil(t, model.Pricing)
	require.NotNil(t, model.Pricing.Tokens.Input)
	assert.Zero(t, model.Pricing.Tokens.Input.PerToken)
	assert.Zero(t, model.Pricing.Tokens.Input.Per1M)
	assert.Equal(t, 2.5, model.Pricing.Tokens.Output.Per1M)
	require.Len(t, model.Pricing.Tiers, 1)
	assert.Equal(t, 0.0025, model.Pricing.Tiers[0].Tokens.Output.Per1M)
	require.NotNil(t, model.Modes["batch"].Pricing.Operations.ImageGen)
	assert.Equal(t, 0.01, *model.Modes["batch"].Pricing.Operations.ImageGen)
	assert.Equal(t, catalogs.ModelStatusActive, model.Status)
	require.NotNil(t, model.Features)
	assert.True(t, model.Features.Tools)
	require.NoError(t, model.Pricing.Validate())
	require.NoError(t, model.Modes["batch"].Pricing.Validate())
}

func TestClientPreservesAbsentPricingAndRejectsUnsafeValues(t *testing.T) {
	mapping := []catalogs.FieldMapping{{From: "pricing.prompt", To: "pricing.tokens.input", Unit: catalogs.ProviderNormalizationUnitPerToken, Currency: catalogs.ModelPricingCurrencyUSD}}

	t.Run("null", func(t *testing.T) {
		client := normalizationTestClient(t, `{"data":[{"id":"unpriced","pricing":{"prompt":null}}]}`, mapping)
		models, err := client.ListModels(context.Background())
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Nil(t, models[0].Pricing)
	})

	t.Run("missing", func(t *testing.T) {
		client := normalizationTestClient(t, `{"data":[{"id":"unpriced"}]}`, mapping)
		models, err := client.ListModels(context.Background())
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Nil(t, models[0].Pricing)
	})

	t.Run("negative", func(t *testing.T) {
		client := normalizationTestClient(t, `{"data":[{"id":"unsafe","pricing":{"prompt":-1}}]}`, mapping)
		models, err := client.ListModels(context.Background())
		require.Error(t, err)
		assert.Nil(t, models)
		var validationErr *pkgerrors.ValidationError
		require.True(t, stderrors.As(err, &validationErr))
		assert.Equal(t, "pricing.prompt", validationErr.Field)
	})
}

func TestClientMapsSafeProviderRawPathsWithoutProviderSpecificGoFields(t *testing.T) {
	client := normalizationTestClient(t, `{"data":[{"id":"rich","capabilities":{"function_calling":true},"limits":{"context":64000},"modalities":{"input":["text","image"]},"provider_detail":"kept"}]}`, []catalogs.FieldMapping{
		{From: "capabilities.function_calling", To: "features.tools"},
		{From: "limits.context", To: "limits.context_window"},
		{From: "modalities.input", To: "features.modalities.input"},
		{From: "provider_detail", To: "extensions.normalization.provider_detail"},
	})

	models, err := client.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 1)
	model := models[0]
	assert.True(t, model.Features.Tools)
	assert.True(t, model.Features.ToolCalls)
	assert.True(t, model.Features.ToolChoice)
	assert.Equal(t, int64(64000), model.Limits.ContextWindow)
	assert.Equal(t, []catalogs.ModelModality{catalogs.ModelModalityText, catalogs.ModelModalityImage}, model.Features.Modalities.Input)
	assert.Equal(t, "kept", model.Extensions["normalization"].Fields["provider_detail"])
}

func TestClientRejectsMappedObjectValue(t *testing.T) {
	client := normalizationTestClient(t, `{"data":[{"id":"unsafe","capabilities":{"nested":{"tools":true}}}]}`, []catalogs.FieldMapping{
		{From: "capabilities.nested", To: "extensions.normalization.capabilities"},
	})
	models, err := client.ListModels(context.Background())
	require.Error(t, err)
	assert.Nil(t, models)
}

func TestClientMergesMappedExtensionsWithDriftEvidence(t *testing.T) {
	client := normalizationTestClient(t, `{"data":[{"id":"rich","provider_detail":"kept","unrecognized":{"nested":true}}]}`, []catalogs.FieldMapping{
		{From: "provider_detail", To: "extensions.normalization-test.provider_detail"},
	})

	models, err := client.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 1)
	fields := models[0].Extensions["normalization-test"].Fields
	assert.Equal(t, "kept", fields["provider_detail"])
	assert.NotEmpty(t, fields["unknown_fields"])
}

func normalizationTestClient(t *testing.T, response string, mappings []catalogs.FieldMapping) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	provider := &catalogs.Provider{
		ID:   "normalization-test",
		Name: "Normalization Test",
		Catalog: testProviderCatalog(catalogs.ProviderSourceEndpoint{
			Type:          catalogs.EndpointTypeOpenAI,
			URL:           server.URL,
			FieldMappings: mappings,
		}),
	}
	client, err := NewClient(testsource.Authenticated(t, provider))
	require.NoError(t, err)
	return client
}
