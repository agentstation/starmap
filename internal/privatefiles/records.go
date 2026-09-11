package privatefiles

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"io/fs"
	"math"
	"os"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
)

// ReadFile reads a bounded private regular file from the bound directory.
// Missing files return the filesystem absence error.
func (d *Directory) ReadFile(name string, limit int64) ([]byte, error) {
	root, err := d.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	data, err := ReadFile(root, name, limit)
	if err != nil {
		return nil, err
	}
	if err := d.validateLocation(root); err != nil {
		return nil, err
	}
	return data, nil
}

// ReadFile reads one bounded private regular file under an open root.
// The caller owns directory access policy and closes the root.
func ReadFile(root *os.Root, name string, limit int64) ([]byte, error) {
	return readCheckedFile(root, name, limit, recordInfo)
}

func readCheckedFile(root *os.Root, name string, limit int64, inspect func(*os.Root, string) (fs.FileInfo, error)) ([]byte, error) {
	if err := childName(name); err != nil {
		return nil, err
	}
	if limit < 0 || limit == math.MaxInt64 {
		return nil, &errors.ValidationError{Field: "private.file_limit", Message: "requires a nonnegative bounded byte limit"}
	}
	info, err := inspect(root, name)
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, oversized(name, limit)
	}
	file, err := openRecord(root, name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !sameRecord(info, opened) {
		return nil, changed(name)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, oversized(name, limit)
	}
	after, err := inspect(root, name)
	if err != nil {
		return nil, err
	}
	if !sameRecord(info, after) || int64(len(data)) != info.Size() {
		return nil, changed(name)
	}
	return data, nil
}

// WriteFile publishes complete private bytes through an exclusively created temporary file.
// Existing destinations must remain private regular files. The caller serializes writers.
func (d *Directory) WriteFile(name string, data []byte, prefix string) error {
	return d.WriteFileContext(context.Background(), name, data, prefix)
}

// WriteFileContext checks cancellation before access and immediately before destination publication.
// The caller serializes writers. Cancellation before publication preserves the previous destination.
// PublicationError identifies a visible record with unconfirmed durability.
func (d *Directory) WriteFileContext(ctx context.Context, name string, data []byte, prefix string) error {
	return d.WriteFileContextWithSync(ctx, name, data, prefix, nil)
}

// WriteFileContextWithSync uses the supplied directory synchronizer after publication.
// A nil synchronizer selects the native filesystem operation. The caller serializes writers.
func (d *Directory) WriteFileContextWithSync(ctx context.Context, name string, data []byte, prefix string, syncDirectory func(*os.Root) error) error {
	return d.writeFileContext(ctx, name, data, prefix, true, syncDirectory)
}

// WriteFileIfAbsentContext publishes private bytes only when no destination exists.
// The write preserves a competing destination, including one created during publication.
func (d *Directory) WriteFileIfAbsentContext(ctx context.Context, name string, data []byte, prefix string) error {
	return d.writeFileContext(ctx, name, data, prefix, false, nil)
}

func (d *Directory) writeFileContext(ctx context.Context, name string, data []byte, prefix string, replace bool, syncDirectory func(*os.Root) error) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := childName(name); err != nil {
		return err
	}
	if err := childName(prefix); err != nil {
		return err
	}
	root, err := d.Open()
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	before, err := optionalRecordInfo(root, name)
	if err != nil {
		return err
	}
	if !replace && before != nil {
		return changed(name)
	}
	stage := prefix + rand.Text()
	file, err := CreateFile(root, stage)
	if err != nil {
		return err
	}
	created, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	owned := created
	ownedDigest := sha256.Sum256(nil)
	published := false
	defer func() {
		removeUnchangedRecord(root, stage, owned, ownedDigest)
		if err := file.Close(); err != nil && resultErr == nil {
			resultErr = err
			if published {
				resultErr = &errors.PublicationError{Resource: "private file", ID: name, Err: err}
			}
		}
	}()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	written, err := file.Stat()
	if err != nil {
		return err
	}
	owned = written
	ownedDigest = sha256.Sum256(data)
	if d.beforePublish != nil {
		if err := d.beforePublish(stage); err != nil {
			return err
		}
	}
	staged, err := recordInfo(root, stage)
	if err != nil {
		return err
	}
	if !sameRecord(written, staged) {
		return changed(stage)
	}
	stagedData, err := ReadFile(root, stage, written.Size())
	if err != nil {
		return err
	}
	if sha256.Sum256(stagedData) != ownedDigest {
		return changed(stage)
	}
	after, err := optionalRecordInfo(root, name)
	if err != nil {
		return err
	}
	if !sameRecord(before, after) {
		return changed(name)
	}
	if err := d.validateLocation(root); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if before == nil {
		if err := root.Link(stage, name); err != nil {
			if os.IsExist(err) {
				return changed(name)
			}
			return err
		}
	} else if err := root.Rename(stage, name); err != nil {
		return err
	}
	published = true
	if syncDirectory == nil {
		syncDirectory = filepublish.SyncDirectory
	}
	if err := syncDirectory(root); err != nil {
		return &errors.PublicationError{Resource: "private file", ID: name, Err: err}
	}
	return nil
}

// ReadDir lists the bound directory without following another directory at its original path.
func (d *Directory) ReadDir() ([]fs.DirEntry, error) {
	root, err := d.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, err
	}
	if err := d.validateLocation(root); err != nil {
		return nil, err
	}
	return entries, nil
}

func recordInfo(root *os.Root, name string) (fs.FileInfo, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, &errors.ValidationError{Field: "private.file", Message: "must be a regular file without a symbolic link"}
	}
	if err := ValidateMetadata(info, "private.file"); err != nil {
		return nil, err
	}
	if err := ValidateACL(root, name, info, "private.file"); err != nil {
		return nil, err
	}
	return info, nil
}

func optionalRecordInfo(root *os.Root, name string) (fs.FileInfo, error) {
	info, err := recordInfo(root, name)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return info, err
}

func sameRecord(a, b fs.FileInfo) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return os.SameFile(a, b) && a.Size() == b.Size() && a.Mode() == b.Mode() && a.ModTime().Equal(b.ModTime())
}

func (d *Directory) validateLocation(root *os.Root) error {
	if err := ValidateAncestors(d.path); err != nil {
		return err
	}
	current, err := os.Lstat(d.path)
	if os.IsNotExist(err) {
		return changed(d.path)
	}
	if err != nil {
		return err
	}
	opened, err := root.Stat(".")
	if err != nil {
		return err
	}
	if current.Mode()&os.ModeSymlink != 0 || !os.SameFile(d.identity, current) || !os.SameFile(d.identity, opened) {
		return changed(d.path)
	}
	if err := ValidateMetadata(opened, "private.directory"); err != nil {
		return err
	}
	return ValidateACL(root, ".", opened, "private.directory")
}

func oversized(name string, limit int64) error {
	return &errors.ValidationError{Field: "private.file_bytes", Value: limit, Message: "file exceeds the bounded read limit: " + name}
}

func removeUnchangedRecord(root *os.Root, name string, original fs.FileInfo, digest [sha256.Size]byte) {
	current, err := root.Lstat(name)
	if err != nil || !sameRecord(original, current) {
		return
	}
	data, err := ReadFile(root, name, original.Size())
	if err == nil && sha256.Sum256(data) == digest {
		_ = root.Remove(name)
	}
}
