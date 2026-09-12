package workspace

import (
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/pkg/errors"
)

type workspaceWriter struct {
	target   string
	lock     *flock.Flock
	info     os.FileInfo
	identity string
}

func acquireWriterLock(target string) (func(), error) {
	writer, err := acquireWorkspaceWriter(target)
	if err != nil {
		return nil, err
	}
	return writer.close, nil
}

func acquireWorkspaceWriter(target string) (_ *workspaceWriter, resultErr error) {
	if err := requireWorkspaceAccess(); err != nil {
		return nil, err
	}
	path := writerLockPath(target)
	before, err := inspectWriterLock(target)
	if err != nil {
		return nil, err
	}
	writer := &workspaceWriter{target: target, lock: flock.New(path)}
	defer func() {
		if resultErr != nil {
			writer.close()
		}
	}()
	locked, err := writer.lock.TryLock()
	if err != nil {
		return nil, errors.WrapIO("lock", path, err)
	}
	if !locked {
		return nil, writerConflict(target)
	}
	writer.info, err = writer.lock.Stat()
	if err != nil {
		return nil, errors.WrapIO("inspect lock", path, err)
	}
	if before != nil && !os.SameFile(before, writer.info) {
		return nil, writerConflict(target)
	}
	if err := writer.check(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	file, err := openSnapshotEntry(root, filepath.Base(path), writer.info)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(opened, writer.info) {
		return nil, writerConflict(target)
	}
	writer.identity, err = entryIdentity(file)
	if err != nil {
		return nil, err
	}
	return writer, writer.check()
}

func (w *workspaceWriter) check() error {
	if w == nil {
		return writerConflict("")
	}
	if w.lock == nil || w.info == nil {
		return writerConflict(w.target)
	}
	current, err := inspectWriterLock(w.target)
	if err != nil {
		return err
	}
	if current == nil || !os.SameFile(w.info, current) {
		return writerConflict(w.target)
	}
	opened, err := w.lock.Stat()
	if err != nil {
		return errors.WrapIO("inspect writer lock", writerLockPath(w.target), err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(w.info, opened) {
		return writerConflict(w.target)
	}
	return nil
}

func (w *workspaceWriter) close() {
	if w != nil && w.lock != nil {
		_ = w.lock.Close()
		w.lock = nil
	}
}

func inspectWriterLock(target string) (os.FileInfo, error) {
	path := writerLockPath(target)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.WrapIO("inspect", path, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, &errors.ValidationError{Field: "workspace_writer.lock", Value: path, Message: "must be a regular file"}
	}
	return info, nil
}

func writerConflict(target string) error {
	return &errors.ConflictError{Resource: "catalog workspace writer", Actual: writerLockPath(target), Message: "writer lock changed or another process owns the workspace"}
}

func writerLockPath(target string) string {
	return filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".starmap-write.lock")
}
