# CAT2 final runtime, transport, and operations review

Date: 2026-09-01

Reviewed baselines:

- Catalog plan: `codex/catalog-publisher-six-hour` at `cbba4d62` before this
  audit.
- Starmap main worktree: `codex/security-baseline` at `fa7c26bb`.
- Starport: `codex/cpl-b1` at `b522d7d`. The worktree contains unrelated owner
  changes and was not edited.

The review changed no production source. The owner ratified its two open
decisions on 2026-09-01. Commit `9017b83b` then changed the publisher cadence.
This proof records the contract before CAT2 verification and CAT5-CAT8 work.
The final audit added no production source. It corrected target transport and
fleet timing after it measured live assets and traced both HTTP stacks.

## Verdict

The retained-layer runtime, connected `starmap.Open` API, orthogonal source and
acquisition policies, Starport acceptance boundary, and detailed operator
status are correct. The owner ratified these decisions:

1. Replace `CATALOG_ACQUISITION_ON_START` with
   `CATALOG_ACQUISITION_ENABLED`. A false value disables all automatic work.
2. Publish every four hours at minute 17. Keep one-hour consumer polling and
   use a six-hour end-to-end freshness objective.

The final audit found that the earlier 30-second source request and five-minute
complete refresh targets were not safe. A Go `http.Client.Timeout` includes
response-body reads. Starmap permits 16 MiB source documents and 64 MiB remote
bodies, so a healthy slow transfer can exceed both targets. The corrected
contract uses connection, TLS, response-header, and idle-progress bounds. It
has no default whole-refresh deadline.

## Steering report verification

| Section | Verdict | Repository evidence or correction |
| --- | --- | --- |
| 1. Runtime contract | agree | Current Starmap has separate offline, import, and remote subscriber paths. Current Starport has separate local and remote runtimes. One retained-layer runtime removes both split meanings without moving Starport's accepted head. |
| 2. Credential outcomes | agree | The provider source currently calls all catalog endpoints. Missing credentials become aggregate issues. It needs neutral preflight skip and provider-scoped retention. |
| 3. Acquisition DX | accepted | `ON_START` plus `INTERVAL` exposes a periodic-only state, but no repository use case requires that public state. The owner selected `ENABLED` plus `INTERVAL`. Stable phase, fresh durable state, and lease ownership control delayed startup work. |
| 4. Catalog, availability, routing | agree with corrections | Starport already separates catalog offering state, credential state, and routing. It already supports explicit fallback lists, model overrides, quotas, and budgets. Starmap model status has no `retired` state or successor field, so historical model lifecycle and `replaced_by` need schema work. |
| 5. Measured timing | disagree with the old target | The measured constants match, but a 30-second total body timeout and five-minute whole-refresh timeout reject valid transfers within current size limits. Current Starport execution also has zero same-route retries by default. |
| 6. CAT-D10 | accepted | Four-hour publication plus one-hour polling gives a nominal five-hour path. The owner selected a six-hour end-to-end objective. It is not a hard bound. |
| 7. Target timing | corrected | Catalog bodies, provider enumeration, and streams use progress-aware idle limits instead of client-wide elapsed limits. Streaming still needs first-token, stream-idle, and explicit operator cancellation. |
| 8. Anti-herd | corrected | A 30-second reconnect cap and one-minute fallback poll are too small after a broad outage. The target uses full-interval stable phases, a 15-minute cold-start spread, decorrelated reconnect delay up to 15 minutes, admission control, and one lease owner for shared state. |
| 9. Status and timestamps | agree | Current generation metadata does not express channel heartbeat, operation state, provider outcome, or direct versus upstream-reported health. |
| 10. Starport API and UI | agree with one correction | The current admin refresh is synchronous. The current console flags a catalog older than seven days with the hard-coded `STALE_AFTER_SECONDS` rule in `FreshnessBar.tsx`. Add server-evaluated freshness and describe the change as the replacement of that rule. The third review corrected this evidence on 2026-09-02. |
| 11. Deployment examples | agree | Current Starport validation prevents the required remote-source plus local-acquisition composition. The target cases prove that source selection and acquisition are independent. |
| 12. Repository gaps | agree with one correction | All listed gaps exist, including the current seven-day badge. Historical release tags remain a read compatibility requirement. Runtime environment aliases do not. |
| 13. Required output | complete | This proof contains the API, timing, fleet, deployment, status, task, verifier, and owner-decision contracts. |

## Canonical runtime and layers

One runtime retains four independent inputs and outputs:

```text
embedded public catalog
        + selected upstream catalog
        + retained operator observations by provider
        = immutable effective catalog
```

The effective catalog is an output. It is never an input to the next rebuild.
A source refresh replaces only the retained source layer. Acquisition replaces
only successful provider layers. A failed provider retains its own
last-known-good layer while successful providers advance.

A custom GitHub, Starmap, or file source replaces the default public source.
It cannot cause a hidden request to public GitHub. `CATALOG_SOURCE=embedded`
disables the network source. Acquisition remains independent.

