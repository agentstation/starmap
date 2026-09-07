package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestNativeAccessCopyPreservesDescriptor(t *testing.T) {
	for _, directory := range []bool{false, true} {
		name := "file"
		if directory {
			name = "directory"
		}
		t.Run(name, func(t *testing.T) {
			root, err := os.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			var destination *os.File
			if directory {
				if err := root.Mkdir("source", 0o700); err != nil {
					t.Fatal(err)
				}
				if err := root.Mkdir("destination", 0o700); err != nil {
					t.Fatal(err)
				}
				destination, err = openStagedDirectory(root, "destination")
			} else {
				if err := root.WriteFile("source", []byte("source"), 0o600); err != nil {
					t.Fatal(err)
				}
				destination, err = createStagedFile(root, "destination")
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = destination.Close() }()
			source, err := root.Open("source")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = source.Close() }()
			before, err := nativeEntryAccess(source)
			if err != nil {
				t.Fatal(err)
			}
			if err := copyNativeAccess(source, destination); err != nil {
				t.Fatal(err)
			}
			after, err := nativeEntryAccess(destination)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatalf("native access changed: before=%s after=%s", before, after)
			}
		})
	}
}

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
