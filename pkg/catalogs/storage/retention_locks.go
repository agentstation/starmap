package storage

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
)

const generationReadLock = ".read.lock"

// AcquireGeneration returns independent bytes while a native shared lock retains the generation.
// Each acquisition owns its lock. Release remains available after context cancellation.
func (s *Filesystem) AcquireGeneration(ctx context.Context, id string) (catalogs.Generation, func() error, error) {
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := s.lockRetention(ctx)
	if err != nil {
		return catalogs.Generation{}, nil, err
	}
	defer func() { _ = unlock() }()
	lock, held, err := s.lockGeneration(id, true)
	if err != nil {
		return catalogs.Generation{}, nil, err
	}
	if !held {
		return catalogs.Generation{}, nil, retentionConflict(id, "generation is retiring")
	}
	generation, err := s.readGeneration(ctx, id)
	if err != nil {
		_ = lock.Close()
		return catalogs.Generation{}, nil, err
	}
	return generation, sync.OnceValue(lock.Close), nil
}

// lockRetention requires the instance mutex and serializes with all filesystem publishers.
func (s *Filesystem) lockRetention(ctx context.Context) (func() error, error) {
	if err := validateFilesystemLayout(s.root); err != nil {
		return nil, err
	}
	if err := s.prepareCommitLock(); err != nil {
		return nil, err
	}
	locked, err := s.commitLock.TryLockContext(ctx, resourcepolicy.StoreLockRetryDelay)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, retentionConflict(s.root, "publication lock is unavailable")
	}
	if err := s.validateCommitLock(); err != nil {
		_ = s.commitLock.Unlock()
		return nil, err
	}
	return s.commitLock.Unlock, nil
}

// lockGeneration runs under the publication lock, so the lease filename cannot retire concurrently.
func (s *Filesystem) lockGeneration(id string, shared bool) (*flock.Flock, bool, error) {
	directory, err := privatefiles.ExistingDirectory(s.generationDir(id))
	if os.IsNotExist(err) {
		return nil, false, generationNotFound(id)
	}
	if err != nil {
		return nil, false, err
	}
	root, err := directory.Open()
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = root.Close() }()
	if !shared {
		if _, err := root.Lstat(generationReadLock); os.IsNotExist(err) {
			return nil, true, nil
		} else if err != nil {
			return nil, false, err
		}
	}
	file, err := privatefiles.CreateFile(root, generationReadLock)
	if err != nil && !os.IsExist(err) {
		return nil, false, err
	}
	if err == nil {
		if err := file.Close(); err != nil {
			return nil, false, err
		}
	}
	path := filepath.Join(s.generationDir(id), generationReadLock)
	if err := validateFilesystemEntry(path, false); err != nil {
		return nil, false, err
	}
	before, err := root.Lstat(generationReadLock)
	if err != nil {
		return nil, false, err
	}
	if before.Size() != 0 {
		return nil, false, retentionConflict(id, "lease file contains unrecognized data")
	}
	lock := flock.New(path, flock.SetFlag(os.O_RDWR))
	var held bool
	if shared {
		held, err = lock.TryRLock()
	} else {
		held, err = lock.TryLock()
	}
	if err != nil || !held {
		_ = lock.Close()
		return nil, held, err
	}
	opened, err := lock.Stat()
	if err == nil {
		var after os.FileInfo
		after, err = root.Lstat(generationReadLock)
		if err == nil && (!os.SameFile(before, opened) || !os.SameFile(opened, after)) {
			err = retentionConflict(id, "lease file changed during access")
		}
	}
	if err == nil {
		err = validateFilesystemEntry(path, false)
	}
	if err != nil {
		_ = lock.Close()
		return nil, false, err
	}
	return lock, true, nil
}

// lockFilesystemRead uses a separate shared handle for each ordinary read.
// An absent lock preserves read-only access to older manually populated stores.
func (s *Filesystem) lockFilesystemRead(ctx context.Context) (func(), error) {
	path := filepath.Join(s.root, ".commit.lock")
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return func() {}, nil
	} else if err != nil {
		return nil, err
	}
	if err := validateFilesystemEntry(path, false); err != nil {
		return nil, err
	}
	lock := flock.New(path, flock.SetFlag(os.O_RDONLY))
	held, err := lock.TryRLockContext(ctx, resourcepolicy.StoreLockRetryDelay)
	if err != nil || !held {
		_ = lock.Close()
		if err == nil {
			err = retentionConflict(path, "publication lock is unavailable")
		}
		return nil, err
	}
	opened, err := lock.Stat()
	if err == nil {
		var current os.FileInfo
		current, err = os.Lstat(path)
		if err == nil && !os.SameFile(opened, current) {
			err = retentionConflict(path, "publication lock changed during access")
		}
	}
	if err != nil {
		_ = lock.Close()
		return nil, err
	}
	return func() { _ = lock.Close() }, nil
}
