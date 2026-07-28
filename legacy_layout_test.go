package starmap

import (
	"context"
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestNewRejectsLegacyGenerationLayoutBeforeAnyMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspace := filepath.Join(root, "catalog")
	generation := filepath.Join(workspace, "generations", "legacy")
	if err := os.MkdirAll(generation, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for path, contents := range map[string]string{
		filepath.Join(workspace, "current"):        "legacy\n",
		filepath.Join(workspace, ".commit.lock"):   "lock\n",
		filepath.Join(generation, "manifest.json"): "{\"legacy\":true}\n",
		filepath.Join(generation, "catalog.json"):  "{\"legacy\":true}\n",
	} {
		if err := os.WriteFile(path, []byte(contents), constants.FilePermissions); err != nil {
			t.Fatalf("WriteFile %s: %v", path, err)
		}
	}
	before := filesystemContents(t, workspace)
	statePath := filepath.Join(root, "state", "catalog")
	store, err := catalogstore.NewFilesystem(statePath)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}

	client, err := New(WithCatalogStore(store), WithCatalogPath(workspace))
	if client != nil {
		t.Fatal("New returned a client for a legacy generation layout")
	}
	var layoutErr *pkgerrors.LegacyCatalogLayoutError
	if !stderrors.As(err, &layoutErr) {
		t.Fatalf("New error = %T %v, want *errors.LegacyCatalogLayoutError", err, err)
	}
	if layoutErr.Path != workspace || layoutErr.MigrationTarget != statePath {
		t.Fatalf("layout error = %#v", layoutErr)
	}
	wantEntries := []string{".commit.lock", "current", "generations"}
	if !reflect.DeepEqual(layoutErr.Entries, wantEntries) {
		t.Fatalf("entries = %v, want %v", layoutErr.Entries, wantEntries)
	}
	if after := filesystemContents(t, workspace); !reflect.DeepEqual(after, before) {
		t.Fatalf("legacy layout changed:\nbefore=%v\nafter=%v", before, after)
	}
	if _, statErr := os.Stat(statePath); !stderrors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("failed construction created machine state: %v", statErr)
	}
}

func TestSyncRejectsExplicitLegacyWorkspaceBeforeGenerationCommit(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(t.TempDir(), "catalog")
	if err := os.MkdirAll(filepath.Join(workspace, "generations"), constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "current"), []byte("legacy\n"), constants.FilePermissions); err != nil {
		t.Fatalf("Write current: %v", err)
	}
	before := filesystemContents(t, workspace)
	store := catalogstore.NewMemory()
	client, err := New(WithCatalogStore(store))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = client.Sync(context.Background(), pkgsync.WithCatalogPath(workspace))
	var layoutErr *pkgerrors.LegacyCatalogLayoutError
	if !stderrors.As(err, &layoutErr) {
		t.Fatalf("Sync error = %T %v, want *errors.LegacyCatalogLayoutError", err, err)
	}
	if _, currentErr := store.Current(context.Background()); !pkgerrors.IsNotFound(currentErr) {
		t.Fatalf("generation store changed before layout rejection: %v", currentErr)
	}
	if after := filesystemContents(t, workspace); !reflect.DeepEqual(after, before) {
		t.Fatalf("legacy layout changed:\nbefore=%v\nafter=%v", before, after)
	}
}

func filesystemContents(t *testing.T, root string) map[string]string {
	t.Helper()
	contents := make(map[string]string)
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			contents[relative+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		contents[relative] = string(data)
		return nil
	}); err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	return contents
}
