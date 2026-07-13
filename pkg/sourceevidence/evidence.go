// Package sourceevidence captures replayable normalized observations and
// protects short-lived raw upstream evidence.
package sourceevidence

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

const (
	normalizedRecordVersion uint64 = 1
	sealedRawRecordVersion  uint64 = 1
	rawEncryptionAlgorithm         = "AES-256-GCM"
	maxRawRetention                = 7 * 24 * time.Hour
	redactedIssueMessage           = "diagnostic omitted from minimized evidence"
)

// MinimizedIssue retains machine-readable degradation without diagnostic text.
type MinimizedIssue struct {
	Scope   catalogmeta.ObservationIssueScope `json:"scope"`
	Code    catalogmeta.ObservationIssueCode  `json:"code"`
	Subject string                            `json:"subject,omitempty"`
}

// NormalizedRecord is the long-term, secret-minimized replay record for one observation.
type NormalizedRecord struct {
	Version          uint64                              `json:"version"`
	ObservationID    string                              `json:"observation_id"`
	SourceID         catalogmeta.SourceID                `json:"source"`
	ObservedAt       time.Time                           `json:"observed_at"`
	Revision         catalogmeta.ObservationRevision     `json:"revision"`
	Completeness     catalogmeta.ObservationCompleteness `json:"completeness"`
	Status           catalogmeta.ObservationStatus       `json:"status"`
	Records          catalogmeta.ObservationRecordCounts `json:"records"`
	Issues           []MinimizedIssue                    `json:"issues,omitempty"`
	EvidenceChecksum string                              `json:"evidence_checksum"`
	Metrics          catalogmeta.ObservationMetrics      `json:"metrics"`
	Payload          []byte                              `json:"payload"`
}

// Capture creates a long-term normalized evidence record from an observation.
func Capture(observation sources.Observation) (NormalizedRecord, error) {
	if err := observation.Validate(); err != nil {
		return NormalizedRecord{}, errors.WrapResource("capture", "source observation", observation.ID, err)
	}
	payload, err := catalogs.EncodeCatalogPayload(observation.Catalog)
	if err != nil {
		return NormalizedRecord{}, errors.WrapResource("encode", "source evidence", observation.ID, err)
	}
	issues := make([]MinimizedIssue, 0, len(observation.Issues))
	for _, issue := range observation.Issues {
		issues = append(issues, MinimizedIssue{Scope: issue.Scope, Code: issue.Code, Subject: issue.Subject})
	}
	return NormalizedRecord{
		Version:          normalizedRecordVersion,
		ObservationID:    observation.ID,
		SourceID:         observation.SourceID,
		ObservedAt:       observation.ObservedAt,
		Revision:         observation.Revision,
		Completeness:     observation.Completeness,
		Status:           observation.Status,
		Records:          observation.Records,
		Issues:           issues,
		EvidenceChecksum: observation.EvidenceChecksum,
		Metrics:          observation.Metrics,
		Payload:          payload,
	}, nil
}

// Replay verifies and reconstructs the exact normalized candidate catalog and provenance.
func Replay(record NormalizedRecord) (sources.Observation, error) {
	if record.Version != normalizedRecordVersion {
		return sources.Observation{}, evidenceValidation("version", record.Version, fmt.Sprintf("must be %d", normalizedRecordVersion))
	}
	descriptor := catalogs.DescribeCatalogPayload(record.Payload)
	if descriptor.Checksum != record.EvidenceChecksum {
		return sources.Observation{}, evidenceValidation("evidence_checksum", record.EvidenceChecksum, "does not match normalized payload")
	}
	catalog, err := catalogstore.DecodeCatalogPayload(record.Payload)
	if err != nil {
		return sources.Observation{}, errors.WrapResource("decode", "normalized source evidence", record.ObservationID, err)
	}
	issues := make([]sources.ObservationIssue, 0, len(record.Issues))
	for _, issue := range record.Issues {
		issues = append(issues, sources.ObservationIssue{
			Scope: issue.Scope, Code: issue.Code, Subject: issue.Subject, Message: redactedIssueMessage,
		})
	}
	observation, err := sources.NewObservation(record.SourceID, catalog, sources.ObservationMetadata{
		ObservedAt: record.ObservedAt, Revision: record.Revision,
		Completeness: record.Completeness, Status: record.Status, Records: record.Records, Issues: issues,
		Scope: record.Metrics.Scope, Kind: record.Metrics.Kind, Coverage: record.Metrics.ProviderCoverage,
		PricingObservedAt: record.Metrics.PricingObservedAt,
	})
	if err != nil {
		return sources.Observation{}, errors.WrapResource("replay", "source observation", record.ObservationID, err)
	}
	if observation.ID != record.ObservationID {
		return sources.Observation{}, evidenceValidation("observation_id", record.ObservationID, "does not match replayed metadata")
	}
	return observation, nil
}

// RawAccess describes the access boundary required for raw evidence storage.
type RawAccess string

const (
	// RawAccessOwnerOnly requires an OS/service identity boundary with no public access.
	RawAccessOwnerOnly RawAccess = "owner_only"
)

// Policy defines raw and normalized evidence retention requirements.
type Policy struct {
	RawRetention      time.Duration `json:"raw_retention"`
	RawAccess         RawAccess     `json:"raw_access"`
	RequireEncryption bool          `json:"require_encryption"`
	RetainHeaders     bool          `json:"retain_headers"`
	RetainQuery       bool          `json:"retain_query"`
}

// DefaultPolicy returns Starmap's bounded, encrypted, owner-only raw policy.
func DefaultPolicy() Policy {
	return Policy{
		RawRetention: 24 * time.Hour, RawAccess: RawAccessOwnerOnly,
		RequireEncryption: true, RetainHeaders: false, RetainQuery: false,
	}
}