`prefer_source` returns after verified durable or embedded state is usable. It
starts connected work according to the automatic controller and reports
`warming`, fallback, or degradation. `require_source` can block within the
`Open` context and fail. A caller uses `Refresh` when it must await one complete
cycle.

## Canonical environment

Starmap uses the `STARMAP_` prefix. Starport uses the `STARPORT_` prefix. The
suffixes and defaults are the same.

| Suffix | Default | Contract |
| --- | --- | --- |
| `CATALOG_SOURCE` | `public` | `public`, `github`, `starmap`, `file`, or `embedded` |
| `CATALOG_SOURCE_URL` | empty | Safe source endpoint or file identity for the selected custom source |
| `CATALOG_SOURCE_API_KEY` | empty | Starmap protocol credential; separate from provider credentials |
| `CATALOG_SOURCE_REPOSITORY` | `agentstation/starmap` | GitHub repository for a public or custom GitHub source |
| `CATALOG_SOURCE_CHANNEL` | `catalog-latest` | Mutable attested discovery channel |
| `CATALOG_SOURCE_SIGNER_WORKFLOW` | publisher preset | Expected GitHub workflow identity |
| `CATALOG_SOURCE_TOKEN` | empty | Optional GitHub API token; local public use does not require it |
| `CATALOG_SOURCE_POLL_INTERVAL` | `1h` | Conditional source check interval |
| `CATALOG_SOURCE_STARTUP_POLICY` | `prefer_source` | `prefer_source` or `require_source` |
| `CATALOG_SOURCE_MAX_AGE` | `6h` | Source freshness warning objective |
| `CATALOG_SOURCE_MAX_HOPS` | `8` | Maximum accepted Starmap source-chain depth |
| `CATALOG_ACQUISITION_ENABLED` | `true` | Enables automatic startup and interval work |
| `CATALOG_ACQUISITION_INTERVAL` | `4h` | Normal cadence; `0s` means startup only while enabled |
| `CATALOG_WORKSPACE_PATH` | empty | Reviewed operator catalog input |
| `CATALOG_STARTUP_SPREAD` | `15m` | Stable admission window for cold automatic source and acquisition work |
| `CATALOG_TRANSFER_IDLE_TIMEOUT` | `2m` | Maximum time with no body read or response write progress. The per-transfer maximum is separate |
| `CATALOG_TRANSFER_MAX_DURATION` | `60m` | Maximum duration of one finite HTTP body transfer. Zero is invalid |
| `CATALOG_REFRESH_TIMEOUT` | `0s` | Optional complete-operation wall-clock cap; zero means no added cap |

There is no `CATALOG_ACQUISITION`, `CATALOG_ACQUISITION_MODE`,
`CATALOG_ACQUISITION_ON_START`, or acquisition schedule expression in the
recommended contract.

The transfer maximum applies to one finite HTTP body, such as a manifest, an
archive, or a provider page. It does not bound an SSE subscription lifetime. A
64 MiB body at the 256 Kbps floor rate takes about 35 minutes, and the
60-minute default provides headroom over that case. A zero value is invalid
and fails startup. An operator on a slower link raises the value or lowers the
size cap, and status reports the observed transfer rate.

| Enabled | Interval | Automatic behavior |
| --- | --- | --- |
| `true` | `4h` | automatic startup work and periodic work; default |
| `true` | `0s` | one startup pass |
| `false` | any value | manual only; `Sync` remains available |

Automatic startup work does not always mean an immediate provider request. A
runtime with fresh durable evidence waits for its stable full-interval phase.
A new runtime, a new credential, or stale evidence uses a stable phase within
the 15-minute startup spread. `require_source` starts immediately. An explicit
`Refresh` or `Sync` also starts immediately and remains single-flight.

A deterministic process uses:

```bash
STARMAP_CATALOG_SOURCE=embedded
STARMAP_CATALOG_ACQUISITION_ENABLED=false
```

## Canonical Go API

`New` and `NewContext` remain deterministic offline constructors. `Open` owns
connected work and is the recommended application entry point.

```go
runtime, err := starmap.Open(ctx,
	starmap.WithCatalogStore(store),
	starmap.WithCatalogCredentialResolver(resolver),
)
if err != nil {
	return err
}
defer runtime.Close()

catalog := runtime.Catalog()
state := runtime.State()
status := runtime.Status()
```

The reads do not access external systems:

```go
func (r *Runtime) Catalog() *catalogs.Catalog
func (r *Runtime) State() CatalogState
func (r *Runtime) Status() RuntimeStatus
```

`Catalog` is non-failing, non-nil after successful `Open`, immutable, O(1),
and safe to retain. `Status` includes the active effective generation ID.

The I/O methods have separate effects:

```go
func (r *Runtime) Refresh(context.Context) (RefreshReport, error)
func (r *Runtime) RefreshSource(context.Context) (SourceRefreshReport, error)
func (r *Runtime) Sync(context.Context, ...SyncOption) (AcquisitionReport, error)
func (r *Runtime) Updates() <-chan CatalogState
func (r *Runtime) Close() error
```

`Refresh` stages source work first. It then resolves acquisition eligibility
against the staged source. It publishes one effective candidate when possible.
`RefreshSource` changes only the source layer. `Sync` changes only provider
layers. Each method rebuilds from all retained layers.

One mutation coordinator serializes durable layer replacement, reconciliation,
publication, and update delivery. Compatible concurrent operations join. A
manual complete refresh can coalesce one pending complete refresh. Publication
cannot occur out of order. `Close` is idempotent, cancels owned work, and joins
it within five seconds.

The proposed options include `WithAcquisitionEnabled(bool)`,
`WithStartupSpread(time.Duration)`,
`WithTransferIdleTimeout(time.Duration)`,
`WithTransferMaxDuration(time.Duration)`, and
`WithRefreshTimeout(time.Duration)`. A zero refresh timeout adds no total
deadline. The caller's context always wins. Do not keep both acquisition
enabled and acquisition-on-start options. Source and acquisition options remain
independent.

## Credential and provider contract

Starmap uses only explicit catalog-acquisition credentials. Starport passes the
same raw deployment lookuper that loaded process variables, `.env`, explicit
secret references, and test values. It must not pass deployment inference
credentials, shared inference records, or account BYOK. An operator can bind
one secret into both roles explicitly.

The runtime resolves credentials on every cycle. It uses catalog authentication
metadata before it decides to call a provider.

| Condition | Outcome | Request | Health |
| --- | --- | --- | --- |
| No usable required credential | `skipped_not_configured` | no | neutral |
| Declared public catalog endpoint | `succeeded` or `failed` | yes | failure degrades |
| Usable credential and valid response | `succeeded` | yes | healthy |
| Invalid or unavailable configured reference | `failed` | no provider call | degrades |
| Provider rejection, timeout, rate limit, transport, or invalid response | `failed` | yes | degrades |

Safe reason codes include `credential_reference_invalid`,
`credential_unavailable`, `credential_rejected`, `credential_expired`,
`insufficient_scope`, `rate_limited`, `request_timeout`, `transport_failed`,
and `response_invalid`. Status never includes secret values, raw references,
credential-bearing URLs, provider bodies, or wrapped errors.

Provider list omission is evidence about that credential and endpoint. It is
not global deletion or entitlement evidence. The public model stays in the
catalog. The local offering can become `not_observed` or
`unavailable_for_credential` only when the endpoint contract supports that
conclusion.

## Current and target timing

