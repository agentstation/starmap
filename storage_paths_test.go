package starmap

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/internal/constants"
	starmaperrors "github.com/agentstation/starmap/pkg/errors"
)

func TestDefaultCatalogWorkspaceAndStatePathsAreDisjoint(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "starmap")
	workspace := strings.Replace(constants.DefaultCatalogPath, "~", home, 1)
	state := strings.Replace(constants.DefaultCatalogStatePath, "~", home, 1)
	if err := validateCatalogPathSeparation(mustFilesystemStore(t, state), workspace); err != nil {
		t.Fatalf("default lifecycle roots overlap: %v", err)
	}
}

func TestCatalogWorkspaceReplacementCannotTouchSiblingState(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "state", "catalog")
	workspace := filepath.Join(root, "catalog")
	if err := os.MkdirAll(state, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll state: %v", err)
	}
	markerPath := filepath.Join(state, "current")
	marker := []byte("immutable-generation\n")
	if err := os.WriteFile(markerPath, marker, constants.FilePermissions); err != nil {
		t.Fatalf("WriteFile marker: %v", err)
	}
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "example", Name: "Example"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	if err := builder.SaveTo(workspace); err != nil {
		t.Fatalf("Save workspace: %v", err)
	}
	if err := builder.DeleteProvider("example"); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}
	if err := builder.SaveTo(workspace); err != nil {
		t.Fatalf("replacement Save workspace: %v", err)
	}
	retained, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("ReadFile marker: %v", err)
	}
	if string(retained) != string(marker) {
		t.Fatalf("database marker changed: %q", retained)
	}
}

func TestHumanWorkspaceLoadCannotTraverseSiblingMachineLifecycleRoots(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "catalog")
	human := catalogs.NewEmpty()
	if err := human.SetProvider(catalogs.Provider{
		ID: "human",
		Models: map[string]*catalogs.Model{
			"human-model": {ID: "human-model", Name: "Human Model"},
		},
	}); err != nil {
		t.Fatalf("SetProvider human: %v", err)
	}
	seedTestModelDefinitions(t, human)
	if err := human.SaveTo(workspace); err != nil {
		t.Fatalf("Save human workspace: %v", err)
	}

	machine := catalogs.NewEmpty()
	if err := machine.SetProvider(catalogs.Provider{
		ID: "machine",
		Models: map[string]*catalogs.Model{
			"machine-model": {ID: "machine-model", Name: "Machine Model"},
		},
	}); err != nil {
		t.Fatalf("SetProvider machine: %v", err)
	}
	seedTestModelDefinitions(t, machine)
	for _, machineRoot := range []string{
		filepath.Join(root, "state", "catalog"),
		filepath.Join(root, "cache", "models.dev"),
		filepath.Join(root, "sources", "models.dev-git"),
		filepath.Join(root, ".catalog.candidate-interrupted"),
	} {
		if err := machine.SaveTo(machineRoot); err != nil {
			t.Fatalf("Save machine fixture %q: %v", machineRoot, err)
		}
	}

	loaded, err := catalogs.NewFromPath(workspace)
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	catalog, err := loaded.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, err := catalog.FindModel("human-model"); err != nil {
		t.Fatalf("human model missing: %v", err)
	}
	if _, err := catalog.FindModel("machine-model"); !stderrors.Is(err, starmaperrors.ErrNotFound) {
		t.Fatalf("machine model lookup error = %v, want not found", err)
	}
}

func TestClientRejectsCatalogStateAndWorkspaceOverlap(t *testing.T) {
	root := t.TempDir()
	store, err := catalogstore.NewFilesystem(filepath.Join(root, "catalog"))
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	_, err = New(WithCatalogStore(store), WithCatalogPath(root))
	assertCatalogLayoutError(t, err)
}

func TestClientRejectsSymlinkedCatalogStateAndWorkspaceOverlap(t *testing.T) {
	root := t.TempDir()
	database := filepath.Join(root, "database")
	if err := os.MkdirAll(database, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(database, alias); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	store, err := catalogstore.NewFilesystem(database)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	_, err = New(WithCatalogStore(store), WithCatalogPath(alias))
	assertCatalogLayoutError(t, err)
}

func TestClientSaveRejectsDurableStateTargets(t *testing.T) {
	root := t.TempDir()
	database := filepath.Join(root, "catalog")
	store, err := catalogstore.NewFilesystem(database)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	client, err := New(WithCatalogStore(store))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	assertCatalogLayoutError(t, client.SaveTo(filepath.Join(database, "exports")))
}

func mustFilesystemStore(t *testing.T, path string) *catalogstore.Filesystem {
	t.Helper()
	store, err := catalogstore.NewFilesystem(path)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	return store
}

func assertCatalogLayoutError(t *testing.T, err error) {
	t.Helper()
	var configError *starmaperrors.ConfigError
	if !stderrors.As(err, &configError) {
		t.Fatalf("error = %T %v, want ConfigError", err, err)
	}
	if configError.Component != "catalog filesystem layout" {
		t.Fatalf("component = %q", configError.Component)
	}
}
