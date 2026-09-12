package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

const legacyAuthorityRecordName = "authority.json"

// LegacyLayoutMigrationResult describes one completed machine-store relocation
// and human-workspace projection.
type LegacyLayoutMigrationResult struct {
	WorkspacePath     string
	StatePath         string
	GenerationID      string
	PayloadChecksum   string
	WorkspaceChecksum string
	RetainedCount     int
}

type legacyLayoutMigrator struct {
	afterMove func() error
	projector projector
}

// MigrateLegacyLayout explicitly relocates the pre-plan filesystem generation
// store and projects its current generation back to the vacated human catalog workspace
// path. Validation and both advisory locks complete before the first rename.
func MigrateLegacyLayout(
	ctx context.Context,
	legacyPath string,
	statePath string,
) (LegacyLayoutMigrationResult, error) {
	return (legacyLayoutMigrator{}).migrate(ctx, legacyPath, statePath)
}

func (m legacyLayoutMigrator) migrate(
	ctx context.Context,
	legacyPath string,
	statePath string,
) (LegacyLayoutMigrationResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	legacy, err := resolveTarget(legacyPath)
	if err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	state, err := resolveTarget(statePath)
	if err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	if err := ValidateMachineSeparation(legacy, state, "catalog state"); err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	if err := requireAbsentMigrationTarget(state); err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	if err := requireLegacyStoreShape(legacy); err != nil {
		return LegacyLayoutMigrationResult{}, err
	}

	releaseStore, err := acquireLegacyStoreLock(ctx, legacy)
	if err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	defer releaseStore()
	originalStore, err := captureLegacyStoreIdentity(legacy)
	if err != nil {
		return LegacyLayoutMigrationResult{}, errors.WrapIO("inspect", legacy, err)
	}

	generation, catalog, retained, err := inspectLegacyStore(ctx, legacy)
	if err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	identity := Identity{
		GenerationID:    generation.Manifest.GenerationID,
		PayloadChecksum: generation.Manifest.Payload.Checksum,
	}
	if err := validateCommittedCatalog(catalog, identity); err != nil {
		return LegacyLayoutMigrationResult{}, err
	}

	releaseWriter, err := acquireWriterLock(legacy)
	if err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	defer releaseWriter()

	if err := os.MkdirAll(filepath.Dir(state), directoryMode); err != nil {
		return LegacyLayoutMigrationResult{}, errors.WrapIO("create", filepath.Dir(state), err)
	}
	move, err := prepareLegacyStoreMove(legacy, state, originalStore)
	if err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	defer move.close()
	if err := move.relocate(); err != nil {
		return LegacyLayoutMigrationResult{}, errors.WrapIO("relocate", legacy, err)
	}
	rollback := func(cause error, projected treeSnapshot) (LegacyLayoutMigrationResult, error) {
		if rollbackErr := move.rollback(ctx, projected); rollbackErr != nil {
			return LegacyLayoutMigrationResult{}, errors.WrapResource(
				"rollback",
				"legacy catalog layout migration",
				legacy,
				stderrors.Join(cause, rollbackErr),
			)
		}
		return LegacyLayoutMigrationResult{}, cause
	}
	if err := move.sync(); err != nil {
		return rollback(errors.WrapIO("sync", state, err), treeSnapshot{})
	}
	if m.afterMove != nil {
		if err := m.afterMove(); err != nil {
			return rollback(err, treeSnapshot{})
		}
	}

	relocated, err := storage.NewFilesystem(state)
	if err != nil {
		return rollback(err, treeSnapshot{})
	}
	relocatedCurrent, err := relocated.Current(ctx)
	if err != nil {
		return rollback(errors.WrapResource("verify", "relocated catalog generation", "current", err), treeSnapshot{})
	}
	if !sameMigrationGeneration(generation, relocatedCurrent) {
		return rollback(&errors.ConflictError{
			Resource: "relocated catalog generation",
			Expected: generation.Manifest.GenerationID,
			Actual:   relocatedCurrent.Manifest.GenerationID,
			Message:  "relocated current generation changed during migration",
		}, treeSnapshot{})
	}

	receipt, projected, err := m.projector.projectLocked(
		ctx,
		legacy,
		catalog,
		identity,
		InputExpectation{Path: legacy, Exists: false},
	)
	if err != nil {
		return rollback(err, projected)
	}
	return LegacyLayoutMigrationResult{
		WorkspacePath:     legacy,
		StatePath:         state,
		GenerationID:      generation.Manifest.GenerationID,
		PayloadChecksum:   generation.Manifest.Payload.Checksum,
		WorkspaceChecksum: receipt.WorkspaceChecksum,
		RetainedCount:     retained,
	}, nil
}

