package privatefiles

import (
	"context"
	stderrors "errors"
	"io"
	"os"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
)

// PublishFileContext publishes a private record with durable staging ownership.
// Recovery never treats an unrecorded temporary filename as proof of ownership.
// The method excludes concurrent publishers and recovers their complete abandoned receipts.
func (d *Directory) PublishFileContext(ctx context.Context, name string, data []byte, prefix string) error {
	return d.publishFileContext(ctx, name, data, prefix, nil)
}

// CompareAndPublishFileContext requires the previously read bytes to remain current.
// A nil previous value requires an absent destination. An empty non-nil value requires an empty file.
func (d *Directory) CompareAndPublishFileContext(ctx context.Context, name string, previous, data []byte, prefix string) error {
	expected := publicationExpectation{present: previous != nil, size: int64(len(previous)), digest: publicationDigest(previous)}
	return d.publishRecordContext(ctx, name, data, prefix, nil, &expected)
}

type publicationExpectation struct {
	present bool
	size    int64
	digest  string
}

func (d *Directory) publishFileContext(ctx context.Context, name string, data []byte, prefix string, syncDirectory func(*os.Root) error) error {
	return d.publishRecordContext(ctx, name, data, prefix, syncDirectory, nil)
}

func (d *Directory) publishRecordContext(ctx context.Context, name string, data []byte, prefix string, syncDirectory func(*os.Root) error, expected *publicationExpectation) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := childName(name); err != nil {
		return err
	}
	if err := childName(prefix); err != nil {
		return err
	}
	if name == publicationDirectory {
		return changed(name)
	}
	if len(data) > publicationRecordMaxBytes {
		return oversized(name, publicationRecordMaxBytes)
	}
	writer, err := d.acquirePublicationWriter(ctx, true)
	if err != nil {
		return err
	}
	defer writer.close()
	if err := writer.recover(ctx); err != nil {
		return err
	}
	var before *publicationRecord
	record, err := publicationRecordOf(writer.root, name, publicationRecordMaxBytes)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		before = &record
	}
	if expected != nil {
		if expected.present != (before != nil) || (before != nil && (before.Size != expected.size || before.Digest != expected.digest)) {
			return changed(name)
		}
	}
	journal, err := writer.newJournal(name, prefix)
	if err != nil {
		return err
	}
	var file *os.File
	published := false
	defer func() {
		var closeErr error
		if file != nil {
			closeErr = file.Close()
		}
		closeErr = stderrors.Join(closeErr, journal.file.Close())
		cleanupErr := writer.cleanJournal(context.WithoutCancel(ctx), journal)
		resultErr = stderrors.Join(resultErr, closeErr, cleanupErr)
		if published && resultErr != nil {
			resultErr = &errors.PublicationError{Resource: "private file", ID: name, Err: resultErr}
		}
	}()
	file, err = CreateFile(writer.root, journal.header.Stage)
	if err != nil {
		return err
	}
	written, err := writePublicationContents(writer, journal, file, data)
	if err != nil {
		return err
	}
	published, err = d.promotePublication(ctx, writer, journal, name, before, written)
	if err != nil {
		return err
	}
	if syncDirectory == nil {
		syncDirectory = filepublish.SyncDirectory
	}
	return syncDirectory(writer.root)
}

func writePublicationContents(writer *publicationWriter, journal *publicationJournal, file *os.File, data []byte) (publicationRecord, error) {
	empty, err := publicationRecordOf(writer.root, journal.header.Stage, 0)
	if err != nil {
		return publicationRecord{}, err
	}
	if err := journal.append(writer, publicationEvent{Record: &empty}); err != nil {
		return publicationRecord{}, err
	}
	if err := filepublish.SyncDirectory(writer.root); err != nil {
		return publicationRecord{}, err
	}
	n, writeErr := file.Write(data)
	if n != len(data) && writeErr == nil {
		writeErr = io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		return publicationRecord{}, stderrors.Join(writeErr, err)
	}
	written, err := publicationRecordOf(writer.root, journal.header.Stage, publicationRecordMaxBytes)
	if err != nil {
		return publicationRecord{}, stderrors.Join(writeErr, err)
	}
	if written.Entry != empty.Entry || written.Size != int64(n) || written.Digest != publicationDigest(data[:n]) {
		return publicationRecord{}, stderrors.Join(writeErr, changed(journal.header.Stage))
	}
	if err := journal.append(writer, publicationEvent{Record: &written}); err != nil {
		return publicationRecord{}, stderrors.Join(writeErr, err)
	}
	if writeErr != nil {
		return publicationRecord{}, writeErr
	}
	return written, nil
}

func (d *Directory) promotePublication(ctx context.Context, writer *publicationWriter, journal *publicationJournal, name string, before *publicationRecord, written publicationRecord) (bool, error) {
	if d.beforePublish != nil {
		if err := d.beforePublish(journal.header.Stage); err != nil {
			return false, err
		}
	}
	if err := writer.check(); err != nil {
		return false, err
	}
	if err := checkPublicationRecord(writer.records, journal.name, journal.receipt); err != nil {
		return false, err
	}
	if err := checkPublicationRecord(writer.root, journal.header.Stage, written); err != nil {
		return false, err
	}
	if before == nil {
		if _, err := writer.root.Lstat(name); !os.IsNotExist(err) {
			if err != nil {
				return false, err
			}
			return false, changed(name)
		}
	} else if err := checkPublicationRecord(writer.root, name, *before); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if before == nil {
		if err := writer.root.Link(journal.header.Stage, name); err != nil {
			if os.IsExist(err) {
				return false, changed(name)
			}
			return false, err
		}
	} else if err := writer.root.Rename(journal.header.Stage, name); err != nil {
		return false, err
	}
	return true, nil
}
