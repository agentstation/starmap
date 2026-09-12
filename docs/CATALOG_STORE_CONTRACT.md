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

Filesystem reads enforce 32 MiB for catalog payloads, 64 MiB for manifests, and 16 KiB for current pointers and authority records.
The current pointer limit includes its trailing newline. Oversized records cause a typed validation error before file bytes enter memory.
Filesystem commits reject oversized payloads, manifests, and pointers before creating generation state or changing current.
These filesystem limits do not configure other storage adapters.

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

`Runtime.UpdateObservations` accepts optional `ObservationReset` scopes.
Provider scopes name a provider and, for scoped acquisition, a binding identity and revision.
Metadata scopes select a models.dev source and optionally one original provider identity.
Empty binding fields select legacy unscoped observations. Each scope requires complete successful replacement evidence.
Failed preparation, cancellation, or rejected publication preserves the accepted reset history.

The runtime journals reset scopes with replacement observations and their accepted generation.
Reset operations change generation identity even when payload bytes remain equal. Recovery can therefore resolve a lost commit reply.
Earlier scheduled provider files enter a separate history batch before the first reset. They cannot restore cleared facts during restart.

Replay selects records within original observations. It preserves the baseline, unrelated providers, peer binding scopes, and original receipts.
Unchanged projected fields from a cleared observation cannot restore its provider facts. Actual operator edits keep local field authority.
Replay also preserves exclusions for metadata resets and their original provider aliases.

Manual heads and batches use version 4. Readers also accept versions 1, 2, and 3.
Version 1 batches cannot contain resets. Version 2 batches permit provider resets, and version 3 also permits metadata resets.
Version 4 adds history checkpoints. New head versions cause older readers to refuse the history.
Recovery validates reset scopes and matching replacement receipts before applying any retained inputs.

A history permits at most 4,096 linked records and 64 MiB of retained encoded data.
A checkpoint occupies one linked record and retains at most 65,536 ordered observation references.
One reset request permits at most 4,096 scopes. Complete compaction and native format qualification remain open under CSP5.

## Interrupted runtime migration

Runtime migration keeps `stage-initialization.json` in its operation journal while it prepares a private stage.
The record binds the manifest digest, parent and stage identities, and the journal writer lock.
Restart verifies those bindings and resumes the same `.migration-build-<id>` directory.
Unknown entries, changed intent, replacement directories, and replaced locks cause refusal without recursive cleanup.

A published stage keeps its files. Recovery removes its initialization record after directory synchronization.
Stages without a valid ownership record remain preserved for explicit recovery.

Each `.migration-work/<target-path-sha256>.partial` file has an immutable sibling `.partial.json` ownership record, limited to 16 KiB.
That record binds the stage, work directory, writer lock, file identity, mode, and source manifest entry.
Before removal, recovery verifies that the mutable partial bytes remain an exact prefix of the original source file.

A missing record, changed identity, mode, or non-prefix content causes refusal. Recovery preserves both files.

An interrupted partial removal can resume from its retained record. Records from unsupported versions remain preserved.

Source inventory permits at most 40,000 entries, including empty directories and metadata.
The stage scan uses the exact allowed manifest layout as its entry limit. Both scans read at most 128 directory entries per batch.
Aggregate path names must fit within 4 MiB. The existing limit of 10,000 source files still applies.

Initialization permits only its two metadata files and empty work directory. It reads at most four entries before refusing an oversized layout.

### Workspace candidate cleanup

Projection and repair retain the candidate's native directory and file identities in memory.
Cleanup checks those identities, content digests, access metadata, and the complete remaining inventory before deletion.
An unknown entry, changed file, replacement file, or replacement directory causes a conflict and preserves the tree.
A same-content file replacement still has a different identity and remains preserved.

After a native directory exchange, cleanup can remove only the recorded old workspace at the candidate path.
Cancellation does not skip cleanup. Cleanup uses a separate 30-second limit and returns any failure with the original operation error.
Cleanup cannot reverse an accepted catalog or remove the published workspace.

Repair publishes its validated candidate without repeating the render and validation passes.
These in-memory inventories do not provide recovery after process exit. Persistent staging recovery remains incomplete.

### Workspace preparation writes

