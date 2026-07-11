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

func TestDefaultCatalogDatabaseAndExportPathsAreDisjoint(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "starmap")
	database := strings.Replace(constants.DefaultCatalogDatabasePath, "~", home, 1)
	export := strings.Replace(constants.DefaultCatalogExportPath, "~", home, 1)
	if pathsContainEachOther(database, export) {
		t.Fatalf("default durable database %q overlaps editable export %q", database, export)
	}
	if pathContains(export, database) || pathContains(database, export) {
		t.Fatal("default lifecycle roots contain one another")
	}
}

func TestCatalogExportReplacementCannotTouchSiblingDatabase(t *testing.T) {
	root := t.TempDir()
	database := filepath.Join(root, "catalog")
	export := filepath.Join(root, "exports", "catalog")
	if pathsContainEachOther(database, export) {
		t.Fatal("test lifecycle roots overlap")
	}
	if err := os.MkdirAll(database, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll database: %v", err)
	}
	markerPath := filepath.Join(database, "current")
	marker := []byte("immutable-generation\n")
	if err := os.WriteFile(markerPath, marker, constants.FilePermissions); err != nil {
		t.Fatalf("WriteFile marker: %v", err)
	}
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "example", Name: "Example"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	if err := builder.Save(save.WithPath(export)); err != nil {
		t.Fatalf("Save export: %v", err)
	}
	if err := builder.DeleteProvider("example"); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}
	if err := builder.Save(save.WithPath(export)); err != nil {
		t.Fatalf("replacement Save export: %v", err)
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
	_, err = New(WithCatalogStore(store), WithCatalogExportPath(root))
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
	_, err = New(WithCatalogStore(store), WithCatalogExportPath(alias))
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
	_, err = client.Sync(context.Background(), pkgsync.WithDryRun(true), pkgsync.WithOutputPath(root))
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
