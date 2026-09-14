package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
	filepolicy "github.com/agentstation/starmap/pkg/productpaths/policy"
)

func TestFileInspectionPreservesPrivateRecordStaging(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	a := NewForCommand("test", "test", "test", "test")
	paths, err := a.ResolvedPaths()
	if err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	wanted := map[string]string{}
	unknown := []string{}
	for _, location := range []struct{ directory, stage, owner string }{
		{"catalog-runtime", ".layer-candidate", "runtime-evidence"},
		{"catalog-runtime/providers", ".layer-candidate", "runtime-evidence"},
		{"catalog-runtime/providers/bindings", ".layer-candidate", "runtime-evidence"},
		{"catalog-runtime/publication-inputs", ".input-candidate", "runtime-evidence"},
		{"github-catalog-source", ".state-candidate", "github-discovery"},
	} {
		path := filepath.Join(paths.Runtime.Path, filepath.FromSlash(location.directory))
		directory, err := privatefiles.NewDirectory(path)
		if err != nil {
			t.Fatal(err)
		}
		metadata, err := directory.Child(privatefiles.PublicationDirectoryName)
		if err != nil {
			t.Fatal(err)
		}
		for _, record := range []struct {
			directory    *privatefiles.Directory
			parent, name string
			data         []byte
		}{
			{directory, path, location.stage, []byte("private candidate contents")},
			{metadata, filepath.Join(path, privatefiles.PublicationDirectoryName), "pending.jsonl", []byte("incomplete private receipt")},
			{metadata, filepath.Join(path, privatefiles.PublicationDirectoryName), ".owner.lock", nil},
			{metadata, filepath.Join(path, privatefiles.PublicationDirectoryName), "operator-notes.txt", []byte("operator contents")},
		} {
			if err := record.directory.WriteFile(record.name, record.data, ".fixture-"); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(record.parent, record.name)
			before[file] = record.data
			if record.name == "operator-notes.txt" {
				unknown = append(unknown, file)
			} else {
				wanted[file] = location.owner
			}
		}
	}
	report, err := a.InspectFiles(t.Context(), 10000)
	if err != nil || !report.Inspection.Complete {
		t.Fatalf("inspection did not complete: %v", err)
	}
	for path, owner := range wanted {
		found := false
		for _, item := range report.Inspection.Observations {
			if item.Path == path {
				found = item.ID == owner && item.State == "present" && item.Kind == "file" && item.AccessPolicy == filepolicy.OwnerOnly
			}
		}
		if !found {
			t.Errorf("inspection omits private recovery file: %s", path)
		}
	}
	for path, data := range before {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(data, after) {
			t.Errorf("inspection changed %s: %v", path, err)
		}
	}
	for _, path := range unknown {
		if manifestCoversFile(t, report, path) {
			t.Errorf("manifest adopts unknown recovery file: %s", path)
		}
	}
	encoded, err := json.Marshal(report)
	if err != nil || bytes.Contains(encoded, []byte("private candidate contents")) || bytes.Contains(encoded, []byte("incomplete private receipt")) {
		t.Errorf("inspection exposed private record contents: %v", err)
	}
	if a.runtime != nil || a.starmap != nil || a.credentialResolver != nil {
		t.Fatal("inspection initialized application state")
	}
}
