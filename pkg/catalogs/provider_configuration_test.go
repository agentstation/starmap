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
	copied.Catalog.Sources[0].Offering.Access.APIs[0] = InvocationAPIMessages
	copied.Catalog.Sources[0].Offering.Regions[0].Residency.Countries[0] = "CA"
	*copied.Catalog.Sources[0].Endpoint.FieldMappings[0].Scale = 0.25
	assert.Equal(t, InvocationAPIChatCompletions, provider.Catalog.Sources[0].Offering.Access.APIs[0])
	assert.Equal(t, "US", provider.Catalog.Sources[0].Offering.Regions[0].Residency.Countries[0])
	assert.Equal(t, 0.5, *provider.Catalog.Sources[0].Endpoint.FieldMappings[0].Scale)
}

func TestProviderValidateConfiguration(t *testing.T) {
	tests := map[string]func(*Provider){
		"valid": func(*Provider) {},
		"unsafe response collection": func(provider *Provider) {
			provider.Catalog.Sources[0].Endpoint.ResponseCollection = "data[0]"
		},
		"duplicate invocation API": func(provider *Provider) {
			provider.Catalog.Sources[0].Offering.Access.APIs = append(provider.Catalog.Sources[0].Offering.Access.APIs, InvocationAPIChatCompletions)
		},
		"missing deployment type": func(provider *Provider) {
			provider.Catalog.Sources[0].Offering.Deployment.Type = ""
		},
		"duplicate region": func(provider *Provider) {
			provider.Catalog.Sources[0].Offering.Regions = append(provider.Catalog.Sources[0].Offering.Regions, provider.Catalog.Sources[0].Offering.Regions[0])
		},
		"routable without endpoint": func(provider *Provider) {
			provider.Catalog.Sources[0].Offering.Endpoint.Type = ""
		},
		"missing acquisition endpoint": func(provider *Provider) {
			provider.Catalog.Sources[0].Endpoint.URL = ""
			provider.Catalog.Sources[0].Endpoint.BaseURLEnv = ""
		},
		"unsafe source path": func(provider *Provider) {
			provider.Catalog.Sources[0].Endpoint.FieldMappings = []FieldMapping{{From: "models[0].limit", To: "limits.context_window"}}
		},
		"secret-bearing source path": func(provider *Provider) {
			provider.Catalog.Sources[0].Endpoint.FieldMappings = []FieldMapping{{From: "metadata.api_key", To: "extensions.example.value"}}
		},
		"unsafe feature rule": func(provider *Provider) {
			provider.Catalog.Sources[0].Endpoint.FeatureRules = []FeatureRule{{Field: "capabilities[0]", Contains: []string{"tools"}, Feature: "tools", Value: true}}
		},
		"negative pricing scale": func(provider *Provider) {
			negative := -0.5
			provider.Catalog.Sources[0].Endpoint.FieldMappings[0].Scale = &negative
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

func TestProviderValidateConfigurationAllowsApplicationWithoutAcquisitionEndpoint(t *testing.T) {
	provider := Provider{
		ID: "application", Name: "Application",
		Catalog: &ProviderCatalog{Sources: []ProviderSource{{
			ID:               "application",
			ObservationScope: ProviderObservationPolicy{Invariant: ProviderObservationScopeGlobalPublic},
			Auth:             ProviderAuthPolicy{Mode: ProviderAuthModeNone},
			Endpoint:         ProviderSourceEndpoint{Type: EndpointTypeApplication},
		}}},
	}
	require.NoError(t, provider.ValidateConfiguration())
}

func TestSourceProviderOfferingUsesConfigurationDefaults(t *testing.T) {
	provider := providerWithOfferingDefaults()
	offering, changes, err := sourceProviderOffering(provider, Model{ID: "example-model", Name: "Example Model"})
	require.NoError(t, err)

	assert.Equal(t, provider.Catalog.Sources[0].Offering.Access, offering.Access)
	assert.Equal(t, provider.Catalog.Sources[0].Offering.Endpoint, offering.Endpoint)
	assert.Equal(t, provider.Catalog.Sources[0].Offering.Deployment, offering.Deployment)
	assert.Equal(t, provider.Catalog.Sources[0].Offering.Regions, offering.Regions)
	assert.Contains(t, changes[1].Message, "provider-configured")

	offering.Access.APIs[0] = InvocationAPIMessages
	offering.Regions[0].Residency.Countries[0] = "CA"
	assert.Equal(t, InvocationAPIChatCompletions, provider.Catalog.Sources[0].Offering.Access.APIs[0])
	assert.Equal(t, "US", provider.Catalog.Sources[0].Offering.Regions[0].Residency.Countries[0])
}

func TestSourceProviderOfferingModelInvocationOverridesConfigurationDefault(t *testing.T) {
	provider := providerWithOfferingDefaults()
	offering, _, err := sourceProviderOffering(provider, Model{
		ID:             "embedding-model",
		Name:           "Embedding Model",
		InvocationAPIs: []InvocationAPI{InvocationAPIEmbeddings},
	})
	require.NoError(t, err)
	assert.Equal(t, []InvocationAPI{InvocationAPIEmbeddings}, offering.Access.APIs)
	assert.Equal(t, OfferingRoutabilityRoutable, offering.Access.Routability)

	offering, _, err = sourceProviderOffering(provider, Model{
		ID:             "unroutable-model",
		Name:           "Unroutable Model",
		InvocationAPIs: []InvocationAPI{},
	})
	require.NoError(t, err)
	assert.Empty(t, offering.Access.APIs)
	assert.Equal(t, OfferingRoutabilityDiscoverable, offering.Access.Routability)
}

func TestSourceProviderOfferingUsesConfiguredCatalogAuthority(t *testing.T) {
	provider := providerWithOfferingDefaults()
	provider.Catalog.Sources[0].Endpoint.BaseURLEnv = "EXAMPLE_BASE_URL"
	provider.Catalog.Sources[0].Endpoint.Path = "/models"
	provider.Catalog.Sources[0].Offering.Endpoint.BaseURL = ""
	t.Setenv("EXAMPLE_BASE_URL", "https://regional.example.com/v2")

	require.NoError(t, provider.ValidateConfiguration())
	offering, _, err := sourceProviderOffering(provider, Model{ID: "example-model", Name: "Example Model"})
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", offering.Endpoint.BaseURL)
	assert.Equal(t, "", provider.Catalog.Sources[0].Offering.Endpoint.BaseURL)
}

func TestConfigurationOnlyProvidersKeepPublicationEndpointConfigured(t *testing.T) {
	builder, err := NewEmbedded()
	require.NoError(t, err)
	for _, providerID := range []ProviderID{ProviderIDBaseten, ProviderIDHyperbolic, ProviderIDNovita, ProviderIDScaleway} {
		t.Run(providerID.String(), func(t *testing.T) {
			provider, err := builder.Provider(providerID)
			require.NoError(t, err)
			require.NoError(t, provider.ValidateConfiguration())
			configured := provider.Catalog.Sources[0].Endpoint.URL
			offering, _, projectErr := sourceProviderOffering(provider, Model{ID: "example-model", Name: "Example Model"})
			require.NoError(t, projectErr)
			assert.Equal(t, catalogBaseURL(configured, provider.Catalog.Sources[0].Endpoint.Path), offering.Endpoint.BaseURL)
		})
	}
}

func providerWithOfferingDefaults() Provider {
	return Provider{
		ID:   "example",
		Name: "Example",
		Catalog: &ProviderCatalog{
			Sources: []ProviderSource{{
				ID:               "models",
				ObservationScope: ProviderObservationPolicy{Invariant: ProviderObservationScopeGlobalPublic},
				Auth:             ProviderAuthPolicy{Mode: ProviderAuthModeNone},
				Endpoint: ProviderSourceEndpoint{
					Type:               EndpointTypeOpenAI,
					URL:                "https://api.example.com/v1/models",
					ResponseCollection: "data.models",
					FieldMappings: []FieldMapping{{
						From: "pricing.prompt", To: "pricing.tokens.input",
						Unit: ProviderNormalizationUnitPerMillionTokens, Currency: ModelPricingCurrencyUSD,
						Mode: "batch", Scale: float64Pointer(0.5),
					}},
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
			}},
		},
	}
}

func float64Pointer(value float64) *float64 { return &value }
