package catalogs

import (
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
)

func TestProviderOfferingRoundTripAndProviderScopedModes(t *testing.T) {
	priority := json.RawMessage(`"priority"`)
	standard := json.RawMessage(`"standard"`)
	offerings := []ProviderOffering{
		{
			ProviderID:      "provider-a",
			ProviderModelID: "shared/model@001",
			DefinitionID:    "shared-model",
			Pricing:         testOfferingPricing(1.25),
			Limits:          &ModelLimits{ContextWindow: 128000},
			Availability:    OfferingAvailabilityAvailable,
			Access: OfferingAccess{Channel: OfferingAccessChannelServerToServer, Routability: OfferingRoutabilityRoutable,
				APIs: []InvocationAPI{InvocationAPIChatCompletions}},
			Regions:    []CloudRegion{{ID: "us-east", Residency: &GeographicBoundary{ID: "us", Kind: GeographicBoundaryCountry, Countries: []string{"US"}}}, {ID: "eu-west"}},
			Deployment: ProviderDeployment{Type: "serverless", Tier: "priority"},
			Endpoint: ProviderOfferingEndpoint{
				Type:    EndpointTypeOpenAI,
				BaseURL: "https://a.example/v1",
				Path:    "/chat/completions",
			},
			Lifecycle: OfferingLifecycleActive,
			Modes: map[string]ProviderOfferingMode{
				"fast": {
					Pricing: testOfferingPricing(2.5),
					Request: ProviderRequestOverrides{
						Headers: OfferingRequestHeaders{"x-service-tier": "priority"},
						Body:    OfferingRequestBody{"service_tier": priority},
					},
				},
			},
		},
		{
			ProviderID:      "provider-b",
			ProviderModelID: "shared/model@001",
			DefinitionID:    "shared-model",
			Pricing:         testOfferingPricing(0.75),
			Availability:    OfferingAvailabilityRestricted,
			Access: OfferingAccess{Channel: OfferingAccessChannelServerToServer, Routability: OfferingRoutabilityRoutable,
				APIs: []InvocationAPI{InvocationAPIMessages}},
			Regions:    []CloudRegion{{ID: "us-central"}},
			Deployment: ProviderDeployment{Type: "provisioned"},
			Endpoint: ProviderOfferingEndpoint{
				Type:    EndpointTypeAnthropic,
				BaseURL: "https://b.example",
				Path:    "/messages",
			},
			Lifecycle: OfferingLifecyclePreview,
			Modes: map[string]ProviderOfferingMode{
				"standard": {
					Request: ProviderRequestOverrides{
						Body: OfferingRequestBody{"service_tier": standard},
					},
				},
			},
		},
	}

	if offerings[0].Key() == offerings[1].Key() {
		t.Fatal("equal provider model IDs collapsed distinct offering keys")
	}
	for _, offering := range offerings {
		if err := offering.Validate(); err != nil {
			t.Fatalf("Validate(%s): %v", offering.ProviderID, err)
		}
		assertOfferingRoundTrip(t, offering)
	}
	if got := offerings[0].Modes["fast"].Pricing.Tokens.Input.Per1M; got != 2.5 {
		t.Fatalf("provider-a fast price = %v, want 2.5", got)
	}
	if _, found := offerings[1].Modes["fast"]; found {
		t.Fatal("provider-b inherited provider-a mode")
	}
}

func TestProviderOfferingValidation(t *testing.T) {
	valid := ProviderOffering{
		ProviderID:      "provider",
		ProviderModelID: "model",
		DefinitionID:    "definition",
		Availability:    OfferingAvailabilityAvailable,
		Access: OfferingAccess{Channel: OfferingAccessChannelServerToServer, Routability: OfferingRoutabilityRoutable,
			APIs: []InvocationAPI{InvocationAPIChatCompletions}},
		Endpoint:   ProviderOfferingEndpoint{Type: EndpointTypeOpenAI},
		Deployment: ProviderDeployment{Type: "serverless"},
		Lifecycle:  OfferingLifecycleActive,
	}
	tests := []struct {
		name   string
		mutate func(*ProviderOffering)
	}{
		{name: "provider", mutate: func(o *ProviderOffering) { o.ProviderID = "" }},
		{name: "provider model", mutate: func(o *ProviderOffering) { o.ProviderModelID = "" }},
		{name: "definition", mutate: func(o *ProviderOffering) { o.DefinitionID = "" }},
		{name: "availability", mutate: func(o *ProviderOffering) { o.Availability = "unknown-value" }},
		{name: "lifecycle", mutate: func(o *ProviderOffering) { o.Lifecycle = "unknown-value" }},
		{name: "empty region", mutate: func(o *ProviderOffering) { o.Regions = []CloudRegion{{}} }},
		{name: "duplicate region", mutate: func(o *ProviderOffering) { o.Regions = []CloudRegion{{ID: "us"}, {ID: "us"}} }},
		{name: "missing deployment", mutate: func(o *ProviderOffering) { o.Deployment = ProviderDeployment{} }},
		{name: "application routable", mutate: func(o *ProviderOffering) {
			o.Access = OfferingAccess{Channel: OfferingAccessChannelApplication, Routability: OfferingRoutabilityRoutable}
		}},
		{name: "invalid body JSON", mutate: func(o *ProviderOffering) {
			o.Modes = map[string]ProviderOfferingMode{"fast": {Request: ProviderRequestOverrides{Body: OfferingRequestBody{"tier": json.RawMessage(`{`)}}}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			offering := valid
			test.mutate(&offering)
			if err := offering.Validate(); err == nil {
				t.Fatal("Validate returned nil error")
			}
		})
	}
}

func assertOfferingRoundTrip(t testing.TB, want ProviderOffering) {
	t.Helper()
	jsonData, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal JSON: %v", err)
	}
	var fromJSON ProviderOffering
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("Unmarshal JSON: %v", err)
	}
	if diff := cmp.Diff(want, fromJSON); diff != "" {
		t.Fatalf("JSON round trip (-want +got):\n%s", diff)
	}

	yamlData, err := yaml.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal YAML: %v", err)
	}
	var fromYAML ProviderOffering
	if err := yaml.Unmarshal(yamlData, &fromYAML); err != nil {
		t.Fatalf("Unmarshal YAML: %v", err)
	}
	if diff := cmp.Diff(want, fromYAML); diff != "" {
		t.Fatalf("YAML round trip (-want +got):\n%s", diff)
	}
}

func testOfferingPricing(input float64) *ModelPricing {
	return &ModelPricing{
		Currency: ModelPricingCurrencyUSD,
		Tokens: &ModelTokenPricing{
			Input: &ModelTokenCost{Per1M: input},
		},
	}
}
