package productpaths

import "testing"

func TestAccessAssessmentPreservesUncertainty(t *testing.T) {
	for _, test := range []struct {
		name   string
		item   FileObservation
		status string
	}{
		{"windows", FileObservation{State: "present", Kind: "file", PermissionScope: "native-permissions-unverified"}, "unverified"},
		{"symlink", FileObservation{State: "present", Kind: "symlink", PermissionScope: "posix-mode-bits-only", Mode: "0777"}, "unverified"},
		{"missing_owner", FileObservation{State: "present", Kind: "file", PermissionScope: "posix-mode-bits-only", Mode: "0600"}, "unverified"},
		{"wrong_owner", FileObservation{State: "present", Kind: "file", PermissionScope: "posix-mode-bits-only", Mode: "0600", Owner: &FileOwner{MatchesEffectiveUser: false}}, "conflict"},
		{"missing", FileObservation{State: "absent"}, "not-assessed"},
		{"disabled", FileObservation{State: "not-applicable"}, "not-assessed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.item.ID = "private"
			report := FileInspection{Observations: []FileObservation{test.item}}
			assessManifestAccess(FileManifest{Files: []FileEntry{{ID: "private", Policy: FilePolicy{Access: "owner-only"}}}}, &report)
			if report.Observations[0].AccessStatus != test.status {
				t.Fatalf("unexpected access status: %+v", report.Observations[0])
			}
		})
	}
}
