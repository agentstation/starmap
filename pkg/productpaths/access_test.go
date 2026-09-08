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

func TestServiceAccessAssessmentPreservesUncertainty(t *testing.T) {
	for _, test := range []struct {
		name, mode, status string
		owner              *FileOwner
	}{
		{"root group readable", "0640", "unverified", &FileOwner{UID: 0}},
		{"process group readable", "0640", "unverified", &FileOwner{UID: 1001, MatchesEffectiveUser: true}},
		{"foreign", "0640", "conflict", &FileOwner{UID: 1002}},
		{"group writable", "0660", "conflict", &FileOwner{UID: 0}},
		{"unknown", "0640", "unverified", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := FileInspection{Observations: []FileObservation{{ID: "configuration", State: "present", Kind: "file", PermissionScope: "posix-mode-bits-only", Mode: test.mode, Owner: test.owner}}}
			assessManifestAccess(FileManifest{Files: []FileEntry{{ID: "configuration", Policy: FilePolicy{Access: "service-managed"}}}}, &report)
			if report.Observations[0].AccessStatus != test.status {
				t.Fatalf("observation=%+v", report.Observations[0])
			}
		})
	}
}

func TestPatternAnchorHasNoManagedFileAccessPolicy(t *testing.T) {
	for _, test := range []struct{ kind, scope string }{
		{"patterns", "posix-mode-bits-only"}, {"tree", "posix-mode-bits-only"},
		{"patterns", "windows-owner-and-dacl"}, {"tree", "windows-owner-and-dacl"},
	} {
		t.Run(test.kind+"/"+test.scope, func(t *testing.T) {
			manifest := FileManifest{Files: []FileEntry{{ID: "private", Location: Path{Path: "/parent"}, Kind: test.kind, Policy: FilePolicy{Access: "owner-only"}}}}
			report := FileInspection{Observations: []FileObservation{
				{ID: "private", Path: "/parent", State: "present", Kind: "directory", PermissionScope: test.scope, WindowsSecurity: &WindowsSecurity{PolicyStatus: "conflict", Reason: "fixture-shared-access"}, Mode: "0755", Owner: &FileOwner{MatchesEffectiveUser: true}},
				{ID: "private", Path: "/parent/private", State: "present", Kind: "file", PermissionScope: test.scope, WindowsSecurity: &WindowsSecurity{PolicyStatus: "conflict", Reason: "fixture-shared-access"}, Mode: "0644", Owner: &FileOwner{MatchesEffectiveUser: true}},
			}}
			assessManifestAccess(manifest, &report)
			anchor, child := report.Observations[0], report.Observations[1]
			if test.kind == "patterns" {
				if anchor.AccessPolicy != "" || anchor.AccessStatus != "not-assessed" || anchor.AccessReason != "pattern-anchor" {
					t.Fatalf("scan parent received a managed file policy: %+v", anchor)
				}
			} else if anchor.AccessPolicy != "owner-only" || anchor.AccessStatus != "conflict" {
				t.Fatalf("managed tree lost its root policy: %+v", anchor)
			}
			if child.AccessPolicy != "owner-only" || child.AccessStatus != "conflict" {
				t.Fatalf("managed child lost its policy: %+v", child)
			}
		})
	}
}
