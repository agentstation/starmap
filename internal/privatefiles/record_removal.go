package privatefiles

import (
	"context"
	"os"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
)

// CompareAndRemoveFileContext removes a private record whose bytes match previous.
// It shares the PublishFileContext writer lock and verifies identity and access before removal.
// A missing file permits an idempotent retry. A nil previous value is invalid.
// The result reports visible removal even when directory synchronization fails.
func (d *Directory) CompareAndRemoveFileContext(ctx context.Context, name string, previous []byte) (bool, error) {
	return d.compareAndRemoveFileContext(ctx, name, previous, filepublish.SyncDirectory)
}

func (d *Directory) compareAndRemoveFileContext(ctx context.Context, name string, previous []byte, syncDirectory func(*os.Root) error) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := childName(name); err != nil {
		return false, err
	}
	if name == publicationDirectory || previous == nil {
		return false, changed(name)
	}
	if len(previous) > publicationRecordMaxBytes {
		return false, oversized(name, publicationRecordMaxBytes)
	}
	writer, err := d.acquirePublicationWriter(ctx, true)
	if err != nil {
		return false, err
	}
	defer writer.close()
	if err := writer.recover(ctx); err != nil {
		return false, err
	}
	record, err := publicationRecordOf(writer.root, name, publicationRecordMaxBytes)
	if os.IsNotExist(err) {
		if err := writer.check(); err != nil {
			return false, err
		}
		if err := ctx.Err(); err != nil {
			return false, err
		}
		return false, syncDirectory(writer.root)
	}
	if err != nil {
		return false, err
	}
	if record.Size != int64(len(previous)) || record.Digest != publicationDigest(previous) {
		return false, changed(name)
	}
	if err := writer.check(); err != nil {
		return false, err
	}
	if err := checkPublicationRecord(writer.root, name, record); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := writer.root.Remove(name); err != nil {
		return false, err
	}
	if err := syncDirectory(writer.root); err != nil {
		return true, &errors.PublicationError{Resource: "private file removal", ID: name, Err: err}
	}
	return true, nil
}
