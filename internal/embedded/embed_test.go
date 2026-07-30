package embedded

import (
	"io/fs"
	"path"
	"strings"
	"testing"
)

func TestCatalogExcludesBackupAndEditorArtifacts(t *testing.T) {
	err := fs.WalkDir(FS, "catalog", func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name := path.Base(filePath)
		if name == ".DS_Store" ||
			strings.HasSuffix(name, "~") ||
			strings.HasSuffix(name, ".bak") ||
			strings.HasSuffix(name, ".orig") ||
			strings.HasSuffix(name, ".tmp") {
			t.Errorf("embedded catalog includes non-canonical artifact %q", filePath)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded catalog: %v", err)
	}
}
