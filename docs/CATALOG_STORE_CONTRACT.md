# Catalog Store Contract

`storage.Store` is Starmap's narrow durable-generation boundary. It lets an
embedding application own persistence without teaching Starmap about that
application's database, credentials, migrations, or lifecycle.

```go
type Store interface {
    Current(context.Context) (catalogs.Generation, error)
    Get(context.Context, string) (catalogs.Generation, error)
    Commit(context.Context, catalogs.Generation, string) error
}
```

Starmap provides memory, filesystem, and conditional object-storage adapters.
Starport may provide SQLite, MySQL, PostgreSQL, or other adapters. Starport owns
each concrete driver, connection pool, schema, migration, credential, backup,
close, and dialect-specific transaction concern.

## Ownership and lifecycle

- The caller constructs, configures, monitors, and closes its adapter and every
  resource behind it.
- `starmap.New` and `starmap.NewContext` never open, migrate, or close a
  caller-owned store.
- The caller must keep the store usable for the complete lifetime of every
  Starmap client that receives it.
- A constructor must not start hidden network work or a long-lived goroutine.
- Methods must honor cancellation and deadlines from the supplied context.
- Returned generations and accepted inputs are caller-owned. Implementations
  must defensively copy mutable payload and manifest slices.

## Payload limits

Canonical catalog payloads have a 32 MiB limit. Encoding and decoding enforce the same byte and JSON nesting limits.
The shared nesting limit remains 64 levels. A failed encoding returns no payload for publication.
Raw provider and auxiliary source JSON retain the default 16 MiB limit.

The complete serialized runtime layer remains limited to 64 MiB, including base64 payload bytes and receipt metadata.
Manual history retains its separate 64 MiB cumulative limit. The embedded bootstrap review budgets remain independent of the canonical payload limit.
A backend can impose a narrower limit. Its publication path must reject a generation that its configured reader cannot restore.

D26 records the 32 MiB engineering default. A recorded generation contains 23,683,266 bytes, which the prior writer accepted but the 16 MiB decoder rejected.
The owner preference remains pending. Publication and native qualification still require their existing gates.

Older readers with a 16 MiB limit cannot restore larger catalogs, even when the schema version matches.
The manifest compatibility range describes schemas, not reader capacity.
Before release, qualify the declared Starmap and Starport pair against the larger payloads.
A downgrade requires a retained catalog that the older reader accepts.

## Startup after provider binding removal

The connected runtime applies current provider bindings before it exposes a stored catalog.
After removal, its effective catalog, runtime-owned client, HTTP views, and durable current head must agree.
A failed publication prevents startup and preserves the accepted stored generation.

Without retained inputs or explicit provider bindings, stored scoped provider evidence requires a typed startup refusal.
An explicit empty binding set, selected with `runtime.WithProviderBindings()`, removes local scoped evidence through the normal baseline rebuild.
Unscoped store-only catalogs retain their existing startup behavior.
These checks belong to the connected runtime. The offline library still permits explicit caller-supplied store reads.

## Generation invariants

Every accepted generation must pass `Generation.Validate()` before any durable
state becomes visible. The manifest binds the exact payload size and digest,
schema version, generation ID, validation result, compatibility range, source
observations, and synchronization identity.

A generation ID is immutable:

- the store may persist valid content for an absent ID.
- the same ID plus unchanged manifest and payload bytes is an idempotent retry.
- the same ID plus different content returns a typed conflict. And
- no successful or failed operation may rewrite content already bound to an ID.

`Get(ctx, id)` returns the complete validated generation bound to `id`.
Committed historical generations remain addressable so rollback and audit do
not depend on reconstructing old content.

## Current and compare-and-swap

`Current` returns the complete generation named by the one current pointer. An
empty store returns a typed not-found error. It must never expose partial
generation data. Each current pointer must identify a generation that the store can read and validate.

`Commit(ctx, candidate, expectedGenerationID)` is one compare-and-swap
operation:

1. reject canceled context or invalid candidate before durable mutation.
2. reject a generation-ID collision with a typed conflict.
3. compare the actual current generation ID with `expectedGenerationID`.
4. persist the complete immutable candidate. And
5. atomically change current only if the comparison still holds.

An empty expected ID means the store must still have no current generation.
Concurrent commits from the same base produce exactly one winner. Every loser
returns a typed conflict containing the expected and observed current IDs.
Implementations must use their backend's native transaction, row/version
predicate, or conditional-write primitive. A read followed by an unconditional
write is not a valid implementation.

An identical retry after an ambiguous successful response returns success even
though the original expected ID no longer equals current.

### Filesystem publication failures

The filesystem store can publish the current pointer before its final directory flush fails.
It returns `*errors.PublicationError` with resource `catalog current generation` and the selected generation ID.
The wrapped error preserves the filesystem cause. The selected bytes remain readable, but the failed operation does not confirm durability.
An error alone cannot prove that the previous generation remains current.

