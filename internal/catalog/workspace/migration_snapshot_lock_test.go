package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"
)

func TestMigrationTreePreservesEmptyLockedEntries(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "owner.lock")
	if err := os.WriteFile(path, nil, fileMode); err != nil {
		t.Fatal(err)
	}
	owner := flock.New(path)
	t.Cleanup(func() { _ = owner.Close() })
	if locked, err := owner.TryLock(); err != nil || !locked {
		t.Fatalf("acquire snapshot fixture lock: %v, %v", locked, err)
	}
	before := migrationTree(t, root)
	if value, present := before["owner.lock"]; !present || value != "" {
		t.Fatalf("snapshot omitted the empty locked file: %v", before)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("changed"), fileMode); err != nil {
		t.Fatal(err)
	}
	after := migrationTree(t, root)
	if after["owner.lock"] != "changed" {
		t.Fatalf("snapshot missed changed file bytes: %v", after)
	}
}
