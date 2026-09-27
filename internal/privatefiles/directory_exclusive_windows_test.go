package privatefiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsExclusiveDirectoryRejectsInheritedSharedRead(t *testing.T) {
	account, err := windowsProcessSID()
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(t.TempDir(), "shared-read-parent")
	if _, err := NewDirectory(parent); err != nil {
		t.Fatal(err)
	}
	private := "D:P(A;OICI;FA;;;" + account + ")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
	setAncestorFixtureDACL(t, parent, private+"(A;OICI;GRGX;;;WD)")
	t.Cleanup(func() { setAncestorFixtureDACL(t, parent, private) })
	inherited, err := os.MkdirTemp(parent, "inherited-")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExistingDirectory(inherited); err == nil {
		t.Fatal("inherited shared-read access unexpectedly satisfies private access")
	}
	child, err := NewExclusiveDirectory(filepath.Join(parent, "private-child"))
	if err != nil {
		t.Fatal(err)
	}
	root, err := child.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
}
