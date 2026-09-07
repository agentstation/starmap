package productpaths

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsInspectionReportsACLWithoutReadingContents(t *testing.T) {
	account, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := account.User.Sid.String()
	private := "O:" + sid + "D:P(A;;FA;;;" + sid + ")"
	for _, test := range []struct{ name, sddl, status string }{
		{"private", private, "unverified"},
		{"shared-read", "O:" + sid + "D:P(A;;FA;;;" + sid + ")(A;;GR;;;WD)", "conflict"},
		{"deny-content-read", "O:" + sid + "D:P(D;;0x1;;;WD)(A;;FA;;;" + sid + ")", "unverified"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "catalog.json")
			const contents = "preserve these private bytes"
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			setInspectionDACL(t, path, test.sddl)
			t.Cleanup(func() { setInspectionDACL(t, path, private) })
			if test.name == "deny-content-read" {
				if _, err := os.ReadFile(path); err == nil {
					t.Fatal("fixture did not deny file-content reads")
				}
			}
			before, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
			if err != nil {
				t.Fatal(err)
			}
			report, err := InspectManifest(t.Context(), FileManifest{Files: []FileEntry{{ID: "catalog-store", Location: Path{Path: path}, Availability: "available", Policy: FilePolicy{Access: "owner-only"}}}}, 10)
			if err != nil || !report.Complete || len(report.Observations) != 1 {
				t.Fatalf("inspection incomplete: %+v %v", report, err)
			}
			item := report.Observations[0]
			if item.AccessStatus != test.status || item.WindowsSecurity == nil || item.WindowsSecurity.OwnerSID != sid || item.WindowsSecurity.DACLState != "present" {
				t.Fatalf("wrong Windows observation: %+v", item)
			}
			after, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
			if err != nil || before.String() != after.String() {
				t.Fatal("inspection changed native permissions", err)
			}
			setInspectionDACL(t, path, private)
			actual, err := os.ReadFile(path)
			if err != nil || string(actual) != contents {
				t.Fatal("inspection changed payload bytes", err)
			}
		})
	}
}

func setInspectionDACL(t *testing.T, path, sddl string) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsInspectionRefusesChangedIdentityAndSymlinkTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selected")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	old, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	item := FileObservation{Path: path}
	inspectPermissions(&item, old)
	if item.WindowsSecurity == nil || item.WindowsSecurity.Reason != "changed-during-inspection" || item.WindowsSecurity.OwnerSID != "" {
		t.Fatalf("inspection accepted replacement metadata: %+v", item)
	}
	link := path + ".link"
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	item = FileObservation{Path: link}
	inspectPermissions(&item, info)
	if item.WindowsSecurity == nil || item.WindowsSecurity.Reason != "unsupported-file-type" || item.WindowsSecurity.OwnerSID != "" {
		t.Fatalf("inspection followed a symlink target: %+v", item)
	}
}
