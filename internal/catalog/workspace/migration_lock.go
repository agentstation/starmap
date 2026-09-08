package workspace

import (
	"context"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/pkg/errors"
)

func acquireLegacyStoreLock(ctx context.Context, legacy string) (func(), error) {
	original := filepath.Join(legacy, ".commit.lock")
	before, err := readTargetInfo(original)
	if err != nil {
		return nil, errors.WrapIO("inspect", original, err)
	}
	if !before.Mode().IsRegular() {
		return nil, migrationLockConflict(original)
	}
	path, cleanup, err := prepareLegacyLockPath(original, before)
	if err != nil {
		return nil, errors.WrapIO("prepare lock", original, err)
	}
	lock := flock.New(path, flock.SetFlag(os.O_RDWR))
	release := func() { _ = lock.Close(); cleanup() }
	locked, err := lock.TryLockContext(ctx, lockRetryDelay)
	if err != nil {
		release()
		return nil, errors.WrapIO("lock", legacy, err)
	}
	if !locked {
		release()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, migrationLockConflict(original)
	}
	opened, err := lock.Stat()
	if err != nil {
		release()
		return nil, errors.WrapIO("inspect lock", original, err)
	}
	for _, name := range []string{original, path} {
		selected, err := readTargetInfo(name)
		if err != nil {
			release()
			return nil, errors.WrapIO("inspect lock", name, err)
		}
		if !selected.Mode().IsRegular() || !os.SameFile(before, selected) || !os.SameFile(before, opened) {
			release()
			return nil, migrationLockConflict(name)
		}
	}
	return release, nil
}

func migrationLockConflict(path string) error {
	return &errors.ConflictError{Resource: "legacy catalog commit lock", Actual: path, Message: "lock identity changed or exclusive access is unavailable"}
}
