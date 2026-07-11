package catalogs

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/utc"
	"github.com/goccy/go-yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestModelDefinitionRoundTripPreservesIntrinsicFacts(t *testing.T) {
	root := ModelDefinitionID("foundation-model")
	definition := ModelDefinition{
		ID:          "shared-model",
		Name:        "Shared Model",
		AuthorIDs:   []AuthorID{"author-a", "author-b"},
		Description: "Provider-independent model definition",
		Metadata: ModelDefinitionMetadata{
			ReleaseDate: utc.New(time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)),
			Tags:        []ModelTag{"chat"},
		},
		Lineage: ModelDefinitionLineage{Family: "shared", Root: &root},
		Weights: ModelDefinitionWeights{
			Open: true,
			Architecture: &ModelArchitecture{
				Type: ArchitectureTypeTransformer,
			},
		},
		Capabilities: ModelDefinitionCapabilities{
			Features: &ModelFeatures{ToolCalls: true, Streaming: true},
			Tools:    &ModelTools{ToolChoices: []ToolChoice{"auto"}},
			Delivery: &ModelDelivery{},
		},
	}
	if err := definition.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	jsonData, err := json.Marshal(definition)
	if err != nil {
		t.Fatalf("Marshal JSON: %v", err)
	}
	var fromJSON ModelDefinition
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("Unmarshal JSON: %v", err)
	}
	if diff := cmp.Diff(definition, fromJSON); diff != "" {
		t.Fatalf("JSON round trip (-want +got):\n%s", diff)
	}

	yamlData, err := yaml.Marshal(definition)
	if err != nil {
		t.Fatalf("Marshal YAML: %v", err)
	}
	var fromYAML ModelDefinition
	if err := yaml.Unmarshal(yamlData, &fromYAML); err != nil {
		t.Fatalf("Unmarshal YAML: %v", err)
	}
	if diff := cmp.Diff(definition, fromYAML, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("YAML round trip (-want +got):\n%s", diff)
	}
}

func TestModelDefinitionExcludesProviderOfferingFacts(t *testing.T) {
	typeOfDefinition := reflect.TypeFor[ModelDefinition]()
	for _, forbidden := range []string{
		"ProviderID", "ProviderModelID", "Pricing", "Limits", "Availability",
		"Regions", "Endpoint", "Lifecycle", "Modes", "Request",
	} {
		if _, found := typeOfDefinition.FieldByName(forbidden); found {
			t.Fatalf("ModelDefinition exposes provider-specific field %s", forbidden)
		}
	}
	for _, required := range []string{"ID", "AuthorIDs", "Metadata", "Lineage", "Weights", "Capabilities"} {
		if _, found := typeOfDefinition.FieldByName(required); !found {
			t.Fatalf("ModelDefinition missing intrinsic field %s", required)
		}
	}
	typeOfOffering := reflect.TypeFor[ProviderOffering]()
	for _, forbidden := range []string{"AuthorIDs", "Metadata", "Lineage", "Weights", "Capabilities"} {
		if _, found := typeOfOffering.FieldByName(forbidden); found {
			t.Fatalf("ProviderOffering exposes definition field %s", forbidden)
		}
	}
}

func TestModelDefinitionValidation(t *testing.T) {
	valid := ModelDefinition{ID: "model", Name: "Model", AuthorIDs: []AuthorID{"author"}}
	tests := []struct {
		name   string
		mutate func(*ModelDefinition)
	}{
		{name: "ID", mutate: func(d *ModelDefinition) { d.ID = "" }},
		{name: "name", mutate: func(d *ModelDefinition) { d.Name = "" }},
		{name: "empty author", mutate: func(d *ModelDefinition) { d.AuthorIDs = []AuthorID{""} }},
		{name: "duplicate author", mutate: func(d *ModelDefinition) { d.AuthorIDs = []AuthorID{"author", "author"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition := valid
			test.mutate(&definition)
			if err := definition.Validate(); err == nil {
				t.Fatal("Validate returned nil error")
			}
		})
	}
	unknownAuthorship := valid
	unknownAuthorship.AuthorIDs = nil
	if err := unknownAuthorship.Validate(); err != nil {
		t.Fatalf("unknown authorship must remain representable: %v", err)
	}
}
