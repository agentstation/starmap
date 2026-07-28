package starmap

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/constants"
	starmaperrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/save"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestDefaultCatalogWorkspaceAndStatePathsAreDisjoint(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "starmap")
	workspace := strings.Replace(constants.DefaultCatalogPath, "~", home, 1)
	state := strings.Replace(constants.DefaultCatalogStatePath, "~", home, 1)
	if pathsContainEachOther(workspace, state) {
		t.Fatalf("default human workspace %q overlaps machine state %q", workspace, state)
	}
	if pathContains(workspace, state) || pathContains(state, workspace) {
		t.Fatal("default lifecycle roots contain one another")
	}
}

func TestCatalogWorkspaceReplacementCannotTouchSiblingState(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "state", "catalog")
	workspace := filepath.Join(root, "catalog")
	if pathsContainEachOther(state, workspace) {
		t.Fatal("test lifecycle roots overlap")
	}
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
	if err := builder.Save(save.WithPath(workspace)); err != nil {
		t.Fatalf("Save workspace: %v", err)
	}
	if err := builder.DeleteProvider("example"); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}
	if err := builder.Save(save.WithPath(workspace)); err != nil {
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

func TestClientRejectsCatalogDatabaseAndExportOverlap(t *testing.T) {
	root := t.TempDir()
	store, err := catalogstore.NewFilesystem(filepath.Join(root, "catalog"))
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	_, err = New(WithCatalogStore(store), WithCatalogPath(root))
	assertCatalogLayoutError(t, err)
}

func TestClientRejectsSymlinkedCatalogDatabaseAndExportOverlap(t *testing.T) {
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

func TestClientSaveAndSyncRejectDurableDatabaseTargets(t *testing.T) {
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
	assertCatalogLayoutError(t, client.Save(save.WithPath(filepath.Join(database, "exports"))))
	_, err = client.Sync(context.Background(), pkgsync.WithDryRun(true), pkgsync.WithCatalogPath(root))
	assertCatalogLayoutError(t, err)
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
