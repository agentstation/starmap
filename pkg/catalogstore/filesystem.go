package catalogstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	currentFilename  = "current"
	manifestFilename = "manifest.json"
	payloadFilename  = "catalog.json"
)

// Filesystem stores immutable generation directories and an atomically replaced
// current pointer beneath one root directory.
type Filesystem struct {
	mu                     sync.RWMutex
	root                   string
	commitLock             *flock.Flock
	beforeCurrentPromotion func() error
}

// NewFilesystem creates a filesystem catalog store rooted at path.
func NewFilesystem(path string) (*Filesystem, error) {
	if strings.TrimSpace(path) == "" {
		return nil, &errors.ConfigError{Component: catalogStoreComponent, Message: "filesystem path is required"}
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.WrapIO("resolve", path, err)
	}
	return &Filesystem{
		root:       root,
		commitLock: flock.New(filepath.Join(root, ".commit.lock")),
	}, nil
}

// Root returns the configured filesystem root without creating it.
func (s *Filesystem) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

// Current returns the currently active generation.
func (s *Filesystem) Current(ctx context.Context) (Generation, error) {
	if err := ctx.Err(); err != nil {
		return Generation{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, err := s.currentID()
	if err != nil {
		return Generation{}, err
	}
	return s.readGeneration(ctx, id)
}

// Get returns an immutable generation by ID.
func (s *Filesystem) Get(ctx context.Context, id string) (Generation, error) {
	if err := ctx.Err(); err != nil {
		return Generation{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readGeneration(ctx, id)
}

// Commit writes an immutable generation before atomically replacing current.
func (s *Filesystem) Commit(ctx context.Context, generation Generation, expectedGenerationID string) error {
	if err := validateCandidate(ctx, generation); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.root, "generations"), constants.DirPermissions); err != nil {
		return errors.WrapIO("create", s.root, err)
	}
	candidate := generation.Copy()
	id := candidate.Manifest.GenerationID
	locked, err := s.commitLock.TryLockContext(ctx, constants.CatalogStoreLockRetryDelay)
	if err != nil {
		return errors.WrapIO("lock", s.root, err)
	}
	if !locked {
		if err := ctx.Err(); err != nil {
			return err
		}
		return &errors.ConflictError{Resource: "catalog filesystem commit lock", Message: "lock was not acquired"}
	}
	defer func() { _ = s.commitLock.Unlock() }()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	existing, existingErr := s.readGeneration(ctx, id)
	if existingErr == nil {
		if !sameGeneration(existing, candidate) {
			return identityConflict(id)
		}
		currentID, err := s.currentIDOrEmpty()
		if err != nil {
			return err
		}
		if currentID == id {
			return nil
		}
	} else if !errors.IsNotFound(existingErr) {
		return existingErr
	}

	currentID, err := s.currentIDOrEmpty()
	if err != nil {
		return err
	}
	if currentID != expectedGenerationID {
		return casConflict(expectedGenerationID, currentID)
	}

	if existingErr != nil {
		if err := s.writeGeneration(candidate); err != nil {
			return err
		}
	}
	return s.writeCurrent(id)
}

func (s *Filesystem) currentID() (string, error) {
	id, err := s.currentIDOrEmpty()
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", currentNotFound()
	}
	return id, nil
}

func (s *Filesystem) currentIDOrEmpty() (string, error) {
	data, err := os.ReadFile(filepath.Join(s.root, currentFilename))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", errors.WrapIO("read", filepath.Join(s.root, currentFilename), err)
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", &errors.ValidationError{Field: currentFilename, Message: "generation ID is empty"}
	}
	return id, nil
}

