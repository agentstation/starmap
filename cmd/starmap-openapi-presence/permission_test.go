package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestNormalizePermissionMediaType(t *testing.T) {
	for _, description := range []string{"Permission receipt", "許可 receipt"} {
		t.Run(description, func(t *testing.T) {
			response := map[string]any{"description": description, "content": map[string]any{
				"application/json":  map[string]any{"schema": map[string]any{"$ref": permissionSchemaRef}},
				permissionMediaType: map[string]any{"schema": map[string]any{"type": "string"}},
			}}
			doc := map[string]any{"openapi": "3.1.0", "paths": map[string]any{permissionPath: map[string]any{"get": map[string]any{"responses": map[string]any{"200": response, "503": map[string]any{"description": "Unavailable"}}}}}}
			j, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			y, err := yaml.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			gotJ, gotY, err := normalize(j, y)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(gotJ, &got); err != nil {
				t.Fatal(err)
			}
			responses := got["paths"].(map[string]any)[permissionPath].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)
			body := responses["200"].(map[string]any)
			content := body["content"].(map[string]any)
			if len(content) != 1 || content[permissionMediaType] == nil {
				t.Fatalf("wrong media types: %+v", content)
			}
			schema := content[permissionMediaType].(map[string]any)["schema"].(map[string]any)
			if schema["$ref"] != permissionSchemaRef || body["description"] != description || responses["503"].(map[string]any)["description"] != "Unavailable" {
				t.Fatal("normalization changed a response contract")
			}
			againJ, againY, err := normalize(gotJ, gotY)
			if err != nil || !bytes.Equal(againJ, gotJ) || !bytes.Equal(againY, gotY) {
				t.Fatalf("normalization is not idempotent: %v", err)
			}
		})
	}
}

func TestNormalizePermissionRefusesUnexpectedSchema(t *testing.T) {
	doc := map[string]any{"paths": map[string]any{permissionPath: map[string]any{"get": map[string]any{"responses": map[string]any{"200": map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "number"}}}}}}}}}
	before, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := normalizePermissionContent(doc); changed || err == nil {
		t.Fatal("unexpected generator output was accepted")
	}
	after, err := json.Marshal(doc)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("refusal changed the document")
	}
}
