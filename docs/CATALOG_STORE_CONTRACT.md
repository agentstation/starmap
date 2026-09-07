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
This version 1 contract contains the following fields:

| Field | Contract |
| --- | --- |
| `schema_version` | Exactly `1`. |
| `id`, `revision` | Stable deployment-owned identity and declared scope revision. |
| `provider_id` | The single canonical provider contained in the observation. |
| `public` | An explicit public scope, without account or project selectors. |
| `account_id`, `project_id` | At least one is required when `public` is false. Both can apply to a private scope. |
| `region`, `api_surface` | Required selectors, including an explicit region such as `global`. |
| `credential_role` | Exactly `catalog_acquisition`. |
| `credential_profile_id` | The declared authentication profile. This field contains no resolved credential values. |

Each identifier or selector permits at most 4,096 UTF-8 bytes, without control characters or surrounding whitespace.
JSON decoding rejects unknown fields, unsupported schemas, and invalid bindings. Errors identify fields without echoing their values.
A binding requires the provider source and exactly one matching provider. It does not authenticate the account or verify the declared credential profile.

Scoped observations use `observation:v3:<sha256>` identities. Their identities include every binding field with the same byte-length encoding used for other metadata.
Changing or removing a binding invalidates the existing receipt identity. Unscoped v2 and safe legacy identities cannot validate a scoped observation.
The constructor, receipt, clone, and restore operations copy the binding. Retention and restart preserve its metadata and reject altered evidence.

The runtime retains separate records for each provider, binding identity, and binding revision.
CSP3 must connect configured bindings to scheduled acquisition before operator support.
A missing binding does not declare public scope. Existing unscoped evidence requires explicit treatment during that integration.

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
When retained inputs exist, reconstruction starts from the compiled baseline and applies the retained source and provider observations.
With an explicit binding set, startup reconstructs and publishes the selected state before returning.
Without that option, an empty retained set preserves the accepted current state under legacy startup behavior.
That legacy path still needs active authority and configuration-omission checks before operator qualification.

Reconstruction treats baseline generation IDs as opaque values. It does not remove a `.local.` substring to infer an earlier identity.
The same immutable inputs produce the same derived identity across restarts.

The regression excludes provider evidence at the reconstruction boundary after reopening a real filesystem catalog store.
The excluded provider no longer returns through the baseline. This check does not exercise an operator revocation API or active binding policy.
Explicit local inputs still need source evidence when the runtime reconstructs a generation. A previously merged head cannot substitute for that evidence.
