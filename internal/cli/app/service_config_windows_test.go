package app

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/internal/test/windowstoken"
	"github.com/agentstation/starmap/pkg/productpaths"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

func TestServiceConfigurationWindowsAdministratorOwnerAndDeniedRead(t *testing.T) {
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	file, err := privatefiles.CreateFile(root, "config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	contents := []byte("catalog_source: embedded\n")
	if _, err := file.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	account := user.User.Sid.String()
	administrators, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "config.yaml")
	private := "D:P(A;;FA;;;" + account + ")(A;;FA;;;SY)(A;;FA;;;BA)"
	setWindowsServiceConfigurationDACL(t, path, private)
	// Owner changes affect only this temporary fixture. Hosted CI must execute the owner check.
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, administrators, nil, nil, nil); err != nil {
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			t.Fatalf("native CI cannot create its administrator-owned fixture: %v", err)
		}
		t.Skipf("administrator-owned fixture requires a token that can assign Administrators ownership: %v", err)
	}
	t.Cleanup(func() {
		setWindowsServiceConfigurationDACL(t, path, private)
		if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, user.User.Sid, nil, nil, nil); err != nil {
			t.Errorf("restore temporary fixture owner: %v", err)
		}
	})
	for _, test := range []struct {
		name, deny string
		readable   bool
	}{
		{"administrator-owner", "", true},
		{"denied-data-read", "(D;;0x1;;;" + account + ")", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			windowstoken.WithoutPrivileges(t, func() {
				setWindowsServiceConfigurationDACL(t, path, "D:P"+test.deny+"(A;;FA;;;"+account+")(A;;FA;;;SY)(A;;FA;;;BA)")
				data, err := readSelectedConfiguration(path, policy.ServiceManaged)
				if (err == nil) != test.readable {
					t.Fatalf("readable=%v, error=%v", test.readable, err)
				}
				if test.readable && string(data) != string(contents) {
					t.Fatal("service read returned different bytes")
				}
				if !test.readable && len(data) != 0 {
					t.Fatal("denied read returned configuration bytes")
				}
				if _, err := readConfigurationFile(path); err == nil {
					t.Fatal("owner-only reader accepted administrator ownership")
				}
				manifest := productpaths.FileManifest{Files: []productpaths.FileEntry{{
					ID: "configuration", Location: productpaths.Path{Path: path}, Kind: "file", Availability: "available", Policy: productpaths.FilePolicy{Access: policy.ServiceManaged},
				}}}
				inspection, err := productpaths.InspectManifest(t.Context(), manifest, 10)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, observed := range inspection.Observations {
					if observed.ID != "configuration" {
						continue
					}
					found = true
					if observed.WindowsSecurity == nil || observed.WindowsSecurity.OwnerSID != administrators.String() {
						t.Fatalf("native ownership observation: %+v", observed)
					}
					if observed.WindowsSecurity.ServicePolicyStatus != "compatible" {
						t.Fatalf("trusted descriptor rejected: %+v", observed.WindowsSecurity)
					}
					if observed.WindowsSecurity.PolicyStatus != "conflict" {
						t.Fatal("private policy did not report the different owner")
					}
					if observed.AccessStatus != "unverified" {
						t.Fatalf("metadata claimed effective service access: %+v", observed)
					}
				}
				if !found {
					t.Fatal("inspection omitted the selected configuration")
				}
			})
		})
	}
	setWindowsServiceConfigurationDACL(t, path, private)
	data, err := readSelectedConfiguration(path, policy.ServiceManaged)
	if err != nil || string(data) != string(contents) {
		t.Fatalf("explicit access correction did not preserve and restore the file: %v", err)
	}
}

func setWindowsServiceConfigurationDACL(t *testing.T, path, sddl string) {
	t.Helper()
	descriptor, err := windows.SecurityDescriptorFromString(sddl)
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
}
