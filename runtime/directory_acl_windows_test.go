package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func setWindowsFixtureDACL(t *testing.T, path, sddl string) {
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

func TestWindowsRuntimeRefusesPublicACLs(t *testing.T) {
	account := testWindowsAccount(t)
	for _, name := range []string{".", directoryLockName, ownerRecordName, instanceSeedFileName} {
		t.Run(name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "runtime")
			if err := createPrivateRuntimeDirectory(directory); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			if name != "." {
				file, err := createPrivateRuntimeFile(root, name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.WriteString("preserve fixture bytes"); err != nil {
					_ = file.Close()
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(directory, name)
			setWindowsFixtureDACL(t, path, "D:P(A;;FA;;;"+account+")(A;;GR;;;WD)")
			before, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateDirectoryPermissions(t.Context(), directory); err == nil {
				t.Fatal("accepted public access")
			}
			after, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
			if err != nil || before.String() != after.String() {
				t.Fatal("validation changed fixture DACL")
			}
			if name != "." {
				raw, err := os.ReadFile(path)
				if err != nil || string(raw) != "preserve fixture bytes" {
					t.Fatal("validation changed fixture bytes")
				}
			}
			setWindowsFixtureDACL(t, path, "D:P(A;;FA;;;"+account+")(A;;FA;;;SY)(A;;FA;;;BA)")
			if err := ValidateDirectoryPermissions(t.Context(), directory); err != nil {
				t.Fatalf("refused operator correction: %v", err)
			}
		})
	}
}

func TestWindowsPrivateCreationExcludesParentGrants(t *testing.T) {
	account := testWindowsAccount(t)
	parent := t.TempDir()
	setWindowsFixtureDACL(t, parent, "D:P(A;OICI;FA;;;"+account+")(A;OICI;GR;;;WD)")
	directory := filepath.Join(parent, "runtime")
	if err := createPrivateRuntimeDirectory(directory); err != nil {
		t.Fatal(err)
	}
	if err := preparePrivateRuntimeLock(directory); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	for _, name := range []string{ownerRecordName, instanceSeedFileName} {
		file, err := createPrivateRuntimeFile(root, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateDirectoryPermissions(t.Context(), directory); err != nil {
		t.Fatalf("private creation: %v", err)
	}
	if _, err := createPrivateRuntimeFile(root, ownerRecordName); !os.IsExist(err) {
		t.Fatalf("exclusive creation error: %v", err)
	}
	if err := createPrivateRuntimeChild(root, "staging"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDirectoryPermissions(t.Context(), filepath.Join(directory, "staging")); err != nil {
		t.Fatal(err)
	}
}

func testWindowsAccount(t *testing.T) string {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if user.User.Sid == nil || !user.User.Sid.IsValid() {
		t.Fatal("invalid process account SID")
	}
	return user.User.Sid.String()
}
