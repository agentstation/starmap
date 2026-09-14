package workspace

import (
	"context"
	stderrors "errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestLegacyGenerationScanBatchesAndCancellation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "generations")
	if err := privatefiles.CreateDirectory(root); err != nil {
		t.Fatal(err)
	}
	const total = legacyGenerationReadBatch*2 + 3
	for i := range total {
		if err := os.Mkdir(filepath.Join(root, fmt.Sprint(i)), privatefiles.DirectoryMode); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("complete", func(t *testing.T) {
		seen := make(map[string]bool)
		count, err := scanLegacyGenerations(t.Context(), root, func(entry fs.DirEntry) error {
			if seen[entry.Name()] {
				t.Fatalf("visited %s twice", entry.Name())
			}
			seen[entry.Name()] = true
			return nil
		})
		if err != nil || count != total || len(seen) != total {
			t.Fatalf("retained scan = %d entries, %d visits, %v", count, len(seen), err)
		}
	})
	for _, stop := range []int{1, legacyGenerationReadBatch, legacyGenerationReadBatch + 1, total} {
		t.Run(fmt.Sprintf("cancel-after-%d", stop), func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			visits := 0
			count, err := scanLegacyGenerations(ctx, root, func(fs.DirEntry) error {
				visits++
				if visits == stop {
					cancel()
				}
				return nil
			})
			if !stderrors.Is(err, context.Canceled) || count != stop || visits != stop {
				t.Fatalf("scan continued after cancellation: count=%d, visits=%d, error=%v", count, visits, err)
			}
		})
	}
	t.Run("invalid-retained-entry", func(t *testing.T) {
		fault := stderrors.New("invalid retained generation")
		visits := 0
		count, err := scanLegacyGenerations(t.Context(), root, func(fs.DirEntry) error {
			visits++
			if visits == legacyGenerationReadBatch+1 {
				return fault
			}
			return nil
		})
		if !stderrors.Is(err, fault) || count != legacyGenerationReadBatch || visits != count+1 {
			t.Fatalf("scan ignored a failed generation after its first batch: count=%d, visits=%d, error=%v", count, visits, err)
		}
	})
}

func TestLegacyPreflightPreservesExcessEntries(t *testing.T) {
	for _, location := range []string{"root", "generation"} {
		t.Run(location, func(t *testing.T) {
			root := t.TempDir()
			legacy := filepath.Join(root, "legacy")
			state := filepath.Join(root, "state", "catalog")
			store := migrationStore(t, legacy)
			generation := migrationGeneration(t, "current", "model", "Model")
			if err := store.Commit(t.Context(), generation, ""); err != nil {
				t.Fatal(err)
			}
			dir := legacy
			if location == "generation" {
				dir = filepath.Dir(migrationManifestPath(legacy, generation.Manifest.GenerationID))
			}
			for i := range 20 {
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("operator-%d", i)), []byte("preserve"), privatefiles.FileMode); err != nil {
					t.Fatal(err)
				}
			}
			entries, err := readLegacyLayoutEntries(dir)
			if err != nil || len(entries) != legacyLayoutReadLimit {
				t.Fatalf("fixed layout read exceeded its bound: count=%d, error=%v", len(entries), err)
			}
			before := migrationTree(t, legacy)
			if _, err := MigrateLegacyLayout(t.Context(), legacy, state); err == nil {
				t.Fatal("migration accepted excess entries")
			}
			if after := migrationTree(t, legacy); !reflect.DeepEqual(before, after) {
				t.Fatal("preflight changed the source tree")
			}
			if _, err := os.Stat(filepath.Dir(state)); !stderrors.Is(err, os.ErrNotExist) {
				t.Fatalf("preflight created the target parent: %v", err)
			}
		})
	}
}

func TestLegacyMigrationPreservesGenerationsAcrossBatches(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "legacy")
	state := filepath.Join(root, "state", "catalog")
	store := migrationStore(t, legacy)
	generation := migrationGeneration(t, "first", "model", "Model")
	ids := make([]string, 0, legacyGenerationReadBatch+1)
	previous := ""
	for i := range legacyGenerationReadBatch + 1 {
		generation.Manifest.GenerationID = fmt.Sprintf("retained-%d", i)
		if err := store.Commit(t.Context(), generation, previous); err != nil {
			t.Fatal(err)
		}
		previous = generation.Manifest.GenerationID
		ids = append(ids, previous)
	}
	result, err := MigrateLegacyLayout(t.Context(), legacy, state)
	if err != nil || result.RetainedCount != len(ids) || result.GenerationID != previous {
		t.Fatalf("migration lost retained generations: result=%+v, error=%v", result, err)
	}
	relocated := migrationStore(t, state)
	for _, id := range ids {
		generation.Manifest.GenerationID = id
		got, err := relocated.Get(t.Context(), id)
		if err != nil || !sameMigrationGeneration(generation, got) {
			t.Fatalf("retained generation %s changed: %v", id, err)
		}
	}
}

func TestLegacyPreflightRejectsOversizedRetainedManifestBeforeParsing(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "legacy")
	state := filepath.Join(root, "state", "catalog")
	store := migrationStore(t, legacy)
	retained := migrationGeneration(t, "retained", "old", "Old")
	current := migrationGeneration(t, "current", "new", "New")
	if err := store.Commit(t.Context(), retained, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), current, retained.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	manifest := migrationManifestPath(legacy, retained.Manifest.GenerationID)
	if err := os.Truncate(manifest, storage.MaxFilesystemManifestBytes+1); err != nil {
		t.Fatal(err)
	}
	_, err := MigrateLegacyLayout(t.Context(), legacy, state)
	var validation *pkgerrors.ValidationError
	if !stderrors.As(err, &validation) || validation.Field != "private.file_bytes" {
		t.Fatalf("expected the bounded file reader to refuse the retained manifest, got %v", err)
	}
	info, statErr := os.Stat(manifest)
	if statErr != nil || info.Size() != storage.MaxFilesystemManifestBytes+1 {
		t.Fatalf("preflight changed the retained manifest: %v", statErr)
	}
	if _, err := os.Stat(filepath.Dir(state)); !stderrors.Is(err, os.ErrNotExist) {
		t.Fatalf("preflight created the target parent: %v", err)
	}
	got, err := store.Current(t.Context())
	if err != nil || !sameMigrationGeneration(current, got) {
		t.Fatalf("preflight changed current: %v", err)
	}
}
