package catalogs

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestGenerationParameterPresence(t *testing.T) {
	typ := reflect.TypeFor[ModelGeneration]()
	fields := make(map[GenerationParameter]reflect.StructField)
	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.IsExported() && field.Type.Kind() == reflect.Pointer {
			fields[GenerationParameter(strings.Split(field.Tag.Get("json"), ",")[0])] = field
		}
	}
	parameters := PublishedGenerationParameters()
	if len(parameters) != len(fields) {
		t.Fatal("parameter descriptors do not cover the public controls")
	}
	for _, parameter := range parameters {
		field, found := fields[parameter]
		if !found {
			t.Fatal("parameter has no public control")
		}
		for _, format := range []string{"json", "yaml"} {
			for _, state := range []string{"missing", "unknown", "known"} {
				t.Run(string(parameter)+"/"+format+"/"+state, func(t *testing.T) {
					body := `{}`
					if state != "missing" {
						value := "null"
						if state == "known" {
							value = "0"
							if field.Type.Elem().Kind() == reflect.Struct {
								value = `{"min":0,"max":10,"default":0}`
							}
						}
						body = `{"` + string(parameter) + `":` + value + `}`
					}
					var control ModelGeneration
					if err := json.Unmarshal([]byte(body), &control); err != nil {
						t.Fatal(err)
					}
					want := map[string]ValuePresence{"missing": ValueMissing, "unknown": ValueUnknown, "known": ValueKnown}[state]
					for range 3 {
						control = *DeepCopyModel(Model{Generation: &control}).Generation
						var data []byte
						var err error
						if format == "json" {
							data, err = json.Marshal(control)
						} else {
							data, err = yaml.Marshal(control)
						}
						if err != nil {
							t.Fatal(err)
						}
						var raw map[string]any
						if err := yaml.Unmarshal(data, &raw); err != nil {
							t.Fatal(err)
						}
						value, present := raw[string(parameter)]
						if present != (state != "missing") || (present && (value == nil) != (state == "unknown")) {
							t.Fatalf("parameter changed presence in %s", data)
						}
						if format == "json" {
							err = json.Unmarshal(data, &control)
						} else {
							err = yaml.Unmarshal(data, &control)
						}
						if err != nil {
							t.Fatal(err)
						}
						if got := control.ParameterPresence(parameter); got != want {
							t.Fatalf("presence=%v, want %v", got, want)
						}
					}
					if !control.SetParameterUnknown(parameter) || control.ParameterPresence(parameter) != ValueUnknown {
						t.Fatal("unknown control did not replace its value")
					}
					if !control.UnsetParameter(parameter) || control.ParameterPresence(parameter) != ValueMissing {
						t.Fatal("unset control retained its claim")
					}
				})
			}
		}
	}
	parameters[0] = "caller-change"
	if PublishedGenerationParameters()[0] == "caller-change" {
		t.Fatal("caller changed supported parameters")
	}
	for _, control := range []*ModelGeneration{nil, {}} {
		if control.SetParameterUnknown("invalid") || control.UnsetParameter("invalid") || control.ParameterPresence("invalid") != ValueMissing {
			t.Fatal("invalid parameter changed a control")
		}
	}
}
