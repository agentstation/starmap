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

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

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
}

// MigrateLegacyLayout explicitly relocates the pre-plan filesystem generation
// store and projects its current generation back to the vacated human workspace
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

	storeLock := flock.New(filepath.Join(legacy, ".commit.lock"))
	locked, err := storeLock.TryLockContext(ctx, constants.CatalogStoreLockRetryDelay)
	if err != nil {
		return LegacyLayoutMigrationResult{}, errors.WrapIO("lock", legacy, err)
	}
	if !locked {
		if err := ctx.Err(); err != nil {
			return LegacyLayoutMigrationResult{}, err
		}
		return LegacyLayoutMigrationResult{}, &errors.ConflictError{
			Resource: "legacy catalog generation store",
			Message:  "commit lock was not acquired",
		}
	}
	defer func() { _ = storeLock.Unlock() }()

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

	writerPath := writerLockPath(legacy)
	_, writerStatErr := os.Lstat(writerPath)
	writerLockExisted := writerStatErr == nil
	if writerStatErr != nil && !stderrors.Is(writerStatErr, fs.ErrNotExist) {
		return LegacyLayoutMigrationResult{}, errors.WrapIO("inspect", writerPath, writerStatErr)
	}
	releaseWriter, err := acquireWriterLock(legacy)
	if err != nil {
		return LegacyLayoutMigrationResult{}, err
	}
	succeeded := false
	defer func() {
		releaseWriter()
		if !succeeded && !writerLockExisted {
			_ = os.Remove(writerPath)
		}
	}()

	if err := os.MkdirAll(filepath.Dir(state), constants.DirPermissions); err != nil {
		return LegacyLayoutMigrationResult{}, errors.WrapIO("create", filepath.Dir(state), err)
	}
	if err := os.Rename(legacy, state); err != nil {
		return LegacyLayoutMigrationResult{}, errors.WrapIO("relocate", legacy, err)
	}
	rollback := func(cause error, projectedChecksum string) (LegacyLayoutMigrationResult, error) {
		if rollbackErr := rollbackLegacyMove(legacy, state, projectedChecksum); rollbackErr != nil {
			return LegacyLayoutMigrationResult{}, errors.WrapResource(
				"rollback",
				"legacy catalog layout migration",
				legacy,
				stderrors.Join(cause, rollbackErr),
			)
		}
		return LegacyLayoutMigrationResult{}, cause
	}
	if err := syncMigrationParents(legacy, state); err != nil {
		return rollback(errors.WrapIO("sync", state, err), "")
	}
	if m.afterMove != nil {
		if err := m.afterMove(); err != nil {
			return rollback(err, "")
		}
	}

	relocated, err := catalogstore.NewFilesystem(state)
	if err != nil {
		return rollback(err, "")
	}
	relocatedCurrent, err := relocated.Current(ctx)
	if err != nil {
		return rollback(errors.WrapResource("verify", "relocated catalog generation", "current", err), "")
	}
	if !sameMigrationGeneration(generation, relocatedCurrent) {
		return rollback(&errors.ConflictError{
			Resource: "relocated catalog generation",
			Expected: generation.Manifest.GenerationID,
			Actual:   relocatedCurrent.Manifest.GenerationID,
			Message:  "relocated current generation changed during migration",
		}, "")
	}

	receipt, err := (projector{}).projectLocked(
		ctx,
		legacy,
		catalog,
		identity,
		InputExpectation{Path: legacy, Exists: false},
	)
	if err != nil {
		return rollback(err, receipt.WorkspaceChecksum)
	}
	succeeded = true
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
	entries, err := os.ReadDir(path)
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
) (catalogstore.Generation, *catalogs.Catalog, int, error) {
	store, err := catalogstore.NewFilesystem(path)
	if err != nil {
		return catalogstore.Generation{}, nil, 0, err
	}
	current, err := store.Current(ctx)
	if err != nil {
		return catalogstore.Generation{}, nil, 0, errors.WrapResource(
			"validate", "legacy catalog generation", "current", err,
		)
	}
	catalog, err := validateMigrationGeneration(current)
	if err != nil {
		return catalogstore.Generation{}, nil, 0, err
	}

	entries, err := os.ReadDir(filepath.Join(path, "generations"))
	if err != nil {
		return catalogstore.Generation{}, nil, 0, errors.WrapIO(
			"read", filepath.Join(path, "generations"), err,
		)
	}
	if len(entries) == 0 {
		return catalogstore.Generation{}, nil, 0, &errors.ValidationError{
			Field: "legacy_catalog_layout.generations", Message: "must not be empty",
		}
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			return catalogstore.Generation{}, nil, 0, &errors.ValidationError{
				Field: "legacy_catalog_layout.generation", Value: entry.Name(),
				Message: "must be a real directory",
			}
		}
		dir := filepath.Join(path, "generations", entry.Name())
		children, err := os.ReadDir(dir)
		if err != nil {
			return catalogstore.Generation{}, nil, 0, errors.WrapIO("read", dir, err)
		}
		if len(children) != 2 ||
			children[0].Name() != "catalog.json" ||
			children[1].Name() != "manifest.json" {
			return catalogstore.Generation{}, nil, 0, &errors.ValidationError{
				Field: "legacy_catalog_layout.generation", Value: entry.Name(),
				Message: "must contain exactly catalog.json and manifest.json",
			}
		}
		manifestData, err := os.ReadFile(filepath.Join(dir, "manifest.json")) //nolint:gosec
		if err != nil {
			return catalogstore.Generation{}, nil, 0, errors.WrapIO(
				"read", filepath.Join(dir, "manifest.json"), err,
			)
		}
		manifest, err := catalogs.ParseGenerationManifestJSON(manifestData)
		if err != nil {
			return catalogstore.Generation{}, nil, 0, err
		}
		digest := sha256.Sum256([]byte(manifest.GenerationID))
		if entry.Name() != hex.EncodeToString(digest[:]) {
			return catalogstore.Generation{}, nil, 0, &errors.ValidationError{
				Field: "legacy_catalog_layout.generation", Value: entry.Name(),
				Message: "directory does not match the generation identity",
			}
		}
		generation, err := store.Get(ctx, manifest.GenerationID)
		if err != nil {
			return catalogstore.Generation{}, nil, 0, errors.WrapResource(
				"validate", "retained catalog generation", manifest.GenerationID, err,
			)
		}
		if _, err := validateMigrationGeneration(generation); err != nil {
			return catalogstore.Generation{}, nil, 0, err
		}
	}
	return current, catalog, len(entries), nil
}

