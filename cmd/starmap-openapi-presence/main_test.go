package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func schemaFixture(t *testing.T, description string, kind any, marker any) ([]byte, []byte) {
	t.Helper()
	property := map[string]any{"description": description, "type": kind, nullableAnnotation: marker}
	doc := map[string]any{"openapi": "3.1.0", "components": map[string]any{"schemas": map[string]any{"catalogs.ModelArchitecture": map[string]any{"type": "object", "properties": map[string]any{"quantized": property, "fine_tuned": property}}}}, "info": map[string]any{"title": "Catalog", "version": "1"}}
	j, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	y, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return j, y
}

func TestNormalizeNullableBooleanContract(t *testing.T) {
	for _, text := range []string{"Boolean claim", "量子 Boolean claim"} {
		t.Run(text, func(t *testing.T) {
			j, y := schemaFixture(t, text, "boolean", true)
			gotJ, gotY, err := normalize(j, y)
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal(gotJ, &doc); err != nil {
				t.Fatal(err)
			}
			props := doc["components"].(map[string]any)["schemas"].(map[string]any)["catalogs.ModelArchitecture"].(map[string]any)["properties"].(map[string]any)
			for _, name := range []string{"quantized", "fine_tuned"} {
				p := props[name].(map[string]any)
				kinds, ok := p["type"].([]any)
				if !ok || len(kinds) != 2 || kinds[0] != "boolean" || kinds[1] != "null" {
					t.Fatalf("%s type=%v", name, p["type"])
				}
				if p["description"] != text {
					t.Fatal("description changed")
				}
			}
			againJ, againY, err := normalize(gotJ, gotY)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(againJ, gotJ) || !bytes.Equal(againY, gotY) {
				t.Fatal("normalization is not idempotent")
			}
			if !bytes.Equal(bytes.ReplaceAll(gotY, []byte(`[boolean, "null"]`), []byte("boolean")), y) {
				t.Fatal("unrelated YAML bytes changed")
			}
		})
	}
}

func TestNormalizeRefusesInvalidContracts(t *testing.T) {
	for _, tc := range []struct {
		name         string
		kind, marker any
	}{
		{"wrong scalar", "string", true}, {"missing type", nil, true}, {"invalid marker", "boolean", "true"}, {"false marker", "boolean", false}, {"wrong union", []string{"boolean", "string"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j, y := schemaFixture(t, "claim", tc.kind, tc.marker)
			if _, _, err := normalize(j, y); err == nil {
				t.Fatal("invalid contract was accepted")
			}
		})
	}
	j, y := schemaFixture(t, "claim", "boolean", true)
	for _, tc := range []struct {
		name string
		j, y []byte
	}{
		{"mismatched formats", j, bytes.Replace(y, []byte("title: Catalog"), []byte("title: Different"), 1)},
		{"invalid JSON", []byte("{"), y}, {"invalid YAML", j, []byte("[broken")},
		{"old specification", bytes.Replace(j, []byte("3.1.0"), []byte("3.0.0"), 1), bytes.Replace(y, []byte("3.1.0"), []byte("3.0.0"), 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := normalize(tc.j, tc.y); err == nil {
				t.Fatal("invalid document was accepted")
			}
		})
	}
}

func TestRunPreservesFilesOnValidationFailure(t *testing.T) {
	j, y := schemaFixture(t, "claim", "string", true)
	dir := t.TempDir()
	jp := filepath.Join(dir, "spec.json")
	yp := filepath.Join(dir, "spec.yaml")
	for path, data := range map[string][]byte{jp: j, yp: y} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := run([]string{jp, yp}); err == nil {
		t.Fatal("invalid schema was accepted")
	}
	for path, want := range map[string][]byte{jp: j, yp: y} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatal("validation failure changed input")
		}
	}
}

func TestUnannotatedSchemaRetainsBytes(t *testing.T) {
	j := []byte(`{"openapi":"3.1.0","components":{"schemas":{}}}`)
	y := []byte("openapi: 3.1.0\ncomponents:\n  schemas: {}\n")
	gotJ, gotY, err := normalize(j, y)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(j, gotJ) || !bytes.Equal(y, gotY) {
		t.Fatal("unannotated document changed")
	}
}

func TestArchitectureSchemaIncludesUnknown(t *testing.T) {
	path := filepath.Join("..", "..", "internal", "embedded", "openapi", "openapi.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	props := doc["components"].(map[string]any)["schemas"].(map[string]any)["catalogs.ModelArchitecture"].(map[string]any)["properties"].(map[string]any)
	for _, name := range []string{"quantized", "fine_tuned"} {
		encoded, err := json.Marshal(props[name].(map[string]any)["type"])
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(encoded)) != `["boolean","null"]` {
			t.Errorf("%s schema rejects explicit unknown: %s", name, encoded)
		}
	}
}

func TestNormalizePreservesExactNumericBounds(t *testing.T) {
	j, y := schemaFixture(t, "claim", "boolean", true)
	j = bytes.Replace(j, []byte(`"info":{`), []byte(`"x-bound":9007199254740993,"info":{`), 1)
	y = append([]byte("x-bound: 9007199254740993\n"), y...)
	gotJ, gotY, err := normalize(j, y)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{gotJ, gotY} {
		if !bytes.Contains(data, []byte("9007199254740993")) {
			t.Fatal("unrelated numeric bound changed")
		}
	}
}