func requireAbsentMigrationTarget(path string) error {
	_, err := os.Lstat(path)
	switch {
	case stderrors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return errors.WrapIO("inspect", path, err)
	default:
		return &errors.ConflictError{
			Resource: "catalog migration target",
			Actual:   path,
			Message:  "target already exists",
		}
	}
}

func requireLegacyStoreShape(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return errors.WrapIO("inspect", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return &errors.ValidationError{
			Field: "legacy_catalog_path", Value: path, Message: "must be a real directory",
		}
	}
	entries, err := readLegacyLayoutEntries(path)
	if err != nil {
		return errors.WrapIO("read", path, err)
	}
	allowed := map[string]bool{
		".commit.lock": false,
		"current":      false,
		"generations":  false,
	}
	for _, entry := range entries {
		if _, ok := allowed[entry.Name()]; !ok {
			return &errors.ValidationError{
				Field: "legacy_catalog_layout.entry", Value: entry.Name(),
				Message: "is not part of the immutable generation-store layout",
			}
		}
		allowed[entry.Name()] = true
	}
	for name, found := range allowed {
		if !found {
			return &errors.ValidationError{
				Field: "legacy_catalog_layout.entry", Value: name, Message: "is required",
			}
		}
	}
	for _, name := range []string{".commit.lock", "current"} {
		entryInfo, err := os.Lstat(filepath.Join(path, name))
		if err != nil {
			return errors.WrapIO("inspect", filepath.Join(path, name), err)
		}
		if !entryInfo.Mode().IsRegular() {
			return &errors.ValidationError{
				Field: "legacy_catalog_layout.entry", Value: name, Message: "must be a regular file",
			}
		}
	}
	generations, err := os.Lstat(filepath.Join(path, "generations"))
	if err != nil {
		return errors.WrapIO("inspect", filepath.Join(path, "generations"), err)
	}
	if !generations.IsDir() || generations.Mode()&os.ModeSymlink != 0 {
		return &errors.ValidationError{
			Field: "legacy_catalog_layout.entry", Value: "generations", Message: "must be a real directory",
		}
	}
	return nil
}

func inspectLegacyStore(
	ctx context.Context,
	path string,
) (catalogs.Generation, *catalogs.Catalog, int, error) {
	store, err := storage.NewFilesystem(path)
	if err != nil {
		return catalogs.Generation{}, nil, 0, err
	}
	current, err := store.Current(ctx)
	if err != nil {
		return catalogs.Generation{}, nil, 0, errors.WrapResource(
			"validate", "legacy catalog generation", "current", err,
		)
	}
	catalog, err := validateMigrationGeneration(current)
	if err != nil {
		return catalogs.Generation{}, nil, 0, err
	}

	retained, err := scanLegacyGenerations(ctx, filepath.Join(path, "generations"), func(entry fs.DirEntry) error {
		return inspectLegacyGeneration(ctx, path, store, entry)
	})
	if err != nil {
		return catalogs.Generation{}, nil, 0, err
	}
	if retained == 0 {
		return catalogs.Generation{}, nil, 0, &errors.ValidationError{
			Field: "legacy_catalog_layout.generations", Message: "must not be empty",
		}
	}
	return current, catalog, retained, nil
}

