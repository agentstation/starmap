package catalogs

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
)

type numericRange[T int | float64] interface {
	Value(RangeField) (T, ValuePresence)
	SetValue(RangeField, T) bool
	SetValueUnknown(RangeField) bool
	UnsetValue(RangeField) bool
}

func TestParameterRangePresence(t *testing.T) {
	t.Run("float", func(t *testing.T) {
		checkParameterRange(t, func() numericRange[float64] { return &FloatRange{} })
	})
	t.Run("integer", func(t *testing.T) {
		checkParameterRange(t, func() numericRange[int] { return &IntRange{} })
	})
}

func checkParameterRange[T int | float64](t *testing.T, create func() numericRange[T]) {
	t.Helper()
	for _, field := range []RangeField{RangeMinimum, RangeMaximum, RangeDefault} {
		for _, format := range []string{"json", "yaml"} {
			for _, state := range []ValuePresence{ValueKnown, ValueUnknown, ValueMissing} {
				t.Run(string(field)+"/"+format+"/"+map[ValuePresence]string{ValueKnown: "known", ValueUnknown: "unknown", ValueMissing: "missing"}[state], func(t *testing.T) {
					r := create()
					value, presence := r.Value(field)
					if value != 0 || presence != ValueKnown {
						t.Fatal("legacy zero must remain known")
					}
					switch state {
					case ValueUnknown:
						r.SetValueUnknown(field)
					case ValueMissing:
						r.UnsetValue(field)
					}
					for range 3 {
						var data []byte
						var err error
						if format == "json" {
							data, err = json.Marshal(r)
						} else {
							data, err = yaml.Marshal(r)
						}
						if err != nil {
							t.Fatal(err)
						}
						var raw map[string]any
						if err := yaml.Unmarshal(data, &raw); err != nil {
							t.Fatal(err)
						}
						entry, found := raw[string(field)]
						if found != (state != ValueMissing) || (found && (entry == nil) != (state == ValueUnknown)) {
							t.Fatalf("field %s changed presence in %s", field, data)
						}
						if format == "json" {
							err = json.Unmarshal(data, r)
						} else {
							err = yaml.Unmarshal(data, r)
						}
						if err != nil {
							t.Fatal(err)
						}
						if _, got := r.Value(field); got != state {
							t.Fatalf("presence=%v, want %v", got, state)
						}
					}
					if !r.SetValue(field, 0) {
						t.Fatal("zero replacement failed")
					}
					if value, got := r.Value(field); value != 0 || got != ValueKnown {
						t.Fatal("zero replacement is not known")
					}
					if err := json.Unmarshal([]byte(`{}`), r); err != nil {
						t.Fatal(err)
					}
					for _, sibling := range []RangeField{RangeMinimum, RangeMaximum, RangeDefault} {
						if _, got := r.Value(sibling); got != ValueMissing {
							t.Fatal("reused destination retained a prior field")
						}
					}
				})
			}
		}
	}
}

func TestParameterRangeLegacyBytesAndCopy(t *testing.T) {
	for _, value := range []any{FloatRange{}, IntRange{}} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, []byte(`{"min":0,"max":0,"default":0}`)) {
			t.Fatalf("legacy bytes changed: %s", data)
		}
	}
	model := Model{Generation: &ModelGeneration{Temperature: &FloatRange{}, TopK: &IntRange{}}}
	model.Generation.Temperature.SetValueUnknown(RangeMinimum)
	model.Generation.TopK.UnsetValue(RangeDefault)
	copied := DeepCopyModel(model)
	copied.Generation.Temperature.SetValue(RangeMinimum, 0)
	copied.Generation.TopK.SetValue(RangeDefault, 0)
	if _, state := model.Generation.Temperature.Value(RangeMinimum); state != ValueUnknown {
		t.Fatal("copy changed original range presence")
	}
	if _, state := model.Generation.TopK.Value(RangeDefault); state != ValueMissing {
		t.Fatal("copy changed original integer presence")
	}
}
