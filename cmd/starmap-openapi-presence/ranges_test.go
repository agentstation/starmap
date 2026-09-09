package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestEmbeddedGenerationParameterNullContract(t *testing.T) {
	data, err := os.ReadFile("../../internal/embedded/openapi/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)
	generation := schemas["catalogs.ModelGeneration"].(map[string]any)["properties"].(map[string]any)
	typ := reflect.TypeFor[catalogs.ModelGeneration]()
	for index := range typ.NumField() {
		field := typ.Field(index)
		if !field.IsExported() {
			continue
		}
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		t.Run(name, func(t *testing.T) {
			var value catalogs.ModelGeneration
			if err := json.Unmarshal([]byte(`{"`+name+`":null}`), &value); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &raw); err != nil {
				t.Fatal(err)
			}
			if string(raw[name]) != "null" {
				t.Fatal("codec did not retain null")
			}
			assertSchemaAllowsNull(t, generation[name].(map[string]any))
		})
	}
	for _, schemaName := range []string{"catalogs.FloatRange", "catalogs.IntRange"} {
		fields := schemas[schemaName].(map[string]any)["properties"].(map[string]any)
		for _, name := range []string{"min", "max", "default"} {
			t.Run(schemaName+"/"+name, func(t *testing.T) { assertSchemaAllowsNull(t, fields[name].(map[string]any)) })
		}
	}
}

func assertSchemaAllowsNull(t *testing.T, property map[string]any) {
	t.Helper()
	if _, exists := property["$ref"]; exists {
		t.Fatal("outer reference rejects null")
	}
	if types, ok := property["type"].([]any); ok {
		for _, item := range types {
			if item == "null" {
				return
			}
		}
	}
	if arms, ok := property["anyOf"].([]any); ok {
		for _, item := range arms {
			if schema, ok := item.(map[string]any); ok && schema["type"] == "null" {
				return
			}
		}
	}
	t.Fatal("schema rejects emitted null")
}