Retry the same generation, complete content, and original expected ID.
The filesystem store verifies that retained content matches the candidate.
It synchronizes the generation directory, its parent, and the current directory before reporting success.
The retry preserves the current pointer and generation identity. A continuing flush failure returns another publication error.
Different content under the same ID returns a conflict.

Private-file publication uses the same error type with resource `private file` and the file name.
That record can be generation metadata. Its publication does not imply selection as the current catalog.
These durability operations run during explicit publication and recovery. Catalog lookups continue to use the active in-memory state.

## Independent current authority observations

`storage.AuthorityHeadReader` is an optional role for receipt issuers.
Its `CurrentAuthorityHead(ctx)` method returns a publication that was current between invocation and completion, including another writer's publication.
The root client's method has a different signature and returns only its cached publication.
A receipt issuer must start validity before the store read and qualify its clock separately.
Neither method creates a permission receipt.

Memory reads select the head under the publication lock without copying catalog payloads.
The filesystem adapter reads the current pointer and its immutable `authority.json` record.
This guarantee requires a local filesystem with the documented atomic publication semantics. Shared network filesystems remain unqualified.
The method loads no catalog manifest or payload. Missing metadata returns an error and causes no repair.

Authority generations store the record beside `manifest.json` and `catalog.json` before pointer promotion.
The version-1 record contains the complete authority head and no receipt timestamps.
Its 16 KiB limit and strict parser apply independently of catalog schema compatibility.
The head's generation ID must match the selected pointer. Unknown positive permission schema versions remain observable.
Consumers still refuse unsupported permission semantics.

An ordinary generation has no authority record. Selecting one prevents renewal of a previous authority receipt.
Existing authority generations without a record require an explicit identical commit that validates the complete stored generation.
The commit creates missing metadata before reporting success, including when that generation is already current.
An existing matching record is idempotent. A conflicting record causes refusal without replacement.

Filesystem repair uses the commit lock and exclusive private-file publication. Object repair uses an immutable conditional create.
Legacy store relocation preserves authority records and verifies their binding to each complete generation.
Relocation leaves missing legacy records absent until an explicit commit repairs them.

The object adapter requires its backend to implement `storage.CurrentObjectReader.GetCurrent` for the current pointer.
That method must observe a version current during its call. An ETag or conditional write alone does not establish this guarantee.
Backends without this capability retain ordinary catalog storage and refuse authority observations.
The memory object backend implements the guarantee under its publication lock.
The S3 adapter does not yet assert this capability for caller-selected endpoints and transports.

These reads belong to receipt acquisition. Starport admission remains an in-memory decision.
The library issuer described below is available.
Production clock adapters, server wiring, shared followers, and Starport consumer qualification remain separate CSP4 requirements.

## Library permission issuer

`permission.NewIssuer` selects one `storage.AuthorityHeadReader`, authority ID, policy ID, and clock callback.
Its constructor starts no I/O or background activity.
The caller must authorize that issuer and enforce durable publication order.
This constructor does not establish either guarantee for an arbitrary catalog store.

Each `ReadPermission(ctx)` call observes the current stored head.
The receipt interval starts at the earliest qualified clock time before that read.
Storage delay and issuer uncertainty consume the interval. Consumers also account for their own uncertainty.

A zero lifetime selects five minutes. Explicit positive lifetimes cannot exceed that maximum.
Unknown clock validity or uncertainty outside zero through 30 seconds refuses issuance.

`ClockReading` binds the returned time to cached qualification evidence for that time.
The callback must support concurrent calls and account for clock corrections, suspend, restart, and expired evidence.
Returning `time.Now()` with an assumed uncertainty does not qualify a clock.

`runtime.WithPermissionClock` supplies a complete sample for each admission check, receipt relay, and permission status report.
Each check uses the time and uncertainty from one callback result. The callback reads cached evidence and supports concurrent calls.
The scheduler retains its own clock. Combining the complete sample with `WithPermissionClockUncertainty` causes a configuration error.

The legacy uncertainty callback remains available when it qualifies the time from `WithClock`. Neither option provides native qualification.

`permission.NewClockCache` separates explicit clock observations from cached permission checks.
Its constructor starts no observation. `Refresh(ctx)` serializes observations with a finite deadline.
`Read()` uses one immutable sample and the configured elapsed counter. It does not consult the time service.

The host qualifies the native source, the counter, and their error bounds.
The counter must include system sleep. The cache includes query delay, counter error, and bounded rate drift in its uncertainty.
Maximum age cannot exceed five minutes. An uncertainty above 30 seconds or an exhausted age bound refuses the sample.

Failed observations, counter regressions, and explicit invalidation clear the evidence.
An unfinished observation cannot undo concurrent invalidation. A caller canceled before observation preserves existing evidence.
A new process starts without qualified evidence. The host must schedule refresh and invalidate known native failures.
These APIs do not select or qualify a production native clock adapter.

