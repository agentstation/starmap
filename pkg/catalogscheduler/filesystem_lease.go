package catalogscheduler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

// FilesystemLease coordinates scheduler processes that share one filesystem root.
type FilesystemLease struct {
	root string
}

// NewFilesystemLease creates a shared-filesystem lease adapter.
func NewFilesystemLease(root string) (*FilesystemLease, error) {
	if strings.TrimSpace(root) == "" {
		return nil, &errors.ValidationError{Field: "catalog_scheduler.filesystem_lease_root", Message: validationRequiredMessage}
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, errors.WrapIO("resolve", root, err)
	}
	return &FilesystemLease{root: absolute}, nil
}

// Acquire takes a non-blocking OS-backed lock before provider work.
func (l *FilesystemLease) Acquire(ctx context.Context, request LeaseRequest) (LeaseGuard, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(l.root, constants.DirPermissions); err != nil {
		return nil, errors.WrapIO("create", l.root, err)
	}
	sum := sha256.Sum256([]byte(request.Key))
	path := filepath.Join(l.root, hex.EncodeToString(sum[:])+".lock")
	lock := flock.New(path)
	locked, err := lock.TryLock()
	if err != nil {
		return nil, errors.WrapIO("lock", path, err)
	}
	if !locked {
		return nil, &errors.ConflictError{
			Resource: "catalog scheduler filesystem lease", Expected: request.Owner,
			Message: "publisher lease is already held",
		}
	}
	return &filesystemLeaseGuard{lock: lock, path: path}, nil
}

type filesystemLeaseGuard struct {
	once sync.Once
	lock *flock.Flock
	path string
	err  error
}

func (g *filesystemLeaseGuard) Release(ctx context.Context) error {
	g.once.Do(func() {
		if err := ctx.Err(); err != nil {
			g.err = err
			return
		}
		if err := g.lock.Unlock(); err != nil {
			g.err = errors.WrapIO("unlock", g.path, err)
		}
	})
	return g.err
}
