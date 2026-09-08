package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestStagedDirectoryWriteGrantIsTemporaryAndNotInherited(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "directory")
	if err := os.Mkdir(path, directoryMode); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	file, err := openStagedDirectory(root, "directory")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	owner := user.User.Sid.String()
	defer setWorkspaceTestDACL(t, path, "D:P(A;OICI;FA;;;"+owner+")(A;OICI;FA;;;SY)")
	setWorkspaceTestDACL(t, path, "D:P(A;OICI;GR;;;"+owner+")(A;OICI;GR;;;SY)")
	before, err := nativeEntryAccess(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Mkdir("directory/child", directoryMode); !os.IsPermission(err) {
		t.Fatal("read-only fixture permitted child creation", err)
	}
	restore, err := writableStagedDirectory(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Mkdir("directory/child", directoryMode); err != nil {
		t.Fatal("assembly cannot create child", err)
	}
	child, err := root.Open("directory/child")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Close() }()
	sd, err := windows.GetSecurityInfo(windows.Handle(child.Fd()), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		t.Fatal("child DACL unavailable", err)
	}
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil {
			t.Fatal(err)
		}
		if ace.Header.AceType == windows.ACCESS_ALLOWED_ACE_TYPE && ace.Header.AceFlags&windows.INHERITED_ACE != 0 && ace.Mask&(windows.FILE_WRITE_DATA|windows.FILE_APPEND_DATA) != 0 {
			t.Fatal("child inherited the temporary write grant", sd.String())
		}
	}
	if err := restore(); err != nil {
		t.Fatal(err)
	}
	after, err := nativeEntryAccess(file)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("directory access changed: before=%s after=%s error=%v", before, after, err)
	}
	if err := root.Mkdir("directory/after", directoryMode); !os.IsPermission(err) {
		t.Fatal("write grant survived restoration", err)
	}
}
