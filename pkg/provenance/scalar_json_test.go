package provenance

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
)

func TestCanonicalDynamicJSONScalarAllocationBound(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
		want  string
	}{
		{"nil", nil, "null"},
		{"boolean", true, "true"},
		{"string", "<catalog>\u2028", `"\u003ccatalog\u003e\u2028"`},
		{"signed", int64(math.MinInt64), "-9223372036854775808"},
		{"unsigned", uint64(math.MaxUint64), "18446744073709551615"},
		{"float", 0.0000001, "1e-7"},
		{"number", json.Number("1.00e+10"), "1.00e+10"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var encoded json.RawMessage
			var err error
			allocations := testing.AllocsPerRun(100, func() {
				encoded, err = canonicalDynamicJSON(test.value)
			})
			if err != nil || string(encoded) != test.want {
				t.Fatalf("scalar encoding = %s, %v; want %s", encoded, err, test.want)
			}
			if allocations > 2 {
				t.Fatalf("scalar encoding allocated %.0f objects; want at most 2", allocations)
			}
		})
	}
}

type escapedEvidence string

func (escapedEvidence) MarshalJSON() ([]byte, error) {
	return []byte(`"\u0061"`), nil
}

func TestCanonicalDynamicJSONPreservesNonScalarNormalization(t *testing.T) {
	type orderedEvidence struct {
		Z int `json:"z"`
		A int `json:"a"`
	}
	for _, test := range []struct {
		name  string
		value any
		want  string
	}{
		{"struct-order", orderedEvidence{Z: 2, A: 1}, `{"a":1,"z":2}`},
		{"custom-string", escapedEvidence("a"), `"a"`},
		{"raw-object", json.RawMessage(`{"z":2,"a":1}`), `{"a":1,"z":2}`},
		{"raw-string", json.RawMessage(`"\u0061"`), `"a"`},
		{"nested", []any{escapedEvidence("a"), orderedEvidence{Z: 2, A: 1}}, `["a",{"a":1,"z":2}]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := canonicalDynamicJSON(test.value)
			if err != nil || !bytes.Equal(encoded, []byte(test.want)) {
				t.Fatalf("normalized encoding = %s, %v; want %s", encoded, err, test.want)
			}
		})
	}
	for _, value := range []any{math.NaN(), math.Inf(1), json.Number("invalid"), json.RawMessage(`{"unfinished":`)} {
		if encoded, err := canonicalDynamicJSON(value); err == nil || encoded != nil {
			t.Fatalf("invalid value encoded as %s with error %v", encoded, err)
		}
	}
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	if encoded, err := canonicalDynamicJSON(cyclic); err == nil || encoded != nil {
		t.Fatalf("cyclic value encoded as %s with error %v", encoded, err)
	}
}

func TestCanonicalDynamicJSONContainerAllocationBound(t *testing.T) {
	for _, value := range []any{
		map[string]any{"z": []any{nil, true, "<model>", json.Number("18446744073709551615")}, "a": map[string]any{"count": float64(1)}},
		[]any{map[string]any{"enabled": false}, []any{int64(-1), uint64(2)}},
		map[string]any(nil),
		[]any(nil),
	} {
		var direct []byte
		var encoded json.RawMessage
		var err error
		directAllocations := testing.AllocsPerRun(100, func() { direct, err = json.Marshal(value) })
		if err != nil {
			t.Fatal(err)
		}
		allocations := testing.AllocsPerRun(100, func() { encoded, err = canonicalDynamicJSON(value) })
		if err != nil || !bytes.Equal(direct, encoded) {
			t.Fatalf("container encoding = %s, %v; want %s", encoded, err, direct)
		}
		if allocations > directAllocations+1 {
			t.Fatalf("container encoding allocated %.0f objects; direct encoding allocated %.0f", allocations, directAllocations)
		}
	}
}
