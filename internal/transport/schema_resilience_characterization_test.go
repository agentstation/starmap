package transport

import "testing"

// TestF009CharacterizationMalformedProviderSiblingRejectsResponse pins the
// single target decode. P4.8 must expose a bounded record-level decode path so
// callers can retain the valid sibling and report the malformed record.
func TestF009CharacterizationMalformedProviderSiblingRejectsResponse(t *testing.T) {
	var target struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	err := DecodeResponse(
		jsonResponse(`{"data":[{"id":"valid"},{"id":["schema-drift"]}]}`),
		&target,
	)
	if err == nil {
		t.Fatal("F-009 characterization changed: malformed sibling did not reject provider response")
	}
	if len(target.Data) == 0 || target.Data[0].ID != "valid" {
		t.Fatalf("fixture did not demonstrate parsed valid sibling before collection failure: %#v", target.Data)
	}
}
