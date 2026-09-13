package privatefiles

import (
	"context"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// PublicationDirectoryName identifies the reserved metadata child for private record publication.
	PublicationDirectoryName = ".record-publications"
	publicationDirectory     = PublicationDirectoryName
	publicationLock          = ".owner.lock"
)

type publicationWriter struct {
	directory *Directory
	metadata  *Directory
	root      *os.Root
	records   *os.Root
	lock      *flock.Flock
	lockInfo  os.FileInfo
	parent    publicationEntry
	meta      publicationEntry
	owner     publicationRecord
}

func (d *Directory) acquirePublicationWriter(ctx context.Context, create bool) (_ *publicationWriter, resultErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var metadata *Directory
	var err error
	if create {
		metadata, err = d.Child(publicationDirectory)
	} else {
		metadata, err = d.ExistingChild(publicationDirectory)
	}
	if !create && os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	writer := &publicationWriter{directory: d, metadata: metadata}
	defer func() {
		if resultErr != nil {
			writer.close()
		}
	}()
	writer.root, err = d.Open()
	if err != nil {
		return nil, err
	}
	writer.records, err = metadata.Open()
	if err != nil {
		return nil, err
	}
	writer.parent, err = publicationEntryOf(writer.root, ".", true)
	if err != nil {
		return nil, err
	}
	writer.meta, err = publicationEntryOf(writer.records, ".", true)
	if err != nil {
		return nil, err
	}
	before, err := optionalRecordInfo(writer.records, publicationLock)
	if err != nil {
		return nil, err
	}
	if before == nil {
		if !create {
			return nil, changed(publicationLock)
		}
		file, err := CreateFile(writer.records, publicationLock)
		if err != nil && !os.IsExist(err) {
			return nil, err
		}
		if err == nil {
			syncErr := file.Sync()
			closeErr := file.Close()
			if syncErr != nil {
				return nil, syncErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
		}
		before, err = recordInfo(writer.records, publicationLock)
		if err != nil {
			return nil, err
		}
	}
	writer.lock = flock.New(filepath.Join(metadata.path, publicationLock))
	locked, err := writer.lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, &errors.ConflictError{Resource: "private record writer", Message: "another process owns record publication"}
	}
	writer.lockInfo, err = writer.lock.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, writer.lockInfo) {
		return nil, changed(publicationLock)
	}
	writer.owner, err = publicationRecordOf(writer.records, publicationLock, 0)
	if err != nil {
		return nil, err
	}
	if err := writer.check(); err != nil {
		return nil, err
	}
	if create {
		if err := filepublish.SyncDirectory(writer.records); err != nil {
			return nil, err
		}
		if err := filepublish.SyncDirectory(writer.root); err != nil {
			return nil, err
		}
	}
	return writer, ctx.Err()
}

func (w *publicationWriter) check() error {
	if err := w.directory.validateLocation(w.root); err != nil {
		return err
	}
	if err := w.metadata.validateLocation(w.records); err != nil {
		return err
	}
	if err := checkPublicationEntry(w.root, ".", true, w.parent); err != nil {
		return err
	}
	if err := checkPublicationEntry(w.records, ".", true, w.meta); err != nil {
		return err
	}
	if err := checkPublicationRecord(w.records, publicationLock, w.owner); err != nil {
		return err
	}
	held, err := w.lock.Stat()
	if err != nil {
		return err
	}
	current, err := recordInfo(w.records, publicationLock)
	if err != nil {
		return err
	}
	if !os.SameFile(w.lockInfo, held) || !os.SameFile(w.lockInfo, current) {
		return changed(publicationLock)
	}
	return nil
}

func (w *publicationWriter) close() {
	if w.lock != nil {
		_ = w.lock.Close()
		w.lock = nil
	}
	if w.records != nil {
		_ = w.records.Close()
		w.records = nil
	}
	if w.root != nil {
		_ = w.root.Close()
		w.root = nil
	}
}
