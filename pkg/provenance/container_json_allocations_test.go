//go:build !race

package provenance

import (
	"bytes"
	"encoding/json"
	"testing"
)

// Race instrumentation randomly discards sync.Pool entries used by encoding/json.
// Native tests measure the allocation bound. Race tests verify the same bytes.
func TestCanonicalDynamicJSONContainerAllocationBound(t *testing.T) {
	for _, value := range canonicalJSONContainers() {
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