func (s *Filesystem) readGeneration(ctx context.Context, id string) (Generation, error) {
	if err := ctx.Err(); err != nil {
		return Generation{}, err
	}
	dir := s.generationDir(id)
	// dir is derived from a SHA-256 digest, not a caller-controlled path.
	manifestData, err := os.ReadFile(filepath.Join(dir, manifestFilename)) //nolint:gosec
	if os.IsNotExist(err) {
		return Generation{}, generationNotFound(id)
	}
	if err != nil {
		return Generation{}, errors.WrapIO("read", filepath.Join(dir, manifestFilename), err)
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestData)
	if err != nil {
		return Generation{}, err
	}
	if manifest.GenerationID != id {
		return Generation{}, &errors.ValidationError{
			Field:   "generation_id",
			Value:   manifest.GenerationID,
			Message: "does not match requested generation",
		}
	}
	// dir is derived from a SHA-256 digest, not a caller-controlled path.
	payload, err := os.ReadFile(filepath.Join(dir, payloadFilename)) //nolint:gosec
	if err != nil {
		return Generation{}, errors.WrapIO("read", filepath.Join(dir, payloadFilename), err)
	}
	generation := Generation{Manifest: manifest, Payload: payload}
	if err := generation.Validate(); err != nil {
		return Generation{}, err
	}
	return generation, nil
}

func (s *Filesystem) writeGeneration(generation Generation) error {
	manifest, err := marshalManifest(generation.Manifest)
	if err != nil {
		return err
	}
	base := filepath.Join(s.root, "generations")
	temp, err := os.MkdirTemp(base, ".candidate-")
	if err != nil {
		return errors.WrapIO("create", base, err)
	}
	defer func() { _ = os.RemoveAll(temp) }()
	if err := os.Chmod(temp, constants.DirPermissions); err != nil {
		return errors.WrapIO("chmod", temp, err)
	}
	if err := writeSyncedFile(filepath.Join(temp, manifestFilename), manifest); err != nil {
		return errors.WrapIO("write", manifestFilename, err)
	}
	if err := writeSyncedFile(filepath.Join(temp, payloadFilename), generation.Payload); err != nil {
		return errors.WrapIO("write", payloadFilename, err)
	}
	if err := syncDirectory(temp); err != nil {
		return errors.WrapIO("sync", temp, err)
	}
	if err := os.Rename(temp, s.generationDir(generation.Manifest.GenerationID)); err != nil {
		return errors.WrapIO("promote", generation.Manifest.GenerationID, err)
	}
	if err := syncDirectory(base); err != nil {
		return errors.WrapIO("sync", base, err)
	}
	return nil
}

func (s *Filesystem) writeCurrent(id string) error {
	temp, err := os.CreateTemp(s.root, ".current-")
	if err != nil {
		return errors.WrapIO("create", currentFilename, err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(constants.FilePermissions); err != nil {
		_ = temp.Close()
		return errors.WrapIO("chmod", tempPath, err)
	}
	if _, err := temp.WriteString(id + "\n"); err != nil {
		_ = temp.Close()
		return errors.WrapIO("write", tempPath, err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return errors.WrapIO("sync", tempPath, err)
	}
	if err := temp.Close(); err != nil {
		return errors.WrapIO("close", tempPath, err)
	}
	if s.beforeCurrentPromotion != nil {
		if err := s.beforeCurrentPromotion(); err != nil {
			return errors.WrapIO("promote", currentFilename, err)
		}
	}
	if err := os.Rename(tempPath, filepath.Join(s.root, currentFilename)); err != nil {
		return errors.WrapIO("promote", currentFilename, err)
	}
	if err := syncDirectory(s.root); err != nil {
		return errors.WrapIO("sync", s.root, err)
	}
	return nil
}

func (s *Filesystem) generationDir(id string) string {
	digest := sha256.Sum256([]byte(id))
	return filepath.Join(s.root, "generations", hex.EncodeToString(digest[:]))
}

func writeSyncedFile(path string, data []byte) error {
	// path is constructed beneath a store-owned temporary generation directory.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, constants.FilePermissions) //nolint:gosec
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func syncDirectory(path string) error {
	directory, err := os.Open(path) //nolint:gosec // path is store-owned and digest-confined.
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	return directory.Sync()
}

var _ Store = (*Filesystem)(nil)
