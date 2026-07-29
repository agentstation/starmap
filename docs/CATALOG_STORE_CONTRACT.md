# Catalog Store Contract

`catalogstore.Store` is Starmap's narrow durable-generation seam. It lets an
embedding application own persistence without teaching Starmap about that
application's database, credentials, migrations, or lifecycle.

```go
type Store interface {
    Current(context.Context) (catalogstore.Generation, error)
    Get(context.Context, string) (catalogstore.Generation, error)
    Commit(context.Context, catalogstore.Generation, string) error
}
```

Starmap provides memory, filesystem, and conditional object-storage adapters.
Starport may provide SQLite, MySQL, PostgreSQL, or other adapters, but Starport
owns each concrete driver, connection pool, schema, migration, credential,
backup, close, and dialect-specific transaction concern.

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

- absent ID plus valid content may be stored;
- the same ID plus byte-identical manifest and payload is an idempotent retry;
- the same ID plus different content returns a typed conflict; and
- no successful or failed operation may rewrite content already bound to an ID.

`Get(ctx, id)` returns the complete validated generation bound to `id`.
Committed historical generations remain addressable so rollback and audit do
not depend on reconstructing old content.

## Current and compare-and-swap

`Current` returns the complete generation named by the one current pointer. An
empty store returns a typed not-found error. It must never expose a partially
written generation or a pointer whose generation cannot be read and validated.

`Commit(ctx, candidate, expectedGenerationID)` is one compare-and-swap
operation:

1. reject canceled context or invalid candidate before durable mutation;
2. reject a generation-ID collision with a typed conflict;
3. compare the actual current generation ID with `expectedGenerationID`;
4. persist the complete immutable candidate; and
5. atomically change current only if the comparison still holds.

An empty expected ID means the store must still have no current generation.
Concurrent commits from the same base produce exactly one winner; every loser
returns a typed conflict containing the expected and observed current IDs.
Implementations must use their backend's native transaction, row/version
predicate, or conditional-write primitive. A read followed by an unconditional
write is not a valid implementation.

An identical retry after an ambiguous successful response returns success even
though the original expected ID no longer equals current.

## Failure preservation and rollback

A failed commit never changes `Current`. It must leave the previous current
generation complete and readable.

An implementation may retain a complete immutable addressed candidate when a
failure occurs after candidate persistence but before current-pointer
promotion. It must never expose partial content through `Get`, and retrying the
same candidate must remain safe. Garbage collection of unreferenced immutable
generations is an implementation-owned maintenance policy, not part of
`Commit`.

Rollback is normal compare-and-swap promotion of a retained prior generation:

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

- missing current or addressed generation with `errors.IsNotFound`;
- stale compare-and-swap or immutable-ID collision with
  `errors.IsConflict` / `*errors.ConflictError`;
- invalid manifest, payload, identity, or checksum with
  `errors.IsInvalidInput`; and
- cancellation or deadline with the standard `context` errors.

Backend details may be wrapped with operation/resource context, but reusable
credentials, connection strings, query arguments, payload bodies, or other
secrets must not enter returned errors or logs.

## External adapter verification

The `testdata/consumers/store-only` module is compiled with `GOWORK=off`. It
defines a Starport-owned adapter outside Starmap's module packages, proves the
interface at compile time, injects it through `WithCatalogStore`, publishes a
real generation, and executes the ownership, validation, idempotency, retained
history, conflict, rollback, failure-preservation, and cancellation contract.
Its dependency gate rejects CLI, server, acquisition, SQLite, MySQL, and
PostgreSQL implementations.

Starmap deliberately does not export a `testing.T`-based conformance helper.
The interface and executable external module are the stable product surface;
backend-specific transaction, corruption, and fault injection remain local to
each adapter. A public helper should be reconsidered only when multiple
external repositories demonstrate a shared deep testing module that cannot be
expressed through this behavioral contract.

## S3-compatible object storage

`pkg/catalogstore/s3` is Starmap's optional production
`catalogstore.ObjectBackend`. It accepts an already-configured, caller-owned AWS
SDK v2 S3 client and performs no network request during construction. Compose
it with the generation store:

```go
backend, err := s3store.New(callerOwnedS3Client, s3store.Config{
    Bucket: "starmap-catalogs",
})
if err != nil {
    return err
}
store, err := catalogstore.NewObject(backend, "production")
```

The selected S3-compatible service must implement conditional `PutObject`
writes. Immutable generation objects use `If-None-Match: *`; the current
pointer uses `If-Match` with the exact opaque ETag returned by the prior read.
The adapter rejects unconditional writes, requires ETags, bounds object bodies,
and maps 409/412 precondition failures to typed conflicts. A service that
rejects conditional writes fails the operation explicitly. A service that
silently ignores those standard headers is not S3-compatible for this contract
and must not be selected.

Starmap never discovers credentials, constructs a default AWS configuration,
opens a network connection in `New`, or closes the client. The embedding
deployment owns those concerns and may configure the AWS client with a
caller-selected S3-compatible `BaseEndpoint`. See AWS's
[conditional-write contract](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html)
and the AWS SDK v2
[endpoint configuration guide](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-endpoints.html).
