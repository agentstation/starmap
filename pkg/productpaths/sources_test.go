package productpaths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceDirectoriesUseCacheRootWithoutWrites(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	directories, err := SourceDirectoriesAt(filepath.Join(root, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	if directories.Cache != filepath.Join(root, "cache") || directories.Checkouts != filepath.Join(root, "cache", "sources") {
		t.Fatalf("directories = %+v", directories)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("source directories created files")
	}
	for _, invalid := range []SourceDirectories{{}, {Cache: "relative", Checkouts: root}, {Cache: root}, {Cache: root, Checkouts: "relative"}} {
		if err := invalid.Validate(); err == nil {
			t.Fatalf("invalid source roots accepted: %+v", invalid)
		}
	}
}
