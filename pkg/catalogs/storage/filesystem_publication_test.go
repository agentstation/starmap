package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestFilesystemCatalogStorePreservesChangedStage(t *testing.T) {
	for _, mutation := range []string{"contents", "extra-file", "directory"} {
		t.Run(mutation, func(t *testing.T) {
			store, err := NewFilesystem(privateFilesystemRoot(t))
			if err != nil {
				t.Fatal(err)
			}
			var stagePath string
			store.beforeGenerationPromotion = func(path string) error {
				stagePath = path
				switch mutation {
				case "contents":
					return os.WriteFile(filepath.Join(path, payloadFilename), []byte("operator bytes"), privatefiles.FileMode)
				case "extra-file":
					return os.WriteFile(filepath.Join(path, "operator.txt"), []byte("operator bytes"), privatefiles.FileMode)
				default:
					if err := os.Rename(path, path+"-preserved"); err != nil {
						return err
					}
					_, err := privatefiles.NewDirectory(path)
					if err != nil {
						return err
					}
					return os.WriteFile(filepath.Join(path, "operator.txt"), []byte("operator bytes"), privatefiles.FileMode)
				}
			}
			if err := store.Commit(t.Context(), testGeneration("changed-stage", "first"), ""); err == nil {
				t.Fatal("published changed candidate")
			}
			if _, err := store.Current(t.Context()); !pkgerrors.IsNotFound(err) {
				t.Fatalf("current after refusal: %v", err)
			}
			if _, err := store.Get(t.Context(), "changed-stage"); !pkgerrors.IsNotFound(err) {
				t.Fatalf("candidate visible after refusal: %v", err)
			}
			name := "operator.txt"
			if mutation == "contents" {
				name = payloadFilename
			}
			data, err := os.ReadFile(filepath.Join(stagePath, name))
			if err != nil || string(data) != "operator bytes" {
				t.Fatalf("changed file lost: %q, %v", data, err)
			}
			if mutation == "directory" {
				if _, err := os.Stat(filepath.Join(stagePath+"-preserved", payloadFilename)); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestFilesystemCatalogStoreCancellationBeforePublication(t *testing.T) {
	for _, phase := range []string{"generation", "current"} {
		t.Run(phase, func(t *testing.T) {
			store, err := NewFilesystem(privateFilesystemRoot(t))
			if err != nil {
				t.Fatal(err)
			}
			first := testGeneration("cancel-first", "first")
			if err := store.Commit(t.Context(), first, ""); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if phase == "generation" {
				store.beforeGenerationPromotion = func(string) error { cancel(); return nil }
			} else {
				store.beforeCurrentPromotion = func() error { cancel(); return nil }
			}
			if err := store.Commit(ctx, testGeneration("cancel-second", "second"), first.Manifest.GenerationID); !errors.Is(err, context.Canceled) {
				t.Fatalf("commit = %v", err)
			}
			assertStoredGeneration(t, store, first)
			_, err = store.Get(t.Context(), "cancel-second")
			if phase == "generation" && !pkgerrors.IsNotFound(err) {
				t.Fatalf("canceled candidate visible: %v", err)
			}
			if phase == "current" && err != nil {
				t.Fatalf("completed candidate unavailable: %v", err)
			}
		})
	}
}

func TestFilesystemCatalogStoreRefusesReplacedRootAtPromotion(t *testing.T) {
	root := privateFilesystemRoot(t)
	store, err := NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	first := testGeneration("replace-first", "first")
	if err := store.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	var renameErr error
	attempted := false
	store.beforeCurrentPromotion = func() error {
		attempted = true
		renameErr = os.Rename(root, root+"-preserved")
		if renameErr != nil {
			return renameErr
		}
		_, err := privatefiles.NewDirectory(root)
		return err
	}
	if err := store.Commit(t.Context(), testGeneration("replace-second", "second"), first.Manifest.GenerationID); err == nil {
		t.Fatal("published into replacement root")
	}
	if !attempted {
		t.Fatal("commit did not reach root replacement")
	}
	if runtime.GOOS == "windows" && os.IsPermission(renameErr) {
		// Windows can refuse the rename while the store holds its commit lock.
		assertStoredGeneration(t, store, first)
		if _, err := os.Lstat(root + "-preserved"); !os.IsNotExist(err) {
			t.Fatalf("denied rename created a destination: %v", err)
		}
		return
	}
	if renameErr != nil {
		t.Fatalf("fixture could not replace the root: %v", renameErr)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("replacement directory changed: %v, %v", entries, err)
	}
	preserved, err := NewFilesystem(root + "-preserved")
	if err != nil {
		t.Fatal(err)
	}
	assertStoredGeneration(t, preserved, first)
}
