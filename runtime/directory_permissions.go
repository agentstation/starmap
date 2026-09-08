package runtime

import (
	"context"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

// ValidateDirectoryPermissions checks existing runtime identity paths without writes.
// Linux and macOS require the effective UID and no group or other mode bits.
// macOS also restricts ACL grants to the owner. Deny entries remain valid.
// Windows requires the process account owner and limits grants to that account, SYSTEM, and Administrators.
//
// Linux and macOS also check ancestor ownership, directory-entry protection, and selected symlink routes.
// Windows also checks ancestor DACLs and selected symlink routes.
// Effective access and native Windows execution require separate qualification.
// Other platforms check file types only. Windows native qualification remains required.
//
// An empty directory selects in-memory state. Missing paths remain absent.
func ValidateDirectoryPermissions(ctx context.Context, directory string) error {
	if ctx == nil {
		return &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if directory == "" {
		return nil
	}
	if !filepath.IsAbs(directory) {
		return &errors.ValidationError{Field: "runtime.directory", Message: "must be absolute"}
	}
	for _, role := range []string{"runtime-owner", "runtime-lock", "runtime-seed", "runtime-record-staging",
		"migration-pending", "migration-receipt", "migration-completed", "migration-retired", "migration-journal", "runtime-migration"} {
		if err := policy.Require(role, policy.OwnerOnly); err != nil {
			return err
		}
	}
	if err := privatefiles.ValidateAncestors(directory); err != nil {
		return err
	}
	info, err := os.Lstat(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return errors.WrapIO("inspect runtime permissions", directory, err)
	}
	if !info.IsDir() {
		return &errors.ValidationError{Field: "runtime.directory", Message: "must be a real directory"}
	}
	if err := validatePrivateMetadata(info, "runtime.directory"); err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	opened, err := root.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(info, opened) {
		return &errors.ConflictError{Resource: "runtime directory", Message: "directory changed during permission inspection"}
	}
	if err := validatePrivateACL(root, ".", opened, "runtime.directory"); err != nil {
		return err
	}
	for _, item := range []struct{ name, field string }{
		{directoryLockName, "runtime.owner_lock"},
		{ownerRecordName, "runtime.owner_record"},
		{instanceSeedFileName, "runtime.instance_seed"},
	} {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := root.Lstat(item.name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			if item.name == instanceSeedFileName {
				return &errors.ConflictError{Resource: "runtime instance seed", Message: "seed is not a valid private identity record"}
			}
			if item.name == ownerRecordName {
				return &errors.ConflictError{Resource: "runtime directory owner", Message: "record requires explicit recovery"}
			}
			return &errors.ValidationError{Field: item.field, Message: "must be a regular file"}
		}
		if err := validatePrivateMetadata(info, item.field); err != nil {
			return err
		}
		if err := validatePrivateACL(root, item.name, info, item.field); err != nil {
			return err
		}
	}
	return ctx.Err()
}
