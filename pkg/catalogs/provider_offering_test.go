package catalogs

import (
	"encoding/json"
	"strings"
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
			Regions:         []string{"us-east", "eu-west"},
			Service:         ProviderOfferingServiceCapabilities{Operations: []ProviderOperation{ProviderOperationChatCompletions}},
			Endpoints: []ProviderOfferingEndpoint{{
				Operation: ProviderOperationChatCompletions,
				Type:      EndpointTypeOpenAI,
				URL:       "https://a.example/v1/chat/completions",
			}},
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
			Regions:         []string{"us-central"},
			Service:         ProviderOfferingServiceCapabilities{Operations: []ProviderOperation{ProviderOperationChatCompletions}},
			Endpoints: []ProviderOfferingEndpoint{{
				Operation: ProviderOperationChatCompletions,
				Type:      EndpointTypeAnthropic,
				URL:       "https://b.example/messages",
			}},
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
		Lifecycle:       OfferingLifecycleActive,
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
		{name: "empty region", mutate: func(o *ProviderOffering) { o.Regions = []string{""} }},
		{name: "duplicate region", mutate: func(o *ProviderOffering) { o.Regions = []string{"us", "us"} }},
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

func TestOfferingRequestBodyYAMLUsesNativeValuesNotRawBytes(t *testing.T) {
	t.Parallel()

	body := OfferingRequestBody{
		"string": json.RawMessage(`"priority"`),
		"number": json.RawMessage(`1.25`),
		"bool":   json.RawMessage(`true`),
		"array":  json.RawMessage(`["a",2]`),
		"object": json.RawMessage(`{"mode":"pro"}`),
		"null":   json.RawMessage(`null`),
	}
	data, err := yaml.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal YAML: %v", err)
	}
	rendered := string(data)
	for _, want := range []string{
		"string: priority",
		"number: 1.25",
		"bool: true",
		"- a",
		"mode: pro",
		`"null": null`,
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("YAML missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "[34,") || strings.Contains(rendered, "- 34") {
		t.Fatalf("YAML encoded JSON string bytes:\n%s", rendered)
	}
	if !strings.Contains(rendered, "object:\n  mode: pro") {
		t.Fatalf("nested object escaped its request-body field:\n%s", rendered)
	}

	var roundTrip OfferingRequestBody
	if err := yaml.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("Unmarshal YAML: %v", err)
	}
	for field, want := range body {
		var wantValue any
		if err := json.Unmarshal(want, &wantValue); err != nil {
			t.Fatalf("decode expected %s: %v", field, err)
		}
		var gotValue any
		if err := json.Unmarshal(roundTrip[field], &gotValue); err != nil {
			t.Fatalf("decode round-trip %s: %v", field, err)
		}
		if diff := cmp.Diff(wantValue, gotValue); diff != "" {
			t.Fatalf("%s round trip (-want +got):\n%s\nYAML:\n%s", field, diff, rendered)
		}
	}
}

func assertOfferingRoundTrip(t testing.TB, want ProviderOffering) {
	t.Helper()
	limitComparer := cmp.Comparer(func(left, right ModelLimits) bool {
		return equalPresenceJSON(left, right)
	})
	jsonData, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal JSON: %v", err)
	}
	var fromJSON ProviderOffering
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("Unmarshal JSON: %v", err)
	}
	if diff := cmp.Diff(want, fromJSON, limitComparer); diff != "" {
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
	if diff := cmp.Diff(want, fromYAML, limitComparer); diff != "" {
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
