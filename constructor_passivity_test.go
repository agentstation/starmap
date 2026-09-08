package starmap

import (
	"maps"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestReadOnlyConstructorDoesNotInitializeFilesystemStore(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)
	for _, name := range []string{"HOME", "USERPROFILE", "LOCALAPPDATA", "APPDATA", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "STARMAP_HOME"} {
		t.Setenv(name, directory)
	}
	before := filesystemContents(t, directory)
	if _, err := New(); err != nil {
		t.Fatal(err)
	}
	store, err := storage.NewFilesystem(filepath.Join(directory, "store"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewContext(t.Context(), WithCatalogStore(store), WithCatalogPath(filepath.Join(directory, "workspace"))); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(before, filesystemContents(t, directory)) {
		t.Fatal("read-only construction initialized filesystem state")
	}
}

func TestReadOnlyConstructorPreservesWorkspaceWithDurableCurrent(t *testing.T) {
	for _, present := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "stale"}[present], func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "workspace")
			if present {
				builder := catalogs.NewEmpty()
				if err := builder.SetProvider(catalogs.Provider{ID: "old", Name: "Old Provider"}); err != nil {
					t.Fatal(err)
				}
				old, err := builder.Build()
				if err != nil {
					t.Fatal(err)
				}
				payload, err := catalogs.EncodeCatalogPayload(old)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := workspace.Project(t.Context(), path, old, workspace.Identity{
					GenerationID: "old-generation", PayloadChecksum: catalogs.DescribeCatalogPayload(payload).Checksum,
				}); err != nil {
					t.Fatal(err)
				}
			}
			store := storage.NewMemory()
			current := rootRemoteGeneration(t)
			if err := store.Commit(t.Context(), current, ""); err != nil {
				t.Fatal(err)
			}
			before := filesystemContents(t, directory)
			client, err := NewContext(t.Context(), WithCatalogStore(store), WithCatalogPath(path))
			if err != nil {
				t.Fatal(err)
			}
			if client.CurrentGenerationID() != current.Manifest.GenerationID {
				t.Fatal("constructor did not select the durable catalog")
			}
			if _, err := client.Catalog().Provider("remote-root"); err != nil {
				t.Fatal("durable catalog is not readable")
			}
			if !maps.Equal(before, filesystemContents(t, directory)) {
				t.Fatal("read-only constructor changed workspace files or created directories")
			}
		})
	}
}