`Builder.WriteYAML` emits catalog records and logo sidecars through a callback without filesystem access.
Workspace preparation owns those writes and records each created entry's native identity, access metadata, and exact written bytes.
It copies operator files after generating managed catalog records. It does not remove and rebuild copied model directories.
Partial writes remain identifiable when serialization, copying, or cancellation interrupts preparation.

Verification uses the same writer and checked cleanup. Unexpected entries and changed files remain preserved.
The enclosure stays private. Candidate assembly retains the selected workspace access policy and synchronizes the completed candidate before publication.
The writer checks resource limits before creating another entry. Assembly reads only the expected number of children, in bounded batches.

Before publication, assembly compares the finished tree with the identities and bytes recorded during writes.
It refuses a replacement file even when the file contains identical bytes.

Ownership records remain in process memory. Persistent recovery after process exit still requires a separate journal.

### Legacy migration rollback

Preflight reads at most four entries from each fixed-layout directory. The layout permits at most three entries.
A fourth entry makes the layout invalid.
Retained generation scans use batches of 128 entries, with cancellation checks before each batch and generation.

Preflight validates every retained generation and reports the complete count. It does not discard history to meet a directory-read limit.
Manifest reads use the filesystem adapter's 64 MiB limit before parsing. Preflight failures preserve existing files and do not create the destination parent.

Legacy layout migration records the original store's native directory identity before relocation.
It holds both parent directories open and checks that identity before moving or restoring the store.
Neither move replaces an existing destination, including an empty directory.

Rollback checks the projected workspace against the candidate's recorded file identities, bytes, and access metadata.
A matching semantic catalog checksum alone does not authorize removal. Unknown or changed files and replacement directories remain preserved.
Rollback uses a separate 30-second context after caller cancellation and returns conflicts with the original operation error.

Rollback does not delete projection-marker paths. It preserves the stable writer-lock file after releasing the lock.
Operators must inspect an invalid marker or an unexpected blocking directory before removal.
The recorded store identity and candidate inventory remain in memory. They do not establish migration recovery after process exit.

### Workspace marker and journal records

Projection markers and replacement journals share a bounded record writer. Both record formats use the existing 4 MiB journal limit, including the trailing newline.
This limit differs from the filesystem catalog store's manifest and pointer limits.

The writer records each temporary file's native identity, access metadata, and written bytes. It retains the open file until cleanup completes.
Before publication, it checks both the temporary record and the destination snapshot. A journal cannot replace an existing destination.
Marker replacement requires the destination to match its earlier snapshot.

Cleanup removes only unchanged temporary files that match the recorded identity, access, and bytes.
It preserves operator changes, replacement files, and paths recreated after publication. Partial writes record their actual byte count before cleanup.
Cancellation uses a separate 30-second cleanup context. The operation returns cleanup errors with the original failure.

Ordinary projection markers retain their explicit file mode. Journal writes retain the process umask and inherited access behavior.
These ownership records remain in memory. Persistent workspace preparation, replacement, and legacy relocation recovery still require qualification.

Replacement completion checks the accepted journal's native file identity, exact bytes, and access metadata before removal.
Publication returns the original staged file state. Recovery binds decoded journal content to the same bounded file read that supplies its identity and access metadata.
An identical replacement file, a JSON whitespace edit, or an access change prevents removal during that operation.

Cancellation preserves the journal for a later recovery attempt. Recovery validates the journal again and captures a new receipt for that attempt.
Journal acceptance receipts remain in memory. Version 3 journals persist separate child identity maps.
The preparation journal below supports recovery inside private staging. Candidate handoff and legacy relocation recovery remain incomplete.

### Persisted replacement child identities

Version 3 replacement journals record native identities for every entry in both directory inventories.
The `old_identities` and `new_identities` maps must cover exactly the corresponding inventory paths, including the root.
Each identity must be nonempty and at most 128 bytes. Both maps remain subject to the existing 4 MiB journal limit.

Recovery compares identities, content, and access before moving a live tree or candidate.
Backup cleanup compares each remaining entry with the persisted inventory, then repeats the identity check before removal.
It also checks the backup root's path binding before each child removal. Identical replacement files and directories remain preserved.

Version 1 and 2 journals lack the required evidence. Recovery returns `workspace_replacement.version` without changing the journal, workspace, candidate, or backup.
Preserve these files for explicit recovery. An upgrade cannot infer child ownership from matching content and access metadata.
This restriction applies to pending legacy workspace replacements. Accepted inference catalog state remains separate from the optional workspace.