func inspectLegacyGeneration(ctx context.Context, path string, store *storage.Filesystem, entry fs.DirEntry) error {
	if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
		return &errors.ValidationError{
			Field: "legacy_catalog_layout.generation", Value: entry.Name(),
			Message: "must be a real directory",
		}
	}
	dir := filepath.Join(path, "generations", entry.Name())
	children, err := readLegacyLayoutEntries(dir)
	if err != nil {
		return errors.WrapIO("read", dir, err)
	}
	hasAuthorityRecord := len(children) == 3 && children[0].Name() == legacyAuthorityRecordName
	catalogChildren := children
	if hasAuthorityRecord {
		catalogChildren = children[1:]
	}
	if len(catalogChildren) != 2 ||
		catalogChildren[0].Name() != "catalog.json" ||
		catalogChildren[1].Name() != "manifest.json" {
		return &errors.ValidationError{
			Field: "legacy_catalog_layout.generation", Value: entry.Name(),
			Message: "must contain catalog.json, manifest.json, and only an optional authority.json record",
		}
	}
	directory, err := privatefiles.ExistingDirectory(dir)
	if err != nil {
		return err
	}
	manifestData, err := directory.ReadFile("manifest.json", storage.MaxFilesystemManifestBytes)
	if err != nil {
		return errors.WrapIO(
			"read", filepath.Join(dir, "manifest.json"), err,
		)
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestData)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(manifest.GenerationID))
	if entry.Name() != hex.EncodeToString(digest[:]) {
		return &errors.ValidationError{
			Field: "legacy_catalog_layout.generation", Value: entry.Name(),
			Message: "directory does not match the generation identity",
		}
	}
	generation, err := store.Get(ctx, manifest.GenerationID)
	if err != nil {
		return errors.WrapResource(
			"validate", "retained catalog generation", manifest.GenerationID, err,
		)
	}
	if hasAuthorityRecord {
		if err := validateLegacyAuthorityRecord(dir, generation); err != nil {
			return err
		}
	}
	if _, err := validateMigrationGeneration(generation); err != nil {
		return err
	}
	return nil
}

func validateLegacyAuthorityRecord(path string, generation catalogs.Generation) error {
	directory, err := privatefiles.ExistingDirectory(path)
	if err != nil {
		return err
	}
	data, err := directory.ReadFile(legacyAuthorityRecordName, catalogs.MaxCatalogAuthorityRecordBytes)
	if err != nil {
		return err
	}
	record, err := catalogs.ParseCatalogAuthorityRecord(data)
	if err != nil {
		return err
	}
	if record.Head != generation.Manifest.AuthorityHead {
		return &errors.ValidationError{Field: "legacy_catalog_layout.authority", Message: "must match the complete generation authority head"}
	}
	return nil
}

func validateMigrationGeneration(generation catalogs.Generation) (*catalogs.Catalog, error) {
	if generation.Manifest.SchemaVersion != catalogs.CurrentCatalogSchemaVersion ||
		!generation.Manifest.ConsumerCompatibility.SupportsSchema(catalogs.CurrentCatalogSchemaVersion) {
		return nil, &errors.ValidationError{
			Field: "schema_version", Value: generation.Manifest.SchemaVersion,
			Message: "generation is not compatible with this Starmap binary",
		}
	}
	catalog, err := catalogs.DecodeCatalogPayload(generation.Payload)
	if err != nil {
		return nil, errors.WrapResource(
			"decode", "catalog generation", generation.Manifest.GenerationID, err,
		)
	}
	return catalog, nil
}

func sameMigrationGeneration(left, right catalogs.Generation) bool {
	leftManifest, leftErr := json.Marshal(left.Manifest)
	rightManifest, rightErr := json.Marshal(right.Manifest)
	return leftErr == nil &&
		rightErr == nil &&
		bytes.Equal(leftManifest, rightManifest) &&
		bytes.Equal(left.Payload, right.Payload)
}
