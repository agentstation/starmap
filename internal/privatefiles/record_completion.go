package privatefiles

import (
	"bytes"
	"context"
	"os"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
)

// CompletePublication finishes only an exact record after a host-owned completion check.
// It refuses foreign journals, changed staging identities, and incomplete staging evidence.
func (d *Directory) CompletePublication(ctx context.Context, name string, previous, data []byte, prefix string, check func(context.Context) error) error {
	if ctx == nil || check == nil {
		return &errors.ValidationError{Field: "private.publication_completion", Message: "requires context and a host completion check"}
	}
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
	previous, data = bytes.Clone(previous), bytes.Clone(data)
	writer, err := d.acquirePublicationWriter(ctx, true)
	if err != nil {
		return err
	}
	defer writer.close()
	names, err := writer.journalNames(ctx)
	if err != nil {
		return err
	}
	journals := make([]*publicationJournal, 0, len(names))
	for _, pending := range names {
		journal, err := writer.readJournal(pending)
		if err != nil {
			return err
		}
		if journal.header.Destination != name || journal.header.Prefix != prefix {
			return changed(pending)
		}
		if err := writer.checkCompletionStage(journal, data); err != nil {
			return err
		}
		journals = append(journals, journal)
	}
	expected := publicationExpectation{present: previous != nil, size: int64(len(previous)), digest: publicationDigest(previous)}
	before, complete, err := readCompletionDestination(writer.root, name, previous, data)
	if err != nil {
		return err
	}
	if err := check(ctx); err != nil {
		return err
	}
	if err := writer.check(); err != nil {
		return err
	}
	if before != nil {
		if err := checkPublicationRecord(writer.root, name, *before); err != nil {
			return err
		}
	} else if _, err := writer.root.Lstat(name); !os.IsNotExist(err) {
		if err != nil {
			return err
		}
		return changed(name)
	}
	for _, journal := range journals {
		if err := writer.checkCompletionStage(journal, data); err != nil {
			return err
		}
		if err := writer.cleanJournal(ctx, journal); err != nil {
			return err
		}
	}
	if err := check(ctx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if complete {
		if err := writer.check(); err != nil {
			return err
		}
		if err := checkPublicationRecord(writer.root, name, *before); err != nil {
			return err
		}
		return filepublish.SyncDirectory(writer.root)
	}
	return d.publishWithWriter(ctx, writer, name, data, prefix, nil, &expected)
}

func (w *publicationWriter) checkCompletionStage(journal *publicationJournal, data []byte) error {
	if err := w.check(); err != nil {
		return err
	}
	if err := checkPublicationRecord(w.records, journal.name, journal.receipt); err != nil {
		return err
	}
	if journal.state != nil {
		state := *journal.state
		empty := state.Size == 0 && state.Digest == publicationDigest(nil)
		exact := state.Size == int64(len(data)) && state.Digest == publicationDigest(data)
		if !empty && !exact {
			return changed(journal.header.Stage)
		}
	}
	if _, err := w.root.Lstat(journal.header.Stage); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if journal.state == nil {
		return changed(journal.header.Stage)
	}
	return checkPublicationRecord(w.root, journal.header.Stage, *journal.state)
}
