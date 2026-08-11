package openai

import (
	stderrors "errors"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

const testCapabilityEvidence = "https://provider.example/docs/model-capabilities"

func TestCapabilityMappingPreservesTrueFalseAndUnknown(t *testing.T) {
	provider := &catalogs.Provider{
		ID: "fireworks-like", Name: "Fireworks-like",
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
			Type: catalogs.EndpointTypeOpenAI,
			CapabilityMappings: []catalogs.CapabilityMapping{{
				From: "supports_tools",
				To: []catalogs.ModelFeature{
					catalogs.ModelFeatureTools,
					catalogs.ModelFeatureToolCalls,
					catalogs.ModelFeatureToolChoice,
				},
				Evidence: testCapabilityEvidence,
			}},
		}},
	}
	client := newTestClient(t, provider)
	for _, test := range []struct {
		name      string
		source    *bool
		wantState catalogs.ValuePresence
		wantValue bool
	}{
		{name: "true", source: boolPointer(true), wantState: catalogs.ValueKnown, wantValue: true},
		{name: "false", source: boolPointer(false), wantState: catalogs.ValueKnown, wantValue: false},
		{name: "unknown", source: nil, wantState: catalogs.ValueMissing},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := mustConvertModel(t, client, Model{ID: test.name, SupportsTools: test.source})
			for _, target := range provider.Catalog.Endpoint.CapabilityMappings[0].To {
				value, state := model.Features.Support(target)
				if value != test.wantValue || state != test.wantState {
					t.Fatalf("Support(%s) = %t/%v, want %t/%v", target, value, state, test.wantValue, test.wantState)
				}
			}
		})
	}
}

func TestCapabilityMappingCombinationRulesAreDeterministic(t *testing.T) {
	for _, test := range []struct {
		name        string
		combination catalogs.ProviderCapabilityCombination
		tools       *bool
		reasoning   *bool
		wantValue   bool
		wantState   catalogs.ValuePresence
		wantError   bool
	}{
		{name: "conflict equal", combination: catalogs.ProviderCapabilityConflict, tools: boolPointer(true), reasoning: boolPointer(true), wantValue: true, wantState: catalogs.ValueKnown},
		{name: "conflict unequal", combination: catalogs.ProviderCapabilityConflict, tools: boolPointer(true), reasoning: boolPointer(false), wantError: true},
		{name: "first known", combination: catalogs.ProviderCapabilityFirstKnown, tools: nil, reasoning: boolPointer(false), wantState: catalogs.ValueKnown},
		{name: "any partial true", combination: catalogs.ProviderCapabilityAny, tools: boolPointer(true), reasoning: nil, wantValue: true, wantState: catalogs.ValueKnown},
		{name: "any partial false unknown", combination: catalogs.ProviderCapabilityAny, tools: boolPointer(false), reasoning: nil, wantState: catalogs.ValueMissing},
		{name: "all partial false", combination: catalogs.ProviderCapabilityAll, tools: boolPointer(false), reasoning: nil, wantState: catalogs.ValueKnown},
		{name: "all partial true unknown", combination: catalogs.ProviderCapabilityAll, tools: boolPointer(true), reasoning: nil, wantState: catalogs.ValueMissing},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := capabilityCombinationProvider(test.combination)
			client := newTestClient(t, provider)
			model, err := client.ConvertToModel(Model{
				ID: test.name, SupportsTools: test.tools, SupportsReasoning: test.reasoning,
			})
			if test.wantError {
				var validationErr *pkgerrors.ValidationError
				if !stderrors.As(err, &validationErr) {
					t.Fatalf("error = %T: %v, want ValidationError", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ConvertToModel: %v", err)
			}
			value, state := model.Features.Support(catalogs.ModelFeatureReasoning)
			if value != test.wantValue || state != test.wantState {
				t.Fatalf("reasoning = %t/%v, want %t/%v", value, state, test.wantValue, test.wantState)
			}
		})
	}
}

func TestCapabilityMappingValidationRejectsUnsupportedConfiguration(t *testing.T) {
	for _, test := range []struct {
		name    string
		mapping catalogs.CapabilityMapping
		field   string
	}{
		{name: "source", mapping: catalogs.CapabilityMapping{From: "model_name", To: []catalogs.ModelFeature{catalogs.ModelFeatureReasoning}, Evidence: testCapabilityEvidence}, field: "capability_mappings.from"},
		{name: "target", mapping: catalogs.CapabilityMapping{From: "supports_tools", To: []catalogs.ModelFeature{catalogs.ModelFeatureWebSearch}, Evidence: testCapabilityEvidence}, field: "capability_mappings.to"},
		{name: "evidence", mapping: catalogs.CapabilityMapping{From: "supports_tools", To: []catalogs.ModelFeature{catalogs.ModelFeatureTools}, Evidence: "provider docs"}, field: "capability_mappings.evidence"},
		{name: "combination", mapping: catalogs.CapabilityMapping{From: "supports_tools", To: []catalogs.ModelFeature{catalogs.ModelFeatureTools}, Combine: "last", Evidence: testCapabilityEvidence}, field: "capability_mappings.combine"},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := &catalogs.Provider{
				ID: "invalid", Name: "Invalid",
				Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
					Type: catalogs.EndpointTypeOpenAI, CapabilityMappings: []catalogs.CapabilityMapping{test.mapping},
				}},
			}
			client, err := NewClient(provider)
			if client != nil {
				t.Fatalf("client = %#v, want nil", client)
			}
			var validationErr *pkgerrors.ValidationError
			if !stderrors.As(err, &validationErr) || validationErr.Field != test.field {
				t.Fatalf("error = %T: %v, want %s ValidationError", err, err, test.field)
			}
		})
	}
}

func capabilityCombinationProvider(
	combination catalogs.ProviderCapabilityCombination,
) *catalogs.Provider {
	return &catalogs.Provider{
		ID: "combination", Name: "Combination",
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{
			Type: catalogs.EndpointTypeOpenAI,
			CapabilityMappings: []catalogs.CapabilityMapping{
				{From: "supports_tools", To: []catalogs.ModelFeature{catalogs.ModelFeatureReasoning}, Combine: combination, Evidence: testCapabilityEvidence},
				{From: "supports_reasoning", To: []catalogs.ModelFeature{catalogs.ModelFeatureReasoning}, Combine: combination, Evidence: testCapabilityEvidence},
			},
		}},
	}
}

func boolPointer(value bool) *bool { return &value }
