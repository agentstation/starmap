package openai_test

import (
	"math"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/providers/openai"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/internal/test/providerfixture"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/stretchr/testify/require"
)

func TestDeepInfraDurationPricesKeepTheirObservedUnits(t *testing.T) {
	fixture, err := providerfixture.Find(providerFixtureRoot, "deepinfra")
	require.NoError(t, err)
	require.NoError(t, fixture.Verify(time.Now()))
	var response openai.Response
	require.NoError(t, fixture.Decode(&response))
	builder, err := testcatalog.EmbeddedBuilder()
	require.NoError(t, err)
	provider, found := builder.Providers().Get(catalogs.ProviderIDDeepInfra)
	require.True(t, found)
	client, err := openai.NewClient(provider)
	require.NoError(t, err)
	checked := 0
	for _, source := range response.Data {
		if source.Metadata == nil || source.Metadata.Pricing == nil {
			continue
		}
		pricing := source.Metadata.Pricing
		if pricing.InputSeconds == nil && pricing.OutputSeconds == nil {
			continue
		}
		model, err := client.ConvertToModel(source)
		require.NoError(t, err, source.ID)
		require.NotNil(t, model.Pricing, source.ID)
		require.NotNil(t, model.Pricing.Operations, source.ID)
		operations := model.Pricing.Operations
		for _, pair := range [][2]*float64{{pricing.InputSeconds, operations.InputSecond}, {pricing.OutputSeconds, operations.OutputSecond}} {
			if pair[0] == nil {
				require.Nil(t, pair[1], source.ID)
				continue
			}
			require.NotNil(t, pair[1], source.ID)
			tolerance := math.Max(1e-12, math.Abs(*pair[0])*1e-7)
			require.InDelta(t, *pair[0], *pair[1], tolerance, source.ID)
		}
		require.Nil(t, operations.VideoGen, source.ID)
		require.Nil(t, operations.AudioGen, source.ID)
		require.Nil(t, operations.AudioInput, source.ID)
		checked++
	}
	require.Positive(t, checked)
	t.Logf("verified duration units for %d observed models", checked)
}
