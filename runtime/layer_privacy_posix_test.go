//go:build darwin || linux

package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRetainedLayerFilesRequirePrivateAccess(t *testing.T) {
	directory := t.TempDir()
	store, err := newLayerStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.saveSource(t.Context(), sourceLayer{Identity: "fixture"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{store.root, filepath.Join(store.root, providerLayerDirectoryName), filepath.Join(store.root, sourceLayerFileName)} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm()&0o077 != 0 {
			t.Errorf("retained state has nonprivate permissions: %s, %v", path, err)
		}
	}
}

func TestRetainedLayerRefusesExposedExistingDirectory(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, layerDirectoryName)
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := newLayerStore(directory); err == nil {
		t.Fatal("exposed existing evidence directory accepted")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("refused directory received new evidence paths", err)
	}
}

func TestRetainedLayerRefusesLinkedDirectory(t *testing.T) {
	directory, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(directory, layerDirectoryName)); err != nil {
		t.Fatal(err)
	}
	if _, err := newLayerStore(directory); err == nil {
		t.Fatal("linked evidence directory accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("evidence initialization changed the linked directory", err)
	}
}
