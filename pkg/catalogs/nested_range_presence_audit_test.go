package catalogs

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNestedGenerationRangePresenceAudit(t *testing.T) {
	for _, parameter := range []string{"temperature", "top_k"} {
		for _, scenario := range []struct {
			name, value string
		}{
			{"unknown parameter", "null"},
			{"unknown bounds", `{"min":null,"max":10,"default":null}`},
			{"missing bounds", `{"max":10}`},
			{"known zero bounds", `{"min":0,"max":10,"default":0}`},
		} {
			t.Run(parameter+"/"+scenario.name, func(t *testing.T) {
				input := []byte(`{"id":"model","name":"Model","generation":{"` + parameter + `":` + scenario.value + `}}`)
				var model Model
				if err := json.Unmarshal(input, &model); err != nil {
					t.Fatal(err)
				}
				output, err := json.Marshal(DeepCopyModel(model))
				if err != nil {
					t.Fatal(err)
				}
				var raw struct {
					Generation map[string]json.RawMessage `json:"generation"`
				}
				if err := json.Unmarshal(output, &raw); err != nil {
					t.Fatal(err)
				}
				actual, present := raw.Generation[parameter]
				if !present {
					t.Fatalf("generation parameter %s lost its explicit claim", parameter)
				}
				var want, got any
				if err := json.Unmarshal([]byte(scenario.value), &want); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(actual, &got); err != nil {
					t.Fatal(err)
				}
				wantJSON, _ := json.Marshal(want)
				gotJSON, _ := json.Marshal(got)
				if !bytes.Equal(wantJSON, gotJSON) {
					t.Fatalf("range presence changed: got %s, want %s", gotJSON, wantJSON)
				}
			})
		}
	}
}
