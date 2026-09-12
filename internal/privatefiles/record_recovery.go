package privatefiles

import (
	"context"
	stderrors "errors"
	"io"
	"os"
	"strings"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	publicationScanBatch = 128
	publicationScanLimit = 4096
)

// RecoverPublications removes unchanged staging records under an exclusive writer lock.
// It preserves unknown files and incomplete receipts. A missing metadata directory requires no recovery.
func (d *Directory) RecoverPublications(ctx context.Context) error {
	writer, err := d.acquirePublicationWriter(ctx, false)
	if err != nil {
		return err
	}
	if writer == nil {
		return nil
	}
	defer writer.close()
	return writer.recover(ctx)
}

func (w *publicationWriter) recover(ctx context.Context) error {
	names, err := w.journalNames(ctx)
	if err != nil {
		return err
	}
	var result error
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return stderrors.Join(result, err)
		}
		journal, err := w.readJournal(name)
		if err == nil {
			err = w.cleanJournal(ctx, journal)
		}
		result = stderrors.Join(result, err)
	}
	return result
}

func (w *publicationWriter) journalNames(ctx context.Context) ([]string, error) {
	file, err := w.records.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	var names []string
	total := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, readErr := file.ReadDir(publicationScanBatch)
		total += len(entries)
		if total > publicationScanLimit {
			return nil, &errors.ValidationError{Field: "private.publication_entries", Value: publicationScanLimit, Message: "publication metadata exceeds the entry limit"}
		}
		for _, entry := range entries {
			if entry.Name() == publicationLock {
				continue
			}
			if strings.HasSuffix(entry.Name(), publicationJournalSuffix) {
				names = append(names, entry.Name())
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	return names, w.check()
}

func (w *publicationWriter) cleanJournal(ctx context.Context, journal *publicationJournal) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := w.check(); err != nil {
		return err
	}
	if err := checkPublicationRecord(w.records, journal.name, journal.receipt); err != nil {
		return err
	}
	stage := journal.header.Stage
	info, err := w.root.Lstat(stage)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if info != nil {
		if journal.state == nil {
			return changed(stage)
		}
		if err := checkPublicationRecord(w.root, stage, *journal.state); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := w.check(); err != nil {
			return err
		}
		if err := checkPublicationRecord(w.records, journal.name, journal.receipt); err != nil {
			return err
		}
		if err := w.root.Remove(stage); err != nil {
			return err
		}
	}
	if err := filepublish.SyncDirectory(w.root); err != nil {
		return err
	}
	if err := w.check(); err != nil {
		return err
	}
	if err := checkPublicationRecord(w.records, journal.name, journal.receipt); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := w.records.Remove(journal.name); err != nil {
		return err
	}
	return filepublish.SyncDirectory(w.records)
}
