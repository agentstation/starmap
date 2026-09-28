//go:build darwin || linux

package productfiles_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateFileCopyRefusesLinksAndSharedFiles(t *testing.T) {
	for _, mode := range []string{"shared-file", "symlink", "directory"} {
		t.Run(mode, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "private")
			directory := newDirectory(t, root)
			name := filepath.Join(root, "record")
			switch mode {
			case "shared-file":
				if err := os.WriteFile(name, []byte("shared"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(name, 0o644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := directory.CompareAndPublish(t.Context(), "other", nil, []byte("private")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("other", name); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if _, err := directory.CreateChild("record"); err != nil {
					t.Fatal(err)
				}
			}
			if n, err := directory.CopyFile(t.Context(), "record", io.Discard, 1024); err == nil || n != 0 {
				t.Fatalf("unsafe copy: %d, %v", n, err)
			}
		})
	}
}