### Workspace writer identity

Workspace writers retain a checked lock handle and its native identity through projection, repair, or legacy layout migration.
Acquisition compares the existing path, locked file, and current path before returning a writer lease.
Failed acquisition closes its handle. Release closes the lease and preserves the stable lock file for later writers.

Directory publication and marker or journal publication recheck that lease. Replacement recovery also checks it before directory moves and cleanup.
Each version 3 replacement journal records `lock_identity`. Recovery requires that identity to match the current held writer lease.
A missing writer identity or a replacement lock prevents recovery and preserves its journal state.

Journal phases and backup cleanup recheck the held lock before further changes. The lease check refuses a writer after its lock path changes.
Windows can also refuse to rename an open lock file. Native qualification must verify that platform behavior.
These writer checks also bind the preparation journal described below.


### Durable preparation records

Each private workspace preparation directory contains `.preparation.jsonl`.
Its first record binds the target path, enclosure identity, journal identity, and stable writer-lock identity.
Later records contain entry identities, actual written bytes, content digests, and access metadata for render, verification, and assembly trees.
Records append without rewriting the preceding inventory. File contents flush before their receipts, and each appended receipt flushes before preparation continues.

Projection and repair recover recorded preparation trees under the workspace writer lock.
Cleanup accepts only remaining entries whose identities, bytes, and access settings match their recorded values.
It checks the writer and journal before each removal. Missing entries permit recovery to resume after an interrupted cleanup.
Unknown files, changed entries, malformed records, and replaced journals remain preserved with an error.
A process exit before its receipt completes leaves uncertain state for explicit recovery.

Recovery limits each journal to 32 MiB and 120,000 events, with at most four preparation trees.
Each tree retains the existing 10,000-entry, 256 MiB content, and 1 MiB path-name limits.
The parent scan stops at 4,096 entries before cleanup starts. These records contain no provider credentials.

Version 1 records cover entries inside private preparation. Version 2 also records the candidate handoff described below.
Version 3 also owns temporary publication records, as described below. Version 4 adds the legacy relocation records described below.
Native Linux and Windows execution remains subject to the task's verification gate.


### Durable candidate handoff

Version 2 preparation journals append a handoff record before exporting the assembled candidate.
That record binds the candidate name to the preparation directory, its prepared inventory, and the original workspace inventory with native child identities.
The journal remains after the render and verification trees close. Candidate cleanup accepts only entries that match the prepared tree or the exchanged original tree.
Cleanup never selects the installed workspace path.

Publication checks the original journal receipt and the complete candidate inventory before replacing the workspace.
The replacement protocol also compares its candidate with that prepared inventory before recording ownership.
Changed candidate bytes, child identities, or journal contents prevent publication. Cleanup preserves changed or unknown files.

Replacement recovery runs before preparation recovery. A pending replacement journal prevents preparation cleanup from collecting its candidate.
After replacement settles, preparation recovery can remove remaining owned candidate files and retire its journal.
First installation, native directory exchange, and journaled replacement share this handoff contract.
Version 1 journals remain readable for private preparation recovery and cannot authorize candidate cleanup.

A process exit before a complete ownership receipt preserves uncertain state.
A crash after journal removal can leave an unrecorded empty enclosure, which remains preserved.
Other stages, retention, compaction, and full task qualification remain open.

### Durable temporary publication records

Version 3 preparation journals can own one temporary projection marker or replacement journal.
Each receipt binds the destination, temporary basename, native identity, content digest, byte count, and access metadata.
The temporary basename includes the preparation directory's unique suffix. Receipts cannot select the destination or an unrelated file for cleanup.
Record stages contain no catalog trees or candidate handoff.

The publisher records the empty file and each completed write, including a returned partial write.
File contents and the parent directory flush before the receipt. The publisher verifies the unchanged receipt before replacing or linking the destination.
An interrupted write without a complete receipt remains uncertain and requires explicit recovery.

Recovery checks the retained journal, enclosure, and writer identities before removing a temporary file.
The file must match the recorded identity, contents, and access settings. Changed files and unknown entries remain preserved with an error.
A missing temporary file permits journal cleanup to finish after an interrupted rename or removal.
Unlinking a recorded temporary name does not remove its published destination.

