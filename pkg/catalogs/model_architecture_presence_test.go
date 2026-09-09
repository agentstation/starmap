package catalogs

import (
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestArchitecturePresenceRoundTrip(t *testing.T) {
	for _, field := range []string{"quantized", "fine_tuned"} {
		for _, claim := range []string{"missing", "null", "false", "true"} {
			for _, format := range []string{"json", "yaml"} {
				t.Run(field+"/"+claim+"/"+format, func(t *testing.T) {
					body := "{}"
					if claim != "missing" {
						body = `{"` + field + `":` + claim + `}`
					}
					var architecture ModelArchitecture
					var data []byte
					var err error
					if format == "json" {
						err = json.Unmarshal([]byte(body), &architecture)
						if err == nil {
							data, err = json.Marshal(architecture)
						}
					} else {
						err = yaml.Unmarshal([]byte(body), &architecture)
						if err == nil {
							data, err = yaml.Marshal(architecture)
						}
					}
					if err != nil {
						t.Fatal(err)
					}
					var values map[string]any
					if format == "json" {
						err = json.Unmarshal(data, &values)
					} else {
						err = yaml.Unmarshal(data, &values)
					}
					if err != nil {
						t.Fatal(err)
					}
					actual, found := values[field]
					if claim == "missing" {
						if found {
							t.Errorf("missing claim became %v", actual)
						}
					} else {
						var expected any
						if err := json.Unmarshal([]byte(claim), &expected); err != nil {
							t.Fatal(err)
						}
						if !found || actual != expected {
							t.Errorf("claim=%v/%t, want %s", actual, found, claim)
						}
					}
				})
			}
		}
	}
}

func TestArchitecturePresenceDefinition(t *testing.T) {
	for _, field := range []string{"quantized", "fine_tuned"} {
		for _, claim := range []string{"missing", "null", "false", "true"} {
			t.Run(field+"/"+claim, func(t *testing.T) {
				body := "{}"
				if claim != "missing" {
					body = `{"` + field + `":` + claim + `}`
				}
				architecture := &ModelArchitecture{}
				if err := json.Unmarshal([]byte(body), architecture); err != nil {
					t.Fatal(err)
				}
				builder := NewEmpty()
				if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
					t.Fatal(err)
				}
				if err := builder.SetAuthorModel("author", Model{ID: "model", Name: "Model", Authors: []Author{{ID: "author", Name: "Author"}}, Metadata: &ModelMetadata{Architecture: architecture}}); err != nil {
					t.Fatal(err)
				}
				definition, err := mustCatalog(t, builder).Definition("author/model")
				if err != nil {
					t.Fatal(err)
				}
				data, err := json.Marshal(definition.Weights.Architecture)
				if err != nil {
					t.Fatal(err)
				}
				var values map[string]json.RawMessage
				if err := json.Unmarshal(data, &values); err != nil {
					t.Fatal(err)
				}
				actual, found := values[field]
				if claim == "missing" {
					if found {
						t.Errorf("missing claim became %s", actual)
					}
				} else if !found || string(actual) != claim {
					t.Errorf("claim=%s/%t, want %s", actual, found, claim)
				}
			})
		}
	}
}
