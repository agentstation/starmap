// Package productfiles supplies native private-file operations for Starmap hosts.
// The host owns path selection, record schemas, and transactions across directories.
package productfiles

import (
	"context"
	"math"
	"os"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
)

// Directory binds a private directory to its filesystem identity.
// Operations check native ownership and access without changing existing permissions.
// The zero value is invalid. No directory handle remains open between operations.
type Directory struct {
	private *privatefiles.Directory
}

// NewDirectory creates missing private directories or checks an existing directory.
// The path must be absolute. Existing ancestors must pass the native access policy.
func NewDirectory(path string) (*Directory, error) {
	directory, err := privatefiles.NewDirectory(path)
	return bind(directory, err)
}

// ExistingDirectory checks an existing private directory without creating paths.
func ExistingDirectory(path string) (*Directory, error) {
	directory, err := privatefiles.ExistingDirectory(path)
	return bind(directory, err)
}

// Open returns a confined root after checking the original directory identity and access.
// The caller must close the root. Raw root operations retain their standard library contract.
func (d *Directory) Open() (*os.Root, error) {
	if err := d.check(); err != nil {
		return nil, err
	}
	return d.private.Open()
}

// Child checks or privately creates one direct child directory.
func (d *Directory) Child(name string) (*Directory, error) {
	if err := d.check(); err != nil {
		return nil, err
	}
	directory, err := d.private.Child(name)
	return bind(directory, err)
}

// ExistingChild checks one existing direct child without creating paths.
func (d *Directory) ExistingChild(name string) (*Directory, error) {
	if err := d.check(); err != nil {
		return nil, err
	}
	directory, err := d.private.ExistingChild(name)
	return bind(directory, err)
}

// ReadFile reads one private regular file within a caller-selected byte limit.
// It refuses links, changed identities, and invalid native access.
func (d *Directory) ReadFile(name string, maxBytes int64) ([]byte, error) {
	if err := d.check(); err != nil {
		return nil, err
	}
	if maxBytes < 0 || maxBytes == math.MaxInt64 {
		return nil, &errors.ValidationError{Field: "private.max_bytes", Message: "must be a nonnegative bounded byte limit"}
	}
	return d.private.ReadFile(name, maxBytes)
}

// CompareAndPublish atomically publishes data if the current bytes equal previous.
// A nil previous value requires absence. A non-nil empty value requires an empty file.
// Publication uses a checked writer lock and durable recovery receipts.
// A PublicationError reports visible bytes whose durability remains unconfirmed.
func (d *Directory) CompareAndPublish(ctx context.Context, name string, previous, data []byte) error {
	if err := d.checkContext(ctx); err != nil {
		return err
	}
	return d.private.CompareAndPublishFileContext(ctx, name, previous, data, ".product-stage-")
}

// CompareAndRemove removes a private record only when its bytes equal previous.
// Missing records permit an idempotent retry. The previous value must not be nil.
// A true result reports visible removal, including an unconfirmed durability error.
func (d *Directory) CompareAndRemove(ctx context.Context, name string, previous []byte) (bool, error) {
	if err := d.checkContext(ctx); err != nil {
		return false, err
	}
	return d.private.CompareAndRemoveFileContext(ctx, name, previous)
}

// RecoverPublications recovers abandoned private-record writes under the checked writer lock.
// It preserves unknown entries and entries whose identity, bytes, or access changed.
// This operation does not recover a host transaction across directories.
func (d *Directory) RecoverPublications(ctx context.Context) error {
	if err := d.checkContext(ctx); err != nil {
		return err
	}
	return d.private.RecoverPublications(ctx)
}

// PublishDirectory moves one direct child between open directories without replacing a destination.
// The roots must refer to the same filesystem. The move uses their open handles.
// Callers must sync the source contents before publication and both parents afterward.
// Success reports namespace publication. It does not establish durability.
func PublishDirectory(sourceRoot *os.Root, source string, targetRoot *os.Root, target string) error {
	return filepublish.DirectoryBetweenRootsNoReplace(sourceRoot, source, targetRoot, target)
}

// SyncDirectory flushes an open directory through the native filesystem API.
// Filesystem and hardware qualification remain the host's responsibility.
func SyncDirectory(root *os.Root) error {
	return filepublish.SyncDirectory(root)
}

func bind(directory *privatefiles.Directory, err error) (*Directory, error) {
	if err != nil {
		return nil, err
	}
	return &Directory{private: directory}, nil
}

func (d *Directory) check() error {
	if d == nil || d.private == nil {
		return &errors.ValidationError{Field: "private.directory", Message: "requires a bound directory"}
	}
	return nil
}

func (d *Directory) checkContext(ctx context.Context) error {
	if err := d.check(); err != nil {
		return err
	}
	if ctx == nil {
		return &errors.ValidationError{Field: "context", Message: "is required"}
	}
	return ctx.Err()
}