Projection, journaled replacement, replacement recovery, and legacy projection use the same record owner.
Versions 1 and 2 remain readable within their original preparation and candidate contracts. They cannot authorize temporary-record cleanup.
The existing journal-size and parent-scan limits apply. Full task and native platform qualification remain required.


### Durable legacy relocation records

Version 4 preparation journals record a legacy relocation before moving the catalog store.
The record binds both parent directories, the store root, current pointer, commit lock, and every retained generation.
Receipts bind content digests, native identities, access metadata, and the optional Windows lock alias.
Generation scans use batches of 128 entries. The existing journal and tree limits apply before relocation.

The workspace inventory flushes before workspace publication. Ordinary projection and repair refuse a pending relocation.

Retrying the same explicit migration checks the retained journal and both advisory locks.
Recovery verifies every recorded generation and any remaining projected workspace entries before restoration.
It removes only owned workspace entries, restores the store without replacing a destination, and retries migration.
Candidate and temporary-record recovery exclude the retained relocation journal until restoration finishes.
Recovery also accepts an already restored store and an incomplete cleanup of the recorded workspace.

Changed files, replaced directories, unknown entries, incomplete receipts, and conflicting destinations remain preserved with an error.
The original journal receipt also guards workspace publication within the running migration.
Windows recovery reuses its recorded hard-link alias. It recreates a missing alias only for the same recorded commit-lock identity.
Cleanup verifies alias contents and access after releasing the store lock. Unknown or changed aliases remain preserved.

A completed operation removes its journal. Repeating that completed migration then reports the existing destination.
A crash after journal removal can leave an unrecorded empty enclosure, which requires explicit recovery.
These checks do not qualify native execution, other stage recovery, retention, history compaction, or full CSP5 acceptance.

## Private record publication recovery

`privatefiles.Directory.PublishFileContext` records staging ownership before destination publication.
Its explicit recovery method uses the same writer lock. Ordinary private-file reads and writes retain their existing behavior.
Runtime evidence and GitHub discovery use this publication API.

Each record directory uses a private `.record-publications` child with a stable `.owner.lock` and one bounded JSONL receipt per pending publication.
Receipts bind both directories, the writer, the journal, and the temporary file to native identities and access snapshots.
Temporary-file receipts also bind mode, modification time, size, and content digest.
Publication checks the destination against its original receipt before replacing it.

Recovery removes a temporary file only when its complete ownership receipt still matches.
It never removes the accepted destination. A hard-linked accepted destination survives removal of its temporary name.
Changed files, incomplete receipts, and unknown files remain preserved. Unknown files do not establish ownership through their names.

Recovery scans at most 4,096 metadata entries in batches of 128. Each journal permits three events within 65,536 bytes.
Each record permits at most 64 MiB. Exceeding a limit stops recovery without inferring ownership.
A visible publication with an unconfirmed flush or cleanup returns `PublicationError`.
Native Linux and Windows execution remains subject to the CSP5 qualification gate.

### Runtime and discovery records

Runtime startup recovers source, provider, binding, publication-input, and pin records before loading retained layers.
Recovery runs under the runtime directory owner and each record writer lock. Passive file inspection does not start recovery.
The file manifest reports private receipt directories and temporary records without exposing their contents or adopting unrelated operator files.

GitHub source construction recovers its local discovery records without contacting the network.
The runtime supplies its caller context through `github.NewContext`. The existing `github.New` wrapper uses a background context.
Cancellation before construction creates no state. Cancellation during recovery preserves unfinished records for a later attempt.

Each verified refresh compares the previously read state bytes under the shared writer lock before publishing the next state.
An absent state and an empty file remain distinct comparison inputs. A conflicting refresh returns a retryable conflict and preserves the newer record.
Retry reads the current replay floor, ETag, and accepted release reference before checking the channel again.

Runtime migration refuses pending private record receipts during source inspection. Copying those receipts would invalidate their native file identities.
The check applies only to declared runtime and discovery paths. Unrelated directories with the same metadata name remain operator-owned migration input.

Recover in the original directory before retrying migration. Inspection preserves the source files and does not create recovery metadata.
Migration can copy inactive writer metadata after recovery. A new publication binds its new native identities.

