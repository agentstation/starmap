//nolint:revive // The generic name is retained only as a deprecated compatibility path.
package types

import "github.com/agentstation/starmap/pkg/catalogmeta"

// Deprecated: use catalogmeta.SourceID.
type SourceID = catalogmeta.SourceID

const (
	// Deprecated: use catalogmeta.ProvidersID.
	ProvidersID = catalogmeta.ProvidersID
	// Deprecated: use catalogmeta.ModelsDevGitID.
	ModelsDevGitID = catalogmeta.ModelsDevGitID
	// Deprecated: use catalogmeta.ModelsDevHTTPID.
	ModelsDevHTTPID = catalogmeta.ModelsDevHTTPID
	// Deprecated: use catalogmeta.LocalCatalogID.
	LocalCatalogID = catalogmeta.LocalCatalogID
)

// SourceIDs returns every catalog source identifier.
//
// Deprecated: use catalogmeta.SourceIDs.
func SourceIDs() []SourceID {
	return catalogmeta.SourceIDs()
}

// Deprecated: use catalogmeta.ResourceType.
type ResourceType = catalogmeta.ResourceType

const (
	// Deprecated: use catalogmeta.ResourceTypeModel.
	ResourceTypeModel = catalogmeta.ResourceTypeModel
	// Deprecated: use catalogmeta.ResourceTypeProvider.
	ResourceTypeProvider = catalogmeta.ResourceTypeProvider
	// Deprecated: use catalogmeta.ResourceTypeAuthor.
	ResourceTypeAuthor = catalogmeta.ResourceTypeAuthor
	// Deprecated: use catalogmeta.ResourceTypeModelDefinition.
	ResourceTypeModelDefinition = catalogmeta.ResourceTypeModelDefinition
	// Deprecated: use catalogmeta.ResourceTypeProviderOffering.
	ResourceTypeProviderOffering = catalogmeta.ResourceTypeProviderOffering
)

// Deprecated: use catalogmeta.ObservationRevisionKind.
type ObservationRevisionKind = catalogmeta.ObservationRevisionKind

const (
	// Deprecated: use catalogmeta.ObservationRevisionKindUnknown.
	ObservationRevisionKindUnknown = catalogmeta.ObservationRevisionKindUnknown
	// Deprecated: use catalogmeta.ObservationRevisionKindETag.
	ObservationRevisionKindETag = catalogmeta.ObservationRevisionKindETag
	// Deprecated: use catalogmeta.ObservationRevisionKindLastModified.
	ObservationRevisionKindLastModified = catalogmeta.ObservationRevisionKindLastModified
	// Deprecated: use catalogmeta.ObservationRevisionKindGitCommit.
	ObservationRevisionKindGitCommit = catalogmeta.ObservationRevisionKindGitCommit
	// Deprecated: use catalogmeta.ObservationRevisionKindSourceVersion.
	ObservationRevisionKindSourceVersion = catalogmeta.ObservationRevisionKindSourceVersion
	// Deprecated: use catalogmeta.ObservationRevisionKindContentDigest.
	ObservationRevisionKindContentDigest = catalogmeta.ObservationRevisionKindContentDigest
)

// Deprecated: use catalogmeta.ObservationRevision.
type ObservationRevision = catalogmeta.ObservationRevision

// Deprecated: use catalogmeta.ObservationCompleteness.
type ObservationCompleteness = catalogmeta.ObservationCompleteness

const (
	// Deprecated: use catalogmeta.ObservationCompletenessComplete.
	ObservationCompletenessComplete = catalogmeta.ObservationCompletenessComplete
	// Deprecated: use catalogmeta.ObservationCompletenessPartial.
	ObservationCompletenessPartial = catalogmeta.ObservationCompletenessPartial
)

// Deprecated: use catalogmeta.ObservationStatus.
type ObservationStatus = catalogmeta.ObservationStatus

const (
	// Deprecated: use catalogmeta.ObservationStatusSucceeded.
	ObservationStatusSucceeded = catalogmeta.ObservationStatusSucceeded
	// Deprecated: use catalogmeta.ObservationStatusDegraded.
	ObservationStatusDegraded = catalogmeta.ObservationStatusDegraded
)

// Deprecated: use catalogmeta.ObservationRecordCounts.
type ObservationRecordCounts = catalogmeta.ObservationRecordCounts

// Deprecated: use catalogmeta.ObservationIssueScope.
type ObservationIssueScope = catalogmeta.ObservationIssueScope

const (
	// Deprecated: use catalogmeta.ObservationIssueScopeRecord.
	ObservationIssueScopeRecord = catalogmeta.ObservationIssueScopeRecord
	// Deprecated: use catalogmeta.ObservationIssueScopeProvider.
	ObservationIssueScopeProvider = catalogmeta.ObservationIssueScopeProvider
	// Deprecated: use catalogmeta.ObservationIssueScopeSource.
	ObservationIssueScopeSource = catalogmeta.ObservationIssueScopeSource
	// Deprecated: use catalogmeta.ObservationIssueScopeStaleFallback.
	ObservationIssueScopeStaleFallback = catalogmeta.ObservationIssueScopeStaleFallback
)

// Deprecated: use catalogmeta.ObservationIssueCode.
type ObservationIssueCode = catalogmeta.ObservationIssueCode

const (
	// Deprecated: use catalogmeta.ObservationIssueCodeInvalidRecord.
	ObservationIssueCodeInvalidRecord = catalogmeta.ObservationIssueCodeInvalidRecord
	// Deprecated: use catalogmeta.ObservationIssueCodeSchemaDrift.
	ObservationIssueCodeSchemaDrift = catalogmeta.ObservationIssueCodeSchemaDrift
	// Deprecated: use catalogmeta.ObservationIssueCodePayloadLimit.
	ObservationIssueCodePayloadLimit = catalogmeta.ObservationIssueCodePayloadLimit
	// Deprecated: use catalogmeta.ObservationIssueCodeMissingCredentials.
	ObservationIssueCodeMissingCredentials = catalogmeta.ObservationIssueCodeMissingCredentials
	// Deprecated: use catalogmeta.ObservationIssueCodeConfiguration.
	ObservationIssueCodeConfiguration = catalogmeta.ObservationIssueCodeConfiguration
	// Deprecated: use catalogmeta.ObservationIssueCodeFetchFailed.
	ObservationIssueCodeFetchFailed = catalogmeta.ObservationIssueCodeFetchFailed
	// Deprecated: use catalogmeta.ObservationIssueCodeStaleFallback.
	ObservationIssueCodeStaleFallback = catalogmeta.ObservationIssueCodeStaleFallback
	// Deprecated: use catalogmeta.ObservationIssueCodeBootstrapFallback.
	ObservationIssueCodeBootstrapFallback = catalogmeta.ObservationIssueCodeBootstrapFallback
)

// Deprecated: use catalogmeta.ObservationIssue.
type ObservationIssue = catalogmeta.ObservationIssue
