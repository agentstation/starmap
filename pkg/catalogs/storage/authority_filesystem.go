package storage

import (
	"bytes"
	"context"
	"os"
	"strings"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// CurrentAuthorityHead reads the current pointer and its independent immutable permission record.
// This guarantee requires a local filesystem with the documented atomic publication semantics.
// The read does not load catalog data or repair missing metadata.
func (s *Filesystem) CurrentAuthorityHead(ctx context.Context) (catalogs.CatalogAuthorityHead, error) {
	if err := ctx.Err(); err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	unlock, err := s.lockFilesystemRead(ctx)
	if err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	defer unlock()
	if err := validateFilesystemLayout(s.root); err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	directory, err := privatefiles.ExistingDirectory(s.root)
	if err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	pointer, err := directory.ReadFile(currentFilename, catalogs.MaxCatalogAuthorityRecordBytes)
	if err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	id := strings.TrimSpace(string(pointer))
	if id == "" {
		return catalogs.CatalogAuthorityHead{}, currentNotFound()
	}
	data, err := s.readAuthorityRecord(id)
	if err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	if err := ctx.Err(); err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	return parseAuthorityRecord(data, id)
}

func (s *Filesystem) readAuthorityRecord(id string) ([]byte, error) {
	directory, err := privatefiles.ExistingDirectory(s.generationDir(id))
	if err != nil {
		return nil, err
	}
	data, err := directory.ReadFile(authorityFilename, catalogs.MaxCatalogAuthorityRecordBytes)
	if os.IsNotExist(err) {
		return nil, authorityNotFound(id)
	}
	return data, err
}

// ensureAuthorityRecord runs only during an explicit commit with the filesystem commit lock held.
func (s *Filesystem) ensureAuthorityRecord(ctx context.Context, generation catalogs.Generation) error {
	data, err := authorityRecordData(generation)
	if err != nil || data == nil {
		return err
	}
	id := generation.Manifest.GenerationID
	existing, err := s.readAuthorityRecord(id)
	if err == nil {
		if !bytes.Equal(existing, data) {
			return identityConflict(id)
		}
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}
	directory, err := privatefiles.ExistingDirectory(s.generationDir(id))
	if err != nil {
		return err
	}
	if err := directory.WriteFileIfAbsentContext(ctx, authorityFilename, data, ".authority-"); err != nil {
		return err
	}
	existing, err = s.readAuthorityRecord(id)
	if err != nil {
		return err
	}
	if !bytes.Equal(existing, data) {
		return identityConflict(id)
	}
	return nil
}

var _ AuthorityHeadReader = (*Filesystem)(nil)
