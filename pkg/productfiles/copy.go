package productfiles

import (
	"context"
	"io"
)

// CopyFile streams one private regular child file into a caller-owned writer.
// It checks native access and identity before and after the bounded copy.
// A failure can follow partial output. The caller must discard that output.
// Writers must remain fenced. Content hashes remain the caller's responsibility.
func (d *Directory) CopyFile(ctx context.Context, name string, output io.Writer, maxBytes int64) (int64, error) {
	if err := d.checkContext(ctx); err != nil {
		return 0, err
	}
	return d.private.CopyFile(ctx, name, output, maxBytes)
}
