package sources

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// RevisionKind identifies how an upstream observation revision was obtained.
type RevisionKind = catalogmeta.ObservationRevisionKind

const (
	// RevisionKindUnknown means the upstream exposes no stable revision.
	RevisionKindUnknown = catalogmeta.ObservationRevisionKindUnknown
	// RevisionKindETag identifies an HTTP entity-tag revision.
	RevisionKindETag = catalogmeta.ObservationRevisionKindETag
	// RevisionKindLastModified identifies an HTTP Last-Modified validator.
	RevisionKindLastModified = catalogmeta.ObservationRevisionKindLastModified
	// RevisionKindGitCommit identifies an exact Git commit.
	RevisionKindGitCommit = catalogmeta.ObservationRevisionKindGitCommit
	// RevisionKindSourceVersion identifies an upstream-declared version.
	RevisionKindSourceVersion = catalogmeta.ObservationRevisionKindSourceVersion
	// RevisionKindContentDigest identifies the normalized observation content.
	RevisionKindContentDigest = catalogmeta.ObservationRevisionKindContentDigest
)

// Revision identifies the exact upstream or normalized content revision.
type Revision = catalogmeta.ObservationRevision

// ObservationCompleteness states whether all expected records were observed.
type ObservationCompleteness = catalogmeta.ObservationCompleteness

const (
	// ObservationCompletenessComplete means every expected record was observed.
	ObservationCompletenessComplete = catalogmeta.ObservationCompletenessComplete
	// ObservationCompletenessPartial means at least one expected record is absent.
	ObservationCompletenessPartial = catalogmeta.ObservationCompletenessPartial
)

// ObservationStatus is the typed outcome of a source observation.
type ObservationStatus = catalogmeta.ObservationStatus

const (
	// ObservationStatusSucceeded means the observation completed without known degradation.
	ObservationStatusSucceeded = catalogmeta.ObservationStatusSucceeded
	// ObservationStatusDegraded means usable catalog data was returned with a known limitation.
	ObservationStatusDegraded = catalogmeta.ObservationStatusDegraded
)

// ObservationRecordCounts reports accepted and rejected source records.
type ObservationRecordCounts = catalogmeta.ObservationRecordCounts

// ObservationIssueScope identifies the level at which degradation occurred.
type ObservationIssueScope = catalogmeta.ObservationIssueScope

// Observation issue scope values.
const (
	ObservationIssueScopeRecord        = catalogmeta.ObservationIssueScopeRecord
	ObservationIssueScopeProvider      = catalogmeta.ObservationIssueScopeProvider
	ObservationIssueScopeSource        = catalogmeta.ObservationIssueScopeSource
	ObservationIssueScopeStaleFallback = catalogmeta.ObservationIssueScopeStaleFallback
)

// ObservationIssueCode is a stable machine-readable degradation reason.
type ObservationIssueCode = catalogmeta.ObservationIssueCode

// Observation issue code values.
const (
	ObservationIssueCodeInvalidRecord      = catalogmeta.ObservationIssueCodeInvalidRecord
	ObservationIssueCodeSchemaDrift        = catalogmeta.ObservationIssueCodeSchemaDrift
	ObservationIssueCodePayloadLimit       = catalogmeta.ObservationIssueCodePayloadLimit
	ObservationIssueCodeMissingCredentials = catalogmeta.ObservationIssueCodeMissingCredentials
	ObservationIssueCodeConfiguration      = catalogmeta.ObservationIssueCodeConfiguration
	ObservationIssueCodeFetchFailed        = catalogmeta.ObservationIssueCodeFetchFailed
	ObservationIssueCodeStaleFallback      = catalogmeta.ObservationIssueCodeStaleFallback
	ObservationIssueCodeBootstrapFallback  = catalogmeta.ObservationIssueCodeBootstrapFallback
)

// ObservationIssue records one classified, non-fatal degradation.
type ObservationIssue = catalogmeta.ObservationIssue

// ObservationMetadata supplies source-owned metadata used to construct an observation.
type ObservationMetadata struct {
	ObservedAt   time.Time
	Revision     Revision
	Completeness ObservationCompleteness
	Status       ObservationStatus
	Records      ObservationRecordCounts
	Issues       []ObservationIssue
}

