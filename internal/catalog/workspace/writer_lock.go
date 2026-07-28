package workspace

import (
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/pkg/errors"
)

func acquireWriterLock(target string) (func(), error) {
	path := writerLockPath(target)
	info, err := os.Lstat(path)
	if err == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return nil, &errors.ValidationError{
			Field:   "workspace_writer.lock",
			Value:   path,
			Message: "must be a regular file",
		}
	}
	if err != nil && !stderrors.Is(err, fs.ErrNotExist) {
		return nil, errors.WrapIO("inspect", path, err)
	}
	lock := flock.New(path)
	locked, err := lock.TryLock()
	if err != nil {
		return nil, errors.WrapIO("lock", path, err)
	}
	if !locked {
		return nil, &errors.ConflictError{
			Resource: "catalog workspace writer",
			Actual:   path,
			Message:  "another process is projecting or repairing this workspace",
		}
	}
	return func() {
		_ = lock.Unlock()
	}, nil
}

func writerLockPath(target string) string {
	return filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".starmap-write.lock")
}
