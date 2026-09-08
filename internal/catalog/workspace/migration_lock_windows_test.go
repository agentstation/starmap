package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"
)

func TestLegacyLockAliasAllowsRelocationWithoutReleasingLock(t *testing.T) {
	parent := t.TempDir()
	legacy := filepath.Join(parent, "legacy")
	state := filepath.Join(parent, "state")
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(legacy, ".commit.lock")
	if err := os.WriteFile(original, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	release, err := acquireLegacyStoreLock(t.Context(), legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	assertMigrationAliasLocked(t, original)
	if err := os.Rename(legacy, state); err != nil {
		t.Fatal(err)
	}
	relocated := filepath.Join(state, ".commit.lock")
	assertMigrationAliasLocked(t, relocated)
	release()
	aliases, err := filepath.Glob(filepath.Join(parent, ".legacy.starmap-migration-lock-*"))
	if err != nil || len(aliases) != 0 {
		t.Fatalf("migration alias survived release: %v, %v", aliases, err)
	}
	writer := flock.New(relocated, flock.SetFlag(os.O_RDWR))
	locked, err := writer.TryLock()
	if err != nil || !locked {
		t.Fatalf("released alias still blocked the store: %v, %v", locked, err)
	}
	if err := writer.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func assertMigrationAliasLocked(t *testing.T, path string) {
	t.Helper()
	writer := flock.New(path, flock.SetFlag(os.O_RDWR))
	locked, err := writer.TryLock()
	defer func() { _ = writer.Unlock() }()
	if err != nil || locked {
		t.Fatalf("alias did not retain the store lock: %v, %v", locked, err)
	}
}