| Concern | Current live behavior | Target owner and cancellation |
| --- | --- | --- |
| Publisher cadence | Main runs daily at `03:17` UTC. Commit `9017b83b` changes the plan branch to minute 17 every four hours. | Keep the four-hour cadence. GitHub owns dispatch and can delay or drop a run. |
| Publisher execution | No job timeout; GitHub default is 360 minutes. The latest 20 successes were 153-285 seconds, median 222.5 seconds, p95 284 seconds. | CAT3 sets `timeout-minutes: 90` on the job and `timeout-minutes: 75` on the publisher step. The 60-minute transfer bound nests inside the step, and the step nests inside the job, so each outer limit keeps headroom. The workflow remains serialized and GitHub cancels a stuck job at the bound. |
| Complete Starmap sync | `sync.Options` defaults to five minutes. | `Refresh` adds no deadline by default. The caller context or a nonzero configured refresh timeout owns total elapsed time. |
| Starport catalog refresh | Two minutes. | The unified runtime uses the same no-added-total default and exposes asynchronous progress and cancellation. |
| Ordinary provider HTTP | A 30-second `http.Client.Timeout` includes the response body. | Use 30-second connect, 30-second TLS, 60-second header, two-minute body-idle, byte, page, and record bounds. Set a 60-minute per-transfer maximum instead of a client-wide total. |
| Google Vertex list | One two-minute context bounds the whole paginated list. | Use the common per-request stage and idle policy plus Vertex page and record ceilings. Caller or operator context owns the whole enumeration. |
| Provider concurrency | Maximum five. | Keep bounded. Admit cold providers across 15 minutes. A slow provider cannot hold the mutation lock or suppress completed provider layers. |
| Sync cleanup | 30 seconds. | Keep separate from runtime shutdown; cleanup cannot extend the completed operation indefinitely. |
| Catalog projection or repair | One minute. | Keep this local-operation bound and report the stage. It is not a network-body timeout. |
| Durable catalog load | 10 seconds. | Construction stage owns 10 seconds; caller cancellation wins. |
| models.dev HTTP and cache | A 30-second total request and one-hour cache validity. | Use the common source transfer policy. Cache validity remains separate from public channel polling. |
| Release or GitHub source fetch | Starmap uses a 30-second total client timeout. | Use 30-second connect, 30-second TLS, 60-second header, two-minute body-idle, compressed and expanded size bounds, and conditional state. |
| Starport direct Starmap fetch | A two-minute context remains active until body close. | Remove the adapter total and use the same progress-aware Starmap transport. |
| Public source poll | No default connected runtime. | One check per hour at a stable full-interval phase. Cold automatic work uses the 15-minute startup spread. Manual refresh can run sooner. |
| Local acquisition | Starport defaults off; Starmap has explicit `Sync`. | Enable by default at a four-hour stable phase. Cold, rotated, or stale providers use stable offsets across 15 minutes. |
| Starmap SSE | 20-second heartbeat, 60-second liveness, 100 ms to 5-second half-to-full jitter reconnect. | Keep heartbeat and liveness. Spread initial automatic connects across 15 minutes. Use decorrelated retry from one second to 15 minutes, honor `Retry-After`, and use 15-minute stable conditional polling after repeated failure. |
| Remote subscriber shutdown | Five seconds. | Keep five seconds and join owned work. |
| Starmap server shutdown grace | 100 ms. | Increase background-worker join to five seconds. HTTP server shutdown remains independently bounded. |
| Starmap server HTTP | 10-second read, 10-second write, 120-second idle. | Catalog payload and SSE routes clear the total write deadline, reset a two-minute idle write deadline before each chunk or frame, and stop on disconnect. |
| Starmap admin update | Synchronous with a six-minute write deadline. | Return a refresh operation within five seconds. Background work follows progress-aware policy and supports explicit cancellation. |
| Starport admin refresh | Synchronous, two-minute work under a global 60-second request context. | Return or join a `202` operation within five seconds. Background work follows the runtime context and supports explicit cancellation. |
| Starport HTTP | 30-second read and write, 120-second idle, global 60-second request context. | Control-plane routes keep 60 seconds. Inference and streaming routes use route-specific policy. |
| Starport non-stream inference | Executor allows two minutes, but the global request context cancels after one minute. | A configurable route context owns total work; use a 10-minute default. Use a five-minute first-response bound and two-minute body-idle bound. Caller cancellation wins. |
| Starport streaming | Controller clears the 30-second write deadline, but global middleware cancels at 60 seconds and executor cancels at two minutes. | No total route or write deadline after commitment. Use a five-minute first-event bound, two-minute provider and client idle bounds, disconnect, and operator cancellation. |
| Starport retry | Three total attempts, zero same-route retries by default, 100 ms exponential settings unused unless enabled. | Routing policy owns retry eligibility; all attempts remain inside the route budget. |
| Inference transport | 30-second dial, 10-second TLS, 30-second response-header timeout, 90-second idle connection. | Use 30-second dial and TLS. Route policy owns the first-response bound. Body progress owns the idle bound; no client total applies. |
| Direct secret cache | Five-minute refresh. | Add stable fleet phase and shared ownership where available. |
| Credential reconciliation | One-minute interval with a 10-second timeout. | Add stable phase; one owner reconciles shared deployment state. |
| Provider status pages | One-minute interval, five-second request, concurrency eight. | One fleet owner or shared cache; replicas consume the shared projection. |

The target does not use `http.Client.Timeout` for a catalog body, provider
enumeration, SSE stream, or inference stream. That field covers connection,
redirects, and response-body reads. Each transport instead bounds connection,
TLS, response headers, and lack of body progress. Every successful body read or
response write resets the two-minute idle timer. Exact compressed and expanded
byte limits, page and record ceilings, checksum and schema validation, caller
cancellation, and runtime `Close` remain hard bounds.

`Refresh(ctx)` installs no wall-clock deadline by default. A nonzero operator
setting can add one, and the earliest caller or configured deadline wins.
Automatic work uses the runtime context. Network I/O happens outside the
publication lock. Completed provider layers can advance while another provider
is slow. A slow source also cannot stop the independent acquisition controller
from rebuilding against retained source state.

Partial publication uses bounded coalescing. The first completed provider
observation in a run opens a 30-second coalescing window. Every observation
that completes inside the window joins it. At the end of the window the
runtime emits one effective generation with every retained layer. The slow
provider keeps its last-known-good observation in that generation.

A provider that is still running joins the next window. A slow provider
therefore delays no completed peer by more than 30 seconds.

One run emits at most one effective generation per window, and a normal run
emits one or two. The Starport candidate path receives each emitted generation
through the same acceptance transaction. Starport validates one candidate at a
time, and a newer candidate replaces a pending one instead of a queue. The
named test is `TestSyncPublishesCompletedProvidersWhileAnotherBlocked` in
`acquisition`. Two providers complete while a third blocks on an injected
transport. The test proves one publication before the runtime cancels the
blocked provider, the retained last-known-good row for that provider, and at
most two emitted generations.

The publisher run measurements are in
[`cat2-publisher-runs.json`](cat2-publisher-runs.json). The sample supports a
75-minute publisher step and the 90-minute job with more than 15 times the
observed maximum. It does not establish a worst-case service guarantee.

