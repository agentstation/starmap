package app

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/agentstation/starmap/internal/privatefiles"
)

func TestConfigurationRejectsPublicWindowsACL(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	file, err := privatefiles.CreateFile(root, "config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("catalog_source: embedded\n")); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	account := user.User.Sid.String()
	for _, public := range []bool{true, false} {
		acl := "D:P(A;;FA;;;" + account + ")(A;;FA;;;SY)(A;;FA;;;BA)"
		if public {
			acl += "(A;;GR;;;WD)"
		}
		descriptor, err := windows.SecurityDescriptorFromString(acl)
		if err != nil {
			t.Fatal(err)
		}
		dacl, _, err := descriptor.DACL()
		if err != nil {
			t.Fatal(err)
		}
		if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
			t.Fatal(err)
		}
		_, err = readConfigurationFile(path)
		if public && err == nil {
			t.Error("configuration accepted a public ACL grant")
		} else if !public && err != nil {
			t.Fatalf("private configuration after explicit ACL correction: %v", err)
		}
	}
}
