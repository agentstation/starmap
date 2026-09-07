//go:build darwin

package app

import (
	"errors"
	"os"
	"os/exec"
	"testing"

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
	if err := os.MkdirAll(paths.Runtime.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("/bin/chmod", "+a", "everyone allow read,readattr,readextattr,readsecurity", paths.Runtime.Path).CombinedOutput(); err != nil {
		t.Fatalf("set fixture ACL: %v: %s", err, output)
	}
	t.Cleanup(func() { _ = exec.Command("/bin/chmod", "-N", paths.Runtime.Path).Run() })
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
	if output, err := exec.Command("/bin/chmod", "-N", paths.Runtime.Path).CombinedOutput(); err != nil {
		t.Fatalf("operator ACL correction: %v: %s", err, output)
	}
	if _, err := a.Runtime(t.Context()); err != nil {
		t.Fatalf("operator correction did not permit startup: %v", err)
	}
	if err := a.closeRuntime(); err != nil {
		t.Fatal(err)
	}
}