The live asset and code-limit measurements are in
[`cat2-network-measurements.json`](cat2-network-measurements.json). The newest
catalog archive is 393,447 bytes and takes about 24.6 seconds at 128 kilobits
per second before protocol overhead. A 16 MiB source takes about 134 seconds at
one megabit per second. A permitted 64 MiB remote body takes about nine minutes
at one megabit per second and 35 minutes at 256 kilobits per second. These
values disprove the old 30-second and five-minute elapsed targets.

## CAT-D10 freshness contract

The four-hour publisher plus one-hour poll gives a nominal five-hour path. The
product uses a **six-hour end-to-end freshness objective**. This value is not a
hard bound. Workflow dispatch delay, publisher execution, transport, consumer
phase, and retry can extend it.

The policy is:

- Keep the four-hour publisher at minute 17.
- Warn when `channel_updated_at` is older than six hours.
- Mark publisher channel health critical after 10 hours.
- Warn when a consumer source check is older than 90 minutes and mark it
  critical after two hours.
- Warn when eligible acquisition success is older than five hours and mark it
  critical after eight hours.

The cadence gives one hour of nominal operational margin. GitHub can delay or
drop scheduled work, so the objective is not a mathematical guarantee.

`channel_updated_at` advances after each successful publisher verification,
including no-change runs. The signed channel sequence advances too. The
immutable `published_at` and `generated_at` do not change. A consumer `304`
advances `checked_at` only.

## Staggering, retry, and fleet coordination

Each periodic controller uses a stable phase:

```text
phase = hash(instance identity + controller + safe source identity) mod interval
```

The instance identity lives outside the catalog store. The runtime derives
it from three inputs: a random seed in the process-local state directory, the
host name, and the configured listen address. The setting `STARMAP_STATE_DIR`
names the directory, and the default is the user state directory. The seed
never enters the catalog path, a shared volume, the generation record, or the
channel. The setting `STARMAP_SCHEDULER_IDENTITY` replaces the derived value.

Two replicas that share one catalog store get distinct identities because
their host names differ. Two replicas that start from one cloned image and
state directory get distinct identities for the same reason. A restart on the
same host keeps the same phase. Normal public-source work spreads across the
full one-hour poll interval. Normal acquisition spreads across the full
four-hour interval.

The named test is `TestSchedulerIdentityDivergesAcrossClonedState` in the
root package. It clones one state directory and one catalog store into two
replicas with different host names. It proves a distinct phase for every
controller and proves that a restart with the same host name keeps its
phase.

Cold `prefer_source` processes, new credentials, rotated credentials, and stale
evidence use stable offsets across a 15-minute startup window. Fresh durable
evidence waits for the normal full-interval phase. `require_source` and explicit
manual operations bypass the startup delay because their caller chose to wait.

Transient retry uses bounded decorrelated jitter from one second to 15 minutes.
A successful request resets request retry. An SSE connection resets reconnect
delay only after one 60-second healthy liveness window, so a flapping endpoint
does not create a tight loop. After repeated SSE failures, conditional fallback
polling uses a stable 15-minute phase rather than a one-minute fleet ticker.

`Retry-After` and rate-reset timestamps are hard not-before values. The client
adds post-boundary jitter of up to five minutes and never retries earlier.
Without a secondary-rate-limit header, GitHub retry waits at least one minute.
The GitHub source serializes calls and uses conditional requests.

Each source or provider gets at most three transient retries in one cycle.
Authentication and authorization failures wait for credential change or the
next normal phase. They do not enter the transient loop. This retry budget
prevents one automatic cycle from multiplying requests without bound.

One runtime is single-flight. A manual call joins the compatible active run or
returns its operation ID. A complete refresh can subsume a source-only or
acquisition-only request before publication.

Replicas that share durable layer state use a distributed lease and one
refresh owner. Other replicas consume accepted state. Large fleets use a
central Starmap source and SSE. One fleet owner polls provider status pages and
reconciles shared secrets when possible.

The lease has a 90-second time to live. The owner renews it every 30 seconds.
Acquisition returns a lease epoch. The owner carries the epoch in the run
record and in the accepted-head commit. The compare-and-swap rejects a stale
epoch.

An owner that loses the lease cancels its run within one renewal interval. It
discards the run results, reports `lease_lost`, and retries at the next phase.
A deployment without shared storage needs no lease.

The Starmap runtime owns the lease contract. A replicated central Starmap
fences its durable generation commit with the epoch. Starport keeps its own
candidate-to-accepted transaction, which carries the epoch and rejects a stale
one.

No fixed jitter window can make an unbounded fleet safe. The minimum admission
window is:

```text
window seconds >= replica count / allowed source requests per second
```

Ten thousand cold instances across 15 minutes still average about 11 initial
requests per second. One hundred thousand hourly pollers still average about 28
requests per second. If the source cannot accept that rate, the deployment must
increase the window or use a lease-owning central Starmap tier. Jitter removes
bursts. It does not remove request volume.

