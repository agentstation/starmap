package workspace

import (
	"context"
	"os"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// Read coordinates one complete workspace read with Starmap writers.
// The callback must only read and must finish all filesystem access before returning.
// Callers must discard its results if Read returns an error.
// Read creates no files. It does not exclude edits by external tools.
func Read(ctx context.Context, path string, read func(InputExpectation) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if read == nil {
		return &errors.ValidationError{Field: "workspace_read.callback", Message: "is required"}
	}
	if strings.TrimSpace(path) == "" {
		if err := read(InputExpectation{}); err != nil {
			return err
		}
		return ctx.Err()
	}
	target, err := resolveTarget(path)
	if err != nil {
		return err
	}
	lease, err := acquireReaderLock(target)
	if err != nil {
		return err
	}
	defer lease.close()
	if err := pendingReplacement(target); err != nil {
		return err
	}
	before, err := inspectReadTarget(target)
	if err != nil {
		return err
	}
	if err := ValidateHumanLayout(target, ""); err != nil {
		return err
	}
	readErr := read(InputExpectation{Path: target, Exists: before != nil})
	if err := lease.validate(); err != nil {
		return err
	}
	if err := pendingReplacement(target); err != nil {
		return err
	}
	after, err := inspectReadTarget(target)
	if err != nil {
		return err
	}
	if (before == nil) != (after == nil) || (before != nil && !os.SameFile(before, after)) {
		return readConflict(target, "workspace changed during the read")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return readErr
}

func inspectReadTarget(target string) (os.FileInfo, error) {
	info, err := readTargetInfo(target)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.WrapIO("inspect", target, err)
	}
	return info, nil
}

func readConflict(target, message string) error {
	return &errors.ConflictError{
		Resource: "catalog workspace read", Actual: target, Message: message,
	}
}
