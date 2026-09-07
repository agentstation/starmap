//go:build darwin || linux

package app

import (
	"errors"
	"os"
	"testing"

	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	starmaperrors "github.com/agentstation/starmap/pkg/errors"
)

func TestRuntimePermissionRefusalPrecedesBaselineExport(t *testing.T) {
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
	if err := os.Chmod(paths.Runtime.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Runtime(t.Context()); err == nil {
		_ = a.closeRuntime()
		t.Fatal("application started with exposed runtime permissions")
	} else {
		var invalid *starmaperrors.ValidationError
		if !errors.As(err, &invalid) || invalid.Field != "runtime.directory" {
			t.Fatalf("permission error = %v", err)
		}
	}
	if _, err := os.Stat(paths.Baselines.Path); !os.IsNotExist(err) {
		t.Fatal("permission refusal created baseline state")
	}
	entries, err := os.ReadDir(paths.Runtime.Path)
	if err != nil || len(entries) != 0 {
		t.Fatal("permission refusal wrote runtime files")
	}
	if a.runtime != nil || a.starmap != nil || a.credentialResolver != nil {
		t.Fatal("permission refusal initialized application state")
	}
	if _, err := a.InspectFiles(t.Context(), 10000); err != nil {
		t.Fatalf("diagnostics unavailable after startup refusal: %v", err)
	}
	if err := os.Chmod(paths.Runtime.Path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Runtime(t.Context()); err != nil {
		t.Fatalf("operator correction did not permit startup: %v", err)
	}
	if err := a.closeRuntime(); err != nil {
		t.Fatal(err)
	}
}