The issuer rejects changed authority identity, sequence rollback, and conflicting heads at the same sequence.
Concurrent observations cannot replace a newer observed head with an older reply.
A clock failure after a valid head read still retains that requirement in process memory.
This process memory does not establish durable replay protection across issuer restart.

Storage failure returns an error without renewing the previous receipt.
A bounded clock correction can preserve an unchanged, still-valid receipt with its original expiry.
The issuer can report a newer permission schema from independent metadata when catalog data is unreadable.
That receipt grants no permission to consumers that cannot enforce the reported schema.

## Authority publication ordering

`permission.NewPublisher` wraps a caller-selected store for one fixed authority and policy.
Its constructor starts no I/O. It implements `storage.Store` and the optional current-head role.
The underlying store must supply the current-read guarantee before the wrapper can return an authority observation.

Every authority writer must use this publication contract.
The caller authorizes the publisher and supplies a complete authority generation with the correct required permission revision.
This wrapper validates publication order. `permission.PrepareGeneration` derives a revision for a complete permitted catalog.
The origin must select and apply its catalog policy before preparation.
Direct underlying writes, deleted state, and restored older backups need separate recovery procedures.

`Commit` checks the stored predecessor before the final atomic compare-and-swap.
A process restart does not clear this predecessor. A delayed writer cannot overwrite a newer accepted generation.
Changed authority identity, sequence rollback, and conflicting content under an existing sequence cause refusal.
An exact retry still succeeds. The underlying immutable store rejects changed manifest or payload bytes under the same generation ID.

The first authority can populate an empty store.
An existing ordinary catalog requires an explicit `Bootstrap` call with its exact predecessor ID.
Bootstrap cannot replace an established authority. An identical bootstrap retry remains valid after an ambiguous successful response.
A missing predecessor read cannot reset an existing sequence through an empty expectation.

The publisher refuses candidates or predecessors whose mandatory permission semantics it cannot enforce.
Its independent head read still exposes unknown positive permission versions to receipt readers.
Consumers must refuse those semantics. The capability does not authorize publication under an unsupported policy.

## Authority generation construction

`permission.PrepareGeneration` accepts an ordinary generation and explicit authority, policy, and positive sequence values.
It verifies the complete catalog, including schema agreement and accepted membership evidence, before constructing the authority manifest.
The input catalog must already represent the policy's complete permitted catalog. Gateway account grants and budgets remain separate contracts.
An authority generation cannot enter this path. Subscribers and relays preserve its original authority identity instead.

The required permission revision binds authority, policy, permission schema, and the catalog's semantic checksum.
It covers membership, scoped removal policy, canonical identity, alias state, and serving facts.
Provenance and manifest observation metadata do not change this revision. Catalog scope evidence remains part of it.
This conservative contract also changes the revision when other catalog facts change. An incompatible replica must then block new attempts.

The authority generation ID separately binds the complete source manifest, selected identity, sequence, and required revision.
Exact preparation retries return identical bytes. New sequences or changed source evidence receive a different generation ID.
The result preserves exact payload bytes and copies mutable data. Neither preparation nor its hashes run during inference admission.

Preparation starts no I/O. The caller still selects the durable predecessor and uses `Publisher` for atomic publication.

Elapsed publication time cannot expire a canonical alias. Explicit operator or baseline removal changes its retained state and the required revision.
The preparation library does not wire an origin server, authorize publishers, qualify clocks, or select a shared storage service.

`Publisher.PublishCatalog` selects the next sequence from the stored authority and combines preparation with atomic publication.
It verifies an ordinary input before reading storage. An empty store starts at sequence one.
An exact retry derives the current generation identity and verifies immutable equality through the underlying store.
Changed proposals must name the exact predecessor. Concurrent writers cannot both replace that predecessor with different generations.

`Publisher.BootstrapCatalog` explicitly adopts an ordinary store. It cannot reset an established authority.
Unknown permission semantics, unreadable state, changed authority identity, and an exhausted sequence cause refusal.
Publication never retries a stale proposal automatically. A successful call returns the complete generation for activation in the serving client.

`Publisher.PrepareCatalog` selects the final generation without writing it. A later `Commit` or `Bootstrap` retains the same predecessor expectation.
Preparation reserves no sequence. A competing publication can make the final commit conflict.

## Runtime origin transactions

`runtime.WithAuthorityOrigin` explicitly selects an origin identity, its publication store, and a qualified clock callback.
The complete runtime catalog defines the permitted catalog. The deployment controls access to every mutation API and the underlying store.
An origin cannot also use authoritative-subscriber startup. Incoming authoritative generations must retain their original identity through the subscriber path.

The origin option selects the client's store regardless of the order of `WithClientOptions`.
Other client options still configure the workspace and embedded bootstrap limits. All origin commits pass through the authority publisher and runtime publication guard.
The runtime rejects direct mutation through its exposed client. Its acquisition and input transactions own publication instead.