// Validate rejects policies that could expose raw credentials or retain raw data indefinitely.
func (p Policy) Validate() error {
	if p.RawRetention <= 0 || p.RawRetention > maxRawRetention {
		return evidenceValidation("policy.raw_retention", p.RawRetention, "must be greater than zero and at most 7 days")
	}
	if p.RawAccess != RawAccessOwnerOnly {
		return evidenceValidation("policy.raw_access", p.RawAccess, "must be owner_only")
	}
	if !p.RequireEncryption {
		return evidenceValidation("policy.require_encryption", false, "must be true")
	}
	if p.RetainHeaders || p.RetainQuery {
		return evidenceValidation("policy.request_metadata", true, "headers and query parameters must not be retained")
	}
	return nil
}

// RawRecord contains response-body evidence only; request headers, query values,
// and credentials have no representation in this type.
type RawRecord struct {
	SourceID   catalogmeta.SourceID `json:"source"`
	ObservedAt time.Time            `json:"observed_at"`
	MediaType  string               `json:"media_type"`
	Payload    []byte               `json:"payload"`
}

// SealedRawRecord is an encrypted, expiring raw evidence envelope.
type SealedRawRecord struct {
	Version       uint64    `json:"version"`
	Algorithm     string    `json:"algorithm"`
	ObservationID string    `json:"observation_id,omitempty"`
	Nonce         []byte    `json:"nonce"`
	Ciphertext    []byte    `json:"ciphertext"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// SealRaw encrypts short-lived raw evidence with AES-256-GCM.
func SealRaw(key []byte, record RawRecord, expiresAt time.Time) (SealedRawRecord, error) {
	return sealRaw(key, "", record, expiresAt)
}

func sealRaw(key []byte, observationID string, record RawRecord, expiresAt time.Time) (SealedRawRecord, error) {
	if err := validateRawRecord(record); err != nil {
		return SealedRawRecord{}, err
	}
	if expiresAt.IsZero() || !expiresAt.After(record.ObservedAt) || expiresAt.Sub(record.ObservedAt) > maxRawRetention {
		return SealedRawRecord{}, evidenceValidation("expires_at", expiresAt, "must be after observed_at and within 7 days")
	}
	gcm, err := newGCM(key)
	if err != nil {
		return SealedRawRecord{}, err
	}
	plaintext, err := json.Marshal(record)
	if err != nil {
		return SealedRawRecord{}, evidenceValidation("raw_record", nil, fmt.Sprintf("cannot encode: %v", err))
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return SealedRawRecord{}, errors.WrapIO("generate", "raw evidence nonce", err)
	}
	expiresAt = expiresAt.UTC()
	return SealedRawRecord{
		Version: sealedRawRecordVersion, Algorithm: rawEncryptionAlgorithm,
		ObservationID: observationID, Nonce: nonce,
		Ciphertext: gcm.Seal(nil, nonce, plaintext, rawAAD(observationID, expiresAt)), ExpiresAt: expiresAt,
	}, nil
}

// OpenRaw decrypts non-expired raw evidence and authenticates its envelope.
func OpenRaw(key []byte, sealed SealedRawRecord, now time.Time) (RawRecord, error) {
	if sealed.Version != sealedRawRecordVersion || sealed.Algorithm != rawEncryptionAlgorithm {
		return RawRecord{}, evidenceValidation("sealed_raw", sealed.Version, "unsupported version or algorithm")
	}
	if !now.Before(sealed.ExpiresAt) {
		return RawRecord{}, evidenceValidation("expires_at", sealed.ExpiresAt, "raw evidence has expired")
	}
	gcm, err := newGCM(key)
	if err != nil {
		return RawRecord{}, err
	}
	if len(sealed.Nonce) != gcm.NonceSize() {
		return RawRecord{}, evidenceValidation("nonce", len(sealed.Nonce), "has invalid size")
	}
	plaintext, err := gcm.Open(nil, sealed.Nonce, sealed.Ciphertext, rawAAD(sealed.ObservationID, sealed.ExpiresAt))
	if err != nil {
		return RawRecord{}, evidenceValidation("ciphertext", nil, "authentication failed")
	}
	var record RawRecord
	if err := json.Unmarshal(plaintext, &record); err != nil {
		return RawRecord{}, evidenceValidation("raw_record", nil, fmt.Sprintf("cannot decode: %v", err))
	}
	if err := validateRawRecord(record); err != nil {
		return RawRecord{}, err
	}
	return record, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, evidenceValidation("encryption_key", len(key), "must be exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, evidenceValidation("encryption_key", nil, err.Error())
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, evidenceValidation("encryption", nil, err.Error())
	}
	return gcm, nil
}

func validateRawRecord(record RawRecord) error {
	if strings.TrimSpace(record.SourceID.String()) == "" {
		return evidenceValidation("raw.source", record.SourceID, "is required")
	}
	if record.ObservedAt.IsZero() {
		return evidenceValidation("raw.observed_at", record.ObservedAt, "is required")
	}
	if strings.TrimSpace(record.MediaType) == "" || len(record.Payload) == 0 {
		return evidenceValidation("raw.payload", nil, "media type and payload are required")
	}
	return nil
}

func rawAAD(observationID string, expiresAt time.Time) []byte {
	return fmt.Appendf(nil, "starmap-raw-evidence-v%d\x00%s\x00%s", sealedRawRecordVersion, observationID, expiresAt.UTC().Format(time.RFC3339Nano))
}

func evidenceValidation(field string, value any, message string) error {
	return &errors.ValidationError{Field: "source_evidence." + field, Value: value, Message: message}
}
