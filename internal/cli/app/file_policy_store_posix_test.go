//go:build darwin || linux

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileInspectionReportsCatalogStorePrivateConflicts(t *testing.T) {
	for _, relative := range []string{".", "current", "generations/example/catalog.json"} {
		t.Run(relative, func(t *testing.T) {
			clearCatalogEnvironment(t)
			home := t.TempDir()
			t.Setenv("STARMAP_HOME", home)
			a := NewForCommand("test", "test", "test", "test")
			paths, err := a.ResolvedPaths()
			if err != nil {
				t.Fatal(err)
			}
			store := paths.CatalogStore.Path
			target := filepath.Join(store, relative)
			if err := os.MkdirAll(filepath.Join(store, "generations", "example"), 0o700); err != nil {
				t.Fatal(err)
			}
			const contents = "inspection must not parse or change these bytes"
			mode := os.FileMode(0o640)
			if relative == "." {
				mode = 0o750
			} else if err := os.WriteFile(target, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(target, mode); err != nil {
				t.Fatal(err)
			}
			report, err := a.InspectFiles(t.Context(), 10000)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, item := range report.Inspection.Observations {
				if item.ID != "catalog-store" || item.Path != target {
					continue
				}
				found = true
				if item.AccessPolicy != "owner-only" || item.AccessStatus != "conflict" || item.AccessReason != "group-or-other-mode-bits" {
					t.Errorf("catalog store inspection omitted the private access conflict: %+v", item)
				}
			}
			if !found || !report.Inspection.Complete {
				t.Fatal("catalog store observation missing or incomplete")
			}
			info, err := os.Stat(target)
			if err != nil || info.Mode().Perm() != mode {
				t.Fatal("inspection changed store permissions", err)
			}
			if relative != "." {
				actual, err := os.ReadFile(target)
				if err != nil || string(actual) != contents {
					t.Fatal("inspection changed store contents", err)
				}
			}
			if a.runtime != nil || a.starmap != nil || a.credentialResolver != nil {
				t.Fatal("inspection initialized application state")
			}
		})
	}
}