func validateMigrationGeneration(generation catalogstore.Generation) (*catalogs.Catalog, error) {
	if generation.Manifest.SchemaVersion != catalogs.CurrentCatalogSchemaVersion ||
		!generation.Manifest.ConsumerCompatibility.SupportsSchema(catalogs.CurrentCatalogSchemaVersion) {
		return nil, &errors.ValidationError{
			Field: "schema_version", Value: generation.Manifest.SchemaVersion,
			Message: "generation is not compatible with this Starmap binary",
		}
	}
	catalog, err := catalogstore.DecodeCatalogPayload(generation.Payload)
	if err != nil {
		return nil, errors.WrapResource(
			"decode", "catalog generation", generation.Manifest.GenerationID, err,
		)
	}
	return catalog, nil
}

func sameMigrationGeneration(left, right catalogstore.Generation) bool {
	leftManifest, leftErr := json.Marshal(left.Manifest)
	rightManifest, rightErr := json.Marshal(right.Manifest)
	return leftErr == nil &&
		rightErr == nil &&
		bytes.Equal(leftManifest, rightManifest) &&
		bytes.Equal(left.Payload, right.Payload)
}

func rollbackLegacyMove(legacy, state, projectedChecksum string) error {
	info, err := os.Lstat(legacy)
	switch {
	case stderrors.Is(err, fs.ErrNotExist):
		// Nothing became visible at the vacated path.
	case err != nil:
		return errors.WrapIO("inspect", legacy, err)
	default:
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return unexpectedMigrationRollbackPath(legacy)
		}
		if projectedChecksum == "" {
			return unexpectedMigrationRollbackPath(legacy)
		}
		if err := ValidateHumanLayout(legacy, ""); err != nil {
			return errors.WrapResource("validate", "migration rollback workspace", legacy, err)
		}
		visible, err := readSemanticState(legacy)
		if err != nil {
			return errors.WrapResource("validate", "migration rollback workspace", legacy, err)
		}
		if !visible.exists || visible.checksum != projectedChecksum {
			return &errors.ConflictError{
				Resource: "catalog migration rollback workspace",
				Expected: projectedChecksum,
				Actual:   visible.describe(),
				Message:  "vacated catalog path changed after relocation; relocated state was preserved",
			}
		}
		if err := os.RemoveAll(legacy); err != nil {
			return errors.WrapIO("remove", legacy, err)
		}
	}
	if err := os.Remove(projectionMarkerPath(legacy)); err != nil && !stderrors.Is(err, fs.ErrNotExist) {
		return errors.WrapIO("remove", projectionMarkerPath(legacy), err)
	}
	if err := os.Rename(state, legacy); err != nil {
		return errors.WrapIO("restore", legacy, err)
	}
	return syncMigrationParents(legacy, state)
}

func unexpectedMigrationRollbackPath(path string) error {
	return &errors.ConflictError{
		Resource: "catalog migration rollback workspace",
		Actual:   path,
		Message:  "vacated catalog path was recreated after relocation; relocated state was preserved",
	}
}

func syncMigrationParents(first, second string) error {
	parents := []string{filepath.Dir(first)}
	if other := filepath.Dir(second); other != parents[0] {
		parents = append(parents, other)
	}
	for _, parent := range parents {
		if err := syncDirectory(parent); err != nil {
			return err
		}
	}
	return nil
}
