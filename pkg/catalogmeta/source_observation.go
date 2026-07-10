package catalogmeta

// ObservationRevisionKind identifies how an upstream revision was obtained.
type ObservationRevisionKind string

const (
	// ObservationRevisionKindUnknown means the upstream exposes no stable revision.
	ObservationRevisionKindUnknown ObservationRevisionKind = "unknown"
	// ObservationRevisionKindETag identifies an HTTP entity tag.
	ObservationRevisionKindETag ObservationRevisionKind = "etag"
	// ObservationRevisionKindLastModified identifies an HTTP Last-Modified validator.
	ObservationRevisionKindLastModified ObservationRevisionKind = "last_modified"
	// ObservationRevisionKindGitCommit identifies an exact Git commit.
	ObservationRevisionKindGitCommit ObservationRevisionKind = "git_commit"
	// ObservationRevisionKindSourceVersion identifies an upstream-declared version.
	ObservationRevisionKindSourceVersion ObservationRevisionKind = "source_version"
	// ObservationRevisionKindContentDigest identifies normalized observation content.
	ObservationRevisionKindContentDigest ObservationRevisionKind = "content_digest"
)

// ObservationRevision identifies an upstream or normalized content revision.
type ObservationRevision struct {
	Kind          ObservationRevisionKind `json:"kind" yaml:"kind"`
	Value         string                  `json:"value,omitempty" yaml:"value,omitempty"`
	InputName     string                  `json:"input_name,omitempty" yaml:"input_name,omitempty"`
	InputChecksum string                  `json:"input_checksum,omitempty" yaml:"input_checksum,omitempty"`
}

// ObservationCompleteness states whether every expected record was observed.
type ObservationCompleteness string

const (
	// ObservationCompletenessComplete means every expected record was observed.
	ObservationCompletenessComplete ObservationCompleteness = "complete"
	// ObservationCompletenessPartial means at least one expected record is absent.
	ObservationCompletenessPartial ObservationCompleteness = "partial"
)

// ObservationStatus is the typed outcome of a source observation.
type ObservationStatus string

const (
	// ObservationStatusSucceeded means no known degradation occurred.
	ObservationStatusSucceeded ObservationStatus = "succeeded"
	// ObservationStatusDegraded means usable data has a known limitation.
	ObservationStatusDegraded ObservationStatus = "degraded"
)

// ObservationRecordCounts reports source records accepted into or rejected
// from one observation.
type ObservationRecordCounts struct {
	Accepted int `json:"accepted" yaml:"accepted"`
	Rejected int `json:"rejected" yaml:"rejected"`
}

// ObservationIssueScope identifies the level at which degradation occurred.
type ObservationIssueScope string

const (
	// ObservationIssueScopeRecord applies to one upstream record.
	ObservationIssueScopeRecord ObservationIssueScope = "record"
	// ObservationIssueScopeProvider applies to one provider within a source.
	ObservationIssueScopeProvider ObservationIssueScope = "provider"
	// ObservationIssueScopeSource applies to the complete source call.
	ObservationIssueScopeSource ObservationIssueScope = "source"
	// ObservationIssueScopeStaleFallback identifies usable stale fallback data.
	ObservationIssueScopeStaleFallback ObservationIssueScope = "stale_fallback"
)

// ObservationIssueCode is a stable machine-readable degradation reason.
type ObservationIssueCode string

const (
	// ObservationIssueCodeInvalidRecord means one record failed validation or conversion.
	ObservationIssueCodeInvalidRecord ObservationIssueCode = "invalid_record"
	// ObservationIssueCodeSchemaDrift means an upstream identity/container changed shape or type.
	ObservationIssueCodeSchemaDrift ObservationIssueCode = "schema_drift"
	// ObservationIssueCodePayloadLimit means an upstream exceeded a bounded resource budget.
	ObservationIssueCodePayloadLimit ObservationIssueCode = "payload_limit"
	// ObservationIssueCodeMissingCredentials means a provider could not be queried.
	ObservationIssueCodeMissingCredentials ObservationIssueCode = "missing_credentials"
	// ObservationIssueCodeConfiguration means source/provider configuration was invalid.
	ObservationIssueCodeConfiguration ObservationIssueCode = "configuration"
	// ObservationIssueCodeFetchFailed means upstream acquisition failed.
	ObservationIssueCodeFetchFailed ObservationIssueCode = "fetch_failed"
	// ObservationIssueCodeStaleFallback means last-known-good stale evidence was used.
	ObservationIssueCodeStaleFallback ObservationIssueCode = "stale_fallback"
	// ObservationIssueCodeBootstrapFallback means embedded bootstrap evidence was used.
	ObservationIssueCodeBootstrapFallback ObservationIssueCode = "bootstrap_fallback"
)

// ObservationIssue records one classified, non-fatal degradation.
type ObservationIssue struct {
	Scope   ObservationIssueScope `json:"scope" yaml:"scope"`
	Code    ObservationIssueCode  `json:"code" yaml:"code"`
	Subject string                `json:"subject,omitempty" yaml:"subject,omitempty"`
	Message string                `json:"message" yaml:"message"`
}
