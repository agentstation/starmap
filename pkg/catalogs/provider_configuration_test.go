package catalogs

import (
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderConfigurationRoundTripsAndDeepCopies(t *testing.T) {
	provider := providerWithOfferingDefaults()

	jsonPayload, err := json.Marshal(provider)
	require.NoError(t, err)
	var jsonProvider Provider
	require.NoError(t, json.Unmarshal(jsonPayload, &jsonProvider))
	assert.Equal(t, provider, jsonProvider)

	yamlPayload, err := yaml.Marshal(provider)
	require.NoError(t, err)
	var yamlProvider Provider
	require.NoError(t, yaml.Unmarshal(yamlPayload, &yamlProvider))
	assert.Equal(t, provider, yamlProvider)

	copied := DeepCopyProvider(provider)
	copied.Catalog.Offering.Access.APIs[0] = InvocationAPIMessages
	copied.Catalog.Offering.Regions[0].Residency.Countries[0] = "CA"
	assert.Equal(t, InvocationAPIChatCompletions, provider.Catalog.Offering.Access.APIs[0])
	assert.Equal(t, "US", provider.Catalog.Offering.Regions[0].Residency.Countries[0])
}

func TestProviderValidateConfiguration(t *testing.T) {
	tests := map[string]func(*Provider){
		"valid": func(*Provider) {},
		"unsafe response collection": func(provider *Provider) {
			provider.Catalog.Endpoint.ResponseCollection = "data[0]"
		},
		"duplicate invocation API": func(provider *Provider) {
			provider.Catalog.Offering.Access.APIs = append(provider.Catalog.Offering.Access.APIs, InvocationAPIChatCompletions)
		},
		"missing deployment type": func(provider *Provider) {
			provider.Catalog.Offering.Deployment.Type = ""
		},
		"duplicate region": func(provider *Provider) {
			provider.Catalog.Offering.Regions = append(provider.Catalog.Offering.Regions, provider.Catalog.Offering.Regions[0])
		},
		"routable without endpoint": func(provider *Provider) {
			provider.Catalog.Offering.Endpoint.Type = ""
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			provider := providerWithOfferingDefaults()
			mutate(&provider)
			err := provider.ValidateConfiguration()
			if name == "valid" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestSourceProviderOfferingUsesConfigurationDefaults(t *testing.T) {
	provider := providerWithOfferingDefaults()
	offering, changes, err := sourceProviderOffering(provider, Model{ID: "example-model", Name: "Example Model"})
	require.NoError(t, err)

	assert.Equal(t, provider.Catalog.Offering.Access, offering.Access)
	assert.Equal(t, provider.Catalog.Offering.Endpoint, offering.Endpoint)
	assert.Equal(t, provider.Catalog.Offering.Deployment, offering.Deployment)
	assert.Equal(t, provider.Catalog.Offering.Regions, offering.Regions)
	assert.Contains(t, changes[1].Message, "provider-configured")

	offering.Access.APIs[0] = InvocationAPIMessages
	offering.Regions[0].Residency.Countries[0] = "CA"
	assert.Equal(t, InvocationAPIChatCompletions, provider.Catalog.Offering.Access.APIs[0])
	assert.Equal(t, "US", provider.Catalog.Offering.Regions[0].Residency.Countries[0])
}

func TestSourceProviderOfferingDerivesEndpointFromAcquisitionOverride(t *testing.T) {
	provider := providerWithOfferingDefaults()
	provider.Catalog.Endpoint.BaseURLEnvVar = "EXAMPLE_BASE_URL"
	provider.Catalog.Endpoint.Path = "/models"
	provider.Catalog.Offering.Endpoint.BaseURL = ""
	provider.EnvVarValues = map[string]string{"EXAMPLE_BASE_URL": "https://regional.example.com/v2"}

	require.NoError(t, provider.ValidateConfiguration())
	assert.Equal(t, "https://regional.example.com/v2/models", provider.CatalogEndpointURL())
	offering, _, err := sourceProviderOffering(provider, Model{ID: "example-model", Name: "Example Model"})
	require.NoError(t, err)
	assert.Equal(t, "https://regional.example.com/v2", offering.Endpoint.BaseURL)
	assert.Equal(t, "", provider.Catalog.Offering.Endpoint.BaseURL)
}

func TestConfigurationOnlyProvidersShareAcquisitionAndOfferingOverrides(t *testing.T) {
	builder, err := NewEmbedded()
	require.NoError(t, err)
	for _, providerID := range []ProviderID{ProviderIDBaseten, ProviderIDHyperbolic, ProviderIDNovita, ProviderIDScaleway} {
		t.Run(providerID.String(), func(t *testing.T) {
			provider, err := builder.Provider(providerID)
			require.NoError(t, err)
			require.NoError(t, provider.ValidateConfiguration())
			provider.EnvVarValues = map[string]string{provider.Catalog.Endpoint.BaseURLEnvVar: "https://regional.example.test/v9/"}

			assert.Equal(t, "https://regional.example.test/v9/models", provider.CatalogEndpointURL())
			assert.Equal(t, "https://regional.example.test/v9", provider.CatalogOfferingEndpoint().BaseURL)
		})
	}
}

func providerWithOfferingDefaults() Provider {
	return Provider{
		ID:   "example",
		Name: "Example",
		Catalog: &ProviderCatalog{
			Endpoint: ProviderEndpoint{
				Type:               EndpointTypeOpenAI,
				URL:                "https://api.example.com/v1/models",
				ResponseCollection: "data.models",
			},
			Offering: &ProviderOfferingDefaults{
				Access: OfferingAccess{
					Channel:     OfferingAccessChannelServerToServer,
					Routability: OfferingRoutabilityRoutable,
					APIs:        []InvocationAPI{InvocationAPIChatCompletions},
				},
				Endpoint: ProviderOfferingEndpoint{
					Type:    EndpointTypeOpenAI,
					BaseURL: "https://api.example.com/v1",
				},
				Deployment: ProviderDeployment{Type: "serverless", Tier: "managed"},
				Regions: []CloudRegion{{
					ID: "us-example-1",
					Residency: &GeographicBoundary{
						ID:        "united-states",
						Kind:      GeographicBoundaryCountry,
						Countries: []string{"US"},
					},
				}},
			},
		},
	}
}
