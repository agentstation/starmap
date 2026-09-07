package productpaths

import (
	"encoding/json"
	"testing"
)

func TestWindowsSecurityAssessmentReportsOnlyKnownPrivateConflicts(t *testing.T) {
	for _, test := range []struct {
		name, policy, descriptorStatus, reason, want string
	}{
		{"private-conflict", "owner-only", "conflict", "native-private-policy-conflict", "conflict"},
		{"private-compatible", "owner-only", "compatible", "", "unverified"},
		{"private-unavailable", "owner-only", "unverified", "security-descriptor-unavailable", "unverified"},
		{"shared-policy", "deployment-controlled", "conflict", "native-private-policy-conflict", "unverified"},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest := FileManifest{Files: []FileEntry{{ID: "test", Policy: FilePolicy{Access: test.policy}}}}
			report := FileInspection{Observations: []FileObservation{{ID: "test", State: "present", Kind: "file", PermissionScope: "windows-owner-and-dacl", WindowsSecurity: &WindowsSecurity{PolicyStatus: test.descriptorStatus, Reason: test.reason}}}}
			assessManifestAccess(manifest, &report)
			item := report.Observations[0]
			if item.AccessStatus != test.want {
				t.Fatalf("access status = %s, want %s", item.AccessStatus, test.want)
			}
			if test.want == "conflict" && item.AccessReason != test.reason {
				t.Fatalf("missing private policy reason: %+v", item)
			}
		})
	}
}

func TestWindowsSecuritySerializesObservationFields(t *testing.T) {
	observation := WindowsSecurity{OwnerSID: "S-1-5-21-1-2-3-1001", ProcessSID: "S-1-5-21-1-2-3-1001", DACLState: "empty", PolicyStatus: "compatible"}
	encoded, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{"owner_sid": observation.OwnerSID, "process_sid": observation.ProcessSID, "dacl_state": "empty", "entry_count": float64(0), "policy_status": "compatible"} {
		if decoded[key] != want {
			t.Fatalf("Windows observation field %s = %v, want %v", key, decoded[key], want)
		}
	}
}
