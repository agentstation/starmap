//go:build windows

package app

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	starmaperrors "github.com/agentstation/starmap/pkg/errors"
)

func TestRuntimeACLRefusalPrecedesBaselineExport(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	a, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{
		catalogconfig.Source: "embedded", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false",
	}}))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := a.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.Runtime.Path), 0o700); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	account := user.User.Sid.String()
	descriptor, err := windows.SecurityDescriptorFromString("O:" + account + "D:P(A;;FA;;;" + account + ")(A;;GR;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	attributes := windows.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	path, err := windows.UTF16PtrFromString(paths.Runtime.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.CreateDirectory(path, &attributes); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Runtime(t.Context()); err == nil {
		_ = a.closeRuntime()
		t.Fatal("application started with public runtime ACL")
	} else {
		var invalid *starmaperrors.ValidationError
		if !errors.As(err, &invalid) || invalid.Field != "runtime.directory" {
			t.Fatalf("ACL error = %v", err)
		}
	}
	if _, err := os.Stat(paths.Baselines.Path); !os.IsNotExist(err) {
		t.Fatal("ACL refusal created baseline state")
	}
	entries, err := os.ReadDir(paths.Runtime.Path)
	if err != nil || len(entries) != 0 {
		t.Fatal("ACL refusal wrote runtime files")
	}
	if a.runtime != nil || a.starmap != nil || a.credentialResolver != nil {
		t.Fatal("ACL refusal initialized application state")
	}
	if _, err := a.InspectFiles(t.Context(), 10000); err != nil {
		t.Fatalf("diagnostics unavailable after startup refusal: %v", err)
	}
	private, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + account + ")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := private.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(paths.Runtime.Path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Runtime(t.Context()); err != nil {
		t.Fatalf("operator correction did not permit startup: %v", err)
	}
	if err := a.closeRuntime(); err != nil {
		t.Fatal(err)
	}
}
