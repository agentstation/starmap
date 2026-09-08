package runtime

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"testing"
)

func setTestACL(t *testing.T, path, entry string) {
	t.Helper()
	if output, err := exec.Command("/bin/chmod", "+a", entry, path).CombinedOutput(); err != nil {
		t.Fatalf("set fixture ACL: %v: %s", err, output)
	}
	t.Cleanup(func() {
		if output, err := exec.Command("/bin/chmod", "-N", path).CombinedOutput(); err != nil {
			t.Errorf("remove fixture ACL: %v: %s", err, output)
		}
	})
}

func TestDarwinRuntimeAcceptsPrivateACLs(t *testing.T) {
	owner, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{
		"",
		"user:" + owner.Username + " allow read,readattr,readextattr,readsecurity",
		"everyone deny delete",
	} {
		t.Run(entry, func(t *testing.T) {
			directory := privateRuntimeDirectory(t)
			if entry != "" {
				setTestACL(t, directory, entry)
			}
			if err := ValidateDirectoryPermissions(t.Context(), directory); err != nil {
				t.Fatalf("private ACL refused: %v", err)
			}
		})
	}
}

func TestDarwinRuntimeRejectsInheritedACL(t *testing.T) {
	parent := privateRuntimeDirectory(t)
	setTestACL(t, parent, "everyone allow read,readattr,readextattr,readsecurity,file_inherit,directory_inherit")
	directory := filepath.Join(parent, "runtime")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDirectoryPermissions(t.Context(), directory); err == nil {
		t.Fatal("accepted an inherited public grant")
	}
	if output, err := exec.Command("/bin/chmod", "-N", directory).CombinedOutput(); err != nil {
		t.Fatalf("operator ACL correction: %v: %s", err, output)
	}
	if err := ValidateDirectoryPermissions(t.Context(), directory); err != nil {
		t.Fatalf("refused corrected child ACL: %v", err)
	}
}

func TestDarwinRuntimeRejectsACLGrantsBeyondModeBits(t *testing.T) {
	for _, name := range []string{".", directoryLockName, ownerRecordName, instanceSeedFileName} {
		t.Run(name, func(t *testing.T) {
			directory := privateRuntimeDirectory(t)
			path := filepath.Join(directory, name)
			mode := os.FileMode(0o700)
			if name != "." {
				mode = 0o600
				if err := os.WriteFile(path, []byte("preserve fixture bytes"), mode); err != nil {
					t.Fatal(err)
				}
			}
			setTestACL(t, path, "everyone allow read,readattr,readextattr,readsecurity")
			before, err := os.Stat(path)
			if err != nil || before.Mode().Perm() != mode {
				t.Fatal("ACL fixture changed POSIX mode bits")
			}
			if err := ValidateDirectoryPermissions(t.Context(), directory); err == nil {
				t.Error("permission check accepted a public ACL grant")
			}
			after, err := os.Stat(path)
			if err != nil || after.Mode().Perm() != mode {
				t.Fatal("permission check changed POSIX mode bits")
			}
			if name != "." {
				raw, err := os.ReadFile(path)
				if err != nil || string(raw) != "preserve fixture bytes" {
					t.Fatal("permission check changed file bytes")
				}
			}
		})
	}
}
