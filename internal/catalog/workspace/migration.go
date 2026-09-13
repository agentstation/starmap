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
	"strings"

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
	beforeMove      func() error
	afterMove       func() error
	afterProjection func() error
	projector       projector
}

// MigrateLegacyLayout moves the legacy catalog store and projects its current catalog into the vacated human catalog workspace.
// A retry recovers a recorded relocation only when retained files and filesystem identities still match.
// Validation and both advisory locks precede relocation.
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
	legacy, state, err := prepareLegacyLayoutMigration(ctx, legacyPath, statePath)
	if err != nil {
		return LegacyLayoutMigrationResult{}, err
	}

	lease, err := acquireLegacyStoreLease(ctx, legacy)
	if err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	defer lease.close()
	originalStore, err := captureLegacyStoreIdentity(legacy)
	if err != nil {
		return LegacyLayoutMigrationResult{}, errors.WrapIO("inspect", legacy, err)
	}

	generation, catalog, retained, err := inspectLegacyStore(ctx, legacy, lease)
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

	writer, err := acquireWorkspaceWriter(legacy)
	if err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	defer writer.close()
	m.projector.writer = writer
	m.projector.recordWrites.checkWriter = writer.check
	m.projector.recordWrites.writer = writer

	if err := os.MkdirAll(filepath.Dir(state), directoryMode); err != nil {
		return LegacyLayoutMigrationResult{}, errors.WrapIO("create", filepath.Dir(state), err)
	}
	move, err := prepareLegacyStoreMove(legacy, state, originalStore)
	if err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	defer move.close()
	stage, err := prepareRelocation(ctx, legacy, state, generation, retained, writer, lease, nil)
	if err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	defer stage.releaseHandles()
	if stage.relocation.record.Root.Tree.ID != originalStore {
		return LegacyLayoutMigrationResult{}, replacementConflict(legacy, "legacy store identity changed before relocation")
	}
	m.projector.relocation = stage
	m.projector.relocationLease = lease
	move.check = func(store string) error { return stage.checkRelocationLease(context.WithoutCancel(ctx), store, lease) }
	if m.beforeMove != nil {
		if err := m.beforeMove(); err != nil {
			return LegacyLayoutMigrationResult{}, err
		}
	}
	if err := move.relocate(); err != nil {
		return LegacyLayoutMigrationResult{}, errors.WrapIO("relocate", legacy, err)
	}
	rollback := func(cause error, projected treeSnapshot) (LegacyLayoutMigrationResult, error) {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), workspaceCleanupTimeout)
		defer cancel()
		rollbackErr := stage.checkRelocationStore(cleanup, state, lease)
		if rollbackErr == nil {
			rollbackErr = move.rollback(cleanup, projected)
		}
		if rollbackErr == nil {
			rollbackErr = stage.finishRelocation(cleanup, lease)
		}
		if rollbackErr != nil {
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

	relocatedCurrent, _, _, err := inspectLegacyStore(ctx, state, lease)
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
	if m.afterProjection != nil {
		if err := m.afterProjection(); err != nil {
			return rollback(err, projected)
		}
	}
	if err := stage.checkRelocationStore(ctx, state, lease); err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	if err := stage.finishRelocation(ctx, lease); err != nil {
		return LegacyLayoutMigrationResult{}, err
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

// inspectLegacyStore reads the legacy layout while the caller holds its publication lease.
// It validates each generation without requesting another handle for the same lock.
func inspectLegacyStore(
	ctx context.Context,
	path string,
	lease *legacyStoreLease,
) (catalogs.Generation, *catalogs.Catalog, int, error) {
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, nil, 0, err
	}
	if err := lease.check(path); err != nil {
		return catalogs.Generation{}, nil, 0, err
	}
	directory, err := privatefiles.ExistingDirectory(path)
	if err != nil {
		return catalogs.Generation{}, nil, 0, err
	}
	pointer, err := directory.ReadFile("current", catalogs.MaxCatalogAuthorityRecordBytes)
	if err != nil {
		return catalogs.Generation{}, nil, 0, errors.WrapIO("read", filepath.Join(path, "current"), err)
	}
	id := strings.TrimSpace(string(pointer))
	if id == "" {
		return catalogs.Generation{}, nil, 0, &errors.ValidationError{Field: "current", Message: "generation ID is empty"}
	}
	var current catalogs.Generation
	var catalog *catalogs.Catalog
	retained, err := scanLegacyGenerations(ctx, filepath.Join(path, "generations"), func(entry fs.DirEntry) error {
		generation, err := inspectLegacyGeneration(ctx, path, entry)
		if err != nil {
			return err
		}
		decoded, err := validateMigrationGeneration(generation)
		if err != nil {
			return err
		}
		if generation.Manifest.GenerationID == id {
			current, catalog = generation, decoded
		}
		return nil
	})
	if err != nil {
		return catalogs.Generation{}, nil, 0, err
	}
	if catalog == nil {
		return catalogs.Generation{}, nil, 0, &errors.ValidationError{
			Field: "legacy_catalog_layout.generations", Value: id, Message: "current generation is absent",
		}
	}
	if err := lease.check(path); err != nil {
		return catalogs.Generation{}, nil, 0, err
	}
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, nil, 0, err
	}
	return current, catalog, retained, nil
}