### Repeated provider inventories

Before a new acquisition exceeds the retained history limit, the runtime compacts histories that contain only provider observations and no resets.
Compaction removes intermediate successful inventories only when their payload bytes and complete binding declarations match the first and latest successful inventories.
The first inventory preserves original model change times. The latest inventory preserves current source evidence.
It also retains distinct inventories, partial results, and equal-time evidence.

It combines retained provider observations into one replay batch without changing the original payloads or receipts.
An omitted offering therefore keeps the original inventory that contains it.

The publication transaction installs the compacted history only after catalog acceptance. Failure preserves the accepted history.
Other histories use bounded checkpoints that share payload bytes and preserve original replay boundaries.
If necessary evidence still exceeds a history limit, publication returns a conflict and preserves the accepted catalog.
Compaction does not delete immutable observation files or catalog generations. Their collection requires the separate retention contract.

### History checkpoints

When another acquisition would exceed a linked-record or byte limit, the runtime can replace the proposed history with one version 4 checkpoint.
The checkpoint stores each distinct payload once. Separate tables retain original receipts and the ordered observations and reset scopes of every batch.
Sharing payload bytes does not replace source identities, split aggregate receipts, or discard reset exclusions.
Preview and publication use the same history preparation. Only an accepted catalog transaction installs the checkpoint as the retained head.

A checkpoint has no parent and cannot contain ordinary batch fields. Later ordinary batches can reference it as their parent.
Recovery validates payloads, receipts, reference indices, reset scopes, and replacement evidence before applying retained inputs.
Duplicate or unused table entries, invalid indices, and incompatible record versions cause refusal.
The 64 MiB encoded limit and 65,536-reference limit bound each checkpoint. History loading also accounts for later linked records.

A checkpoint preserves the original payloads needed for replay against a replacement baseline or changed source configuration.
It does not collect its predecessor files or catalog generations. Collection must preserve accepted references, pending publication, pins, and active readers.
If distinct required data cannot fit within the limits, publication preserves the accepted head and returns a conflict.
The runtime still needs to reduce superseded distinct inventories. Full CSP5 qualification remains open.

### Immutable payload cache

Each immutable catalog retains its validated encoded payload after the first successful encoding.
The cache belongs to that catalog and expires with it. Its data uses the existing 32 MiB payload limit.
Mutable builders always encode their current records.

Every encoding call returns bytes that belong to the caller. Warm calls allocate one output byte slice.
Concurrent readers share the immutable cached bytes through an atomic pointer. Calls use the initialization lock until the first encoding completes.
Failed encoding leaves the cache empty. Payload validation, schema selection, and canonical bytes retain their existing contracts.

Provenance serialization avoids repeated decoding for built-in scalars and generic JSON containers.
Custom marshalers and source structs still pass through canonical normalization. Unsupported values and cycles still fail during encoding.
This cache reduces catalog serialization work. It does not establish Starport inference latency or complete runtime-suite qualification.

Catalog construction also snapshots nested provenance values and rejection records. Source structs use the generic JSON shape that restored evidence already uses.
Provenance reads return independent nested values. Caller changes cannot alter published catalog facts or make those facts disagree with cached payload bytes.
Unsupported custom values and cyclic provenance cause construction to fail.

### Generation retention

`RetainingStore` adds explicit collection and generation read leases to the catalog store contract.
`Memory` and `Filesystem` implement this contract. Object storage and runtime adoption remain incomplete.
Existing catalog store implementations remain compatible. This interface does not enable automatic collection.

Each collection request names the expected current generation, required generations, and positive generation and byte limits.
Baseline, candidate, and rollback owners supply their required IDs. Every required generation must exist.
The store also protects its current generation and every active read lease.
Callers must coordinate changes to their required IDs with collection.

Collection removes the oldest generated content first. Generation IDs break equal-time ties.
Protected generations remain available even when they exceed the requested limits. The report then sets `OverLimit`.
This policy uses generation timestamps, not activation order.

The default scan permits 4,096 entries. An explicit scan can permit up to 100,000 entries.
Invalid limits, stale current state, missing requirements, and incomplete scans preserve all generations.
Memory collection coordinates reads, leases, and publication under one lock.

