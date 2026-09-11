package catalogs

import (
	"encoding/json"
	"unicode/utf8"
)

const (
	// CatalogAuthorityRecordVersion identifies immutable permission metadata independently of catalog schemas.
	CatalogAuthorityRecordVersion uint64 = 1
	// MaxCatalogAuthorityRecordBytes bounds stored permission metadata before decoding.
	MaxCatalogAuthorityRecordBytes = 16 << 10
)

// CatalogAuthorityRecord binds independent permission metadata to one immutable generation.
// It contains no receipt and cannot establish permission freshness by itself.
type CatalogAuthorityRecord struct {
	Version uint64               `json:"version"`
	Head    CatalogAuthorityHead `json:"head"`
}

// Validate checks the record format and publication identity without reading catalog data.
func (r CatalogAuthorityRecord) Validate() error {
	if r.Version != CatalogAuthorityRecordVersion {
		return validationError("authority_record.version", nil, "must name a supported record version")
	}
	return r.Head.Validate()
}

// ParseCatalogAuthorityRecord strictly decodes bounded immutable permission metadata.
// Unknown positive permission versions remain observable independently of catalog compatibility.
func ParseCatalogAuthorityRecord(data []byte) (CatalogAuthorityRecord, error) {
	if len(data) == 0 || len(data) > MaxCatalogAuthorityRecordBytes || !utf8.Valid(data) {
		return CatalogAuthorityRecord{}, validationError("authority_record", nil, "must contain bounded UTF-8 JSON")
	}
	fields, err := strictJSONObjectMembers(data, "authority_record", []string{"version", "head"})
	if err != nil {
		return CatalogAuthorityRecord{}, err
	}
	head, ok := fields["head"]
	if !ok {
		return CatalogAuthorityRecord{}, validationError("authority_record.head", nil, "is required")
	}
	if _, err := strictJSONObjectMembers(head, "authority_record.head", catalogAuthorityHeadJSONFields); err != nil {
		return CatalogAuthorityRecord{}, err
	}
	var record CatalogAuthorityRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return CatalogAuthorityRecord{}, validationError("authority_record", nil, "must contain a valid authority record")
	}
	if err := record.Validate(); err != nil {
		return CatalogAuthorityRecord{}, err
	}
	return record, nil
}
