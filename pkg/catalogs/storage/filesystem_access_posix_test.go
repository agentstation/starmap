//go:build darwin || linux

package storage

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
)

func TestFilesystemCatalogStoreCreatesPrivateState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state", "catalog-store")
	store, err := NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("constructor touched root: %v", err)
	}
	if err := store.Commit(t.Context(), testGeneration("private", "payload"), ""); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(filepath.Dir(root), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		want := privatefiles.FileMode
		if entry.IsDir() {
			want = privatefiles.DirectoryMode
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s mode = %o, want %o", path, info.Mode().Perm(), want)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFilesystemCatalogStoreRefusesUnsafeAccessAndRecovers(t *testing.T) {
	for _, entry := range []string{"root", "generations", "generation", "manifest.json", "catalog.json", "current", ".commit.lock", "ancestor"} {
		t.Run(entry, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "catalog-store")
			store, err := NewFilesystem(root)
			if err != nil {
				t.Fatal(err)
			}
			first := testGeneration("private-first", "first")
			if err := store.Commit(t.Context(), first, ""); err != nil {
				t.Fatal(err)
			}
			selected := filepath.Join(root, entry)
			switch entry {
			case "root":
				selected = root
			case "ancestor":
				selected = parent
			case "generation":
				selected = store.generationDir(first.Manifest.GenerationID)
			case manifestFilename, payloadFilename:
				selected = filepath.Join(store.generationDir(first.Manifest.GenerationID), entry)
			}
			info, err := os.Stat(selected)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(selected, info.Mode().Perm()) })
			unsafe := info.Mode().Perm() | 0o044
			if entry == "ancestor" {
				unsafe = 0o777
			}
			if err := os.Chmod(selected, unsafe); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Current(t.Context()); err == nil {
				t.Error("Current accepted unsafe access")
			}
			if _, err := store.Get(t.Context(), first.Manifest.GenerationID); err == nil {
				t.Error("Get accepted unsafe access")
			}
			if err := store.Commit(t.Context(), first, first.Manifest.GenerationID); err == nil {
				t.Error("Commit accepted unsafe access")
			}
			unchanged, err := os.Stat(selected)
			if err != nil || unchanged.Mode().Perm() != unsafe {
				t.Fatalf("access refusal changed permissions: %v, %v", unchanged, err)
			}
			if err := os.Chmod(selected, info.Mode().Perm()); err != nil {
				t.Fatal(err)
			}
			assertStoredGeneration(t, store, first)
		})
	}
}

func TestFilesystemCatalogStoreRechecksAccessBeforeCurrentPromotion(t *testing.T) {
	root := privateFilesystemRoot(t)
	store, err := NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	first := testGeneration("private-before", "first")
	if err := store.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	store.beforeCurrentPromotion = func() error { return os.Chmod(root, 0o777) }
	t.Cleanup(func() { _ = os.Chmod(root, privatefiles.DirectoryMode) })
	if err := store.Commit(t.Context(), testGeneration("private-after", "second"), first.Manifest.GenerationID); err == nil {
		t.Error("published after access changed")
	}
	if err := os.Chmod(root, privatefiles.DirectoryMode); err != nil {
		t.Fatal(err)
	}
	assertStoredGeneration(t, store, first)
}
