package app

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	filepolicy "github.com/agentstation/starmap/pkg/productpaths/policy"
)

func TestFileInspectionCoversRetainedRemovalAndPin(t *testing.T) {
	for _, selected := range []bool{false, true} {
		t.Run(map[bool]string{false: "canonical runtime", true: "selected runtime"}[selected], func(t *testing.T) {
			clearCatalogEnvironment(t)
			home := t.TempDir()
			t.Setenv("STARMAP_HOME", home)
			values := map[string]string{
				catalogconfig.Source: "embedded", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false",
			}
			if selected {
				values[catalogconfig.StateDirectory] = filepath.Join(home, "selected-runtime")
			}
			newApp := func() *App {
				t.Helper()
				a, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: values}))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = a.closeRuntime() })
				return a
			}
			a := newApp()
			connected, err := a.Runtime(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			definitions := connected.Catalog().Definitions()
			if len(definitions) == 0 {
				t.Fatal("embedded catalog has no model definition for the removal")
			}
			target, err := catalogs.NewCanonicalRemovalTarget(definitions[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			accepted, err := connected.ReplaceRemovalTargets(t.Context(), connected.State(), target)
			if err != nil {
				t.Fatal(err)
			}
			if err := a.closeRuntime(); err != nil {
				t.Fatal(err)
			}
			values[catalogconfig.GenerationPin] = accepted.GenerationID
			pinned := newApp()
			connected, err = pinned.Runtime(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			receipt, durable := connected.PinAcceptance()
			if !durable || receipt.AcceptedGenerationID != accepted.GenerationID || !connected.Catalog().Removals().ContainsCanonical(definitions[0].ID) {
				t.Fatal("restart did not retain the removal and accept its generation pin")
			}
			if err := pinned.closeRuntime(); err != nil {
				t.Fatal(err)
			}
			inspector := newApp()
			paths, err := inspector.ResolvedPaths()
			if err != nil {
				t.Fatal(err)
			}
			before := map[string][]byte{}
			if err := filepath.WalkDir(home, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil || entry.IsDir() {
					return walkErr
				}
				data, err := os.ReadFile(path)
				before[path] = data
				return err
			}); err != nil {
				t.Fatal(err)
			}
			report, err := inspector.InspectFiles(t.Context(), 10000)
			if err != nil || !report.Inspection.Complete {
				t.Fatalf("inspection did not complete: %v", err)
			}
			for _, name := range []string{"removals.json", "generation-pin.json"} {
				path := filepath.Join(paths.Runtime.Path, "catalog-runtime", name)
				if len(before[path]) == 0 {
					t.Fatalf("runtime did not persist %s", name)
				}
				found := false
				for _, item := range report.Inspection.Observations {
					if item.Path == path {
						found = item.ID == "runtime-evidence" && item.State == "present" && item.Kind == "file" && item.AccessPolicy == filepolicy.OwnerOnly
					}
				}
				if !found {
					t.Errorf("inspection omits the private retained record %s", name)
				}
			}
			for path, data := range before {
				if !manifestCoversFile(t, report, path) {
					t.Errorf("retained runtime file is absent from the manifest: %s", path)
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(data, after) {
					t.Errorf("inspection changed %s: %v", path, err)
				}
			}
			if manifestCoversFile(t, report, filepath.Join(paths.Runtime.Path, "catalog-runtime", "operator-notes.txt")) {
				t.Error("manifest adopts unknown runtime files")
			}
			encoded, err := json.Marshal(report)
			if err != nil || bytes.Contains(encoded, []byte(receipt.OperationID)) {
				t.Errorf("inspection exposed pin receipt contents: %v", err)
			}
			if inspector.runtime != nil || inspector.starmap != nil || inspector.credentialResolver != nil {
				t.Error("inspection initialized application state")
			}
		})
	}
}

func TestFileInspectionPreservesCatalogRetirementRecords(t *testing.T) {
	for _, selected := range []bool{false, true} {
		t.Run(map[bool]string{false: "canonical store", true: "selected store"}[selected], func(t *testing.T) {
			clearCatalogEnvironment(t)
			home := t.TempDir()
			t.Setenv("STARMAP_HOME", home)
			if selected {
				t.Setenv("STARMAP_CATALOG_STORE_PATH", filepath.Join(home, "selected-store"))
			}
			a := NewForCommand("test", "test", "test", "test")
			paths, err := a.ResolvedPaths()
			if err != nil {
				t.Fatal(err)
			}
			known := []string{
				"generations/example/.read.lock", "generations/example/authority.json", "generations/example/.authority-pending",
				"generations/.retirement-pending.json", "generations/.retirement-write-pending",
				"generations/.record-publications/.owner.lock", "generations/.record-publications/pending.jsonl",
				"generations/.retired-pending/manifest.json", "generations/.retired-pending/catalog.json",
				"generations/.retired-pending/authority.json", "generations/.retired-pending/.read.lock",
			}
			unknown := []string{
				"generations/example/operator-notes.txt", "generations/.record-publications/operator-notes.txt",
				"generations/.retired-pending/operator-notes.txt",
			}
			contents := []byte("private catalog retirement fixture contents")
			before := map[string][]byte{}
			for _, group := range [][]string{known, unknown} {
				for _, relative := range group {
					path := filepath.Join(paths.CatalogStore.Path, filepath.FromSlash(relative))
					directory, err := privatefiles.NewDirectory(filepath.Dir(path))
					if err != nil {
						t.Fatal(err)
					}
					if err := directory.WriteFile(filepath.Base(path), contents, ".fixture-"); err != nil {
						t.Fatal(err)
					}
					before[path] = contents
				}
			}
			report, err := a.InspectFiles(t.Context(), 10000)
			if err != nil || !report.Inspection.Complete {
				t.Fatalf("inspection did not complete: %v", err)
			}
			for _, relative := range known {
				path := filepath.Join(paths.CatalogStore.Path, filepath.FromSlash(relative))
				found := false
				for _, item := range report.Inspection.Observations {
					if item.Path == path {
						found = item.ID == "catalog-store" && item.State == "present" && item.Kind == "file" && item.AccessPolicy == filepolicy.OwnerOnly
					}
				}
				if !found || !manifestCoversFile(t, report, path) {
					t.Errorf("inspection omits private catalog retention file: %s", relative)
				}
			}
			for _, relative := range unknown {
				if manifestCoversFile(t, report, filepath.Join(paths.CatalogStore.Path, filepath.FromSlash(relative))) {
					t.Errorf("manifest adopts unknown catalog contents: %s", relative)
				}
			}
			for path, data := range before {
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(data, after) {
					t.Errorf("inspection changed %s: %v", path, err)
				}
			}
			encoded, err := json.Marshal(report)
			if err != nil || bytes.Contains(encoded, contents) {
				t.Errorf("inspection exposed catalog retention contents: %v", err)
			}
			if a.runtime != nil || a.starmap != nil || a.credentialResolver != nil {
				t.Fatal("inspection initialized application state")
			}
		})
	}
}