Starmap sources therefore cache immutable payloads, support ETags, bound fetch
and SSE admission, and return `Retry-After` with `429` or `503`. Large services
can place immutable payloads behind a CDN or object store. Public local setup
does not require a GitHub token. Large fleets use an optional source token or,
preferably, the central Starmap pattern. This pattern stops one NAT from
spending GitHub's unauthenticated hourly budget for every replica.

A direct GitHub consumer budgets its requests from the `x-ratelimit-limit`,
`x-ratelimit-used`, `x-ratelimit-remaining`, and `x-ratelimit-reset` headers.
The runtime records the measured requests per refresh cycle. The fleet
capacity is the remaining budget minus a reserved headroom, divided by the
measured requests per cycle. Status warns when `used` passes 80 percent of
`limit`.

The GitHub documentation states ceilings such as 60 unauthenticated requests
per hour per address and 5,000 per token. A ceiling is not a safe threshold.
A fleet above its budget uses authenticated conditional polling or the central
Starmap source.

## Deployment journeys

### Local developer

Developers need no settings. Embedded state works immediately. With no network
and no keys, source fallback is visible. Provider skips are neutral, and no
provider request occurs. With network but no keys, only declared public
provider endpoints run. With one explicit acquisition key, only that provider
and public provider endpoints run.

Cold automatic network work starts within the stable 15-minute spread.
`Refresh` and `Sync` remain the immediate paths.

Tests and air-gapped programs set the embedded source and disable acquisition:

```bash
STARMAP_CATALOG_SOURCE=embedded
STARMAP_CATALOG_ACQUISITION_ENABLED=false
```

### Single Starport

Starport needs no catalog settings. It starts with embedded state, follows the
public channel, and reads explicit catalog credentials. Manual
refresh remains available. Durable layers and the accepted runtime survive
restart. The console shows source, acquisition, provenance, freshness, and
accepted changes. A slow advancing transfer reports progress and retains the
accepted catalog instead of failing on elapsed time.

### Enterprise source with local replica acquisition

The central Starmap uses its defaults and explicit catalog credentials. Each
Starport replica uses:

```bash
STARPORT_CATALOG_SOURCE=starmap
STARPORT_CATALOG_SOURCE_URL=https://catalog.corp.example/api/v1
```

Replica acquisition stays enabled. A replica with explicit catalog credentials
derives its own effective generation. Status shows both upstream and effective
generation IDs.

### Enterprise central-only acquisition

Replicas use:

```bash
STARPORT_CATALOG_SOURCE=starmap
STARPORT_CATALOG_SOURCE_URL=https://catalog.corp.example/api/v1
STARPORT_CATALOG_ACQUISITION_ENABLED=false
```

Only the central Starmap collects provider observations. A private source never
falls back to public GitHub. The central runtime uses a refresh lease. Replicas
spread initial and outage reconnects across 15 minutes, prefer SSE, and use
15-minute stable conditional polling only as fallback. The central source
returns admission and retry guidance under saturation.

## Runtime status contract

`RuntimeStatus` is immutable caller-owned data. It contains:

- `usable`, runtime `phase`, overall `health`, and active effective generation.
- Effective digest, sequence, `generated_at`, and `activated_at`.
- Selected source kind and safe identity.
- Direct source health observed by this runtime.
- Sanitized upstream-reported source chain and provenance, labeled as
  upstream-reported.
- Upstream and effective generation IDs.
- `published_at`, `channel_updated_at`, `checked_at`, `observed_at`,
  `last_attempt_at`, `last_success_at`, and `next_attempt_at` where applicable.
- Source and acquisition in-progress state, operation kind, and run ID.
- Current stage, bytes or pages completed, `last_progress_at`, configured idle
  timeout, optional total deadline, scheduled reason, and retry-not-before.
- Eligible, attempted, succeeded, skipped, and failed provider counts.
- Sanitized provider outcomes with last-known-good retention.
- Fallback and last-known-good state.
- Configured freshness policy, current server evaluation, and age.

Liveness, usability, freshness, and fallback are independent. A stale or
disconnected source does not make a retained usable catalog unusable.

Each provider outcome contains provider ID, outcome, safe reason code,
observation time, last attempt, last success, next attempt, and
retained-last-known-good state. It does not claim inference availability.

Source chains contain stable runtime identities, upstream generation IDs, and
a bounded hop list. A runtime rejects self-reference, a repeated identity, and
an excessive hop count before activation. URL comparison alone cannot detect
load-balancer or DNS aliases.

## Starport API and operator UI

Keep the safe `GET /api/v1/catalog` and `GET /api/v1/catalog/changes` surfaces
for `models:read`. Add admin-only operations:

