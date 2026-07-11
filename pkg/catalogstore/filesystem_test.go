package catalogstore

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAtomicFilesystemCommitFailurePreservesCurrent(t *testing.T) {
	root := t.TempDir()
	store, err := NewFilesystem(root)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	first := testGeneration("atomic-first", "first")
	if err := store.Commit(context.Background(), first, ""); err != nil {
		t.Fatalf("Commit first: %v", err)
	}

	fault := stderrors.New("injected before current promotion")
	store.beforeCurrentPromotion = func() error { return fault }
	second := testGeneration("atomic-second", "second")
	if err := store.Commit(context.Background(), second, "atomic-first"); !stderrors.Is(err, fault) {
		t.Fatalf("Commit second error = %v, want injected fault", err)
	}
	assertStoredGeneration(t, store, first)

	// The staged generation is complete and addressable, but was never current.
	staged, err := store.Get(context.Background(), "atomic-second")
	if err != nil {
		t.Fatalf("Get staged generation: %v", err)
	}
	if diff := cmp.Diff(second, staged); diff != "" {
		t.Fatalf("staged generation mismatch (-want +got):\n%s", diff)
	}

	reopened, err := NewFilesystem(root)
	if err != nil {
		t.Fatalf("NewFilesystem reopened: %v", err)
	}
	assertStoredGeneration(t, reopened, first)

	store.beforeCurrentPromotion = nil
	if err := store.Commit(context.Background(), second, "atomic-first"); err != nil {
		t.Fatalf("retry Commit second: %v", err)
	}
	assertStoredGeneration(t, store, second)
}

func TestFilesystemCatalogStoreReopensCurrentGeneration(t *testing.T) {
	root := t.TempDir()
	first, err := NewFilesystem(root)
	if err != nil {
		t.Fatalf("NewFilesystem first: %v", err)
	}
	want := testGeneration("reopen-generation", "durable")
	if err := first.Commit(context.Background(), want, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reopened, err := NewFilesystem(root)
	if err != nil {
		t.Fatalf("NewFilesystem reopened: %v", err)
	}
	got, err := reopened.Current(context.Background())
	if err != nil {
		t.Fatalf("Current reopened: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("reopened generation mismatch (-want +got):\n%s", diff)
	}
}
