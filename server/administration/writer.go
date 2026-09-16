package administration

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

const (
	identitiesFile = "identities.json"
	ownerFile      = ".owner.lock"
)

// stateWriter holds the process lease for one standalone administration directory.
type stateWriter struct {
	storageErr error
	directory  *privatefiles.Directory
	audit      *privatefiles.Directory
	operations *privatefiles.Directory
	lock       *flock.Flock
	owner      privatefiles.RecordReceipt
}

func openStateWriter(ctx context.Context, statePath string, create bool) (_ *stateWriter, resultErr error) {
	for _, role := range []string{"admin-identities", "admin-audit", "admin-operations", "admin-owner"} {
		if err := policy.Require(role, policy.OwnerOnly); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var state *privatefiles.Directory
	var err error
	if create {
		state, err = privatefiles.NewDirectory(statePath)
	} else {
		state, err = privatefiles.ExistingDirectory(statePath)
	}
	if err != nil {
		return nil, err
	}
	writer := &stateWriter{}
	if create {
		writer.directory, err = state.Child("admin")
	} else {
		writer.directory, err = state.ExistingChild("admin")
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			_ = writer.close()
		}
	}()
	root, err := writer.directory.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	before, err := root.Lstat(ownerFile)
	if os.IsNotExist(err) && create {
		file, createErr := privatefiles.CreateFile(root, ownerFile)
		if createErr != nil && !os.IsExist(createErr) {
			return nil, createErr
		}
		if createErr == nil {
			if err := stderrors.Join(file.Sync(), file.Close()); err != nil {
				return nil, err
			}
			if err := filepublish.SyncDirectory(root); err != nil {
				return nil, err
			}
		}
		before, err = root.Lstat(ownerFile)
	}
	if err != nil {
		return nil, err
	}
	writer.owner, err = privatefiles.CaptureRecord(root, ownerFile, 0)
	if err != nil {
		return nil, err
	}
	writer.lock = flock.New(filepath.Join(statePath, "admin", ownerFile))
	locked, err := writer.lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, &errors.ConflictError{Resource: "server administration", Message: "another process owns administration state"}
	}
	held, err := writer.lock.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, held) {
		return nil, administrationChanged()
	}
	if err := writer.check(); err != nil {
		return nil, err
	}
	if create {
		writer.audit, err = writer.directory.Child("audit")
	} else {
		writer.audit, err = writer.directory.ExistingChild("audit")
	}
	if err != nil {
		if !create {
			writer.storageErr = err
			return writer, nil
		}
		return nil, err
	}
	if create {
		writer.operations, err = writer.directory.Child("operations")
	} else {
		writer.operations, err = writer.directory.ExistingChild("operations")
	}
	if err != nil {
		if !create {
			writer.storageErr = err
			return writer, nil
		}
		return nil, err
	}
	return writer, ctx.Err()
}

func (w *stateWriter) check() error {
	if w.lock == nil {
		return administrationChanged()
	}
	root, err := w.directory.Open()
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if err := privatefiles.CheckRecord(root, ownerFile, w.owner); err != nil {
		return err
	}
	held, err := w.lock.Stat()
	if err != nil {
		return err
	}
	current, err := root.Lstat(ownerFile)
	if err != nil {
		return err
	}
	if !os.SameFile(held, current) {
		return administrationChanged()
	}
	return nil
}

func (w *stateWriter) close() error {
	if w.lock == nil {
		return nil
	}
	err := w.lock.Close()
	w.lock = nil
	return err
}

func administrationChanged() error {
	return &errors.ConflictError{Resource: "server administration", Message: "state identity or writer ownership changed"}
}
