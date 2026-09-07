package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestTreeSnapshotDetectsNativeACLChange(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".", "note.txt"} {
		t.Run(name, func(t *testing.T) {
			path := t.TempDir()
			if err := os.WriteFile(filepath.Join(path, "note.txt"), []byte("note"), 0o600); err != nil {
				t.Fatal(err)
			}
			selected := filepath.Join(path, name)
			base := "D:P(A;;FA;;;" + user.User.Sid.String() + ")(A;;FA;;;SY)(A;;FA;;;BA)"
			setWorkspaceTestDACL(t, selected, base)
			before, err := snapshotTree(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			setWorkspaceTestDACL(t, selected, base+"(A;;RA;;;WD)")
			after, err := snapshotTree(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if before.Entries[0].Mode != after.Entries[0].Mode || sameTree(before, after) {
				t.Fatal("snapshot did not distinguish a DACL change from unchanged mode bits")
			}
		})
	}
}

func setWorkspaceTestDACL(t *testing.T, path, sddl string) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
}
