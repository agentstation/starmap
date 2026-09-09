package catalogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestOptionalModelRecordCodecStates(t *testing.T) {
	fields := []string{"deprecated_at", "retires_at", "metadata", "lineage", "features", "attachments", "generation", "reasoning", "reasoning_tokens", "verbosity", "tools", "response", "pricing", "limits"}
	for _, format := range []string{"json", "yaml"} {
		for _, field := range fields {
			for _, state := range []string{"missing", "unknown", "known"} {
				t.Run(format+"/"+field+"/"+state, func(t *testing.T) {
					payload := `{"id":"record-model","name":"Record model"}`
					if state != "missing" {
						value := `{}`
						if field == "deprecated_at" || field == "retires_at" {
							value = `"2026-09-01T00:00:00Z"`
						}
						if state == "unknown" {
							value = `null`
						}
						payload = fmt.Sprintf(`{"id":"record-model","name":"Record model",%q:%s}`, field, value)
					}
					var model Model
					decode := func(data []byte, model *Model) error { return json.Unmarshal(data, model) }
					encode := func(model Model) ([]byte, error) { return json.Marshal(model) }
					if format == "yaml" {
						decode = func(data []byte, model *Model) error { return yaml.Unmarshal(data, model) }
						encode = func(model Model) ([]byte, error) { return yaml.Marshal(model) }
					}
					if err := decode([]byte(payload), &model); err != nil {
						t.Fatal(err)
					}
					for cycle := range 3 {
						model = DeepCopyModel(model)
						data, err := encode(model)
						if err != nil {
							t.Fatal(err)
						}
						var raw map[string]any
						if err := yaml.Unmarshal(data, &raw); err != nil {
							t.Fatal(err)
						}
						value, present := raw[field]
						if present != (state != "missing") || (present && (value == nil) != (state == "unknown")) {
							t.Fatalf("cycle %d: %s record = %#v, present=%t, want %s; payload=%s", cycle, field, value, present, state, data)
						}
						if err := decode(data, &model); err != nil {
							t.Fatal(err)
						}
					}
					if err := decode([]byte(`{"id":"record-model","name":"Record model"}`), &model); err != nil {
						t.Fatal(err)
					}
					data, err := encode(model)
					if err != nil {
						t.Fatal(err)
					}
					var raw map[string]any
					if err := yaml.Unmarshal(data, &raw); err != nil {
						t.Fatal(err)
					}
					if _, present := raw[field]; present {
						t.Fatal("reused destination retained an earlier record claim")
					}
				})
			}
		}
	}
}

func TestOptionalModelRecordLegacyBytes(t *testing.T) {
	var model Model
	input := []byte(`{"id":"legacy","name":"Legacy","features":{"tools":false},"limits":{"output_tokens":0},"description":""}`)
	if err := json.Unmarshal(input, &model); err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("legacy_payload=%s", before)
	var roundtrip Model
	if err := json.Unmarshal(before, &roundtrip); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(roundtrip)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("legacy bytes changed: %s -> %s", before, after)
	}
}
