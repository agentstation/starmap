package storage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/filepublish"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestFilesystemCommitAmbiguousFlushOutcome(t *testing.T) {
	for _, initial := range []bool{true, false} {
		name := "replacement"
		if initial {
			name = "initial"
		}
		t.Run(name, func(t *testing.T) {
			root := privateFilesystemRoot(t)
			store, err := NewFilesystem(root)
			if err != nil {
				t.Fatal(err)
			}
			previous := testGeneration("flush-previous", "previous")
			expected := ""
			if !initial {
				if err := store.Commit(t.Context(), previous, ""); err != nil {
					t.Fatal(err)
				}
				expected = previous.Manifest.GenerationID
			}
			selected := testGeneration("flush-selected", "selected")
			failedFlushes := 0
			failSync := func(directory *os.Root) error {
				pointer, err := os.ReadFile(filepath.Join(directory.Name(), currentFilename))
				if err != nil || string(pointer) != selected.Manifest.GenerationID+"\n" {
					t.Fatalf("flush ran before pointer publication: %q, %v", pointer, err)
				}
				failedFlushes++
				return fs.ErrInvalid
			}
			store.syncCurrentDirectory = failSync
			assertUncertain := func(err error) {
				t.Helper()
				var publication *pkgerrors.PublicationError
				if !errors.As(err, &publication) || publication.Resource != "catalog current generation" || publication.ID != selected.Manifest.GenerationID || !errors.Is(err, fs.ErrInvalid) {
					t.Errorf("commit result does not identify its visible publication: %v", err)
				}
			}
			assertUncertain(store.Commit(t.Context(), selected, expected))
			assertStoredGeneration(t, store, selected)
			pointerBefore, err := os.Stat(filepath.Join(root, currentFilename))
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := NewFilesystem(root)
			if err != nil {
				t.Fatal(err)
			}
			assertStoredGeneration(t, reopened, selected)
			reopened.syncCurrentDirectory = failSync
			assertUncertain(reopened.Commit(t.Context(), selected, expected))
			if failedFlushes != 2 {
				t.Errorf("retry skipped the failed durability boundary: %d flush attempts", failedFlushes)
			}
			collision := testGeneration(selected.Manifest.GenerationID, "different content")
			if err := reopened.Commit(t.Context(), collision, expected); !errors.Is(err, pkgerrors.ErrConflict) {
				t.Fatalf("retry accepted a different payload: %v", err)
			}
			confirmedFlushes := 0
			reopened.syncCurrentDirectory = func(directory *os.Root) error {
				confirmedFlushes++
				return filepublish.SyncDirectory(directory)
			}
			if err := reopened.Commit(t.Context(), selected, expected); err != nil {
				t.Fatal(err)
			}
			if confirmedFlushes != 1 {
				t.Errorf("successful retry did not confirm durability: %d flushes", confirmedFlushes)
			}
			assertStoredGeneration(t, reopened, selected)
			pointerAfter, err := os.Stat(filepath.Join(root, currentFilename))
			if err != nil || !os.SameFile(pointerBefore, pointerAfter) || !pointerBefore.ModTime().Equal(pointerAfter.ModTime()) {
				t.Fatalf("retry rewrote the selected pointer: %v", err)
			}
			if !initial {
				retained, err := reopened.Get(t.Context(), expected)
				if err != nil || !sameGeneration(retained, previous) {
					t.Fatalf("retry changed the predecessor: %v", err)
				}
			}
		})
	}
}

func TestFilesystemFailureBeforePublicationDoesNotClaimAcceptance(t *testing.T) {
	store, err := NewFilesystem(privateFilesystemRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	previous := testGeneration("unpublished-previous", "previous")
	if err := store.Commit(t.Context(), previous, ""); err != nil {
		t.Fatal(err)
	}
	store.beforeCurrentPromotion = func() error { return fs.ErrPermission }
	err = store.Commit(t.Context(), testGeneration("unpublished-selected", "selected"), previous.Manifest.GenerationID)
	var publication *pkgerrors.PublicationError
	if !errors.Is(err, fs.ErrPermission) || errors.As(err, &publication) {
		t.Fatalf("unpublished error claims visible acceptance: %v", err)
	}
	assertStoredGeneration(t, store, previous)
}
