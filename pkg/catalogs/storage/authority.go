package storage

import (
	"context"
	"encoding/json"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const authorityFilename = "authority.json"

// AuthorityHeadReader observes the current stored publication independently of catalog payload compatibility.
// The returned head must be current between invocation and completion, including other writers' publications.
// Reads do not repair metadata or renew receipts. Missing authority metadata returns an error.
// Receipt issuers start validity before this read and qualify their clocks separately.
type AuthorityHeadReader interface {
	CurrentAuthorityHead(context.Context) (catalogs.CatalogAuthorityHead, error)
}

// CurrentAuthorityHead returns the selected authority head under the publication lock without copying its payload.
func (s *Memory) CurrentAuthorityHead(ctx context.Context) (catalogs.CatalogAuthorityHead, error) {
	if err := ctx.Err(); err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	head := s.generations[s.currentID].Manifest.AuthorityHead
	if head == (catalogs.CatalogAuthorityHead{}) {
		return catalogs.CatalogAuthorityHead{}, authorityNotFound(s.currentID)
	}
	return head, nil
}

func authorityNotFound(id string) error {
	return &errors.NotFoundError{Resource: "catalog authority metadata", ID: id}
}

func authorityRecordData(generation catalogs.Generation) ([]byte, error) {
	head := generation.Manifest.AuthorityHead
	if head == (catalogs.CatalogAuthorityHead{}) {
		return nil, nil
	}
	record := catalogs.CatalogAuthorityRecord{Version: catalogs.CatalogAuthorityRecordVersion, Head: head}
	if err := record.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return nil, &errors.ValidationError{Field: "authority_record", Message: "cannot encode JSON"}
	}
	return data, nil
}

func parseAuthorityRecord(data []byte, id string) (catalogs.CatalogAuthorityHead, error) {
	record, err := catalogs.ParseCatalogAuthorityRecord(data)
	if err != nil {
		return catalogs.CatalogAuthorityHead{}, err
	}
	if record.Head.GenerationID != id {
		return catalogs.CatalogAuthorityHead{}, &errors.ValidationError{Field: "authority_record.generation_id", Message: "does not match the selected generation"}
	}
	return record.Head, nil
}

var _ AuthorityHeadReader = (*Memory)(nil)
