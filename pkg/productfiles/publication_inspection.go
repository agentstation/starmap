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
