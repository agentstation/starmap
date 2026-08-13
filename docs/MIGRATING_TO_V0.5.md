# Migrate to Starmap v0.5.0

Starmap v0.5.0 is a direct pre-v1 Go package break. It moves public catalog
concepts into one `pkg/catalogs` tree. It does not provide old-path wrappers or
aliases.

This release does not change catalog schema 5, generation manifest 2, artifact
bytes, or the remote wire protocol. Existing valid v0.4 generation stores and
catalog artifacts remain valid.

## Update the module

Update the dependency and remove any local Starmap replacement:

```bash
go get github.com/agentstation/starmap@v0.5.0
go mod tidy
```

Then update every removed import. The following table is complete.

| Removed import | Replacement | Purpose |
|---|---|---|
| `github.com/agentstation/starmap/pkg/catalogmeta` | `github.com/agentstation/starmap/pkg/catalogs/evidence` | Source observations, resource identity, and review candidates. |
| `github.com/agentstation/starmap/pkg/catalogmeta` | `github.com/agentstation/starmap/pkg/catalogs/projection` | Post-commit workspace projection results. |
| `github.com/agentstation/starmap/pkg/catalogstore` | `github.com/agentstation/starmap/pkg/catalogs` | Immutable generations and payload codecs. |
| `github.com/agentstation/starmap/pkg/catalogstore` | `github.com/agentstation/starmap/pkg/catalogs/storage` | Generation stores and storage adapters. |
| `github.com/agentstation/starmap/pkg/catalogstore/s3` | `github.com/agentstation/starmap/pkg/catalogs/storage/s3` | The caller-owned S3 adapter. |
| `github.com/agentstation/starmap/pkg/catalogartifact` | `github.com/agentstation/starmap/pkg/catalogs/artifact` | Deterministic portable generation artifacts. |
| `github.com/agentstation/starmap/pkg/catalogremote` | `github.com/agentstation/starmap/pkg/catalogs/remote` | The versioned manifest, payload, and SSE client. |

## Replace catalogmeta selectors

Import `pkg/catalogs/evidence` as `evidence`. Change the qualifier from
`catalogmeta` to `evidence` for every name in this list:

```text
SourceID
SourceIDs
ProvidersID
ModelsDevGitID
ModelsDevHTTPID
LocalCatalogID
EmbeddedCatalogID
ReleaseArtifactID
ResourceType
ResourceTypeModel
ResourceTypeProvider
ResourceTypeAuthor
ResourceTypeModelDefinition
ResourceTypeProviderOffering
ReviewCandidateCode
ReviewCandidateUnresolvedModelReference
ReviewCandidate
CompareReviewCandidates
ObservationRevisionKind
ObservationRevisionKindUnknown
ObservationRevisionKindETag
ObservationRevisionKindLastModified
ObservationRevisionKindGitCommit
ObservationRevisionKindSourceVersion
ObservationRevisionKindContentDigest
ObservationRevision
ObservationCompleteness
ObservationCompletenessComplete
ObservationCompletenessPartial
ObservationStatus
ObservationStatusSucceeded
ObservationStatusDegraded
ObservationRecordCounts
ObservationIssueScope
ObservationIssueScopeRecord
ObservationIssueScopeProvider
ObservationIssueScopeSource
ObservationIssueScopeStaleFallback
ObservationIssueCode
ObservationIssueCodeInvalidRecord
ObservationIssueCodeSchemaDrift
ObservationIssueCodePayloadLimit
ObservationIssueCodeMissingCredentials
ObservationIssueCodeConfiguration
ObservationIssueCodeFetchFailed
ObservationIssueCodeStaleFallback
ObservationIssueCodeBootstrapFallback
ObservationIssueCodeVolumeCollapse
ObservationIssue
```

Import `pkg/catalogs/projection` as `projection`. Change the qualifier from
`catalogmeta` to `projection` for these names:

```text
ProjectionStatus -> Status
ProjectionStatusApplied -> StatusApplied
ProjectionStatusPendingRepair -> StatusPendingRepair
ProjectionIssueWorkspaceFailed -> IssueWorkspaceFailed
ProjectionResult -> Result
```

The types and values keep their v0.4 wire representation. The move gives
evidence and projection separate dependency boundaries.

## Replace catalogstore selectors

Import `pkg/catalogs` as `catalogs`. Use these replacements:

| Removed selector | Replacement |
|---|---|
| `catalogstore.Generation` | `catalogs.Generation` |
| `catalogstore.EncodeCatalogPayload` | `catalogs.EncodeCatalogPayload` |
| `catalogstore.DecodeCatalogPayload` | `catalogs.DecodeCatalogPayload` |
| `catalogstore.DecodeSourceObservationPayload` | `catalogs.DecodeSourceObservationPayload` |

`catalogs.Generation.Copy` and `catalogs.Generation.Validate` keep the same
method contracts.

Import `pkg/catalogs/storage` as `storage`. Change the qualifier from
`catalogstore` to `storage` for these names:

```text
Store
Filesystem
NewFilesystem
Memory
NewMemory
ObjectValue
ObjectPutCondition
ObjectBackend
MemoryObjectBackend
NewMemoryObjectBackend
Object
NewObject
```

Store methods now accept and return `catalogs.Generation`. Their names and
compare-and-swap behavior do not change.

For S3, update the import to `pkg/catalogs/storage/s3`. The package name stays
`s3`. The `Config`, `Backend`, and `New` selectors do not change.

## Replace catalogartifact selectors

Import `pkg/catalogs/artifact` as `artifact`. Change the qualifier from
`catalogartifact` to `artifact` for every public name:

```text
FormatVersion
MediaType
DescriptorMediaType
AttestationPredicateType
AttestationStatementType
Filename
AttestationFilename
ChecksumFilename
OCIMirrorArtifactType
OCIGenerationAnnotation
FileDescriptor
Descriptor
DigestSet
Subject
AttestationPredicate
AttestationStatement
Bundle
Build
Open
Inspect
Release
PublisherVerifier
VerifyRelease
ReleaseAssets
StageReleaseAssets
```

`Build` now accepts `catalogs.Generation`. `Open` and `VerifyRelease` now return
`catalogs.Generation`. The deterministic archive and detached statement hashes
do not change.

## Replace catalogremote selectors

Import `pkg/catalogs/remote` as `remote` when no package-name conflict exists.
Change the qualifier from `catalogremote` to `remote` for these names:

```text
CatalogPath
ManifestPath
GenerationsPath
EventStreamPath
CatalogPublishedEvent
ManifestMediaType
EventStreamMediaType
Publication
GenerationManifestPath
PayloadPath
Client
NewClient
ManifestETag
MarshalManifest
StreamEvent
EventStream
```

These exported methods keep their names:

```text
Client.FetchCurrent
Client.FetchCurrentIfChanged
Client.FetchGeneration
Client.OpenEventStream
EventStream.Next
EventStream.Close
```

The fetch methods now return `catalogs.Generation`. Wire routes, media types,
bounds, TLS policy, redirect policy, and SSE behavior do not change.

The top-level `github.com/agentstation/starmap/remote` package still owns the
reactive subscriber. Import the wire package as `protocol` when one file uses
both packages:

```go
import (
    protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
    "github.com/agentstation/starmap/remote"
)
```

Use `protocol.Client` for one-shot verified wire access. Use
`remote.Subscriber` for activation, retry, catch-up, durable state, and health.

## Verify the migration

Run the complete consumer suite after all imports and selectors compile:

```bash
GOWORK=off go test ./...
GOWORK=off go test -race ./...
```

For a remote deployment, test one restart while the publisher is unavailable.
For an artifact consumer, verify the pinned archive checksum again. No v0.4
data conversion or network protocol migration is necessary.
