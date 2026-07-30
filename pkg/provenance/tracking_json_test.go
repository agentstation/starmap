package provenance

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEntryJSONIsStableAcrossDynamicValueRoundTrip(t *testing.T) {
	t.Parallel()

	type evidence struct {
		Path     string `json:"path"`
		Checksum string `json:"checksum"`
	}
	entry := Entry{
		Field: "extensions",
		Value: map[string]any{
			"unknown_fields": []evidence{{
				Path: "data[].new_field", Checksum: "sha256:evidence",
			}},
		},
		PreviousValue: evidence{Path: "data[].old_field", Checksum: "sha256:previous"},
	}
	first, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal typed entry: %v", err)
	}
	var decoded Entry
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatalf("Unmarshal entry: %v", err)
	}
	second, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("Marshal decoded entry: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("provenance JSON changed across round trip:\nfirst:  %s\nsecond: %s", first, second)
	}
}