| Method and path | Contract |
| --- | --- |
| `GET /api/v1/admin/catalog/status` | detailed runtime, direct source, upstream report, acquisition, freshness, and provenance |
| `POST /api/v1/admin/catalog/refresh` | start or join a complete refresh; return `202` with an operation ID |
| `GET /api/v1/admin/catalog/refreshes/{run_id}` | return active or completed operation state and report |
| `DELETE /api/v1/admin/catalog/refreshes/{run_id}` | request cancellation of the active operation and audit the actor |

A refresh report contains run ID, operation kind, start, completion, and
duration. It gives prior, current, and upstream generation IDs. It also gives
change and activation flags, source result, provider outcomes, retained state,
next attempts, current transfer progress, and the last progress time. Manual
start, cancellation, and completion enter the existing admin audit trail.
Overlapping refreshes join one run.

The console shows:

1. Effective catalog identity, digest, generated and activated times, age,
   server-evaluated freshness, and model, provider, and offering counts.
2. Embedded, selected upstream, and operator-acquisition rows with safe
   identity, direct or upstream-reported health, generations, times, and
   fallback.
3. Acquisition policy and provider outcome counts. Show last attempt, last
   success, next run, active stage, and last progress.
4. Provider outcomes and safe reasons. The UI collapses neutral skipped rows by
   default.
5. Accepted-generation changes and distinct candidate, accepted, rejected, or
   pending route-validation state.
6. Model and offering provenance where field evidence exists.
7. Catalog lifecycle, credential-specific availability, and routing state as
   separate concepts.

The server evaluates freshness. The API, metrics, alerts, and console use the
same evaluation. Acquisition status does not appear on inference credential or
account BYOK screens.

## Catalog lifecycle and Starport routing

Starmap owns canonical model lifecycle, compatibility facts, and declared
successors. Starport owns traffic policy. Add a historical `retired` model
state, `retired_at`, and optional `replaced_by` without deleting the old model.
Default active views can hide retired models. Audit and request interpretation
must retain them.

The routing order is:

1. Apply explicit account, key, tenant, or deployment policy to the request.
2. Route the requested model when it is active, locally available, and allowed.
3. Apply the caller's or operator's explicit fallback chain.
4. Apply a catalog-declared successor only when the request or operator policy
   permits successor fallback.
5. Apply an operator-defined capability-compatible fallback only when policy
   permits it.
6. Return an actionable failure.

The explicit `auto` request remains allowed to select any eligible route. A
named model request never gets an arbitrary nearest-model substitution.

Starport already records requested and used model, provider, API key, account,
team, usage, cost, latency, attempts, and credential source. Extend the record
with the routing reason and applied policy. Safe reasons include
`provider_retired`, `credential_unavailable`, `operator_denied`,
`quota_exceeded`, and `budget_exceeded`.

## Hidden effects

1. Retained layers need their own durable compare-and-swap contract. Effective
   generations cannot restore those inputs.
2. Provider retention needs provider-scoped evidence, checksums, expiry, and
   garbage collection.
3. Source identity is part of retained-layer identity. Reconfiguration retires
   old source health and cannot reuse the old layer as the new source.
4. A source update can change provider credential metadata. Complete refresh
   stages source state before it resolves provider eligibility.
5. A no-change channel run changes health but not catalog identity. Health and
   generation persistence are separate.
6. A downstream with local observations derives a new effective identity. It
   cannot reuse the upstream generation ID.
7. Starport can coalesce pending candidates, but it cannot advance its accepted
   head before connector and routing validation completes.
8. Shared Starport storage needs accepted-head compare-and-swap and idempotent
   candidate handling in addition to the Starmap refresh lease.
9. Source chains disclose topology. Upstream reports and admin APIs must use
   safe identities and bounded detail.
10. Reason sanitization must happen before logging as well as before status
    serialization because provider errors can contain credentials.
11. Metrics use bounded source-kind, operation, stage, and outcome labels.
    URLs, providers, generations, run IDs, and error text are not labels.
12. Split route timeouts require middleware ownership before handlers. Clearing
    an HTTP write deadline does not remove a parent context deadline.
13. Stable phase needs a stable instance identity. Ephemeral identities create
    restart storms.
14. Catalog successor data changes request behavior only after an explicit
    Starport policy opt-in. Publishing metadata alone does not reroute traffic.
15. `http.Client.Timeout` cannot coexist with progress-aware large-body
    transfers because it includes response-body reads.
16. Removing the total refresh deadline requires network work to stay outside
    the mutation lock. A slow provider must not suppress completed provider
    publication or source and acquisition independence.
17. Idle progress alone permits a trusted endpoint to transfer very slowly.
    Size, page, and record ceilings bound that tradeoff. Cancellation,
    single-flight, and optional operator wall-clock limits add further bounds
    without rejecting healthy slow links by default.
18. A 15-minute spread is a default, not a fleet capacity proof. Deployments
    calculate their required window and use a central lease owner when the
    result exceeds source capacity.
19. Multi-hop fallback polling can add one 15-minute interval per degraded hop.
    Every downstream evaluates the propagated `channel_updated_at`, not only
    its direct source check time.