Reports count manifest and payload bytes. They exclude filesystem overhead, journals, and backend replication.
A dry run returns candidates and projected usage without deletion. Actual usage remains unchanged in its `After` field.
A normal pass reports the generations it removed and the resulting usage.

`AcquireGeneration` returns independent generation bytes and an idempotent release function.
The lease protects stored content until release, even after current changes or the acquisition context ends.
Each successful caller must release its lease. Repeated release cannot end another caller's lease.
Ordinary `Get` returns independent bytes without retaining the stored generation after the call completes.

An empty filesystem store needs no collection. Missing generation leases and missing requirements return the catalog not-found error.
A nonempty expected head conflicts with an absent store. These outcomes do not require a prior catalog publication.

### Filesystem retention and read leases

Filesystem collection holds the existing `.commit.lock` while it scans, selects generations, and completes retirement.
Ordinary generation and authority reads use independent shared handles for that lock. They cannot observe a partially removed generation.
Each explicit generation lease holds a separate shared `.read.lock` inside that generation directory.

Multiple callers own independent handles. Publication can proceed while a generation lease remains active.
An ended process releases its native locks. No lease expiry clock controls deletion.

A collection scan includes generation entries and retention metadata. Unknown names or contents stop collection and remain preserved.
Dry runs do not create generation lease files. Pending retirement or record recovery requires a normal collection pass.

Capacity reports count encoded manifests and payloads. They exclude lock files, journals, and other filesystem overhead.
A failed pass reports only the deletions that completed before the error.

Before retirement, the collector records native identities, access policy, file metadata, and content digests in a `.retirement-<token>.json` journal.
The generation directory moves atomically to `.retired-<token>` before any file deletion. Each completed removal synchronizes directory metadata.
Journal publication uses the existing recoverable private-record writer under `generations/.record-publications`.
Collection recovers that writer before it processes retirement journals.

Recovery removes only remaining files that match the journal. Changed entries and unknown files preserve the retired directory and journal.
An interruption before the directory move leaves the original generation available. Recovery cancels that preparation and makes a new retention decision.

This permits a new reader or pin to protect the generation before the next pass.
An interruption after the move resumes checked cleanup. A changed writer lock or parent directory prevents that cleanup.

This API does not enable automatic collection. Runtime owners still must supply every baseline, candidate, and rollback requirement before adopting collection.
Native platform qualification, object storage, and observation-file collection remain part of CSP5.

### Object inventory and conditional deletion

`ObjectCollectionBackend` adds bounded inventory pages and conditional current-object deletion.
The reference memory backend and S3 adapter implement these operations. The minimum `ObjectBackend` interface remains unchanged.
These operations do not implement generation collection or protect pins and readers. The collector must coordinate publication before using them.

Each list request requires a nonempty prefix. Its page limit permits 1 through 1,000 entries.

Returned entries contain the object key, conditional validator, and current byte size.
Pass a nonempty continuation cursor unchanged with the same prefix. Pages do not form a snapshot across requests.
Concurrent mutations can change later pages. A collector cannot infer safe deletion from an inventory alone.

The S3 adapter uses `ListObjectsV2` with URL encoding and no delimiter. It bounds each response body to 8 MiB.
It rejects incomplete pagination metadata, mismatched namespaces, duplicate keys, invalid sizes, and missing validators.
Malformed or excessive responses return an error without a partial page.

S3 deletion sends one exact quoted ETag through `DeleteObject` with `If-Match`.
Wildcard and multiple validators fail before network access. Conditional conflicts, missing objects, and service failures retain their error classifications.
The adapter does not retry an unsupported condition as an unconditional deletion.

