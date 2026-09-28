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

func TestDeepInfraImageUnitFixture(t *testing.T) {
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
		if source.Metadata == nil || source.Metadata.Pricing == nil || source.Metadata.Pricing.PerImageUnit == nil {
			continue
		}
		converted, err := client.ConvertToModel(source)
		require.NoError(t, err, source.ID)
		require.NotNil(t, converted.Pricing, source.ID)
		require.NotNil(t, converted.Pricing.Operations, source.ID)
		require.NotNil(t, converted.Pricing.Operations.ImageUnit, source.ID)
		price := *source.Metadata.Pricing.PerImageUnit
		// Existing acquisition normalization removes binary-float representation noise.
		tolerance := 2 * (math.Nextafter(price, math.Inf(1)) - price)
		require.InDelta(t, price, *converted.Pricing.Operations.ImageUnit, tolerance, source.ID)
		require.Nil(t, converted.Pricing.Operations.ImageGen, source.ID)
		checked++
	}
	require.Positive(t, checked)
	t.Logf("verified %d observed image-unit prices", checked)
}
