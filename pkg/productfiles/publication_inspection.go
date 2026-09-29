package productfiles

import (
	"context"

	"github.com/agentstation/starmap/internal/privatefiles"
)

// PublicationDirectoryName identifies metadata owned by private-record publication.
// Callers must preserve this directory and use its owner APIs to inspect or recover it.
const PublicationDirectoryName = privatefiles.PublicationDirectoryName

// CheckNoPendingPublications refuses retained publication journals without changing files.
// It checks an existing writer lock but creates no lock or metadata directory.
// The caller must fence writers before copying or publishing inspected state.
func (d *Directory) CheckNoPendingPublications(ctx context.Context) error {
	if err := d.checkContext(ctx); err != nil {
		return err
	}
	return d.private.CheckNoPendingPublications(ctx)
}

// RetainedFile identifies bytes in a verified backup.
type RetainedFile = privatefiles.RetainedFile

// RetainedRecordReader reads bounded bytes from a verified backup.
type RetainedRecordReader = privatefiles.RetainedRecordReader

// InspectRetainedPublications selects matching journals and staging files for inactive retention.
// The owner callback approves destination names and prefixes within each parent directory.
// The caller preserves selected files and validates the remaining tree before publication.
// This check never promotes staged bytes or approves current permission.
func InspectRetainedPublications(ctx context.Context, files map[string]RetainedFile, read RetainedRecordReader, accepts func(parent, destination, prefix string) bool) ([]string, error) {
	return privatefiles.InspectRetainedPublications(ctx, files, read, accepts)
}
