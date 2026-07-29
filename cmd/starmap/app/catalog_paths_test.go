package app

import (
	"context"
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/constants"
)

func TestCatalogPathsFreshInstallAreCanonicalSeparatedAndPassive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	app, err := New("test", "test", "test", "test", WithConfig(&Config{}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	workspace, err := app.CatalogPath()
	if err != nil {
		t.Fatalf("CatalogPath: %v", err)
	}
	wantWorkspace := filepath.Join(home, ".starmap", "catalog")
	if workspace != wantWorkspace {
		t.Fatalf("workspace = %q, want %q", workspace, wantWorkspace)
	}
	state, err := app.catalogStatePath()
	if err != nil {
		t.Fatalf("catalogStatePath: %v", err)
	}
	wantState := filepath.Join(home, ".starmap", "state", "catalog")
	if state != wantState {
		t.Fatalf("state = %q, want %q", state, wantState)
	}
	if _, err := app.Starmap(); err != nil {
		t.Fatalf("Starmap: %v", err)
	}
	if _, err := os.Stat(wantWorkspace); !os.IsNotExist(err) {
		t.Fatalf("passive construction created %q: %v", wantWorkspace, err)
	}
	if _, err := os.Stat(wantState); !os.IsNotExist(err) {
		t.Fatalf("passive construction created %q: %v", wantState, err)
	}
	store, err := catalogstore.NewFilesystem(wantState)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	generation := validCatalogGeneration(t, "fresh-generation")
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wantState, "current")); err != nil {
		t.Fatalf("canonical first commit: %v", err)
	}
	if _, err := os.Stat(wantWorkspace); !os.IsNotExist(err) {
		t.Fatalf("state commit created human workspace: %v", err)
	}
}

func TestCatalogStatePathIgnoresUnlaunchedDraftLocation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	draft := filepath.Join(home, ".starmap", "catalog-store")
	if err := os.MkdirAll(draft, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	markerPath := filepath.Join(draft, "current")
	marker := []byte("prelaunch-draft-must-remain-untouched\n")
	if err := os.WriteFile(markerPath, marker, constants.FilePermissions); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	app, err := New("test", "test", "test", "test", WithConfig(&Config{}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	path, err := app.CatalogPath()
	if err != nil {
		t.Fatalf("CatalogPath: %v", err)
	}
	canonical := filepath.Join(home, ".starmap", "catalog")
	if path != canonical {
		t.Fatalf("path = %q, want %q", path, canonical)
	}
	if _, err := app.Starmap(); err != nil {
		t.Fatalf("Starmap: %v", err)
	}
	retained, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("ReadFile marker: %v", err)
	}
	if string(retained) != string(marker) {
		t.Fatalf("draft location changed: %q", retained)
	}
	if _, err := os.Stat(canonical); !os.IsNotExist(err) {
		t.Fatalf("read-only startup created workspace: %v", err)
	}
}

func TestExplicitCatalogWorkspaceBypassesDefaultInspection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, name := range []string{"catalog", "catalog-store"} {
		root := filepath.Join(home, ".starmap", name)
		if err := os.MkdirAll(root, constants.DirPermissions); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "current"), []byte(name+"\n"), constants.FilePermissions); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	explicit := filepath.Join(t.TempDir(), "chosen")
	app, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogPath: explicit}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	path, err := app.CatalogPath()
	if err != nil {
		t.Fatalf("CatalogPath: %v", err)
	}
	if path != explicit {
		t.Fatalf("path = %q, want %q", path, explicit)
	}
	if _, err := app.Starmap(); err != nil {
		t.Fatalf("explicit workspace inspected defaults: %v", err)
	}
}

func TestCatalogWorkspaceMigrationAndRestartPreserveExactGeneration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	workspace := filepath.Join(home, ".starmap", "catalog")
	state := filepath.Join(home, ".starmap", "state", "catalog")
	store, err := catalogstore.NewFilesystem(workspace)
	if err != nil {
		t.Fatalf("NewFilesystem legacy: %v", err)
	}
	generation := validCatalogGeneration(t, "migration-restart")
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatalf("Commit legacy: %v", err)
	}
	app, err := New("test", "test", "test", "test", WithConfig(&Config{}))
	if err != nil {
		t.Fatalf("New app: %v", err)
	}
	result, err := app.MigrateCatalogWorkspace(context.Background())
	if err != nil {
		t.Fatalf("MigrateCatalogWorkspace: %v", err)
	}
	if result.GenerationID != generation.Manifest.GenerationID {
		t.Fatalf("migrated generation = %q, want %q", result.GenerationID, generation.Manifest.GenerationID)
	}
	beforeRestart := appCatalogTree(t, workspace)

	first, err := app.Starmap()
	if err != nil {
		t.Fatalf("Starmap first restart: %v", err)
	}
	if first.CurrentGenerationID() != generation.Manifest.GenerationID {
		t.Fatalf("first restart generation = %q", first.CurrentGenerationID())
	}
	assertCatalogPayload(t, first.Catalog(), generation.Payload)
	if after := appCatalogTree(t, workspace); !reflect.DeepEqual(after, beforeRestart) {
		t.Fatalf("first restart changed workspace:\nbefore=%v\nafter=%v", beforeRestart, after)
	}

	secondApp, err := New("test", "test", "test", "test", WithConfig(&Config{}))
	if err != nil {
		t.Fatalf("New second app: %v", err)
	}
	second, err := secondApp.Starmap()
	if err != nil {
		t.Fatalf("Starmap second restart: %v", err)
	}
	if second.CurrentGenerationID() != generation.Manifest.GenerationID {
		t.Fatalf("second restart generation = %q", second.CurrentGenerationID())
	}
	assertCatalogPayload(t, second.Catalog(), generation.Payload)
	if after := appCatalogTree(t, workspace); !reflect.DeepEqual(after, beforeRestart) {
		t.Fatalf("second restart changed workspace:\nbefore=%v\nafter=%v", beforeRestart, after)
	}
	relocated, err := catalogstore.NewFilesystem(state)
	if err != nil {
		t.Fatalf("NewFilesystem relocated: %v", err)
	}
	if _, err := relocated.Current(context.Background()); err != nil {
		t.Fatalf("relocated Current: %v", err)
	}
}