`Client.PrepareGeneration` encodes the candidate with exact evidence, times, and sync-run identity without writing storage.
The runtime derives the authority sequence and final generation before it stages the retained-input journal.
That journal binds the preceding catalog and the proposed authority identity. The runtime commits and activates the same prepared bytes.
A lost commit reply leaves recovery evidence. On restart, the accepted store identity decides whether to apply or discard the staged inputs.

An unchanged reconstruction proves its source identity against the accepted authority generation digest and preserves the existing sequence.
An existing ordinary store requires explicit `OriginConfig.Bootstrap`. Restart cannot use that permission to reset an established authority.
A different authority or unsupported permission schema prevents startup. The current runtime origin requires the publication lease during startup.

`Runtime.ReadPermission` issues origin receipts from the current durable authority head.
It can report a committed revision whose activation reply failed. A subscriber must enforce that revision before admitting inference.
The clock callback must return qualified time evidence. Unknown clock validity prevents receipts while catalog diagnostics remain available.

This API supplies origin runtime composition. Canonical CLI settings, native clock qualification, and shared-store follower activation still require implementation and evidence.

## Failure preservation and rollback

A refusal before current-pointer publication leaves the previous current generation complete and readable.
A failure after publication can report an ambiguous outcome, including a failed directory flush after rename.
The current filesystem implementation does not yet classify this boundary with a dedicated outcome error.
CSP5 owns the explicit error contract and recovery qualification. Callers must not infer pointer rollback from an arbitrary I/O error.

An implementation may retain a complete immutable addressed candidate when a
failure occurs after candidate persistence but before current-pointer
promotion. It must never expose partial content through `Get`, and retrying the
same candidate must remain safe. Each implementation owns cleanup of
unreferenced immutable generations. That maintenance policy is not part of
`Commit`.

Rollback uses compare-and-swap to promote a retained prior generation:

```go
prior, err := store.Get(ctx, priorID)
if err != nil {
    return err
}
if err := store.Commit(ctx, prior, currentID); err != nil {
    return err
}
```

No special mutable rollback channel exists.

## Error contract

Callers must be able to classify:

- missing current or addressed generation with `errors.IsNotFound`.
- stale compare-and-swap or immutable-ID collision with
  `errors.IsConflict` / `*errors.ConflictError`.
- invalid manifest, payload, identity, or checksum with
  `errors.IsInvalidInput`. And
- cancellation or deadline with the standard `context` errors.

Backend wrappers may add operation and resource context. Returned errors and
logs must not contain reusable credentials, connection strings, query
arguments, payload bodies, or other secrets.

## External adapter verification

The verification command compiles the `testdata/consumers/store-only` module
with `GOWORK=off`. The module defines a Starport-owned adapter outside Starmap's
packages and proves the interface at compile time. It injects the adapter through
`WithCatalogStore` and publishes a real generation. It tests ownership,
validation, idempotency, retained history, conflict, rollback, failure preservation,
and cancellation.
Its dependency gate rejects CLI, server, acquisition, SQLite, MySQL, and
PostgreSQL implementations.

Starmap deliberately does not export a `testing.T`-based conformance helper.
The interface and executable external module are the stable product surface.
Backend-specific transaction, corruption, and fault injection remain local to
each adapter. Reconsider a public helper only if multiple external repositories
need a shared deep testing module. This behavioral contract must be unable to
express that module.

## S3-compatible object storage

`pkg/catalogs/storage/s3` is Starmap's optional production
`storage.ObjectBackend`. It accepts an already-configured, caller-owned AWS
SDK v2 S3 client and sends no network request during construction. Compose
it with the catalog store:

```go
backend, err := s3store.New(callerOwnedS3Client, s3store.Config{
    Bucket: "starmap-catalogs",
})
if err != nil {
    return err
}
store, err := storage.NewObject(backend, "production")
```

The selected S3-compatible service must implement conditional `PutObject`
writes. Immutable generation objects use `If-None-Match: *`. The current
pointer uses `If-Match` with the exact opaque ETag returned by the prior read.
The adapter rejects unconditional writes, requires ETags, bounds object bodies,
and maps 409/412 precondition failures to typed conflicts. A service that
rejects conditional writes fails the operation explicitly. A service that
silently ignores those standard headers is not S3-compatible for this contract.

Do not select such a service.

Starmap never discovers credentials, constructs a default AWS configuration,
opens a network connection in `New`, or closes the client. The embedding
deployment owns those concerns and may configure the AWS client with a
caller-selected S3-compatible `BaseEndpoint`. See AWS's
[conditional-write contract](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html)
and the AWS SDK v2
[endpoint configuration guide](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-endpoints.html).


## Filesystem access policy

The filesystem catalog store contains private application state. Public YAML exports use a separate access policy.
`NewFilesystem` resolves its selected path without reading or creating the store.
Store operations validate existing entries and create missing directories and files with private access.

