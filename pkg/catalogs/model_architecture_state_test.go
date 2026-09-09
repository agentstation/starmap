package catalogs

import (
	"encoding/json"
	"testing"
)

func TestArchitectureClaimStateTransitions(t *testing.T) {
	for _, field := range []struct {
		name    string
		get     func(*ModelArchitecture) (bool, ValuePresence)
		set     func(*ModelArchitecture, bool)
		unknown func(*ModelArchitecture)
		unset   func(*ModelArchitecture)
	}{
		{"quantized", (*ModelArchitecture).QuantizedValue, (*ModelArchitecture).SetQuantized, (*ModelArchitecture).SetQuantizedUnknown, (*ModelArchitecture).UnsetQuantized},
		{"fine_tuned", (*ModelArchitecture).FineTunedValue, (*ModelArchitecture).SetFineTuned, (*ModelArchitecture).SetFineTunedUnknown, (*ModelArchitecture).UnsetFineTuned},
	} {
		t.Run(field.name, func(t *testing.T) {
			if v, p := field.get(nil); v || p != ValueMissing {
				t.Fatalf("nil=%v/%v", v, p)
			}
			a := &ModelArchitecture{}
			for _, step := range []struct {
				name     string
				apply    func()
				value    bool
				presence ValuePresence
				wire     string
			}{
				{"initial", func() {}, false, ValueMissing, "{}"},
				{"true", func() { field.set(a, true) }, true, ValueKnown, `{"` + field.name + `":true}`},
				{"unknown", func() { field.unknown(a) }, false, ValueUnknown, `{"` + field.name + `":null}`},
				{"false", func() { field.set(a, false) }, false, ValueKnown, `{"` + field.name + `":false}`},
				{"unset", func() { field.unset(a) }, false, ValueMissing, "{}"},
			} {
				t.Run(step.name, func(t *testing.T) {
					step.apply()
					if v, p := field.get(a); v != step.value || p != step.presence {
						t.Fatalf("got %v/%v, want %v/%v", v, p, step.value, step.presence)
					}
					data, err := json.Marshal(a)
					if err != nil {
						t.Fatal(err)
					}
					if string(data) != step.wire {
						t.Fatalf("wire=%s, want %s", data, step.wire)
					}
					copy := *a
					if !copy.Equal(*a) {
						t.Fatal("copy changed presence")
					}
					field.unknown(&copy)
					if step.presence != ValueUnknown && copy.Equal(*a) {
						t.Fatal("equality ignores presence")
					}
					if v, p := field.get(a); v != step.value || p != step.presence {
						t.Fatal("copy mutation changed original")
					}
				})
			}
		})
	}
}
