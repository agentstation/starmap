//go:build darwin || linux

package productfiles_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateFileCopyRefusesDirectoryReplacement(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	directory := newDirectory(t, root)
	body := bytes.Repeat([]byte("x"), 128<<10)
	if err := directory.CompareAndPublish(t.Context(), "record", nil, body); err != nil {
		t.Fatal(err)
	}
	writes := 0
	_, err := directory.CopyFile(t.Context(), "record", privateCopyWriter(func(p []byte) (int, error) {
		writes++
		if writes == 1 {
			if err := os.Rename(root, root+"-old"); err != nil {
				t.Fatal(err)
			}
			newDirectory(t, root)
		}
		return len(p), nil
	}), int64(len(body)))
	if err == nil || writes == 0 {
		t.Fatalf("directory replacement: writes=%d err=%v", writes, err)
	}
}

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
