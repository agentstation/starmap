package workspace

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func TestLegacyStoreLockCancellationPreservesStore(t *testing.T) {
	parent := t.TempDir()
	legacy := filepath.Join(parent, "legacy")
	if err := os.Mkdir(legacy, directoryMode); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(legacy, ".commit.lock")
	if err := os.WriteFile(path, nil, fileMode); err != nil {
		t.Fatal(err)
	}
	writer := flock.New(path, flock.SetFlag(os.O_RDWR))
	if err := writer.Lock(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Close() }()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	release, err := acquireLegacyStoreLock(ctx, legacy)
	if release != nil {
		release()
		t.Fatal("migration got a held store lock")
	}
	if !stderrors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lock wait: %v", err)
	}
	aliases, err := filepath.Glob(filepath.Join(parent, ".legacy.starmap-migration-lock-*"))
	if err != nil || len(aliases) != 0 {
		t.Fatalf("canceled lock left aliases: %v, %v", aliases, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	contender := flock.New(path, flock.SetFlag(os.O_RDWR))
	defer func() { _ = contender.Close() }()
	if locked, err := contender.TryLock(); err != nil || locked {
		t.Fatalf("cancellation released the writer: %v, %v", locked, err)
	}
}

func TestLegacyStoreLockDoesNotRecreateMissingLock(t *testing.T) {
	legacy := t.TempDir()
	release, err := acquireLegacyStoreLock(t.Context(), legacy)
	if release != nil {
		release()
		t.Fatal("migration created a missing lock")
	}
	if err == nil {
		t.Fatal("missing lock did not fail")
	}
	entries, err := os.ReadDir(legacy)
	if err != nil || len(entries) != 0 {
		t.Fatalf("missing store changed: %v, %v", entries, err)
	}
}