func inspectLegacyGeneration(ctx context.Context, path string, entry fs.DirEntry) (catalogs.Generation, error) {
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, err
	}
	if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
		return catalogs.Generation{}, &errors.ValidationError{
			Field: "legacy_catalog_layout.generation", Value: entry.Name(),
			Message: "must be a real directory",
		}
	}
	dir := filepath.Join(path, "generations", entry.Name())
	children, err := readLegacyLayoutEntries(dir)
	if err != nil {
		return catalogs.Generation{}, errors.WrapIO("read", dir, err)
	}
	hasAuthorityRecord := len(children) == 3 && children[0].Name() == legacyAuthorityRecordName
	catalogChildren := children
	if hasAuthorityRecord {
		catalogChildren = children[1:]
	}
	if len(catalogChildren) != 2 ||
		catalogChildren[0].Name() != "catalog.json" ||
		catalogChildren[1].Name() != "manifest.json" {
		return catalogs.Generation{}, &errors.ValidationError{
			Field: "legacy_catalog_layout.generation", Value: entry.Name(),
			Message: "must contain catalog.json, manifest.json, and only an optional authority.json record",
		}
	}
	directory, err := privatefiles.ExistingDirectory(dir)
	if err != nil {
		return catalogs.Generation{}, err
	}
	manifestData, err := directory.ReadFile("manifest.json", storage.MaxFilesystemManifestBytes)
	if err != nil {
		return catalogs.Generation{}, errors.WrapIO(
			"read", filepath.Join(dir, "manifest.json"), err,
		)
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestData)
	if err != nil {
		return catalogs.Generation{}, err
	}
	digest := sha256.Sum256([]byte(manifest.GenerationID))
	if entry.Name() != hex.EncodeToString(digest[:]) {
		return catalogs.Generation{}, &errors.ValidationError{
			Field: "legacy_catalog_layout.generation", Value: entry.Name(),
			Message: "directory does not match the generation identity",
		}
	}
	if manifest.Payload.SizeBytes <= 0 || manifest.Payload.SizeBytes > storage.MaxFilesystemPayloadBytes {
		return catalogs.Generation{}, &errors.ValidationError{Field: "payload.size_bytes", Message: "exceeds the filesystem reader limit"}
	}
	payload, err := directory.ReadFile("catalog.json", manifest.Payload.SizeBytes)
	if err != nil {
		return catalogs.Generation{}, errors.WrapIO("read", filepath.Join(dir, "catalog.json"), err)
	}
	generation := catalogs.Generation{Manifest: manifest, Payload: payload}
	if err := generation.Validate(); err != nil {
		return catalogs.Generation{}, err
	}
	if hasAuthorityRecord {
		if err := validateLegacyAuthorityRecord(dir, generation); err != nil {
			return catalogs.Generation{}, err
		}
	}
	return generation, nil
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

func prepareLegacyLayoutMigration(ctx context.Context, legacyPath, statePath string) (string, string, error) {
	legacy, err := resolveTarget(legacyPath)
	if err != nil {
		return "", "", err
	}
	state, err := resolveTarget(statePath)
	if err != nil {
		return "", "", err
	}
	if err := ValidateMachineSeparation(legacy, state, "catalog state"); err != nil {
		return "", "", err
	}
	if err := recoverLegacyRelocation(ctx, legacy, state); err != nil {
		return "", "", err
	}
	if err := requireAbsentMigrationTarget(state); err != nil {
		return "", "", err
	}
	if err := requireLegacyStoreShape(legacy); err != nil {
		return "", "", err
	}

	return legacy, state, nil
}
