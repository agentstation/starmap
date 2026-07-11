package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/constants"
)

func TestCatalogDatabasePathFreshInstallIsCanonicalAndPassive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	app, err := New("test", "test", "test", "test", WithConfig(&Config{}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	path, err := app.catalogDatabasePath()
	if err != nil {
		t.Fatalf("catalogDatabasePath: %v", err)
	}
	want := filepath.Join(home, ".starmap", "catalog")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	exportPath, err := app.CatalogExportPath()
	if err != nil {
		t.Fatalf("catalogExportPath: %v", err)
	}
	wantExport := filepath.Join(home, ".starmap", "exports", "catalog")
	if exportPath != wantExport {
		t.Fatalf("export path = %q, want %q", exportPath, wantExport)
	}
	if _, err := app.Starmap(); err != nil {
		t.Fatalf("Starmap: %v", err)
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("passive construction created %q: %v", want, err)
	}
	if _, err := os.Stat(wantExport); !os.IsNotExist(err) {
		t.Fatalf("passive construction created %q: %v", wantExport, err)
	}
	store, err := catalogstore.NewFilesystem(want)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	generation := validCatalogGeneration(t, "fresh-generation")
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(want, "current")); err != nil {
		t.Fatalf("canonical first commit: %v", err)
	}
}

func TestCatalogDatabasePathIgnoresUnlaunchedDraftLocation(t *testing.T) {
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
	path, err := app.catalogDatabasePath()
	if err != nil {
		t.Fatalf("catalogDatabasePath: %v", err)
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
		t.Fatalf("read-only startup created canonical path: %v", err)
	}
}

func TestCatalogDatabasePathExplicitWinsWithoutDefaultInspection(t *testing.T) {
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
	path, err := app.catalogDatabasePath()
	if err != nil {
		t.Fatalf("catalogDatabasePath: %v", err)
	}
	if path != explicit {
		t.Fatalf("path = %q, want %q", path, explicit)
	}
}

func validCatalogGeneration(t *testing.T, id string) catalogstore.Generation {
	t.Helper()
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
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
