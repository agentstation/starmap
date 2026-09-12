package workspace

import (
	"context"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/pkg/errors"
)

type legacyStoreLease struct {
	lock    *flock.Flock
	info    os.FileInfo
	path    string
	cleanup func()
	closed  bool
}

func acquireLegacyStoreLock(ctx context.Context, legacy string) (func(), error) {
	lease, err := acquireLegacyStoreLease(ctx, legacy)
	if err != nil {
		return nil, err
	}
	return lease.close, nil
}

func acquireLegacyStoreLease(ctx context.Context, legacy string) (*legacyStoreLease, error) {
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
	return lockLegacyStore(ctx, legacy, path, before, cleanup)
}

func lockLegacyStore(ctx context.Context, legacy, path string, before os.FileInfo, cleanup func()) (*legacyStoreLease, error) {
	original := filepath.Join(legacy, ".commit.lock")
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
	return &legacyStoreLease{lock: lock, info: opened, path: path, cleanup: cleanup}, nil
}

func (l *legacyStoreLease) close() {
	if l != nil && !l.closed {
		l.closed = true
		_ = l.lock.Close()
		l.cleanup()
	}
}

func (l *legacyStoreLease) check(store string) error {
	opened, err := l.lock.Stat()
	if err != nil {
		return err
	}
	current, err := readTargetInfo(filepath.Join(store, ".commit.lock"))
	if err != nil {
		return err
	}
	if !opened.Mode().IsRegular() || !current.Mode().IsRegular() || !os.SameFile(l.info, opened) || !os.SameFile(opened, current) {
		return migrationLockConflict(store)
	}
	return nil
}

func migrationLockConflict(path string) error {
	return &errors.ConflictError{Resource: "legacy catalog commit lock", Actual: path, Message: "lock identity changed or exclusive access is unavailable"}
}
