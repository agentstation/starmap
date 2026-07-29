package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"github.com/agentstation/starmap/internal/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestMigrateLegacyLayoutRelocatesStoreAndProjectsCurrent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	legacy := filepath.Join(root, "catalog")
	state := filepath.Join(root, "state", "catalog")
	store := migrationStore(t, legacy)
	first := migrationGeneration(t, "migration-first", "first", "First")
	second := migrationGeneration(t, "migration-second", "second", "Second")
	if err := store.Commit(context.Background(), first, ""); err != nil {
		t.Fatalf("Commit first: %v", err)
	}
	if err := store.Commit(context.Background(), second, first.Manifest.GenerationID); err != nil {
		t.Fatalf("Commit second: %v", err)
	}

	result, err := MigrateLegacyLayout(context.Background(), legacy, state)
	if err != nil {
		t.Fatalf("MigrateLegacyLayout: %v", err)
	}
	if result.GenerationID != second.Manifest.GenerationID ||
		result.PayloadChecksum != second.Manifest.Payload.Checksum ||
		result.RetainedCount != 2 ||
		result.WorkspacePath != legacy ||
		result.StatePath != state ||
		result.WorkspaceChecksum == "" {
		t.Fatalf("migration result = %#v", result)
	}
	if err := ValidateHumanLayout(legacy, state); err != nil {
		t.Fatalf("migrated workspace layout: %v", err)
	}
	for _, machineEntry := range legacyGenerationEntries {
		if _, err := os.Lstat(filepath.Join(legacy, machineEntry)); !stderrors.Is(err, fs.ErrNotExist) {
			t.Fatalf("machine entry %q remains in workspace: %v", machineEntry, err)
		}
	}

	relocated := migrationStore(t, state)
	gotCurrent, err := relocated.Current(context.Background())
	if err != nil {
		t.Fatalf("Current relocated: %v", err)
	}
	if !sameMigrationGeneration(second, gotCurrent) {
		t.Fatalf("relocated current = %#v, want %#v", gotCurrent.Manifest, second.Manifest)
	}
	gotFirst, err := relocated.Get(context.Background(), first.Manifest.GenerationID)
	if err != nil {
		t.Fatalf("Get retained first: %v", err)
	}
	if !sameMigrationGeneration(first, gotFirst) {
		t.Fatal("retained first generation changed")
	}
	workspaceCatalog, err := catalogs.NewFromPath(legacy)
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	projected, err := workspaceCatalog.Build()
	if err != nil {
		t.Fatalf("Build projected: %v", err)
	}
	projectedPayload, err := catalogs.EncodeCatalogPayload(projected)
	if err != nil {
		t.Fatalf("Encode projected: %v", err)
	}
	if checksum := catalogs.DescribeCatalogPayload(projectedPayload).Checksum; checksum != result.WorkspaceChecksum {
		t.Fatalf("workspace checksum = %q, want %q", checksum, result.WorkspaceChecksum)
	}
}

func TestMigrateLegacyLayoutFailureRollsBackExactStore(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	legacy := filepath.Join(root, "catalog")
	state := filepath.Join(root, "state", "catalog")
	store := migrationStore(t, legacy)
	generation := migrationGeneration(t, "migration-rollback", "rollback", "Rollback")
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	before := migrationTree(t, legacy)
	fault := stderrors.New("injected after move")

	_, err := (legacyLayoutMigrator{afterMove: func() error { return fault }}).
		migrate(context.Background(), legacy, state)
	if !stderrors.Is(err, fault) {
		t.Fatalf("migration error = %v, want injected fault", err)
	}
	if after := migrationTree(t, legacy); !reflect.DeepEqual(after, before) {
		t.Fatalf("legacy store changed after rollback:\nbefore=%v\nafter=%v", before, after)
	}
	if _, err := os.Lstat(state); !stderrors.Is(err, fs.ErrNotExist) {
		t.Fatalf("migration target survived rollback: %v", err)
	}
	for _, path := range []string{projectionMarkerPath(legacy), writerLockPath(legacy)} {
		if _, err := os.Lstat(path); !stderrors.Is(err, fs.ErrNotExist) {
			t.Fatalf("migration artifact %q survived rollback: %v", path, err)
		}
	}
}

func TestMigrateLegacyLayoutProjectionFailureRollsBackOwnedWorkspace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	legacy := filepath.Join(root, "catalog")
	state := filepath.Join(root, "state", "catalog")
	store := migrationStore(t, legacy)
	generation := migrationGeneration(t, "migration-projection-failure", "projection", "Projection")
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	before := migrationTree(t, legacy)
	markerPath := projectionMarkerPath(legacy)
	if err := os.Mkdir(markerPath, constants.DirPermissions); err != nil {
		t.Fatalf("Mkdir blocking marker: %v", err)
	}

	_, err := MigrateLegacyLayout(context.Background(), legacy, state)
	if err == nil {
		t.Fatal("MigrateLegacyLayout succeeded with a blocking marker directory")
	}
	if after := migrationTree(t, legacy); !reflect.DeepEqual(after, before) {
		t.Fatalf("legacy store changed after projection rollback:\nbefore=%v\nafter=%v", before, after)
	}
	if _, err := os.Lstat(state); !stderrors.Is(err, fs.ErrNotExist) {
		t.Fatalf("migration target survived projection rollback: %v", err)
	}
	if _, err := os.Lstat(markerPath); !stderrors.Is(err, fs.ErrNotExist) {
		t.Fatalf("blocking marker survived projection rollback: %v", err)
	}
}

