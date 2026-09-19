package starmap_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap"
)

func TestExportEmbeddedBaselineUsesOnlyTheSelectedDirectory(t *testing.T) {
	parent := t.TempDir()
	directory := filepath.Join(parent, "starport", "data", "catalog", "baseline")
	first, err := starmap.ExportEmbeddedBaseline(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || first.GenerationID == "" || filepath.Dir(first.Directory) != directory {
		t.Fatalf("unexpected baseline identity: %+v", first)
	}
	for _, name := range []string{"manifest.json", "catalog.json"} {
		bytes, err := os.ReadFile(filepath.Join(first.Directory, name))
		if err != nil || len(bytes) == 0 {
			t.Fatalf("baseline %s is unavailable: %v", name, err)
		}
	}
	second, err := starmap.ExportEmbeddedBaseline(t.Context(), directory)
	if err != nil || second.Created || first.Directory != second.Directory || first.GenerationID != second.GenerationID {
		t.Fatalf("repeated export changed the baseline: %+v, %v", second, err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 1 || entries[0].Name() != "starport" {
		t.Fatalf("export used another product root: %v", err)
	}
}

func TestExportEmbeddedBaselineCancellationLeavesNoFiles(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "uncreated")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := starmap.ExportEmbeddedBaseline(ctx, directory)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled export error: %v", err)
	}
	if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled export touched the directory: %v", err)
	}
}