// NewObservation binds an immutable catalog to typed, deterministic audit metadata.
func NewObservation(sourceID ID, catalog *catalogs.Catalog, metadata ObservationMetadata) (Observation, error) {
	if sourceID == "" {
		return Observation{}, &errors.ValidationError{Field: "observation.source", Message: "is required"}
	}
	if catalog == nil {
		return Observation{}, &errors.ValidationError{Field: "observation.catalog", Message: "is required"}
	}
	metadata.ObservedAt = metadata.ObservedAt.UTC()
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		return Observation{}, errors.WrapResource("encode", "observation evidence", string(sourceID), err)
	}
	checksum := catalogs.DescribeCatalogPayload(payload).Checksum
	if metadata.Revision.Kind == RevisionKindContentDigest && metadata.Revision.Value == "" {
		metadata.Revision.Value = checksum
	}
	observation := Observation{
		SourceID:         sourceID,
		ObservedAt:       metadata.ObservedAt,
		Revision:         metadata.Revision,
		Completeness:     metadata.Completeness,
		Status:           metadata.Status,
		Records:          metadata.Records,
		Issues:           append([]ObservationIssue(nil), metadata.Issues...),
		EvidenceChecksum: checksum,
		Catalog:          catalog,
	}
	observation.ID = observationID(observation)
	if err := observation.Validate(); err != nil {
		return Observation{}, err
	}
	return observation, nil
}

// Link returns the immutable manifest/audit projection of this observation.
func (o Observation) Link() catalogs.SourceObservationLink {
	return catalogs.SourceObservationLink{
		Source: o.SourceID, ObservationID: o.ID, ObservedAt: o.ObservedAt,
		Revision: o.Revision, Completeness: o.Completeness,
		Status: o.Status, EvidenceChecksum: o.EvidenceChecksum,
	}
}

// Validate verifies required metadata and binds the evidence checksum to Catalog.
func (o Observation) Validate() error {
	if o.SourceID == "" {
		return observationValidationError("source", o.SourceID, "is required")
	}
	if o.Catalog == nil {
		return observationValidationError("catalog", nil, "is required")
	}
	if o.ObservedAt.IsZero() {
		return observationValidationError("observed_at", o.ObservedAt, "is required")
	}
	_, offset := o.ObservedAt.Zone()
	if offset != 0 {
		return observationValidationError("observed_at", o.ObservedAt, "must be UTC")
	}
	if err := validateRevision(o.Revision); err != nil {
		return err
	}
	if o.Completeness != ObservationCompletenessComplete && o.Completeness != ObservationCompletenessPartial {
		return observationValidationError("completeness", o.Completeness, "must be complete or partial")
	}
	if o.Status != ObservationStatusSucceeded && o.Status != ObservationStatusDegraded {
		return observationValidationError("status", o.Status, "must be succeeded or degraded")
	}
	if o.Completeness == ObservationCompletenessPartial && o.Status != ObservationStatusDegraded {
		return observationValidationError("status", o.Status, "partial observations must be degraded")
	}
	if o.Status == ObservationStatusSucceeded && len(o.Issues) != 0 {
		return observationValidationError("issues", o.Issues, "succeeded observations cannot contain degradation issues")
	}
	if o.Status == ObservationStatusDegraded && len(o.Issues) == 0 {
		return observationValidationError("issues", o.Issues, "degraded observations require at least one issue")
	}
	if o.Records.Accepted < 0 {
		return observationValidationError("records.accepted", o.Records.Accepted, "must be non-negative")
	}
	if o.Records.Rejected < 0 {
		return observationValidationError("records.rejected", o.Records.Rejected, "must be non-negative")
	}
	if o.Records.Rejected > 0 && (o.Completeness != ObservationCompletenessPartial || o.Status != ObservationStatusDegraded) {
		return observationValidationError("records.rejected", o.Records.Rejected, "requires a partial degraded observation")
	}
	for index, issue := range o.Issues {
		if err := validateObservationIssue(index, issue); err != nil {
			return err
		}
	}
	payload, err := catalogs.EncodeCatalogPayload(o.Catalog)
	if err != nil {
		return errors.WrapResource("encode", "observation evidence", string(o.SourceID), err)
	}
	actualChecksum := catalogs.DescribeCatalogPayload(payload).Checksum
	if o.EvidenceChecksum != actualChecksum {
		return observationValidationError("evidence_checksum", o.EvidenceChecksum, fmt.Sprintf("must match %s", actualChecksum))
	}
	if o.Revision.Kind == RevisionKindContentDigest && o.Revision.Value != actualChecksum {
		return observationValidationError("revision.value", o.Revision.Value, "must match the normalized content digest")
	}
	expectedID := observationID(o)
	if o.ID != expectedID {
		return observationValidationError("id", o.ID, fmt.Sprintf("must match %s", expectedID))
	}
	return nil
}

