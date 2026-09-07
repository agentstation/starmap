package workspace

import (
	"os"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/pkg/errors"
)

type readerLock struct {
	target string
	before os.FileInfo
	lock   *flock.Flock
}

func acquireReaderLock(target string) (readerLock, error) {
	if err := requireWorkspaceAccess(); err != nil {
		return readerLock{}, err
	}
	lease := readerLock{target: target}
	info, err := inspectReaderLock(target)
	if err != nil {
		return readerLock{}, err
	}
	lease.before = info
	if info == nil {
		// Writers retain their lock file. A first writer invalidates this read,
		// even if it finishes before the final inspection.
		return lease, nil
	}
	lock := flock.New(writerLockPath(target), flock.SetFlag(os.O_RDONLY))
	locked, err := lock.TryRLock()
	if err != nil {
		_ = lock.Close()
		return readerLock{}, errors.WrapIO("read lock", writerLockPath(target), err)
	}
	if !locked {
		_ = lock.Close()
		return readerLock{}, readConflict(target, "workspace writer is active; retry the read")
	}
	lease.lock = lock
	if err := lease.validate(); err != nil {
		lease.close()
		return readerLock{}, err
	}
	return lease, nil
}

func (r readerLock) validate() error {
	after, err := inspectReaderLock(r.target)
	if err != nil {
		return err
	}
	if (r.before == nil) != (after == nil) || (r.before != nil && !os.SameFile(r.before, after)) {
		return readConflict(r.target, "workspace lock changed during the read; retry the read")
	}
	return nil
}

func (r readerLock) close() {
	if r.lock != nil {
		_ = r.lock.Close()
	}
}

func inspectReaderLock(target string) (os.FileInfo, error) {
	path := writerLockPath(target)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.WrapIO("inspect", path, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, &errors.ValidationError{
			Field: "workspace_read.lock", Value: path, Message: "must be a regular file",
		}
	}
	return info, nil
}
