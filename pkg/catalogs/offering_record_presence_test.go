package catalogs

import (
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestOfferingRecordProjection(t *testing.T) {
	for _, format := range []string{"json", "yaml"} {
		for _, field := range []string{"pricing", "limits", "deprecated_at", "retires_at"} {
			for _, state := range []string{"missing", "unknown", "known"} {
				t.Run(format+"/"+field+"/"+state, func(t *testing.T) {
					body := `{"id":"model","model":"author/model","name":"Model"}`
					if state != "missing" {
						value := "null"
						if state == "known" {
							value = "{}"
							if field == "deprecated_at" || field == "retires_at" {
								value = `"2026-09-01T00:00:00Z"`
							}
							if field == "pricing" {
								value = `{"currency":"USD","tokens":{"input":{"per_1m_tokens":1}}}`
							}
						}
						body = body[:len(body)-1] + `,"` + field + `":` + value + `}`
					}
					var model Model
					if err := json.Unmarshal([]byte(body), &model); err != nil {
						t.Fatal(err)
					}
					builder := NewEmpty()
					setTestReadViewDefinition(t, builder, "model", "Model")
					if err := builder.SetProvider(Provider{ID: "provider", Name: "Provider", Models: map[string]*Model{"model": &model}}); err != nil {
						t.Fatal(err)
					}
					offering, err := mustCatalog(t, builder).Offering("provider", "model")
					if err != nil {
						t.Fatal(err)
					}
					expected := map[string]ValuePresence{"missing": ValueMissing, "unknown": ValueUnknown, "known": ValueKnown}[state]
					for cycle := range 3 {
						offering = copyProviderOffering(offering)
						if got := offering.RecordPresence(ModelRecord(field)); got != expected {
							t.Fatalf("cycle %d presence=%v, want %v", cycle, got, expected)
						}
						var data []byte
						if format == "yaml" {
							data, err = yaml.Marshal(offering)
						} else {
							data, err = json.Marshal(offering)
						}
						if err != nil {
							t.Fatal(err)
						}
						var raw map[string]any
						if err := yaml.Unmarshal(data, &raw); err != nil {
							t.Fatal(err)
						}
						value, present := raw[field]
						if present != (state != "missing") || (present && (value == nil) != (state == "unknown")) {
							t.Fatalf("offering %s=%#v present=%t, want %s", field, value, present, state)
						}
						if format == "yaml" {
							err = yaml.Unmarshal(data, &offering)
						} else {
							err = json.Unmarshal(data, &offering)
						}
						if err != nil {
							t.Fatal(err)
						}
					}
					if err := json.Unmarshal([]byte(`{}`), &offering); err != nil {
						t.Fatal(err)
					}
					if got := offering.RecordPresence(ModelRecord(field)); got != ValueMissing {
						t.Fatalf("reused destination retains %v", got)
					}
				})
			}
		}
	}
}

func TestOfferingUnknownRecordPreservesRequestBody(t *testing.T) {
	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			var offering ProviderOffering
			payload := []byte(`{"pricing":null,"modes":{"batch":{"request":{"body":{"limit":9223372036854775807,"enabled":false}}}}}`)
			if err := json.Unmarshal(payload, &offering); err != nil {
				t.Fatal(err)
			}
			var data []byte
			var err error
			if format == "yaml" {
				data, err = yaml.Marshal(offering)
			} else {
				data, err = json.Marshal(offering)
			}
			if err != nil {
				t.Fatal(err)
			}
			var restored ProviderOffering
			if format == "yaml" {
				err = yaml.Unmarshal(data, &restored)
			} else {
				err = json.Unmarshal(data, &restored)
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := string(restored.Modes["batch"].Request.Body["limit"]); got != "9223372036854775807" {
				t.Fatalf("request limit changed to %s", got)
			}
			if got := string(restored.Modes["batch"].Request.Body["enabled"]); got != "false" {
				t.Fatalf("request Boolean changed to %s", got)
			}
			if restored.RecordPresence(ModelRecordPricing) != ValueUnknown {
				t.Fatal("unknown price changed")
			}
			if restored.RecordPresence(ModelRecordFeatures) != ValueMissing {
				t.Fatal("intrinsic capability became an offering record")
			}
		})
	}
}

func TestOfferingUnknownLimitPreservesKnownPricing(t *testing.T) {
	var offering ProviderOffering
	if err := json.Unmarshal([]byte(`{"limits":null,"pricing":{"currency":"USD","tokens":{"input":{"per_1m_tokens":1.25}}}}`), &offering); err != nil {
		t.Fatal(err)
	}
	assertOfferingRoundTrip(t, offering)
}