func TestRestartCompletesProjectionAfterMigrationMove(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	workspace := filepath.Join(home, ".starmap", "catalog")
	state := filepath.Join(home, ".starmap", "state", "catalog")
	store, err := catalogstore.NewFilesystem(workspace)
	if err != nil {
		t.Fatalf("NewFilesystem legacy: %v", err)
	}
	generation := validCatalogGeneration(t, "migration-interrupted")
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatalf("Commit legacy: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(state), constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll state parent: %v", err)
	}
	if err := os.Rename(workspace, state); err != nil {
		t.Fatalf("simulate committed migration move: %v", err)
	}
	if _, err := os.Lstat(workspace); !stderrors.Is(err, fs.ErrNotExist) {
		t.Fatalf("workspace exists before restart repair: %v", err)
	}

	app, err := New("test", "test", "test", "test", WithConfig(&Config{}))
	if err != nil {
		t.Fatalf("New app: %v", err)
	}
	client, err := app.Starmap()
	if err != nil {
		t.Fatalf("Starmap restart: %v", err)
	}
	if client.CurrentGenerationID() != generation.Manifest.GenerationID {
		t.Fatalf("restart generation = %q", client.CurrentGenerationID())
	}
	assertCatalogPayload(t, client.Catalog(), generation.Payload)
	if _, err := os.Stat(filepath.Join(workspace, "providers.yaml")); err != nil {
		t.Fatalf("restart did not restore human workspace: %v", err)
	}
}

func assertCatalogPayload(t *testing.T, catalog *catalogs.Catalog, want []byte) {
	t.Helper()
	got, err := catalogstore.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("catalog payload changed")
	}
}

func appCatalogTree(t *testing.T, root string) map[string]string {
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

func validCatalogGeneration(t *testing.T, id string) catalogstore.Generation {
	t.Helper()
	builder := catalogs.NewEmpty()
	author := catalogs.Author{ID: "test-author", Name: "Test Author"}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	if err := builder.SetAuthorModel(author.ID, catalogs.Model{
		ID:      "migration-model",
		Name:    "Migration Model",
		Authors: []catalogs.Author{author},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	if err := builder.SetProvider(catalogs.Provider{
		ID:   "migration-provider",
		Name: "Migration Provider",
		Models: map[string]*catalogs.Model{
			"migration-model": {
				ID:       "migration-model",
				ModelRef: "test-author/migration-model",
				Name:     "Migration Model",
			},
		},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	payload, err := catalogstore.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	generatedAt := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	descriptor := catalogs.DescribeCatalogPayload(payload)
	return catalogstore.Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   catalogs.CurrentCatalogSchemaVersion,
			GenerationID:    id,
			GeneratedAt:     generatedAt,
			Payload:         descriptor,
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: "test/v1",
				ValidatedAt:      generatedAt,
				Status:           catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{
					{Name: "catalog", Status: catalogs.GenerationValidationCheckPassed},
				},
			},
			SyncRunID: "sync-" + id,
			SourceObservations: []catalogs.SourceObservationLink{
				{
					Source:        catalogmeta.LocalCatalogID,
					ObservationID: "observation-" + id,
					ObservedAt:    generatedAt,
					Revision: catalogmeta.ObservationRevision{
						Kind:  catalogmeta.ObservationRevisionKindContentDigest,
						Value: descriptor.Checksum,
					},
					Completeness:     catalogmeta.ObservationCompletenessComplete,
					Status:           catalogmeta.ObservationStatusSucceeded,
					EvidenceChecksum: descriptor.Checksum,
				},
			},
			Completeness: catalogs.GenerationCompletenessComplete,
			ConsumerCompatibility: catalogs.ConsumerCompatibility{
				MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
				MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			},
		},
		Payload: payload,
	}
}
