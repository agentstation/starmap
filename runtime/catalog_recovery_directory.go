package runtime

import (
	"context"
	stderrors "errors"
	"path/filepath"
)

// PrepareCatalogRecoveryDirectory establishes private owner, seed, and empty layer directories.
// It opens no runtime, reads no catalog source, and creates no selection or permission.
// Existing owner and seed records must match. Missing records in retained state require recovery.
// An already selected materialization requires passive inspection instead of preparation.
//
// The host must fence writers and call this only before sealing its native activation decision.
// Restart paths must inspect the retained directory instead of invoking preparation.
func PrepareCatalogRecoveryDirectory(ctx context.Context, path string, owner DirectoryOwner, identity string) (resultErr error) {
	if ctx == nil || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return invalidInputPublication("recovery preparation requires a context and canonical absolute directory")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := owner.Validate(); err != nil {
		return err
	}
	lock, err := acquireDirectory(ctx, path)
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, lock.Close()) }()
	if err := refusePendingMigration(path); err != nil {
		return err
	}
	if err := refuseRetiredMigration(path); err != nil {
		return err
	}
	selection, err := checkMaterializationSelection(ctx, path)
	if err != nil {
		return err
	}
	if selection != nil {
		return invalidInputPublication("selected recovery materialization requires passive inspection")
	}
	if err := bindDirectoryOwner(ctx, path, owner, identity); err != nil {
		return err
	}
	if _, err := prepareInstanceSeed(ctx, path); err != nil {
		return err
	}
	if _, err := newLayerStore(path); err != nil {
		return err
	}
	return InspectRetainedDirectory(ctx, path, owner, identity)
}
