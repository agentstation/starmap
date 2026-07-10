package sources

import (
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/constants"
)

func TestPayloadLimitBytesAndNesting(t *testing.T) {
	if err := ValidateJSONPayload([]byte(`{"quoted":"[[[","nested":{"ok":true}}`)); err != nil {
		t.Fatalf("ValidateJSONPayload valid: %v", err)
	}
	if err := ValidateJSONPayload([]byte(strings.Repeat("[", MaxJSONNestingDepth+1))); err == nil {
		t.Fatal("ValidateJSONPayload accepted excessive nesting")
	}
	if err := ValidateJSONPayload([]byte(strings.Repeat("x", constants.MaxSourcePayloadBytes+1))); err == nil {
		t.Fatal("ValidateJSONPayload accepted excessive bytes")
	}
}
