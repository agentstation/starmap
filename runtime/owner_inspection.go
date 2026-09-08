package runtime

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/pkg/errors"
)

// OwnerRecordStatus describes a stored binding, not an active lock or seed verification.
type OwnerRecordStatus string

const (
	// OwnerRecordMatches means the canonical record matches the configured owner and identity override.
	OwnerRecordMatches OwnerRecordStatus = "matches"
	// OwnerRecordAbsent means the selected directory or record does not exist.
	OwnerRecordAbsent OwnerRecordStatus = "absent"
	// OwnerRecordConflict means the record or directory is invalid or has another binding.
	OwnerRecordConflict OwnerRecordStatus = "conflict"
	// OwnerRecordUnavailable means the metadata or bounded record read failed.
	OwnerRecordUnavailable OwnerRecordStatus = "unavailable"
	// OwnerRecordUnverified means the native inspection adapter is unavailable.
	OwnerRecordUnverified OwnerRecordStatus = "unverified"
)

// InspectDirectoryOwnerRecord compares one bounded canonical record without writes or locks.
// It does not read the seed or verify active ownership, migration completion, or fleet fencing.
func InspectDirectoryOwnerRecord(ctx context.Context, directory string, owner DirectoryOwner, identity string) (OwnerRecordStatus, error) {
	if ctx == nil {
		return OwnerRecordUnavailable, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return OwnerRecordUnavailable, err
	}
	if !filepath.IsAbs(directory) {
		return OwnerRecordUnavailable, &errors.ValidationError{Field: "runtime.directory", Message: "must be absolute"}
	}
	if err := owner.Validate(); err != nil {
		return OwnerRecordUnavailable, err
	}
	if !ownerRecordInspectionSupported {
		return OwnerRecordUnverified, nil
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return ownerInspectionError(err), nil
	}
	if !info.IsDir() {
		return OwnerRecordConflict, nil
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return ownerInspectionError(err), nil
	}
	defer func() { _ = root.Close() }()
	actual, err := root.Stat(".")
	current, currentErr := os.Lstat(directory)
	if err != nil || currentErr != nil || !current.IsDir() || !os.SameFile(info, actual) || !os.SameFile(info, current) {
		return OwnerRecordUnavailable, nil
	}
	info, err = root.Lstat(ownerRecordName)
	if err != nil {
		return ownerInspectionError(err), nil
	}
	if !info.Mode().IsRegular() || info.Size() > ownerRecordMaxBytes {
		return OwnerRecordConflict, nil
	}
	file, err := openOwnerRecordForInspection(root)
	if err != nil {
		return ownerInspectionError(err), nil
	}
	defer func() { _ = file.Close() }()
	actual, err = file.Stat()
	current, currentErr = root.Lstat(ownerRecordName)
	if err != nil || currentErr != nil || !actual.Mode().IsRegular() || !current.Mode().IsRegular() || !os.SameFile(info, actual) || !os.SameFile(info, current) {
		return OwnerRecordUnavailable, nil
	}
	encoded, err := encodeOwnerRecord(owner, identity)
	if err != nil {
		return OwnerRecordUnavailable, err
	}
	raw, err := io.ReadAll(io.LimitReader(file, ownerRecordMaxBytes+1))
	if err != nil {
		return OwnerRecordUnavailable, nil
	}
	if err := ctx.Err(); err != nil {
		return OwnerRecordUnavailable, err
	}
	if !bytes.Equal(raw, encoded) {
		return OwnerRecordConflict, nil
	}
	return OwnerRecordMatches, nil
}

func ownerInspectionError(err error) OwnerRecordStatus {
	if os.IsNotExist(err) {
		return OwnerRecordAbsent
	}
	return OwnerRecordUnavailable
}