On Linux and macOS, store directories require effective-user ownership and no group or other access.
New directories use `0700`, and new regular files use `0600`, subject to the process umask.
Ancestor directories may permit shared read and search access. Writable ancestors require the trusted sticky-directory policy described in [CLI access rules](CLI.md#ancestor-access-on-linux-and-macos).

The native macOS ACL check rejects non-owner grants on private store entries.
The Windows adapter uses protected private ACL creation and validates allowed principals through the shared private-file implementation.
Native Windows execution and ancestor qualification remain pending. Cross-compilation does not establish those guarantees.

Existing public store permissions cause refusal. Starmap does not change permissions or ownership automatically.
This requirement also applies before migration reads a legacy filesystem store.
Operators must inspect the selected store and correct its access under the intended service identity before retrying.
Retained catalog generations and file contents remain unchanged by access refusal.

Publication validates staged bytes before installing an immutable generation. The current pointer changes through its original private directory binding.
Cancellation or an access conflict before publication preserves the previous current pointer.
These filesystem checks run during store access, outside the active in-memory catalog lookup path.

## Retained provider evidence

The connected runtime stores provider observations separately from the accepted catalog head.
Unscoped evidence uses `<state-directory>/catalog-runtime/providers/<provider-id>.json`.
Scoped evidence uses `<state-directory>/catalog-runtime/providers/bindings/<key-digest>.json`.
The runtime validates these records during retention and restart. It does not silently discard an invalid record.

A custom `runtime.Acquirer` must supply these `ProviderLayer` fields:

| Field | Required value |
| --- | --- |
| `ProviderID` | The single canonical provider identifier contained in the payload. An alias does not substitute for this identity. |
| `Payload` | A bounded, decodable provider observation with exactly one provider. |
| `Digest` | The checksum returned by `catalogs.DescribeCatalogPayload(payload).Checksum`. |
| `ObservedAt` | A nonzero observation time. |
| `Receipt` | Validated source identity, revision, completeness, status, record counts, and classified issues bound to the payload. |

Construct a layer from the source observation:

```go
layer, err := runtime.NewProviderLayer(providerID, observation)
```

The constructor validates the observation and retains its receipt without original diagnostic messages.

Restoration uses stable issue codes as diagnostics.
It verifies the observation identity against the supplied catalog through `ObservationReceipt.Restore`.

The runtime copies payload bytes and validates the complete supplied batch before it retains any member.
Caller mutation after retention therefore cannot alter retained payload bytes or issue records.
The 64 MiB limit applies to the complete serialized layer, including base64 payload bytes and receipt metadata.
An oversized batch member causes refusal before any batch write.
This guarantee does not make multiple durable writes transactional after an I/O failure.

Restart checks the directory and filename against the record's provider and binding revision.
Missing receipts, inconsistent payloads, missing observation times, and mismatched identities cause refusal.
Receipt validation also checks completeness, status, record counts, issue metadata, and the observation identity.

Legacy provider records without receipts require explicit recovery from verified source evidence. Do not invent a complete receipt for old bytes.
Preserve the rejected files and restore verified evidence or reacquire through the configured source.
Do not manufacture a checksum for bytes whose origin is unknown.

Retention selects the newest observation for each provider scope revision in a supplied batch, independent of input order.
It rejects a batch if its selected observation predates retained evidence or if different payloads share an observation time.
These conflicts cause no batch writes. An identical observation at the retained time requires no rewrite.
Different receipt identities at the same time conflict even when their payload bytes match.

Concurrent retention calls serialize observation selection and provider writes. Restart restores the retained observation time before subsequent comparisons.

Retention checks cancellation before preparation, after waiting for its write lock, and before each durable record publication.
Cancellation does not undo an earlier record that already committed.

These checks bind evidence bytes, provider identity, and observation order.
They do not authenticate a timestamp or establish a permitted clock-skew bound.
A partial receipt makes acquisition health degraded even when its provider request succeeds.
Receipt completeness remains a source assertion.

Account scope, field authority, and authorized deletion remain separate reconciliation requirements.
The receipt does not prove upstream coverage.

Unscoped source observations use `observation:v2:<sha256>` identities.
The identity encodes each field with its byte length and includes record counts and the issue count.
Opaque values retain their exact bytes. Diagnostic messages remain excluded.

Legacy observation IDs remain readable when their identity fields contain no NUL separators and their original hash matches.
Ambiguous legacy records require verified source evidence before recovery. The runtime does not silently upgrade their identity.

A binary that validates only legacy observation IDs cannot validate a v2 receipt.
CSP18 must qualify downgrade and recovery procedures before release. Do not assume that legacy read support also permits downgrade.


The provider source pins resolved credentials within each observation run.
Preflight and the provider request share that run's memo. Concurrent runs cannot replace its selected credential material.
The process resolver still owns credential caching and renewal policy.
A credential profile or material version does not establish account scope. Named scope bindings and their retained-evidence policy remain separate reconciliation requirements.


### Provider acquisition bindings

A provider observation can carry a typed `sources.ProviderAcquisitionBinding` in its metadata and receipt.
The current contract uses schema 2. Schema 1 remains readable without membership replacement authority.

| Field | Contract |
| --- | --- |
| `schema_version` | `2` for new bindings. Legacy `1` remains readable. |
| `id`, `revision` | Stable deployment-owned identity and declared scope revision. |
| `provider_id` | The single canonical provider contained in the observation. |
| `public` | An explicit public scope, without account or project selectors. |
| `account_id`, `project_id` | At least one is required when `public` is false. Both can apply to a private scope. |
| `region`, `api_surface` | Required selectors, including an explicit region such as `global`. |
| `membership_authority` | Empty for positive evidence only, `scope` for scope replacement, or `provider` for public provider replacement. Requires schema 2. |
| `credential_role` | Exactly `catalog_acquisition`. |
| `credential_profile_id` | The declared authentication profile. This field contains no resolved credential values. |

Each identifier or selector permits at most 4,096 UTF-8 bytes, without control characters or surrounding whitespace.
JSON decoding rejects unknown fields, unsupported schemas, and invalid bindings. Errors identify fields without echoing their values.
A binding requires the provider source and exactly one matching provider. It does not authenticate the account or verify the declared credential profile.

Schema-2 bindings produce `observation:v4:<sha256>` identities, including membership authority.
Schema-1 bindings retain their original v3 identities. Unscoped observations retain v2 identities.
Changing or removing a binding invalidates its receipt. The runtime never rewrites an old receipt into the new format.

The constructor, receipt, clone, and restore operations copy the binding. Retention and restart reject altered evidence.

The runtime retains separate records for each provider, binding identity, and binding revision.
Configured bindings apply to manual and scheduled acquisition. A missing binding does not declare public scope or permit deletion.
Changing selectors or membership authority requires a new binding revision.

### Scope replacement and transport

A complete successful observation can replace scoped availability evidence only when its binding explicitly permits inventory replacement.
Partial, failed, fallback, and unscoped observations cannot establish new absence.
An empty complete inventory means known absence within its authorized scope. A missing inventory means unknown membership.

`scope` replacement affects only the declared account, project, region, and API surface.
`provider` authority requires a public binding and records the provider public inventory.
Both forms preserve visible catalog offerings and definitions when a provider stops reporting a model.
Independent accounts retain their own availability evidence. Inventory replacement does not authorize catalog deletion.

Catalog schema 7 transports effective `membership_scopes` and references their original provider observation receipts in the generation manifest.
Each record names its publisher, binding ID, binding revision, provider, scope selectors, and membership authority.
The last complete inventory and later positive observations retain their original receipt IDs and times.
Credential references and resolved values do not appear in these scope records.

`DecodeCatalogGeneration` verifies the manifest, payload, schema agreement, and referenced receipts before activation.
The current decoder accepts schemas 6 through 9. Schema 6 has no scope records. Schema 7 adds effective scopes. Consumers reject schemas beyond their declared support.
Legacy receipts keep their original payload bytes and identity during restore. Older binaries cannot read the new format merely because the new decoder reads old formats.

The selected trusted source supplies the authority for imported publisher claims.
A publisher ID is not an authentication credential. Upstream records cannot claim the local runtime publisher ID or its configured aliases.
Derivative publication and restart preserve accepted upstream IDs and original receipts.

`acquisition.ImportRelease` merges verified artifacts with the current catalog.
It preserves independent scopes and their original receipts, including imports that change only scope records.
Repeated identical imports do not publish another generation.
Different inventories for the same publisher and binding revision cause a conflict before publication.
This merge cannot select a replacement scope authority. Use a configured catalog source or explicit trusted activation for replacement.

Starport must resolve an explicit inference-profile link before applying account-specific restrictions.
That link must bind the selected catalog authority, publisher, binding ID, and revision. An authority change requires link revalidation.
Starport integration remains incomplete under `CSP8` and `CSP10`.


Complete accepted scoped inventories retain their original receipts through the acquisition volume guard.
Unscoped volume regressions retain the existing degraded classification. Failed and incomplete evidence cannot establish new absence.

Starport must show observed absence and exclude the affected provider/account from automatic routing.
Explicit operator removal defaults to that entry. Global canonical removal remains a separate action.
A replacement Starmap baseline can remove entries it no longer contains. Internal authoritative permission withdrawals retain immediate enforcement.

Starmap retains canonical rename history in schema 9. Starport discovery and routing integration remain incomplete under `CSP8` and `CSP10`.

Old canonical aliases must remain valid until explicit operator or replacement baseline removal. Elapsed time cannot remove an alias.

### Operator removal policy

Catalog schema 8 adds `removal_policies`. Each snapshot names the operator publisher and its explicit targets.
A scoped target names one provider entry and its stable account or public scope. A canonical target requires a separate explicit selection.

Schema 9 adds alias targets that name one old canonical ID. These use the same removal snapshot and publication journal.
Credential rotation preserves a stable account target. Reassigning a binding to another account cannot transfer that target.

`Runtime.ReplaceRemovalTargets` replaces only the local operator snapshot. It requires the expected generation ID and payload checksum from `Runtime.State`.
An empty target list explicitly restores local removals. Other publishers retain their own policies.
Provider refresh cannot clear a local removal. Catalog facts remain available for reconstruction and operator diagnostics.

Identity lookup does not enforce local operator policy. Consumers check `ContainsAlias` before serving a request through that alias.

The runtime stores the accepted snapshot in `R/catalog-runtime/removals.json` with the private-file policy.
Publication journal version 3 binds its immutable input to the resulting catalog generation. Readers retain support for journal versions 1 and 2.
Recovery follows the accepted catalog commit before loading retained inputs. Missing or changed retention cannot silently restore an accepted removal.
Backups and directory migrations must preserve the removal snapshot and any pending publication inputs.

`Catalog.Removals` exposes immutable scoped, canonical, and alias queries. Consumers validate publisher authority and account links before applying these results.
Acquisition observations cannot contain operator policy. Generic catalog merges preserve targets when incoming data omits them.
A trusted replacement and an explicit operator restore remain separate operations. A schema-7 reader rejects schema-8 policy payloads.

An accepted upstream policy requires the original generation manifest. A legacy replacement format cannot clear that policy.
Upstream publishers cannot claim the runtime's local identity or its configured aliases.
Ordinary release imports preserve current operator policy and reject incoming policy. Configured catalog sources carry accepted upstream policy through reconciliation and restart.

Starport discovery, routing, and operator UI integration remain incomplete. This component does not qualify A08 or the released product pair.


### Acquisition provider configuration

Manual and scheduled provider acquisition use the accepted catalog's provider definitions.
An accepted upstream baseline can supply a custom provider without a local YAML workspace.
An explicit local workspace can override provider configuration or add a provider, subject to the runtime's acquisition policy.
An embedded provider outside the accepted registry cannot bypass that registry during manual provider acquisition.

A metadata-only source filter can still name an embedded provider. That filter does not enable a provider client or resolve its credentials.

Provider selection does not resolve credentials. The selected acquisition profile and binding checks govern later credential resolution.
These changes do not grant inference access or change credential-plane precedence.
Without a local workspace, acquisition reuses the immutable accepted provider catalog.

### Explicit acquisition binding calls

`Acquirer.ObserveProviderBinding` observes one binding without storing or publishing the result.
The default observer checks the declared provider and acquisition profile before credential resolution. It restricts the resolver to that profile.
A different resolved profile causes refusal before client creation. Each concurrent call owns its resolver selection and observation memo.

A custom observer must implement `ProviderBindingObserver`. An unscoped observer cannot serve a bound call.
A returned layer must carry the requested binding in its receipt. Runtime validation and scope policy still govern activation.

Account, project, region, and API-surface selectors remain deployment declarations. These checks do not prove upstream account ownership or coverage.
The runtime now accepts an explicit active binding set. The built-in batch acquirer still needs binding integration under CSP3.

### Scope storage and revision consistency

The scoped filename hashes byte-length fields for the provider, binding identity, and revision. It does not contain raw selectors.
The separate `bindings` directory prevents collisions with unscoped provider filenames. Both directories use the private-file access policy.
Restart validates each record's receipt, directory, and filename before accepting the retained set.
Scoped records in the former provider-only location require explicit migration or verified reacquisition under CSP18.

A binding identity and revision describe one deployment declaration across providers.
Changing any declared selector or profile under that same identity and revision causes refusal before batch writes.
The same check applies to the complete retained set during restart. Ordering and duplicate checks apply independently within each provider scope revision.

An explicit active binding set selects permitted revisions before reconstruction.
Configuration integration, internal source authority, and scoped deletion remain open.

### Active provider declarations

`runtime.WithProviderBindings` supplies the complete active set for local provider evidence.
Each binding identity has exactly one declaration. The runtime copies the input and rejects duplicate identities, including identical duplicates.
An empty set permits no local provider evidence. A changed set requires a new runtime.

An explicit set excludes unscoped evidence, inactive revisions, and declarations with different selectors.
Startup checks retained declarations for revision reuse. It preserves inactive files but excludes them from reconstruction and acquisition freshness.
Publication rejects an inactive batch before writes. Earlier successful refresh windows remain visible if a later batch fails.

Startup always reconstructs when an explicit set exists, even with no retained files or no active observations.
Before returning, it aligns the runtime, underlying client, and current store generation. Failed publication prevents startup and releases the directory.
Without a supplied store, this mode uses a writable memory store. Caller-supplied stores take precedence.

The selected generation identity hashes the complete declarations, original source identity, and resulting payload checksum.
Declaration order does not change the identity. An explicitly restored selection can reactivate its unchanged retained generation.
These identities do not replace source authentication, permission policy, or provider-account verification.

Explicit sets require the `runtime.BindingAcquirer` role. The runtime passes only selected declarations to `AcquireProviderBindings`.
An empty set makes no provider calls. The built-in batch acquirer does not yet implement this role and fails before provider I/O in this mode.
The existing single-binding observation API remains available without publication.

Without this option, legacy unscoped behavior remains. Scoped publication requires an explicit declaration.
Operators do not yet have a qualified configuration path for this mode. CLI settings, batch acquisition, and configuration-omission guards remain unfinished.
CSP4 still owns internal authority and accepted-head admission. The option must remain present across restarts to enforce its local policy.

A separately injected server `Syncer` can still mutate the underlying client directly. Operator integration must route server mutation through the active runtime policy.

### Provider refresh windows

A refresh window tracks published evidence by provider, binding revision, and validated observation identity.
An early result does not suppress another scope, another revision, or a newer observation in the final result.
The runtime validates final receipts before duplicate checks. A modified receipt cannot bypass validation by reusing an earlier observation identity.
Repeated valid evidence requires no second final publication. Earlier successful windows remain published if a later result fails validation.

The aggregate `AcquisitionReport.Retained` names each provider with at least one unanswered retained scope.
One successful scope does not hide another scope's retained evidence. Binding-level attempt reports and active revision policy remain open under CSP3.

## Embedded baseline and accepted current state

`Client.EmbeddedCatalogState` returns the verified immutable catalog compiled into the module.
Its identity, checksum, and timestamp remain independent of application storage, workspace input, and later client updates.
The getter uses the catalog already verified during construction. It reads no storage and decodes no payload.
`EmbeddedGeneration` remains the constructor-free accessor for a complete manifest and payload.

The connected runtime keeps this compiled baseline separately from the accepted current state.
When retained inputs exist, reconstruction starts from the compiled baseline and applies the retained source, provider, and manual observations.
With an explicit binding set, startup reconstructs and publishes the selected state before returning.
Without that option, an empty retained set preserves the accepted current state under legacy startup behavior.
That legacy path still needs active authority and configuration-omission checks before operator qualification.

Reconstruction treats baseline generation IDs as opaque values. It does not remove a `.local.` substring to infer an earlier identity.
The same immutable inputs produce the same derived identity across restarts.

The regression excludes provider evidence at the reconstruction boundary after reopening a real filesystem catalog store.
The excluded provider no longer returns through the baseline. This check does not exercise an operator revocation API or active binding policy.
Explicit local inputs still need source evidence when the runtime reconstructs a generation. A previously merged head cannot substitute for that evidence.


## Interrupted baseline export

Application startup recovers interrupted baseline exports before it creates or verifies the installed baseline.
The private `<baseline>/.starmap-baseline/` directory holds `.owner.lock` and one `<operation-id>.json` journal per unfinished export.
The published baseline directory still contains only `manifest.json` and `catalog.json`.

Export and recovery hold the same filesystem lock. Journals bind the original lock, stage, and file identities.
Recovery verifies file modes, modification times, and content digests before deleting a recorded stage.
A durable cleanup phase permits recovery after partial deletion. Recovery never deletes a published baseline.

A replaced lock cannot adopt earlier journals. Changed, unrecorded, or unsupported recovery files remain in place.

The scan refuses more than 4,096 combined entries in the baseline and recovery directories before deleting any stage.
Retained content snapshots have a 64 MiB aggregate verification budget. Stability checks can reread those bytes.
Stages beyond that budget remain in place. These recovery limits do not limit catalog generation size.
The exporter reports completed recovery operations and relative paths that need separate ownership review.

Use `starmap config paths --inspect` to locate recovery files and their owner-only access policy.
Keep the writer lock for the lifetime of the baseline directory. Preserve journals with their stages until verified recovery completes.

## Provider reset retention

`Runtime.UpdateObservations` accepts optional `ProviderObservationReset` scopes.
Each scope names a canonical provider and, for scoped acquisition, a binding identity and revision.
Empty binding fields select legacy unscoped observations. Each scope requires complete successful replacement evidence.
Failed preparation, cancellation, or rejected publication preserves the accepted reset history.

The runtime journals reset scopes with replacement observations and their accepted generation.
Reset operations change generation identity even when payload bytes remain equal. Recovery can therefore resolve a lost commit reply.
Earlier scheduled provider files enter a separate history batch before the first reset. They cannot restore cleared facts during restart.

Replay selects records within original observations. It preserves the baseline, unrelated providers, peer binding scopes, and original receipts.
Unchanged projected fields from a cleared observation cannot restore its provider facts. Actual operator edits keep local field authority.
General source resets and complete projection membership rules remain separate work.

Manual heads and batches now use version 2. Readers still accept version 1 records without resets.
Version 1 batches cannot contain reset scopes. New head versions cause older readers to refuse the history.
Recovery validates reset scopes and matching replacement receipts before applying any retained inputs.

The component limits a history to 4,096 batches and 64 MiB of encoded observations and reset scopes.
One reset request permits at most 4,096 scopes. Production compaction, native format qualification, and CLI/HTTP integration remain open.
