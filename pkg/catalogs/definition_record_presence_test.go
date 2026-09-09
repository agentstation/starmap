package catalogs

import (
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestOptionalRecordDefinitionProjection(t *testing.T) {
	fields := []struct {
		model      string
		definition string
	}{
		{"features", "features"}, {"attachments", "attachments"},
		{"generation", "generation"}, {"reasoning", "reasoning"},
		{"reasoning_tokens", "reasoning_tokens"}, {"verbosity", "verbosity"},
		{"tools", "tools"}, {"response", "delivery"},
	}
	for _, format := range []string{"json", "yaml"} {
		for _, field := range fields {
			for _, state := range []string{"missing", "unknown", "known"} {
				t.Run(format+"/"+field.model+"/"+state, func(t *testing.T) {
					body := `{"id":"model","name":"Model","authors":[{"id":"author","name":"Author"}]}`
					if state != "missing" {
						value := "null"
						if state == "known" {
							value = "{}"
						}
						body = body[:len(body)-1] + `,"` + field.model + `":` + value + `}`
					}
					var model Model
					if err := json.Unmarshal([]byte(body), &model); err != nil {
						t.Fatal(err)
					}
					if state == "known" && (field.model == "reasoning" || field.model == "reasoning_tokens") {
						model.Features = &ModelFeatures{}
						model.Features.SetSupport(ModelFeatureReasoning, true)
					}
					builder := NewEmpty()
					if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
						t.Fatal(err)
					}
					if err := builder.SetAuthorModel("author", model); err != nil {
						t.Fatal(err)
					}
					definition, err := mustCatalog(t, builder).Definition("author/model")
					if err != nil {
						t.Fatal(err)
					}
					expected := map[string]ValuePresence{"missing": ValueMissing, "unknown": ValueUnknown, "known": ValueKnown}[state]
					for cycle := range 3 {
						definition = copyModelDefinition(definition)
						if got := definition.Capabilities.RecordPresence(ModelRecord(field.model)); got != expected {
							t.Fatalf("cycle %d presence=%v, want %v", cycle, got, expected)
						}
						var data []byte
						if format == "yaml" {
							data, err = yaml.Marshal(definition.Capabilities)
						} else {
							data, err = json.Marshal(definition.Capabilities)
						}
						if err != nil {
							t.Fatal(err)
						}
						var raw map[string]any
						if err := yaml.Unmarshal(data, &raw); err != nil {
							t.Fatal(err)
						}
						value, present := raw[field.definition]
						if present != (state != "missing") || (present && (value == nil) != (state == "unknown")) {
							t.Fatalf("definition %s = %#v, present=%t, want %s; payload=%s", field.definition, value, present, state, data)
						}
						if format == "yaml" {
							err = yaml.Unmarshal(data, &definition.Capabilities)
						} else {
							err = json.Unmarshal(data, &definition.Capabilities)
						}
						if err != nil {
							t.Fatal(err)
						}
					}
					if err := json.Unmarshal([]byte(`{}`), &definition.Capabilities); err != nil {
						t.Fatal(err)
					}
					if got := definition.Capabilities.RecordPresence(ModelRecord(field.model)); got != ValueMissing {
						t.Fatalf("reused destination retains %v", got)
					}
				})
			}
		}
	}
}

func TestDefinitionUnknownRecordsPreserveIntegerPrecision(t *testing.T) {
	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			var capabilities ModelDefinitionCapabilities
			if err := json.Unmarshal([]byte(`{"attachments":null}`), &capabilities); err != nil {
				t.Fatal(err)
			}
			maximum := int(^uint(0) >> 1)
			capabilities.ReasoningTokens = &IntRange{Max: maximum}
			var data []byte
			var err error
			if format == "yaml" {
				data, err = yaml.Marshal(capabilities)
			} else {
				data, err = json.Marshal(capabilities)
			}
			if err != nil {
				t.Fatal(err)
			}
			var restored ModelDefinitionCapabilities
			if format == "yaml" {
				err = yaml.Unmarshal(data, &restored)
			} else {
				err = json.Unmarshal(data, &restored)
			}
			if err != nil {
				t.Fatal(err)
			}
			if restored.ReasoningTokens == nil || restored.ReasoningTokens.Max != maximum {
				t.Fatalf("integer changed in %s", data)
			}
			if restored.RecordPresence(ModelRecordAttachments) != ValueUnknown {
				t.Fatal("unknown attachment record changed")
			}
			restored.Attachments = &ModelAttachments{}
			if restored.RecordPresence(ModelRecordAttachments) != ValueKnown {
				t.Fatal("known replacement does not override unknown record")
			}
			if restored.RecordPresence(ModelRecordPricing) != ValueMissing {
				t.Fatal("provider price became an intrinsic capability")
			}
		})
	}
}