func validateRevision(r Revision) error {
	switch r.Kind {
	case RevisionKindUnknown:
		if strings.TrimSpace(r.Value) != "" {
			return observationValidationError("revision.value", r.Value, "must be empty when revision kind is unknown")
		}
	case RevisionKindGitCommit:
		if strings.TrimSpace(r.Value) == "" {
			return observationValidationError("revision.value", r.Value, "is required")
		}
		if (len(r.Value) != 40 && len(r.Value) != 64) || !isHex(r.Value) {
			return observationValidationError("revision.value", r.Value, "must be an exact hexadecimal Git commit")
		}
		if r.InputName == "" || r.InputChecksum == "" {
			return observationValidationError("revision.input", r, "Git revisions require a lockfile name and checksum")
		}
	case RevisionKindETag, RevisionKindLastModified, RevisionKindSourceVersion, RevisionKindContentDigest:
		if strings.TrimSpace(r.Value) == "" {
			return observationValidationError("revision.value", r.Value, "is required")
		}
	default:
		return observationValidationError("revision.kind", r.Kind, "is not supported")
	}
	if (r.InputName == "") != (r.InputChecksum == "") {
		return observationValidationError("revision.input", r, "name and checksum must be supplied together")
	}
	if r.InputName != "" && r.Kind != RevisionKindGitCommit {
		return observationValidationError("revision.input", r, "content-addressed build input is only supported for Git revisions")
	}
	if r.InputName != "" && (!strings.HasPrefix(r.InputChecksum, "sha256:") || len(r.InputChecksum) != len("sha256:")+64) {
		return observationValidationError("revision.input_checksum", r.InputChecksum, "must be a SHA-256 checksum")
	}
	return nil
}

func isHex(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}

func observationID(observation Observation) string {
	var identity strings.Builder
	identity.WriteString(strings.Join([]string{
		string(observation.SourceID),
		observation.ObservedAt.UTC().Format(time.RFC3339Nano),
		string(observation.Revision.Kind),
		observation.Revision.Value,
		string(observation.Completeness),
		string(observation.Status),
		observation.EvidenceChecksum,
	}, "\x00"))
	identity.WriteString("\x00" + observation.Revision.InputName + "\x00" + observation.Revision.InputChecksum)
	if observation.Records.Accepted != 0 || observation.Records.Rejected != 0 {
		identity.WriteString("\x00records:")
		identity.WriteString(strconv.Itoa(observation.Records.Accepted))
		identity.WriteByte(':')
		identity.WriteString(strconv.Itoa(observation.Records.Rejected))
	}
	for _, issue := range observation.Issues {
		// Human-readable diagnostics can contain transport details or secrets and
		// are deliberately excluded from stable identity and long-term evidence.
		identity.WriteString("\x00" + string(issue.Scope) + "\x00" + string(issue.Code) + "\x00" + issue.Subject)
	}
	digest := sha256.Sum256([]byte(identity.String()))
	return "observation:" + hex.EncodeToString(digest[:])
}

func validateObservationIssue(index int, issue ObservationIssue) error {
	prefix := fmt.Sprintf("issues[%d]", index)
	switch issue.Scope {
	case ObservationIssueScopeRecord, ObservationIssueScopeProvider:
		if strings.TrimSpace(issue.Subject) == "" {
			return observationValidationError(prefix+".subject", issue.Subject, "is required for record/provider issues")
		}
	case ObservationIssueScopeSource, ObservationIssueScopeStaleFallback:
	default:
		return observationValidationError(prefix+".scope", issue.Scope, "is not supported")
	}
	switch issue.Code {
	case ObservationIssueCodeInvalidRecord,
		ObservationIssueCodeSchemaDrift,
		ObservationIssueCodePayloadLimit,
		ObservationIssueCodeMissingCredentials,
		ObservationIssueCodeConfiguration,
		ObservationIssueCodeFetchFailed,
		ObservationIssueCodeStaleFallback,
		ObservationIssueCodeBootstrapFallback:
	default:
		return observationValidationError(prefix+".code", issue.Code, "is not supported")
	}
	if strings.TrimSpace(issue.Message) == "" {
		return observationValidationError(prefix+".message", issue.Message, "is required")
	}
	return nil
}

func observationValidationError(field string, value any, message string) error {
	return &errors.ValidationError{
		Field:   "observation." + field,
		Value:   value,
		Message: message,
	}
}
