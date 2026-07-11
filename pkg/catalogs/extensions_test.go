package catalogs

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestSourceExtensionsCopyDeepCopiesNestedFields(t *testing.T) {
	extensions := SourceExtensions{
		"models.dev": {
			Fields: map[string]any{
				"shape": "chat",
				"nested": map[string]any{
					"enabled": true,
					"labels":  []any{"fast", "priority"},
				},
				"ids": []string{"model-a", "model-b"},
			},
		},
	}

	copied := extensions.Copy()
	copied["models.dev"].Fields["shape"] = "responses"
	copied["models.dev"].Fields["ids"].([]string)[0] = "changed"
	nested := copied["models.dev"].Fields["nested"].(map[string]any)
	nested["enabled"] = false
	nested["labels"].([]any)[0] = "changed"

	if extensions["models.dev"].Fields["shape"] != "chat" {
		t.Fatal("top-level extension field was shared between original and copy")
	}
	if extensions["models.dev"].Fields["ids"].([]string)[0] != "model-a" {
		t.Fatal("extension string slice was shared between original and copy")
	}
	originalNested := extensions["models.dev"].Fields["nested"].(map[string]any)
	if originalNested["enabled"] != true {
		t.Fatal("extension nested map was shared between original and copy")
	}
	if originalNested["labels"].([]any)[0] != "fast" {
		t.Fatal("extension nested slice was shared between original and copy")
	}
}

func TestSourceExtensionsCopyDeepCopiesTypedNestedMaps(t *testing.T) {
	extensions := SourceExtensions{
		"provider": {
			Fields: map[string]any{
				"headers": map[string]string{
					"X-Mode": "fast",
				},
				"nested": map[string]any{
					"body": map[string]string{
						"service_tier": "priority",
					},
					"matrix": [][]string{{"a", "b"}},
				},
			},
		},
	}

	copied := extensions.Copy()
	copied["provider"].Fields["headers"].(map[string]string)["X-Mode"] = "standard"
	nested := copied["provider"].Fields["nested"].(map[string]any)
	nested["body"].(map[string]string)["service_tier"] = "default"
	nested["matrix"].([][]string)[0][0] = "changed"

	originalFields := extensions["provider"].Fields
	if originalFields["headers"].(map[string]string)["X-Mode"] != "fast" {
		t.Fatal("typed extension map was shared between original and copy")
	}
	originalNested := originalFields["nested"].(map[string]any)
	if originalNested["body"].(map[string]string)["service_tier"] != "priority" {
		t.Fatal("typed nested extension map was shared between original and copy")
	}
	if originalNested["matrix"].([][]string)[0][0] != "a" {
		t.Fatal("typed nested extension slice was shared between original and copy")
	}
}

func TestSourceExtensionsMarshalRoundTrip(t *testing.T) {
	model := Model{
		ID:   "extension-model",
		Name: "Extension Model",
		Extensions: SourceExtensions{
			"models.dev": {
				Fields: map[string]any{
					"provider_shape": "responses",
					"flags":          []any{"experimental", "priority"},
					"capabilities": map[string]any{
						"context_management": true,
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(model)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	var jsonModel Model
	if err := json.Unmarshal(jsonData, &jsonModel); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(model.Extensions, jsonModel.Extensions) {
		t.Fatalf("json extensions = %#v, want %#v", jsonModel.Extensions, model.Extensions)
	}

	yamlData, err := yaml.Marshal(model)
	if err != nil {
		t.Fatalf("yaml marshal failed: %v", err)
	}
	if !strings.Contains(string(yamlData), "extensions:") ||
		!strings.Contains(string(yamlData), "models.dev:") ||
		!strings.Contains(string(yamlData), "provider_shape: responses") {
		t.Fatalf("yaml did not include expected extension fields:\n%s", yamlData)
	}
	var yamlModel Model
	if err := yaml.Unmarshal(yamlData, &yamlModel); err != nil {
		t.Fatalf("yaml unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(model.Extensions, yamlModel.Extensions) {
		t.Fatalf("yaml extensions = %#v, want %#v", yamlModel.Extensions, model.Extensions)
	}
}

func TestNormalizeSourceExtensionsProducesRoundTripStableTypes(t *testing.T) {
	model := Model{
		ID:   "extension-model",
		Name: "Extension Model",
		Extensions: NormalizeSourceExtensions(SourceExtensions{
			"models.dev": {
				Fields: map[string]any{
					"values": []string{"xhigh", "auto"},
					"limits": map[string]int{
						"min": 1,
						"max": 3,
					},
					"permissions": []map[string]any{{
						"created": int64(1733447754),
					}},
				},
			},
		}),
	}

	jsonData, err := json.Marshal(model)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	var jsonModel Model
	if err := json.Unmarshal(jsonData, &jsonModel); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(model.Extensions, jsonModel.Extensions) {
		t.Fatalf("json extensions = %#v, want %#v", jsonModel.Extensions, model.Extensions)
	}

	yamlData, err := yaml.Marshal(model)
	if err != nil {
		t.Fatalf("yaml marshal failed: %v", err)
	}
	var yamlModel Model
	if err := yaml.Unmarshal(yamlData, &yamlModel); err != nil {
		t.Fatalf("yaml unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(model.Extensions, yamlModel.Extensions) {
		t.Fatalf("yaml extensions = %#v, want %#v", yamlModel.Extensions, model.Extensions)
	}
}
