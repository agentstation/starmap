//go:build darwin || linux || windows

package filepublish

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncDirectoryPreservesPublishedBytes(t *testing.T) {
	root := publicationRoot(t)
	stageDirectory(t, root, "staged")
	stage, err := root.OpenRoot("staged")
	if err != nil {
		t.Fatal(err)
	}
	file, err := stage.OpenFile("payload", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := SyncDirectory(stage); err != nil {
		t.Fatal(err)
	}
	if err := stage.Close(); err != nil {
		t.Fatal(err)
	}
	if err := DirectoryNoReplace(root, "staged", "published"); err != nil {
		t.Fatal(err)
	}
	if err := SyncDirectory(root); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := SyncDirectory(root); err != nil {
			t.Fatal(err)
		}
		payload, err := root.ReadFile("published/payload")
		if err != nil || string(payload) != "staged" {
			t.Fatal("flush changed published bytes", err)
		}
	}
}

func TestSyncDirectoryUsesRenamedOpenRoot(t *testing.T) {
	parent := t.TempDir()
	name, moved := filepath.Join(parent, "original"), filepath.Join(parent, "moved")
	if err := os.Mkdir(name, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	if err := os.Rename(name, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("preserve replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SyncDirectory(root); err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(name)
	if err != nil || string(payload) != "preserve replacement" {
		t.Fatal("flush changed the replacement path", err)
	}
}

func TestSyncDirectoryRefusesInvalidRoots(t *testing.T) {
	if err := SyncDirectory(nil); err == nil {
		t.Fatal("nil root reported a successful flush")
	}
	root := publicationRoot(t)
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := SyncDirectory(root); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed-root error = %v; want os.ErrClosed", err)
	}
}
