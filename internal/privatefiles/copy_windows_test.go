package privatefiles

import (
	"io"
	"path/filepath"
	"testing"
)

func TestPrivateFileCopyRejectsNativeWindowsACL(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	directory, err := NewDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.WriteFile("record", []byte("private"), ".stage-"); err != nil {
		t.Fatal(err)
	}
	account, err := windowsProcessSID()
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(root, "record")
	setAncestorFixtureDACL(t, name, "D:P(A;;FA;;;"+account+")(A;;GR;;;WD)")
	if n, err := directory.CopyFile(t.Context(), "record", io.Discard, 100); err == nil || n != 0 {
		t.Fatalf("public ACL: %d, %v", n, err)
	}
	setAncestorFixtureDACL(t, name, "D:P(A;;FA;;;"+account+")")
	if n, err := directory.CopyFile(t.Context(), "record", io.Discard, 100); err != nil || n != 7 {
		t.Fatalf("private ACL: %d, %v", n, err)
	}
}
