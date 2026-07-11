# Durable Scheduling

Scheduling is an explicit deployment composition above Starmap's idempotent
Sync operation. The root client owns no ticker or cadence lifecycle.
`pkg/catalogscheduler.Runner` accepts a narrow Syncer, acquires a publisher
lease before any source call, invokes Sync once, and releases the lease after
the attempt.

Lease contention returns the successful disposition `skipped_lease_held`; it is
not classified as provider failure and never invokes Sync. This lets every
replica receive the same tick without stampeding providers or racing generation
publication.

The package provides two adapters:

- MemoryLease is the deterministic reference model and supports TTL expiry plus
  fencing tokens, so a stale guard cannot release a newer owner's lease.
- FilesystemLease uses a non-blocking OS lock across independent processes that
  share a volume. Process exit releases the lock. Starport may implement the
  narrow Lease interface with its database or distributed coordination system
  for replicas without shared storage.

Lease keys, owners, and positive TTLs are explicit. The default key coordinates
one catalog publisher group and the default TTL is fifteen minutes. A deployment
must give every replica a stable unique owner identity.

`Runner.RunOnce` is immediate and defaults to one attempt. A deployment may pass
`WithRetryPolicy` to enable bounded exponential backoff. Only deadline, timeout,
rate-limit, provider-unavailable, HTTP 408/425/429/5xx, and temporary network
failures are retryable. Configuration, parsing, authentication, dependency,
validation, conflict, cancellation, ordinary 4xx, and unknown failures stop
immediately. The lease remains held for the complete attempt sequence.

`Runner.RunScheduledOnce` adds a bounded randomized delay before lease
acquisition. This spreads scheduled provider traffic without delaying explicit
manual runs. Both jitter and retry waits honor context cancellation. `RunResult`
records the attempted calls and selected retry delays; the durable audit record
can be enabled with `WithRunLedger`.

The `RunLedger` boundary begins a record before lease acquisition, appends each
Sync attempt, and completes the terminal result. Every record exposes its
trigger, lease owner, base generation, timestamps and duration, attempt status
and retry decision, terminal disposition, and any published generation/sync-run
identity. Contended replicas are retained as `skipped_lease_held` with zero
attempts. If a process terminates between begin and completion, the durable
record remains `running` for operational detection. Error messages are not
persisted because provider payloads may contain secrets; only Go error types and
the typed transient/permanent retry class are retained.

`MemoryRunLedger` is the concurrency-safe reference adapter. `SQLRunLedger`
persists normalized run and attempt rows through `database/sql` using the same
SQLite-compatible bind convention as the catalog SQL store. Both implement
idempotent lifecycle writes, caller-owned reads, newest-first filtered queries,
and bounded query limits. Starport may implement the narrow interface with its
existing enterprise database.

Source freshness is independent of whether reconciliation changes catalog
bytes. Every Sync result carries validated, catalog-free source-observation
links, and `WithFreshnessMonitor` advances them after any successful attempt,
including no-change runs. `FreshnessPolicy` requires an explicit warning and
larger critical threshold for every monitored source plus whether that source is
required. Starmap deliberately supplies no hidden universal durations: Starport
must choose them from its declared update cadence and provider commitments.

`FreshnessMonitor.Report` is a pure fake-clock-friendly evaluation. A source
past its warning threshold or returning partial/degraded evidence preserves
readiness but marks the report degraded and emits a machine-readable warning. A
required missing source, future observation, or source past its critical
threshold fails readiness and emits a critical alert. Missing optional sources
are warning-only. Reports include explicit seconds, observation identity and
time, status, completeness, state, and stable alert codes. Out-of-order run
completion cannot regress a source, and conflicting identities at one source
timestamp reject the entire update atomically.

The monitor is safely empty after startup: required sources are unready until it
is seeded from a validated current generation with `RecordManifest` or receives
a successful Sync. Deployments should expose the report through readiness and
forward its alerts to their existing alert manager. This state remains separate
from liveness and from the durable run ledger.

Last-known-good protection is inherited from the generation transaction, not
implemented as a mutable scheduler cache. Each retry builds a candidate from the
same currently published immutable catalog. Validation and durable compare-and-
swap commit complete before the client swaps its catalog pointer. A fetch,
validation, or commit failure therefore leaves the prior catalog pointer,
generation ID, durable current row, retained immutable generation, and event
stream unchanged; the scheduler records the failed attempt sequence separately.
Freshness can still degrade as that valid generation ages, which is distinct
from replacing it with an invalid candidate.

Startup behavior is mandatory deployment configuration through
`InitialRunPolicy`; there is no default mode and `starmap.New` remains passive.

| Mode | Startup execution | Readiness semantics |
| --- | --- | --- |
| `startup_blocking` | `Start` performs one leased Sync and returns its error | Unready until the attempt succeeds and the catalog/freshness baseline is ready; failure rejects startup even if a last-known-good exists |
| `startup_background` | `Start` launches one leased Sync on the supplied application-lifetime context | An already-ready baseline may serve, explicitly degraded, while pending or after failure; no baseline remains unready |
| `schedule_only` | `Start` performs zero source work | Readiness is exactly the existing catalog/freshness baseline until the deployment supplies a tick |

Every controller accepts a baseline-readiness probe so startup state composes
with catalog and source-freshness truth instead of overriding it. `Start` is
single-use, `Wait` is cancellation-aware, and the readiness report exposes the
mode, lifecycle state, stable issue code, run ID, and secret-safe failure type.

Blocking/background policies require an explicit positive coalescing window.
`RunScheduledAt` receives the deployment's tick due time. If a successful or
lease-held startup attempt covers that due time, the tick is durably recorded as
`skipped_initial_run` with zero source calls. A tick arriving while background
startup is running waits for its outcome: success coalesces, while failure runs
the recovery tick immediately. Ticks outside the window execute normally. This
prevents an immediate duplicate without silently skipping the next real cadence.