func TestMigrateLegacyLayoutRollbackPreservesRecreatedLegacyPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	legacy := filepath.Join(root, "catalog")
	state := filepath.Join(root, "state", "catalog")
	store := migrationStore(t, legacy)
	generation := migrationGeneration(t, "migration-concurrent", "concurrent", "Concurrent")
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	fault := stderrors.New("injected after concurrent recreation")
	operatorFile := filepath.Join(legacy, "operator-data")

	_, err := (legacyLayoutMigrator{afterMove: func() error {
		if err := os.MkdirAll(legacy, constants.DirPermissions); err != nil {
			return err
		}
		if err := os.WriteFile(operatorFile, []byte("preserve me"), constants.FilePermissions); err != nil {
			return err
		}
		return fault
	}}).migrate(context.Background(), legacy, state)
	if !stderrors.Is(err, fault) {
		t.Fatalf("migration error = %v, want injected fault", err)
	}
	var conflict *pkgerrors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("migration error = %T %v, want joined *errors.ConflictError", err, err)
	}
	data, readErr := os.ReadFile(operatorFile)
	if readErr != nil || string(data) != "preserve me" {
		t.Fatalf("recreated path data = %q, %v; want preserved", data, readErr)
	}
	relocated := migrationStore(t, state)
	got, currentErr := relocated.Current(context.Background())
	if currentErr != nil {
		t.Fatalf("Current relocated: %v", currentErr)
	}
	if !sameMigrationGeneration(generation, got) {
		t.Fatal("relocated generation changed after refused rollback")
	}
}

func TestOlderBinaryRejectsNewerSchemaBeforeMigrationMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	legacy := filepath.Join(root, "catalog")
	state := filepath.Join(root, "state", "catalog")
	store := migrationStore(t, legacy)
	generation := migrationGeneration(t, "migration-newer", "newer", "Newer")
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	manifestPath := migrationManifestPath(legacy, generation.Manifest.GenerationID)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile manifest: %v", err)
	}
	var manifest catalogs.GenerationManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("Unmarshal manifest: %v", err)
	}
	manifest.SchemaVersion = catalogs.CurrentCatalogSchemaVersion + 1
	manifest.ConsumerCompatibility = catalogs.ConsumerCompatibility{
		MinSchemaVersion: manifest.SchemaVersion,
		MaxSchemaVersion: manifest.SchemaVersion,
	}
	data, err = json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, data, constants.FilePermissions); err != nil {
		t.Fatalf("WriteFile manifest: %v", err)
	}
	before := migrationTree(t, legacy)

	_, err = MigrateLegacyLayout(context.Background(), legacy, state)
	var validationErr *pkgerrors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("migration error = %T %v, want *errors.ValidationError", err, err)
	}
	if validationErr.Field != "schema_version" {
		t.Fatalf("validation field = %q, want schema_version", validationErr.Field)
	}
	if after := migrationTree(t, legacy); !reflect.DeepEqual(after, before) {
		t.Fatalf("newer-schema store changed:\nbefore=%v\nafter=%v", before, after)
	}
	if _, err := os.Lstat(filepath.Dir(state)); !stderrors.Is(err, fs.ErrNotExist) {
		t.Fatalf("newer-schema rejection created state parent: %v", err)
	}
}

func migrationStore(t *testing.T, path string) *catalogstore.Filesystem {
	t.Helper()
	store, err := catalogstore.NewFilesystem(path)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	return store
}

func migrationGeneration(
	t *testing.T,
	generationID string,
	modelID string,
	modelName string,
) catalogstore.Generation {
	t.Helper()
	catalog, _ := testCatalog(t, modelID, modelName)
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	descriptor := catalogs.DescribeCatalogPayload(payload)
	generatedAt := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	return catalogstore.Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   catalogs.CurrentCatalogSchemaVersion,
			GenerationID:    generationID,
			GeneratedAt:     generatedAt,
			Payload:         descriptor,
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: "migration-test",
				ValidatedAt:      generatedAt,
				Status:           catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{{
					Name:   "migration_test",
					Status: catalogs.GenerationValidationCheckPassed,
				}},
			},
			SyncRunID: generationID + "-run",
			SourceObservations: []catalogs.SourceObservationLink{{
				Source:        catalogmeta.EmbeddedCatalogID,
				ObservationID: generationID + "-observation",
				ObservedAt:    generatedAt,
				Revision: catalogmeta.ObservationRevision{
					Kind:  catalogmeta.ObservationRevisionKindContentDigest,
					Value: descriptor.Checksum,
				},
				Completeness:     catalogmeta.ObservationCompletenessComplete,
				Status:           catalogmeta.ObservationStatusSucceeded,
				EvidenceChecksum: descriptor.Checksum,
			}},
			Completeness: catalogs.GenerationCompletenessComplete,
			ConsumerCompatibility: catalogs.ConsumerCompatibility{
				MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
				MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			},
		},
		Payload: payload,
	}
}

func migrationManifestPath(root, generationID string) string {
	digest := sha256.Sum256([]byte(generationID))
	return filepath.Join(root, "generations", hex.EncodeToString(digest[:]), "manifest.json")
}

func migrationTree(t *testing.T, root string) map[string]string {
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
