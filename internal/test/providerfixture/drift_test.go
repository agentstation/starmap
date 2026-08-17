package providerfixture

import (
	"reflect"
	"testing"
)

func TestWireDriftReportsBothDirections(t *testing.T) {
	recorded := []byte(`{"data":[{"id":"one","context_window":8192},{"id":"two","retired_at":"2026-01-01"}]}`)
	live := []byte(`{"data":[{"id":"one","context_window":8192},{"id":"three","shutdown_date":"2027-01-01"}]}`)

	absent, added, err := WireDrift(recorded, live)
	if err != nil {
		t.Fatalf("WireDrift: %v", err)
	}
	if want := []string{"retired_at"}; !reflect.DeepEqual(absent, want) {
		t.Errorf("absent = %v, want %v", absent, want)
	}
	if want := []string{"shutdown_date"}; !reflect.DeepEqual(added, want) {
		t.Errorf("added = %v, want %v", added, want)
	}
}

func TestWireDriftAcceptsMatchingFieldSets(t *testing.T) {
	recorded := []byte(`{"data":[{"id":"one","owned_by":"vendor"}]}`)
	live := []byte(`{"data":[{"id":"two","owned_by":"vendor"},{"id":"three"}]}`)

	absent, added, err := WireDrift(recorded, live)
	if err != nil {
		t.Fatalf("WireDrift: %v", err)
	}
	if len(absent) != 0 || len(added) != 0 {
		t.Errorf("absent = %v, added = %v, want no drift", absent, added)
	}
}

func TestWireModelFieldsRejectsMalformedResponses(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "missing data", payload: `{}`},
		{name: "null data", payload: `{"data":null}`},
		{name: "renamed data", payload: `{"models":[]}`},
		{name: "wrong data type", payload: `{"data":{}}`},
		{name: "invalid json", payload: `{`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := WireModelFields([]byte(test.payload)); err == nil {
				t.Fatalf("WireModelFields accepted %s", test.payload)
			}
		})
	}
}

func TestWireModelFieldsAcceptsAnEmptyModelList(t *testing.T) {
	fields, err := WireModelFields([]byte(`{"data":[]}`))
	if err != nil {
		t.Fatalf("WireModelFields: %v", err)
	}
	if len(fields) != 0 {
		t.Fatalf("fields = %v, want none", fields)
	}
}
