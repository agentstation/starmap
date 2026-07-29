package consumer

import "testing"

func TestLookup(t *testing.T) {
	if err := Lookup(); err != nil {
		t.Fatalf("Lookup: %v", err)
	}
}
