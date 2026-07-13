package openai

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
			Type:          catalogs.EndpointTypeOpenAI,
			URL:           server.URL,
			FieldMappings: mappings,
		}},
	}
	client, err := NewClient(provider)
	require.NoError(t, err)
	return client
}
