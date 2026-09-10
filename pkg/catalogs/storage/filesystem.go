package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

const (
	currentFilename  = "current"
	manifestFilename = "manifest.json"
	payloadFilename  = "catalog.json"
)

// Filesystem stores immutable generation directories and an atomically replaced
// current pointer beneath one root directory.
type Filesystem struct {
	mu                        sync.RWMutex
	root                      string
	commitLock                *flock.Flock
	beforeCurrentPromotion    func() error
	beforeGenerationPromotion func(string) error
}

// NewFilesystem configures a filesystem catalog store without accessing or creating its root.
// Operations require private access to existing store entries and never change their permissions.
func NewFilesystem(path string) (*Filesystem, error) {
	if strings.TrimSpace(path) == "" {
		return nil, &errors.ConfigError{Component: "catalog store", Message: "filesystem path is required"}
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.WrapIO("resolve", path, err)
	}
	return &Filesystem{
		root:       root,
		commitLock: flock.New(filepath.Join(root, ".commit.lock"), flock.SetFlag(os.O_RDWR)),
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
func (s *Filesystem) Current(ctx context.Context) (catalogs.Generation, error) {
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := validateFilesystemLayout(s.root); err != nil {
		return catalogs.Generation{}, err
	}
	id, err := s.currentID()
	if err != nil {
		return catalogs.Generation{}, err
	}
	return s.readGeneration(ctx, id)
}

// Get returns an immutable generation by ID.
func (s *Filesystem) Get(ctx context.Context, id string) (catalogs.Generation, error) {
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := validateFilesystemLayout(s.root); err != nil {
		return catalogs.Generation{}, err
	}
	return s.readGeneration(ctx, id)
}

// Commit writes an immutable generation before atomically replacing current.
func (s *Filesystem) Commit(ctx context.Context, generation catalogs.Generation, expectedGenerationID string) error {
	if err := validateCandidate(ctx, generation); err != nil {
		return err
	}
	if err := validateFilesystemLayout(s.root); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := privatefiles.CreateDirectory(filepath.Join(s.root, "generations")); err != nil {
		return errors.WrapIO("create", s.root, err)
	}
	if err := validateFilesystemLayout(s.root); err != nil {
		return err
	}
	if err := s.prepareCommitLock(); err != nil {
		return err
	}
	directory, err := privatefiles.ExistingDirectory(s.root)
	if err != nil {
		return err
	}
	candidate := generation.Copy()
	id := candidate.Manifest.GenerationID
	locked, err := s.commitLock.TryLockContext(ctx, resourcepolicy.StoreLockRetryDelay)
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

	if err := s.validateCommitLock(); err != nil {
		return err
	}
	if err := validateFilesystemLayout(s.root); err != nil {
		return err
	}
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
			return s.ensureAuthorityRecord(ctx, candidate)
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
		if err := s.writeGeneration(ctx, candidate); err != nil {
			return err
		}
	}
	if err := s.ensureAuthorityRecord(ctx, candidate); err != nil {
		return err
	}
	return s.writeCurrent(ctx, id, directory)
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
	currentPath := filepath.Join(s.root, currentFilename)
	if err := validateFilesystemEntry(currentPath, false); err != nil {
		return "", err
	}
	data, err := readPrivateStoreFile(currentPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", errors.WrapIO("read", currentPath, err)
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", &errors.ValidationError{Field: "current", Message: "generation ID is empty"}
	}
	return id, nil
}

func (s *Filesystem) readGeneration(ctx context.Context, id string) (catalogs.Generation, error) {
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, err
	}
	dir := s.generationDir(id)
	if err := validateFilesystemEntry(dir, true); err != nil {
		if os.IsNotExist(err) {
			return catalogs.Generation{}, generationNotFound(id)
		}
		return catalogs.Generation{}, err
	}
	manifestPath := filepath.Join(dir, manifestFilename)
	if err := validateFilesystemEntry(manifestPath, false); err != nil {
		if os.IsNotExist(err) {
			return catalogs.Generation{}, generationNotFound(id)
		}
		return catalogs.Generation{}, err
	}
	manifestData, err := readPrivateStoreFile(manifestPath)
	if os.IsNotExist(err) {
		return catalogs.Generation{}, generationNotFound(id)
	}
	if err != nil {
		return catalogs.Generation{}, errors.WrapIO("read", manifestPath, err)
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(manifestData)
	if err != nil {
		return catalogs.Generation{}, err
	}
	if manifest.GenerationID != id {
		return catalogs.Generation{}, &errors.ValidationError{
			Field:   "generation_id",
			Value:   manifest.GenerationID,
			Message: "does not match requested generation",
		}
	}
	payloadPath := filepath.Join(dir, payloadFilename)
	if err := validateFilesystemEntry(payloadPath, false); err != nil {
		return catalogs.Generation{}, err
	}
	payload, err := readPrivateStoreFile(payloadPath)
	if err != nil {
		return catalogs.Generation{}, errors.WrapIO("read", payloadPath, err)
	}
	generation := catalogs.Generation{Manifest: manifest, Payload: payload}
	if err := generation.Validate(); err != nil {
		return catalogs.Generation{}, err
	}
	return generation, nil
}

func (s *Filesystem) writeCurrent(ctx context.Context, id string, directory *privatefiles.Directory) error {
	if s.beforeCurrentPromotion != nil {
		if err := s.beforeCurrentPromotion(); err != nil {
			return errors.WrapIO("promote", currentFilename, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateFilesystemLayout(s.root); err != nil {
		return err
	}
	return directory.WriteFileContext(ctx, currentFilename, []byte(id+"\n"), ".current-")
}

func (s *Filesystem) generationDir(id string) string {
	digest := sha256.Sum256([]byte(id))
	return filepath.Join(s.root, "generations", hex.EncodeToString(digest[:]))
}

func validateFilesystemLayout(root string) error {
	if err := privatefiles.ValidateAncestors(root); err != nil {
		return err
	}
	for _, entry := range []struct {
		path      string
		directory bool
	}{
		{path: root, directory: true},
		{path: filepath.Join(root, "generations"), directory: true},
		{path: filepath.Join(root, ".commit.lock")},
		{path: filepath.Join(root, currentFilename)},
	} {
		if err := validateFilesystemEntry(entry.path, entry.directory); err != nil {
			return err
		}
	}
	return nil
}

func validateFilesystemEntry(path string, directory bool) error {
	if err := policy.Require("catalog-store", policy.OwnerOnly); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return errors.WrapIO("inspect", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return &errors.ValidationError{
			Field:   "catalog_store.path",
			Value:   path,
			Message: "symbolic links are not allowed in the machine store",
		}
	}
	if directory && !info.IsDir() {
		return &errors.ValidationError{
			Field:   "catalog_store.path",
			Value:   path,
			Message: "must be a directory",
		}
	}
	if !directory && !info.Mode().IsRegular() {
		return &errors.ValidationError{
			Field:   "catalog_store.path",
			Value:   path,
			Message: "must be a regular file",
		}
	}
	if err := privatefiles.ValidateMetadata(info, "catalog_store.path"); err != nil {
		return err
	}
	parent, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { _ = parent.Close() }()
	return privatefiles.ValidateACL(parent, filepath.Base(path), info, "catalog_store.path")
}

var _ Store = (*Filesystem)(nil)
