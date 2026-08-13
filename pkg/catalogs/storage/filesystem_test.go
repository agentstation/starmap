package storage

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/agentstation/starmap/internal/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
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

func TestFilesystemCatalogStoreKeepsAndCleansMachineStagingUnderRoot(t *testing.T) {
	root := t.TempDir()
	store, err := NewFilesystem(root)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	if err := store.Commit(context.Background(), testGeneration("layout", "payload"), ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	for _, path := range []string{
		filepath.Join(root, ".commit.lock"),
		filepath.Join(root, "current"),
		filepath.Join(root, "generations"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("machine state %q: %v", path, err)
		}
	}
	for _, pattern := range []string{
		filepath.Join(root, ".current-*"),
		filepath.Join(root, "generations", ".candidate-*"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("Glob %q: %v", pattern, err)
		}
		if len(matches) != 0 {
			t.Fatalf("machine staging survived commit: %v", matches)
		}
	}
}

func TestFilesystemCatalogStoreRejectsSymlinkedMachineEntries(t *testing.T) {
	t.Run("root", func(t *testing.T) {
		parent := t.TempDir()
		target := filepath.Join(parent, "target")
		if err := os.Mkdir(target, constants.DirPermissions); err != nil {
			t.Fatalf("Mkdir target: %v", err)
		}
		root := filepath.Join(parent, "store")
		if err := os.Symlink(target, root); err != nil {
			t.Fatalf("Symlink root: %v", err)
		}
		store, err := NewFilesystem(root)
		if err != nil {
			t.Fatalf("NewFilesystem: %v", err)
		}
		assertInvalidFilesystemCommit(t, store)
	})

	for _, entry := range []string{"generations", ".commit.lock", "current"} {
		t.Run(entry, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(t.TempDir(), "operator-data")
			if entry == "generations" {
				if err := os.Mkdir(target, constants.DirPermissions); err != nil {
					t.Fatalf("Mkdir target: %v", err)
				}
			} else if err := os.WriteFile(target, []byte("preserve\n"), constants.SecureFilePermissions); err != nil {
				t.Fatalf("WriteFile target: %v", err)
			}
			if err := os.Symlink(target, filepath.Join(root, entry)); err != nil {
				t.Fatalf("Symlink %s: %v", entry, err)
			}
			store, err := NewFilesystem(root)
			if err != nil {
				t.Fatalf("NewFilesystem: %v", err)
			}
			assertInvalidFilesystemCommit(t, store)
			if entry != "generations" {
				data, err := os.ReadFile(target)
				if err != nil || string(data) != "preserve\n" {
					t.Fatalf("operator file changed: %q, %v", data, err)
				}
			}
		})
	}
}

func TestFilesystemCatalogStoreRejectsSymlinkedGeneration(t *testing.T) {
	root := t.TempDir()
	store, err := NewFilesystem(root)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "generations"), constants.DirPermissions); err != nil {
		t.Fatalf("Mkdir generations: %v", err)
	}
	id := "symlinked-generation"
	if err := os.Symlink(t.TempDir(), store.generationDir(id)); err != nil {
		t.Fatalf("Symlink generation: %v", err)
	}
	if _, err := store.Get(context.Background(), id); !stderrors.Is(
		err,
		pkgerrors.ErrInvalidInput,
	) {
		t.Fatalf("Get error = %T %v, want invalid input", err, err)
	}
}

func assertInvalidFilesystemCommit(t *testing.T, store *Filesystem) {
	t.Helper()
	err := store.Commit(
		context.Background(),
		testGeneration("symlink-layout", "payload"),
		"",
	)
	if !stderrors.Is(err, pkgerrors.ErrInvalidInput) {
		t.Fatalf("Commit error = %T %v, want invalid input", err, err)
	}
}
