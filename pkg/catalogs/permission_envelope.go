package catalogs

import (
	"bytes"
	"encoding/json"
	"io"
	"time"
	"unicode/utf8"
)

const (
	// CatalogPermissionEnvelopeVersion identifies the small envelope independently of the catalog manifest and payload versions.
	CatalogPermissionEnvelopeVersion uint64 = 1
	// MaxCatalogPermissionValidity bounds the initial internal production profile.
	MaxCatalogPermissionValidity = 5 * time.Minute
	// MaxCatalogPermissionClockUncertainty bounds the initial profile's clock uncertainty.
	MaxCatalogPermissionClockUncertainty = 30 * time.Second
	// MaxCatalogPermissionEnvelopeBytes bounds the complete permission envelope before decoding.
	MaxCatalogPermissionEnvelopeBytes = 16 << 10
)

// CatalogPermissionEnvelope carries a publication head and its finite permission receipt.
// A trusted authority can renew a receipt without changing the immutable publication head.
// Callers authenticate the envelope before acceptance. These methods validate structure and time only.
type CatalogPermissionEnvelope struct {
	Version    uint64               `json:"version"`
	Head       CatalogAuthorityHead `json:"head"`
	IssuedAt   time.Time            `json:"issued_at"`
	ValidUntil time.Time            `json:"valid_until"`
}

// Validate checks the version, publication head, and bounded receipt lifetime.
func (e CatalogPermissionEnvelope) Validate() error {
	if e.Version != CatalogPermissionEnvelopeVersion {
		return validationError("permission.version", nil, "must name a supported envelope version")
	}
	if err := e.Head.Validate(); err != nil {
		return err
	}
	if e.IssuedAt.IsZero() || e.ValidUntil.IsZero() || !e.ValidUntil.After(e.IssuedAt) {
		return validationError("permission.valid_until", nil, "must follow a nonzero issue time")
	}
	if e.ValidUntil.Sub(e.IssuedAt) > MaxCatalogPermissionValidity {
		return validationError("permission.valid_until", nil, "exceeds the supported permission lifetime")
	}
	return nil
}

// ValidAt checks a validated receipt against a known clock bound without allocating memory.
// Unknown clock validity, excessive uncertainty, issue times beyond the clock bound, and expiry refuse the receipt.
// This time check grants no permission and does not authenticate the envelope.
func (e CatalogPermissionEnvelope) ValidAt(now time.Time, uncertainty time.Duration, clockKnown bool) bool {
	if !clockKnown || now.IsZero() || uncertainty < 0 || uncertainty > MaxCatalogPermissionClockUncertainty {
		return false
	}
	return !now.Add(uncertainty).Before(e.IssuedAt) && now.Before(e.ValidUntil.Add(-uncertainty))
}

// ValidateSuccessor rejects head replay and receipt renewal that moves backward in issue time.
// An identical head and issue time identify the same receipt, including its expiry.
func (e CatalogPermissionEnvelope) ValidateSuccessor(next CatalogPermissionEnvelope) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if err := next.Validate(); err != nil {
		return err
	}
	if err := e.Head.ValidateSuccessor(next.Head); err != nil {
		return err
	}
	if next.IssuedAt.Before(e.IssuedAt) {
		return validationError("permission.issued_at", nil, "cannot precede the latest accepted receipt")
	}
	if next.Head == e.Head && next.IssuedAt.Equal(e.IssuedAt) && !next.ValidUntil.Equal(e.ValidUntil) {
		return validationError("permission.valid_until", nil, "cannot change an already accepted receipt")
	}
	return nil
}

// ParseCatalogPermissionEnvelope strictly decodes the small permission envelope without reading a catalog manifest or payload.
// The caller must authenticate the response and apply successor, compatibility, and clock checks before admission.
func ParseCatalogPermissionEnvelope(data []byte) (CatalogPermissionEnvelope, error) {
	if len(data) == 0 || len(data) > MaxCatalogPermissionEnvelopeBytes {
		return CatalogPermissionEnvelope{}, validationError("permission.envelope", nil, "must fit the permission envelope byte limit")
	}
	if !utf8.Valid(data) {
		return CatalogPermissionEnvelope{}, validationError("permission.envelope", nil, "must contain valid UTF-8")
	}
	if err := validatePermissionObjectNames(data); err != nil {
		return CatalogPermissionEnvelope{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var envelope CatalogPermissionEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return CatalogPermissionEnvelope{}, validationError("permission.envelope", nil, "must be a strict permission envelope object")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return CatalogPermissionEnvelope{}, validationError("permission.envelope", nil, "must contain exactly one JSON document")
	}
	if err := envelope.Validate(); err != nil {
		return CatalogPermissionEnvelope{}, err
	}
	return envelope, nil
}
