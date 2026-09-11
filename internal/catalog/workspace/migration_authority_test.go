package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func migrationAuthorityGeneration(t *testing.T, id string, sequence uint64) catalogs.Generation {
	t.Helper()
	generation := migrationGeneration(t, id, id, id)
	generation.Manifest.ManifestVersion = catalogs.AuthorityGenerationManifestVersion
	generation.Manifest.AuthorityHead = catalogs.CatalogAuthorityHead{
		AuthorityID: "enterprise", PolicyID: "production", Sequence: sequence,
		GenerationID: id, PayloadChecksum: generation.Manifest.Payload.Checksum,
		RequiredPermissionRevision: "sha256:" + strings.Repeat("a", 64),
		PermissionSchemaVersion:    catalogs.CatalogPermissionSchemaVersion,
	}
	return generation
}

func TestMigrateLegacyLayoutRetainsAuthorityRecord(t *testing.T) {
	root := t.TempDir()
	legacy, state := filepath.Join(root, "catalog"), filepath.Join(root, "state", "catalog")
	store := migrationStore(t, legacy)
	first := migrationAuthorityGeneration(t, "authority-first", 1)
	second := migrationAuthorityGeneration(t, "authority-second", 2)
	if err := store.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), second, first.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	result, err := MigrateLegacyLayout(t.Context(), legacy, state)
	if err != nil {
		t.Fatal(err)
	}
	if result.RetainedCount != 2 {
		t.Fatalf("retained=%d", result.RetainedCount)
	}
	relocated := migrationStore(t, state)
	got, err := relocated.CurrentAuthorityHead(t.Context())
	if err != nil || got != second.Manifest.AuthorityHead {
		t.Fatalf("head=%+v error=%v", got, err)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(migrationManifestPath(state, first.Manifest.GenerationID)), "authority.json"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := catalogs.ParseCatalogAuthorityRecord(data)
	if err != nil || record.Head != first.Manifest.AuthorityHead {
		t.Fatalf("retained head=%+v error=%v", record.Head, err)
	}
}

func TestMigrateLegacyLayoutRejectsConflictingAuthorityRecord(t *testing.T) {
	root := t.TempDir()
	legacy, state := filepath.Join(root, "catalog"), filepath.Join(root, "state", "catalog")
	store := migrationStore(t, legacy)
	generation := migrationAuthorityGeneration(t, "authority-conflict", 1)
	if err := store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(filepath.Dir(migrationManifestPath(legacy, generation.Manifest.GenerationID)), "authority.json")
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	conflict := strings.Replace(string(data), "\"sequence\":1", "\"sequence\":2", 1)
	if err := os.WriteFile(recordPath, []byte(conflict), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := MigrateLegacyLayout(t.Context(), legacy, state); err == nil {
		t.Fatal("migrated conflicting authority metadata")
	}
	if _, err := os.Lstat(state); !os.IsNotExist(err) {
		t.Fatalf("migration changed target: %v", err)
	}
	current, err := store.Current(t.Context())
	if err != nil || !sameMigrationGeneration(current, generation) {
		t.Fatalf("current generation changed: %v", err)
	}
	retained, err := os.ReadFile(recordPath)
	if err != nil || string(retained) != conflict {
		t.Fatal("migration changed conflicting record", err)
	}
}

func TestMigrateLegacyLayoutPreservesLegacyAuthorityWithoutRecord(t *testing.T) {
	root := t.TempDir()
	legacy, state := filepath.Join(root, "catalog"), filepath.Join(root, "state", "catalog")
	store := migrationStore(t, legacy)
	generation := migrationAuthorityGeneration(t, "authority-legacy", 1)
	if err := store.Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(filepath.Dir(migrationManifestPath(legacy, generation.Manifest.GenerationID)), "authority.json")
	if err := os.Remove(recordPath); err != nil {
		t.Fatal(err)
	}
	if _, err := MigrateLegacyLayout(t.Context(), legacy, state); err != nil {
		t.Fatal(err)
	}
	relocated := migrationStore(t, state)
	current, err := relocated.Current(t.Context())
	if err != nil || !sameMigrationGeneration(current, generation) {
		t.Fatalf("generation changed: %v", err)
	}
	if got, err := relocated.CurrentAuthorityHead(t.Context()); err == nil || got != (catalogs.CatalogAuthorityHead{}) {
		t.Fatalf("migration repaired missing metadata: head=%+v error=%v", got, err)
	}
}
