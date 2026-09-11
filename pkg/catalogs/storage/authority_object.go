package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"unicode/utf8"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// CurrentObjectReader guarantees that GetCurrent observes a version current during the call.
// The guarantee includes other writers' completed publications and prohibits stale replica or cache reads.
// Ordinary ObjectBackend reads do not imply this capability.
type CurrentObjectReader interface {
	GetCurrent(context.Context, string) (ObjectValue, error)
}

// GetCurrent returns the object selected under the same lock as conditional writes.
func (b *MemoryObjectBackend) GetCurrent(ctx context.Context, key string) (ObjectValue, error) {
	return b.Get(ctx, key)
}

// CurrentAuthorityHead observes the current pointer through a qualified backend and reads its immutable permission record.
// An object backend without current-read capability cannot supply fresh authority observations.
func (s *Object) CurrentAuthorityHead(ctx context.Context) (catalogs.CatalogAuthorityHead, error) {
	if err := ctx.Err(); err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	reader, ok := s.backend.(CurrentObjectReader)
	if !ok {
		return catalogs.CatalogAuthorityHead{}, &errors.ConfigError{Component: "catalog authority store", Message: "object backend does not guarantee current reads"}
	}
	value, err := reader.GetCurrent(ctx, s.currentKey())
	if err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	if len(value.Data) > catalogs.MaxCatalogAuthorityRecordBytes || !utf8.Valid(value.Data) {
		return catalogs.CatalogAuthorityHead{}, &errors.ValidationError{Field: "current", Message: "must contain bounded UTF-8 authority pointer data"}
	}
	id, err := currentAuthorityGenerationID(value)
	if err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	record, err := s.backend.Get(ctx, s.generationKey(id, authorityFilename))
	if errors.IsNotFound(err) {
		return catalogs.CatalogAuthorityHead{}, authorityNotFound(id)
	}
	if err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	if err := ctx.Err(); err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	return parseAuthorityRecord(record.Data, id)
}

func currentAuthorityGenerationID(value ObjectValue) (string, error) {
	invalid := func() (string, error) {
		return "", &errors.ValidationError{Field: "current", Message: "must contain one versioned generation pointer"}
	}
	if value.Version == "" {
		return invalid()
	}
	decoder := json.NewDecoder(bytes.NewReader(value.Data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return invalid()
	}
	key, err := decoder.Token()
	if err != nil || key != "generation_id" {
		return invalid()
	}
	var id string
	if err := decoder.Decode(&id); err != nil || id == "" {
		return invalid()
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return invalid()
	}
	if _, err := decoder.Token(); err != io.EOF {
		return invalid()
	}
	return id, nil
}

func (s *Object) ensureAuthorityRecord(ctx context.Context, generation catalogs.Generation) error {
	data, err := authorityRecordData(generation)
	if err != nil || data == nil {
		return err
	}
	return s.putImmutable(ctx, s.generationKey(generation.Manifest.GenerationID, authorityFilename), data)
}

var (
	_ AuthorityHeadReader = (*Object)(nil)
	_ CurrentObjectReader = (*MemoryObjectBackend)(nil)
)
