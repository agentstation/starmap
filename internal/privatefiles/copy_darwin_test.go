package privatefiles

import (
	"io"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPrivateFileCopyRejectsNativeDarwinACL(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	directory, err := NewDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.WriteFile("record", []byte("private"), ".stage-"); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(root, "record")
	if output, err := exec.Command("/bin/chmod", "+a", "everyone allow read", name).CombinedOutput(); err != nil {
		t.Fatalf("ACL fixture: %v: %s", err, output)
	}
	if n, err := directory.CopyFile(t.Context(), "record", io.Discard, 100); err == nil || n != 0 {
		t.Fatalf("public ACL: %d, %v", n, err)
	}
	if output, err := exec.Command("/bin/chmod", "-N", name).CombinedOutput(); err != nil {
		t.Fatalf("ACL recovery: %v: %s", err, output)
	}
	if n, err := directory.CopyFile(t.Context(), "record", io.Discard, 100); err != nil || n != 7 {
		t.Fatalf("private ACL: %d, %v", n, err)
	}
}
