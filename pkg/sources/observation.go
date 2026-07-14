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

// ProviderCoverage reports provider inventory completeness.
type ProviderCoverage = catalogmeta.ProviderCoverage

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
	ObservedAt        time.Time
	Revision          Revision
	Completeness      ObservationCompleteness
	Status            ObservationStatus
	Records           ObservationRecordCounts
	Issues            []ObservationIssue
	Scope             catalogmeta.ObservationScope
	Kind              catalogmeta.SourceKind
	Coverage          catalogmeta.ProviderCoverage
	PricingObservedAt *time.Time
	Acquisitions      []catalogmeta.AcquisitionProvenance
}

// NewObservation binds an immutable catalog to typed, deterministic audit metadata.
func NewObservation(sourceID ID, catalog *catalogs.Catalog, metadata ObservationMetadata) (Observation, error) {
	if sourceID == "" {
		return Observation{}, &errors.ValidationError{Field: "observation.source", Message: validationIsRequired}
	}
	if catalog == nil {
		return Observation{}, &errors.ValidationError{Field: "observation.catalog", Message: validationIsRequired}
	}
	metadata.ObservedAt = metadata.ObservedAt.UTC()
	if metadata.Scope == "" {
		metadata.Scope = catalogmeta.ObservationScopeGlobalPublic
	}
	if metadata.Kind == "" {
		metadata.Kind = defaultSourceKind(sourceID)
	}
	if metadata.PricingObservedAt != nil {
		pricingObservedAt := metadata.PricingObservedAt.UTC()
		metadata.PricingObservedAt = &pricingObservedAt
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		return Observation{}, errors.WrapResource("encode", "observation evidence", string(sourceID), err)
	}
	checksum := catalogs.DescribeCatalogPayload(payload).Checksum
	if metadata.Revision.Kind == RevisionKindContentDigest && metadata.Revision.Value == "" {
		metadata.Revision.Value = checksum
	}
	observation := Observation{
		SourceID:     sourceID,
		ObservedAt:   metadata.ObservedAt,
		Revision:     metadata.Revision,
		Completeness: metadata.Completeness,
		Status:       metadata.Status,
		Records:      metadata.Records,
		Metrics: catalogmeta.ObservationMetrics{
			Scope: metadata.Scope, Kind: metadata.Kind, Records: metadata.Records,
			ProviderCoverage: metadata.Coverage, PricingObservedAt: metadata.PricingObservedAt,
			Acquisitions: append([]catalogmeta.AcquisitionProvenance(nil), metadata.Acquisitions...),
		},
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
	metrics := o.Metrics
	metrics.Acquisitions = append([]catalogmeta.AcquisitionProvenance(nil), o.Metrics.Acquisitions...)
	if o.Metrics.PricingObservedAt != nil {
		observedAt := *o.Metrics.PricingObservedAt
		metrics.PricingObservedAt = &observedAt
	}
	return catalogs.SourceObservationLink{
		Source: o.SourceID, ObservationID: o.ID, ObservedAt: o.ObservedAt,
		Revision: o.Revision, Completeness: o.Completeness,
		Status: o.Status, EvidenceChecksum: o.EvidenceChecksum,
		Metrics: metrics,
	}
}

// Validate verifies required metadata and binds the evidence checksum to Catalog.
func (o Observation) Validate() error {
	if o.SourceID == "" {
		return observationValidationError("source", o.SourceID, validationIsRequired)
	}
	if o.Catalog == nil {
		return observationValidationError("catalog", nil, validationIsRequired)
	}
	if o.ObservedAt.IsZero() {
		return observationValidationError("observed_at", o.ObservedAt, validationIsRequired)
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
	if err := validateObservationMetrics(o.Records, o.Metrics); err != nil {
		return err
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

func defaultSourceKind(sourceID ID) catalogmeta.SourceKind {
	switch sourceID {
	case ProvidersID:
		return catalogmeta.SourceKindDirectInventory
	case ModelsDevGitID, ModelsDevHTTPID:
		return catalogmeta.SourceKindEnrichment
	case AmazonBedrockID, OCIGenerativeAIID:
		return catalogmeta.SourceKindRegionalSweep
	default:
		return catalogmeta.SourceKindCurated
	}
}

func validateObservationMetrics(records ObservationRecordCounts, metrics catalogmeta.ObservationMetrics) error {
	if records != metrics.Records {
		return observationValidationError("metrics.records", metrics.Records, "must match observation record counts")
	}
	switch metrics.Scope {
	case catalogmeta.ObservationScopeGlobalPublic, catalogmeta.ObservationScopeRegionalPublic, catalogmeta.ObservationScopeCredentialScoped:
	default:
		return observationValidationError("metrics.scope", metrics.Scope, "is not supported")
	}
	switch metrics.Kind {
	case catalogmeta.SourceKindDirectInventory, catalogmeta.SourceKindRegionalSweep, catalogmeta.SourceKindPricing,
		catalogmeta.SourceKindEnrichment, catalogmeta.SourceKindCurated:
	default:
		return observationValidationError("metrics.kind", metrics.Kind, "is not supported")
	}
	if metrics.Records.Accepted < 0 || metrics.Records.Rejected < 0 || metrics.ProviderCoverage.Expected < 0 || metrics.ProviderCoverage.Observed < 0 || metrics.ProviderCoverage.Observed > metrics.ProviderCoverage.Expected && metrics.ProviderCoverage.Expected != 0 {
		return observationValidationError("metrics", metrics, "record and provider coverage counts must be non-negative and observed cannot exceed expected")
	}
	if metrics.PricingObservedAt != nil {
		if metrics.PricingObservedAt.IsZero() {
			return observationValidationError("metrics.pricing_observed_at", metrics.PricingObservedAt, "must be non-zero")
		}
		_, offset := metrics.PricingObservedAt.Zone()
		if offset != 0 {
			return observationValidationError("metrics.pricing_observed_at", metrics.PricingObservedAt, "must be UTC")
		}
	}
	seenAcquisitions := make(map[string]struct{}, len(metrics.Acquisitions))
	for index, acquisition := range metrics.Acquisitions {
		field := fmt.Sprintf("metrics.acquisitions[%d]", index)
		if !safeObservationID(acquisition.ProviderID) {
			return observationValidationError(field+".provider_id", acquisition.ProviderID, "must be a safe provider identifier")
		}
		if !safeObservationID(acquisition.SourceID) {
			return observationValidationError(field+".source_id", acquisition.SourceID, "must be a safe source identifier")
		}
		if acquisition.AuthMethod != "" && acquisition.AuthMethod != "none" && !safeObservationID(acquisition.AuthMethod) {
			return observationValidationError(field+".auth_method", acquisition.AuthMethod, "must be a safe authentication method identifier")
		}
		switch acquisition.Scope {
		case catalogmeta.ObservationScopeGlobalPublic, catalogmeta.ObservationScopeRegionalPublic, catalogmeta.ObservationScopeCredentialScoped:
		default:
			return observationValidationError(field+".scope", acquisition.Scope, "is not supported")
		}
		switch acquisition.Topology {
		case catalogmeta.AcquisitionTopologySingleEndpoint, catalogmeta.AcquisitionTopologyPaginated,
			catalogmeta.AcquisitionTopologyRegionalSweep, catalogmeta.AcquisitionTopologyGrouped:
		default:
			return observationValidationError(field+".topology", acquisition.Topology, "is not supported")
		}
		identity := acquisition.ProviderID + "\x00" + acquisition.SourceID
		if _, duplicate := seenAcquisitions[identity]; duplicate {
			return observationValidationError(field, identity, "duplicates a logical provider source")
		}
		seenAcquisitions[identity] = struct{}{}
	}
	return nil
}

func safeObservationID(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || index > 0 && (character == '-' || character == '_') {
			continue
		}
		return false
	}
	return true
}

func validateRevision(r Revision) error {
	switch r.Kind {
	case RevisionKindUnknown:
		if strings.TrimSpace(r.Value) != "" {
			return observationValidationError("revision.value", r.Value, "must be empty when revision kind is unknown")
		}
	case RevisionKindGitCommit:
		if strings.TrimSpace(r.Value) == "" {
			return observationValidationError("revision.value", r.Value, validationIsRequired)
		}
		if (len(r.Value) != 40 && len(r.Value) != 64) || !isHex(r.Value) {
			return observationValidationError("revision.value", r.Value, "must be an exact hexadecimal Git commit")
		}
		if r.InputName == "" || r.InputChecksum == "" {
			return observationValidationError("revision.input", r, "Git revisions require a lockfile name and checksum")
		}
	case RevisionKindETag, RevisionKindLastModified, RevisionKindSourceVersion, RevisionKindContentDigest:
		if strings.TrimSpace(r.Value) == "" {
			return observationValidationError("revision.value", r.Value, validationIsRequired)
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
	identity.WriteString("\x00metrics:" + string(observation.Metrics.Scope) + ":" + string(observation.Metrics.Kind))
	identity.WriteString(":" + strconv.Itoa(observation.Metrics.ProviderCoverage.Expected) + ":" + strconv.Itoa(observation.Metrics.ProviderCoverage.Observed))
	if observation.Metrics.PricingObservedAt != nil {
		identity.WriteString(":" + observation.Metrics.PricingObservedAt.UTC().Format(time.RFC3339Nano))
	}
	for _, acquisition := range observation.Metrics.Acquisitions {
		identity.WriteString("\x00acquisition:" + acquisition.ProviderID + ":" + acquisition.SourceID + ":" + acquisition.AuthMethod + ":" + string(acquisition.Scope) + ":" + string(acquisition.Topology))
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
		return observationValidationError(prefix+".message", issue.Message, validationIsRequired)
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