20. Backoff resets only after stable success. Resetting on a TCP connection or
    first SSE header creates reconnect storms when a source flaps.

## Plan, verifier, and acceptance changes

CAT2 records CAT-D10 through CAT-D13. It then makes the distribution verifier
red. The verifier rejects old acquisition mode and `ON_START` names. It proves
the public default, four-hour acquisition, manual `Sync`, neutral missing
credentials, and public endpoint attempts. It also proves provider retention,
retained-layer rebuild, startup policy, single-flight, async operations,
bounded shutdown, channel heartbeat, source-chain safety, safe status, and
historical tag readback. Transport cases prove slow progress, idle failure,
size rejection, caller cancellation, stable spread, retry not-before, and lease
ownership.

CAT3 adds the canonical release and channel document and advances the
no-change heartbeat. It uses the selected cadence at minute 17 and nests the
90-minute job and 75-minute step limits above the 60-minute transfer bound.

CAT4 implements conditional GitHub discovery and Sigstore verification. It
adds durable sequence and ETag state, progress-aware transfer, optional
authentication, rate-limit handling, full-interval phase, 15-minute cold
spread, and bounded decorrelated retry.

CAT5 implements the retained-layer runtime, provider evidence, and credential
outcomes. It removes the default whole-refresh deadline. It adds transport-stage
and two-minute idle-progress bounds, page and record ceilings, per-provider
progress, single-flight, stable phase, durable state, and five-second join. It
also adds runtime status, historical model lifecycle, and successor metadata.

CAT6 maps the canonical environment into `Open` for the CLI, server, and
container. It proves embedded-first startup, durable restart, default network
behavior, deterministic opt-out, asynchronous cancellable Starmap admin
refresh, and slow-client catalog delivery.

CAT7 adapts Starmap HTTP and SSE into the same runtime. It adds safe source
chains and separates direct from upstream health. It also adds a 20-second
heartbeat, 60-second liveness, 15-minute cold spread, one-second to 15-minute
decorrelated retry, `Retry-After`, stable fallback polling, source admission,
and chain rejection.

CAT8 replaces Starport's two runtimes with one adapter. It injects the exact
deployment acquisition lookuper and preserves candidate and accepted heads. It
removes source-acquisition exclusion and enables acquisition in development.
It consumes `Updates`, adds async admin operations and audit, and splits route
timeouts. It adds progress and cancellation, slow-transfer and long-stream
proof, safe status, provenance, routing reasons, and lease-based scale-out
proof.

CAT9 documents direct pre-v1 Starport environment replacements. It does not add
runtime aliases. It documents retained historical GitHub tag compatibility.

Acceptance tests cover:

- Offline local: immediate embedded use, no provider request, visible fallback.
- Connected without keys: public update, neutral skips, no credential warning.
- Connected with one key: only eligible and public providers run.
- Single Starport: default startup, durable restart, manual operation, accepted
  changes, status, and UI.
- Enterprise: central source, optional replica acquisition, and central-only
  opt-out. Prove no public fallback, separate IDs, chain rejection, a lease,
  and three replicas.
- Timing: a source body that progresses beyond 30 seconds succeeds. A complete
  refresh that progresses beyond five minutes also succeeds.
- Limits: a two-minute idle body and an oversized body fail. Caller
  cancellation wins. Pagination stays bounded.
- Operations: prove async HTTP, route-specific inference, stream lifetime, slow
  response writes, shutdown, stable phase, and 15-minute outage spreading.
  Also prove `Retry-After`, retry budgets, source admission, and a distributed
  lease.

## Owner decisions

1. The owner accepted CAT-D10. Publish every four hours and use a six-hour
   end-to-end freshness objective.
2. The owner accepted CAT-D11. Use `CATALOG_ACQUISITION_ENABLED` plus
   `CATALOG_ACQUISITION_INTERVAL`. A false enabled value means manual-only.

## Evidence

The review inspected the Starmap workflow, constants, provider clients,
subscriber, server deadlines, model lifecycle, and release names. It also
inspected Starport configuration, validation, catalog runtimes, startup, and
acceptance. The HTTP, execution, routing, usage, status, admin, and console
paths supplied the remaining evidence.

GitHub CLI supplied the latest 20 successful publisher runs. GitHub's primary
documentation supplied scheduled-workflow, job-timeout, rate-limit,
conditional-request, and retry behavior. Go documentation supplied exact HTTP
timeout semantics. AWS and Google Cloud supplied retry and jitter guidance:

- <https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows>
- <https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax>
- <https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api>
- <https://docs.github.com/en/enterprise-cloud@latest/rest/using-the-rest-api/best-practices-for-using-the-rest-api>
- <https://pkg.go.dev/net/http#Client>
- <https://pkg.go.dev/net/http#Transport>
- <https://pkg.go.dev/net/http#ResponseController.SetWriteDeadline>
- <https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/>
- <https://docs.cloud.google.com/storage/docs/retry-strategy>
