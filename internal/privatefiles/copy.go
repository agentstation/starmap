package privatefiles

import (
	"context"
	stderrors "errors"
	"io"
	"math"

	"github.com/agentstation/starmap/pkg/errors"
)

// CopyFile streams one private regular file into a caller-owned writer.
// A failure can follow partial output. The caller must discard that output.
// The operation verifies native access, file identity, size, and directory identity.
func (d *Directory) CopyFile(ctx context.Context, name string, output io.Writer, limit int64) (copied int64, resultErr error) {
	if ctx == nil || output == nil {
		return 0, &errors.ValidationError{Field: "private.copy", Message: "requires context and a writer"}
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := childName(name); err != nil {
		return 0, err
	}
	if limit < 0 || limit == math.MaxInt64 {
		return 0, &errors.ValidationError{Field: "private.file_limit", Message: "requires a nonnegative bounded byte limit"}
	}
	root, err := d.Open()
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	info, err := recordInfo(root, name)
	if err != nil {
		return 0, err
	}
	if info.Size() > limit {
		return 0, oversized(name, limit)
	}
	file, err := openRecord(root, name)
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if !sameRecord(info, opened) {
		return 0, changed(name)
	}
	// Do not read empty ownership files. Windows can hold mandatory byte-range locks on them.
	if info.Size() != 0 {
		copied, err = io.CopyN(output, copyContextReader{ctx: ctx, source: file}, info.Size())
		if err != nil {
			return copied, err
		}
	}
	if err := ctx.Err(); err != nil {
		return copied, err
	}
	after, err := recordInfo(root, name)
	if err != nil {
		return copied, err
	}
	if !sameRecord(info, after) {
		return copied, changed(name)
	}
	if err := d.validateLocation(root); err != nil {
		return copied, err
	}
	return copied, nil
}

type copyContextReader struct {
	ctx    context.Context
	source io.Reader
}

func (r copyContextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(buffer)
}
