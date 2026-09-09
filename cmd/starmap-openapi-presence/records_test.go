package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"

	"github.com/goccy/go-yaml"
)

func TestNormalizeNullableCapabilityRecords(t *testing.T) {
	fields := []string{"features", "attachments", "generation", "reasoning", "reasoning_tokens", "verbosity", "tools", "delivery"}
	properties := map[string]any{}
	for _, field := range fields {
		properties[field] = map[string]any{"$ref": "#/components/schemas/Record", "description": "Known record or unknown 量子"}
	}
	target := map[string]any{"type": "object", "properties": map[string]any{"count": map[string]any{"type": "integer"}}}
	doc := map[string]any{"openapi": "3.1.0", "components": map[string]any{"schemas": map[string]any{"catalogs.ModelDefinitionCapabilities": map[string]any{"type": "object", "properties": properties}, "Record": target}}}
	originalJSON, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	originalYAML, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	actualJSON, actualYAML, err := normalize(originalJSON, originalYAML)
	if err != nil {
		t.Fatal(err)
	}
	var actual map[string]any
	if err := json.Unmarshal(actualJSON, &actual); err != nil {
		t.Fatal(err)
	}
	schemas := actual["components"].(map[string]any)["schemas"].(map[string]any)
	if !reflect.DeepEqual(schemas["Record"], target) {
		t.Fatal("referenced record type changed")
	}
	values := schemas["catalogs.ModelDefinitionCapabilities"].(map[string]any)["properties"].(map[string]any)
	for _, field := range fields {
		property := values[field].(map[string]any)
		options, ok := property["anyOf"].([]any)
		if !ok || len(options) != 2 {
			t.Fatalf("%s does not permit both a record and null: %v", field, property)
		}
		if options[0].(map[string]any)["$ref"] != "#/components/schemas/Record" || options[1].(map[string]any)["type"] != "null" {
			t.Fatal("record union changed")
		}
		if _, found := property["$ref"]; found {
			t.Fatal("outer reference still rejects null")
		}
		if property["description"] != "Known record or unknown 量子" {
			t.Fatal("description changed")
		}
	}
	againJSON, againYAML, err := normalize(actualJSON, actualYAML)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actualJSON, againJSON) || !bytes.Equal(actualYAML, againYAML) {
		t.Fatal("normalization is not idempotent")
	}
}

func TestEmbeddedCapabilityNullContract(t *testing.T) {
	data, err := os.ReadFile("../../internal/embedded/openapi/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	properties := doc["components"].(map[string]any)["schemas"].(map[string]any)["catalogs.ModelDefinitionCapabilities"].(map[string]any)["properties"].(map[string]any)
	typ := reflect.TypeFor[catalogs.ModelDefinitionCapabilities]()
	for index := range typ.NumField() {
		field := typ.Field(index)
		if field.PkgPath != "" {
			continue
		}
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		t.Run(name, func(t *testing.T) {
			property, ok := properties[name].(map[string]any)
			if !ok {
				t.Fatal("public capability has no schema")
			}
			var capability catalogs.ModelDefinitionCapabilities
			if err := json.Unmarshal([]byte(`{"`+name+`":null}`), &capability); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(capability)
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &raw); err != nil {
				t.Fatal(err)
			}
			if string(raw[name]) != "null" {
				t.Fatal("codec did not retain the unknown record")
			}
			union, ok := property["anyOf"].([]any)
			if !ok || len(union) != 2 || union[1].(map[string]any)["type"] != "null" {
				t.Fatalf("schema rejects emitted null for %s", name)
			}
			if _, outer := property["$ref"]; outer {
				t.Fatal("outer reference constrains null")
			}
		})
	}
}

func TestNullableCapabilityRefusesConstrainedUnion(t *testing.T) {
	for _, constraint := range []string{"type", "allOf"} {
		t.Run(constraint, func(t *testing.T) {
			property := map[string]any{"anyOf": []any{map[string]any{"$ref": "#/components/schemas/Record"}, map[string]any{"type": "null"}}, constraint: "object"}
			document := map[string]any{"components": map[string]any{"schemas": map[string]any{"catalogs.ModelDefinitionCapabilities": map[string]any{"properties": map[string]any{"features": property}}}}}
			if _, err := markNullableCapabilityRecords(document); err == nil {
				t.Fatal("a constraint that rejects null was accepted")
			}
		})
	}
}
