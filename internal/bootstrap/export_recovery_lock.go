package bootstrap

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

const (
	baselineRecoveryDirectory = ".starmap-baseline"
	baselineRecoveryLock      = ".owner.lock"
	baselineRecoveryLockRetry = 20 * time.Millisecond
)

type baselineRecovery struct {
	parent       *os.Root
	directory    *privatefiles.Directory
	root         *os.Root
	lock         *flock.Flock
	lockInfo     os.FileInfo
	lockIdentity string
}

func openBaselineRecovery(ctx context.Context, parent *os.Root, path string) (_ *baselineRecovery, resultErr error) {
	if err := policy.Require("baseline-recovery", policy.OwnerOnly); err != nil {
		return nil, err
	}
	if err := privatefiles.CreateChild(parent, baselineRecoveryDirectory); err != nil && !os.IsExist(err) {
		return nil, err
	}
	selected, err := privatefiles.ExistingDirectory(filepath.Join(path, baselineRecoveryDirectory))
	if err != nil {
		return nil, err
	}
	root, err := selected.Open()
	if err != nil {
		return nil, err
	}
	guard := &baselineRecovery{parent: parent, directory: selected, root: root}
	defer func() {
		if resultErr != nil {
			resultErr = stderrors.Join(resultErr, guard.close())
		}
	}()
	bound, err := parent.Stat(".")
	if err != nil {
		return nil, err
	}
	current, err := os.Lstat(path)
	if err != nil || !os.SameFile(bound, current) {
		return nil, stderrors.Join(stageConflict(path), err)
	}
	file, err := privatefiles.CreateFile(root, baselineRecoveryLock)
	if err == nil {
		err = stderrors.Join(file.Sync(), file.Close(), filepublish.SyncDirectory(root), filepublish.SyncDirectory(parent))
	}
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	if _, err := privatefiles.ReadFile(root, baselineRecoveryLock, 0); err != nil {
		return nil, err
	}
	before, err := root.Lstat(baselineRecoveryLock)
	if err != nil {
		return nil, err
	}
	guard.lock = flock.New(filepath.Join(path, baselineRecoveryDirectory, baselineRecoveryLock), flock.SetFlag(os.O_RDWR))
	locked, err := guard.lock.TryLockContext(ctx, baselineRecoveryLockRetry)
	if err != nil {
		return nil, errors.WrapIO("lock", path, err)
	}
	if !locked {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, &errors.ConflictError{Resource: "baseline export", Message: "another writer owns baseline recovery"}
	}
	opened, err := guard.lock.Stat()
	if err != nil {
		return nil, err
	}
	after, err := root.Lstat(baselineRecoveryLock)
	if err != nil || !os.SameFile(before, opened) || !os.SameFile(before, after) {
		return nil, stderrors.Join(stageConflict(baselineRecoveryLock), err)
	}
	checked, err := selected.Open()
	if err != nil {
		return nil, err
	}
	if err := checked.Close(); err != nil {
		return nil, err
	}
	guard.lockInfo = opened
	guard.lockIdentity, err = baselineEntryIdentity(root, baselineRecoveryLock, opened)
	if err != nil {
		return nil, err
	}
	return guard, nil
}

func (r *baselineRecovery) ownsLock() error {
	root, err := r.directory.Open()
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	current, err := root.Lstat(baselineRecoveryLock)
	if err != nil || !os.SameFile(r.lockInfo, current) || !sameBaselineMetadata(r.lockInfo, current) {
		return stderrors.Join(stageConflict(baselineRecoveryLock), err)
	}
	opened, err := r.lock.Stat()
	if err != nil || !os.SameFile(r.lockInfo, opened) {
		return stderrors.Join(stageConflict(baselineRecoveryLock), err)
	}
	_, err = privatefiles.ReadFile(root, baselineRecoveryLock, 0)
	return err
}

func (r *baselineRecovery) close() error {
	var result error
	if r.lock != nil {
		result = r.lock.Close()
	}
	if r.root != nil {
		result = stderrors.Join(result, r.root.Close())
	}
	return result
}
