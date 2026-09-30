package privatefiles

import (
	"context"
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

// InspectPublication checks only one exact publication without changing records.
// Returned stage names permit a host to inspect its original decision before completion.
func (d *Directory) InspectPublication(ctx context.Context, name string, previous, data []byte, prefix string) ([]string, error) {
	if ctx == nil {
		return nil, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := childName(name); err != nil {
		return nil, err
	}
	if err := childName(prefix); err != nil {
		return nil, err
	}
	if name == publicationDirectory {
		return nil, changed(name)
	}
	if len(data) > publicationRecordMaxBytes {
		return nil, oversized(name, publicationRecordMaxBytes)
	}
	writer, err := d.acquirePublicationWriter(ctx, false)
	if err != nil {
		return nil, err
	}
	if writer == nil {
		root, err := d.Open()
		if err != nil {
			return nil, err
		}
		defer func() { _ = root.Close() }()
		if err := checkCompletionDestination(root, name, previous, data); err != nil {
			return nil, err
		}
		return nil, ctx.Err()
	}
	defer writer.close()
	names, err := writer.journalNames(ctx)
	if err != nil {
		return nil, err
	}
	var stages []string
	for _, pending := range names {
		journal, err := writer.readJournal(pending)
		if err != nil {
			return nil, err
		}
		if journal.header.Destination != name || journal.header.Prefix != prefix {
			return nil, changed(pending)
		}
		if err := writer.checkCompletionStage(journal, data); err != nil {
			return nil, err
		}
		if _, err := writer.root.Lstat(journal.header.Stage); err == nil {
			stages = append(stages, journal.header.Stage)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if err := checkCompletionDestination(writer.root, name, previous, data); err != nil {
		return nil, err
	}
	if err := writer.check(); err != nil {
		return nil, err
	}
	return stages, ctx.Err()
}

func checkCompletionDestination(root *os.Root, name string, previous, data []byte) error {
	_, _, err := readCompletionDestination(root, name, previous, data)
	return err
}

func readCompletionDestination(root *os.Root, name string, previous, data []byte) (*publicationRecord, bool, error) {
	before, err := publicationRecordOf(root, name, publicationRecordMaxBytes)
	if os.IsNotExist(err) {
		if previous == nil {
			return nil, false, nil
		}
		return nil, false, changed(name)
	}
	if err != nil {
		return nil, false, err
	}
	exact := before.Size == int64(len(data)) && before.Digest == publicationDigest(data)
	prior := previous != nil && before.Size == int64(len(previous)) && before.Digest == publicationDigest(previous)
	if !exact && !prior {
		return nil, false, changed(name)
	}
	return &before, exact, nil
}