Validators can repeat when object bytes repeat. They do not replace a publication fence or a generation retirement protocol.
Bucket versioning can retain historical versions after current-object deletion. Inventory byte totals therefore do not measure total bucket storage.
The adapter does not bypass retention rules or permanently remove historical versions.
See the AWS [listing contract](https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListObjectsV2.html) and [deletion contract](https://docs.aws.amazon.com/AmazonS3/latest/API/API_DeleteObject.html).

Object generation collection, runtime integration, and live service qualification remain incomplete.

### Runtime pin read leases

`GenerationLeaser` separates generation read leases from publication and collection.
`GenerationLeaseProvider` lets a store wrapper forward that optional capability.
The authority publisher forwards a read-only value. That value does not expose the underlying publication store.

The root client exposes `CanLeaseGenerations` and `AcquireGeneration` for explicit reads.
Capability inspection reads no storage. Successful acquisition returns independent bytes and an idempotent release function.
Acquisition preserves the configured store's read checks and verifies that its result matches the protected generation.
Failed reads or validation release protection and preserve both read and release errors.

A missing stored embedded generation uses the verified compiled artifact without creating a storage record.
That fallback also preserves configured read checks and rejects a different artifact.

During pin startup, the runtime leases the original selected generation when the store supports leases.
An authority origin can then publish the same payload under a new generation without making the original selection eligible for collection.
Failed startup releases the lease. Shutdown releases it after runtime-owned work stops and includes release errors in its result.

Memory and filesystem stores support these leases. Minimum store implementations retain their existing read behavior.
This change does not enable object generation collection or automatic runtime collection.
Collectors still must include configured pins and other persistent requirements in their retention requests, including after a runtime closes.
Complete runtime collection and shared-store coordination remain open.


### Checked removal of private input records

Private record removal now shares the native writer lock used by `PublishFileContext`.
It compares expected content and checks file identity, access, and directory ownership before deletion.
Changed content, active publication, unsafe paths, and replaced directories cause refusal.

A successful result distinguishes a newly removed file from an already absent file.
A directory synchronization failure after deletion reports the visible removal through `PublicationError`.
Retry after reopen synchronizes the directory without recreating the file.

The runtime collector identifies unreachable input records before calling this operation.
It holds publication ownership while tracing accepted history and pending publication references.
The removal primitive does not establish reachability or enable automatic collection. Object-store coordination remains separate work.

### Runtime input collection

`Runtime.CollectRetainedInputs` collects immutable files under `catalog-runtime/publication-inputs` in the configured local state directory.
It preserves the current manual history, its ancestors and observations, and every input referenced by a pending publication.
Checkpoint records carry their original payloads and receipts internally. Collection preserves those records without retaining obsolete external copies.
The collector uses exact stored references, including legacy filenames whose bytes differ from current encoding.

`InputCollectionRequest.MaxEntries` bounds directory entries, including unknown files and publication metadata.
It must be positive and cannot exceed `storage.MaxRetentionScanEntries`.
`MaxBytes` bounds captured raw records, including the manual head and publication journal.
This byte limit excludes temporary decoder allocations and repeated filesystem validation work.
`DryRun` validates references and reports candidates without removing files.

Missing or invalid required references cause refusal before deletion. A changed accepted head also causes refusal unless a pending publication explains it.
Unknown names, unsupported schemas, unsafe files, and content mismatches remain available for inspection.
The collector can remove an unreachable batch with valid structure even when its referenced inputs are already absent.
This rule permits retry after a partial cleanup. Required history still passes the complete receipt and reference checks.

Collection joins runtime operation ownership, cancellation, and shutdown. It serializes with catalog publication and retained provider writes.
It works in offline and pinned modes without acquiring a shared lease or reading a catalog source.
The serving catalog stays unchanged in memory. Collection adds no filesystem work to catalog queries.

The report separates protected inputs, preserved entries, deletion candidates, and visible removals.
An error after deletion retains the partial removal report, including unconfirmed directory synchronization.

This API explicitly cleans local inputs. Automatic invocation, retention configuration, and complete catalog generation collection remain part of CSP5.

### Current provider review evidence

Manual replay retains one current unresolved-model review per provider, binding revision, opaque model ID, and review code.
Different accounts and binding revisions keep separate entries. An omitted offering keeps its last review until replacement evidence or an authorized operation changes it.
Direct provider evidence outranks stale fallback. A later direct observation can replace the review for a record it reports, including during a partial reply.
Metadata-source reviews retain their existing selection rules.

Each selected provider review must match its original observation ID, revision, and evidence checksum.
The selected review keeps that original receipt. Superseded duplicate reviews no longer keep obsolete receipts in the effective generation.
Other current fields, membership records, and unresolved offerings can still require those receipts.
Durable acquisition history and previously committed generations remain unchanged.

This selection also preserves review evidence when a replacement baseline no longer defines a formerly known model.
Repeated-inventory compaction must produce the same current review set as complete replay under that baseline.
CSP5 must still retire distinct inventories, automate collection, and complete full qualification.
