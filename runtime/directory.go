package runtime

import (
	"context"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/pkg/errors"
)

const (
	directoryLockName = ".owner.lock"
)

// acquireDirectory holds one runtime directory until all of its work stops.
// The lock file stays in place after release so other processes lock the same file.
func acquireDirectory(ctx context.Context, directory string) (*flock.Flock, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if directory == "" {
		return nil, nil
	}
	if !filepath.IsAbs(directory) {
		return nil, &errors.ValidationError{Field: "runtime.directory", Message: "must be an absolute directory"}
	}
	if err := ValidateDirectoryPermissions(ctx, directory); err != nil {
		return nil, err
	}
	if err := createPrivateRuntimeDirectory(directory); err != nil {
		return nil, errors.WrapIO("create", directory, err)
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, &errors.ValidationError{Field: "runtime.directory", Message: "must be a real directory"}
	}
	if err := ValidateDirectoryPermissions(ctx, directory); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(directory, directoryLockName)
	if err := validateDirectoryLock(lockPath); err != nil {
		return nil, err
	}
	if err := preparePrivateRuntimeLock(directory); err != nil {
		return nil, err
	}
	lock := flock.New(lockPath)
	held, err := lock.TryLock()
	if err != nil {
		_ = lock.Close()
		return nil, errors.WrapIO("lock", directory, err)
	}
	if !held {
		_ = lock.Close()
		return nil, &errors.ConflictError{Resource: "runtime directory", Message: "another process owns this directory"}
	}
	if err := validateDirectoryLock(lockPath); err != nil {
		_ = lock.Close()
		return nil, err
	}
	if err := ValidateDirectoryPermissions(ctx, directory); err != nil {
		_ = lock.Close()
		return nil, err
	}
	current, err := os.Lstat(directory)
	if err != nil || !os.SameFile(info, current) {
		_ = lock.Close()
		return nil, &errors.ConflictError{Resource: "runtime directory", Message: "directory changed during ownership acquisition"}
	}
	if err := ctx.Err(); err != nil {
		_ = lock.Close()
		return nil, err
	}
	return lock, nil
}

func validateDirectoryLock(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return &errors.ValidationError{Field: "runtime.owner_lock", Message: "must be a regular file"}
	}
	return nil
}
