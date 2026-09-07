package config

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/cli/format"
	"github.com/agentstation/starmap/pkg/productpaths"
)

func TestInspectionWideOutputIncludesWindowsOwnerAndDACL(t *testing.T) {
	var output bytes.Buffer
	report := productpaths.FileInspection{Observations: []productpaths.FileObservation{{ID: "catalog-store", Path: "catalog", State: "present", WindowsSecurity: &productpaths.WindowsSecurity{OwnerSID: "S-1-5-21-1-2-3-1001", DACLState: "null", Reason: "native-private-policy-conflict"}}}}
	if err := formatInspection(&output, format.FormatWide, report); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"S-1-5-21-1-2-3-1001", "null", "native-private-policy-conflict"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("wide output omits %s: %s", expected, output.String())
		}
	}
}
