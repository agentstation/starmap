package sourcepayload

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnknownSourcePathsProduceDeterministicFingerprintEvidence(t *testing.T) {
	type schema struct {
		ID string `json:"id"`
	}
	data := []byte(`{"id":"model","z_new":{"secret":"not-retained"},"a_new":true}`)
	first, err := UnknownJSONFields(data, schema{}, "models[]")
	if err != nil {
		t.Fatalf("UnknownJSONFields: %v", err)
	}
	second, err := UnknownJSONFields(data, schema{}, "models[]")
	if err != nil {
		t.Fatalf("UnknownJSONFields second: %v", err)
	}
	if len(first) != 2 || first[0].Path != "models[].a_new" || first[1].Path != "models[].z_new" {
		t.Fatalf("unknown fields = %#v", first)
	}
	if first[0] != second[0] || first[1] != second[1] || !strings.HasPrefix(first[0].Checksum, "sha256:") {
		t.Fatalf("fingerprints are not deterministic: %#v / %#v", first, second)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("Marshal evidence: %v", err)
	}
	if strings.Contains(string(encoded), "not-retained") {
		t.Fatalf("evidence leaked raw unknown value: %s", encoded)
	}
}

func TestValidateExactJSONRejectsDuplicateMembersAtEveryDepth(t *testing.T) {
	for _, payload := range []string{
		`{"id":"one","id":"two"}`,
		`{"items":[{"id":"one","id":"two"}]}`,
		`{"outer":{"value":1,"value":2}}`,
		`{} {}`,
	} {
		if err := ValidateExactJSON([]byte(payload)); err == nil {
			t.Fatalf("ValidateExactJSON(%s) accepted non-exact JSON", payload)
		}
	}
	if err := ValidateExactJSON([]byte(`{"items":[{"id":"one"},{"id":"two"}]}`)); err != nil {
		t.Fatalf("ValidateExactJSON exact document: %v", err)
	}
}
