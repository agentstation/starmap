# CAT2 final runtime and operations review

Date: 2026-09-01

Reviewed baselines:

- Catalog plan: `codex/catalog-publisher-six-hour` at `8e5ddf6a`.
- Starmap main worktree: `codex/security-baseline` at `fa7c26bb`.
- Starport: `main` at `6db57d8c`. The worktree contains unrelated owner
  changes.

No production source changed during this review. This proof records the final
repository-grounded contract before CAT2 verification and CAT5-CAT8 work.

## Verdict

The retained-layer runtime, connected `starmap.Open` API, orthogonal source and
acquisition policies, Starport acceptance boundary, and detailed operator
status are correct. Two owner choices remain:

1. Replace `CATALOG_ACQUISITION_ON_START` with the simpler
   `CATALOG_ACQUISITION_ENABLED`. This review recommends `ENABLED` because it
   gives one complete automatic-work opt-out. Stable phasing and durable
   freshness remove the main reason for a periodic-only state.
2. Keep the six-hour publisher and call seven hours a nominal objective, with
   a warning after eight hours. A six-hour end-to-end objective needs the
   publisher to run every four hours.

## Steering report verification

| Section | Verdict | Repository evidence or correction |
| --- | --- | --- |
| 1. Runtime contract | agree | Current Starmap has separate offline, import, and remote subscriber paths. Current Starport has separate local and remote runtimes. One retained-layer runtime removes both split meanings without moving Starport's accepted head. |
| 2. Credential outcomes | agree | The provider source currently calls all catalog endpoints. Missing credentials become aggregate issues. It needs neutral preflight skip and provider-scoped retention. |
| 3. Acquisition DX | agree with the proposed simplification | `ON_START` plus `INTERVAL` exposes a periodic-only state, but no repository use case requires that public state. Stable phase, fresh durable state, and lease ownership solve delayed startup work without a second Boolean. Keep this choice pending until the owner accepts the replacement. |
| 4. Catalog, availability, routing | agree with corrections | Starport already separates catalog offering state, credential state, and routing. It already supports explicit fallback lists, model overrides, quotas, and budgets. Starmap model status has no `retired` state or successor field, so historical model lifecycle and `replaced_by` need schema work. |
| 5. Measured timing | mostly agree | The 20-run median is 222.5 seconds, or 3 minutes 42.5 seconds, not 3 minutes 44 seconds. Current Starport execution has zero same-route retries by default. Other measured constants match the source. |
| 6. CAT-D10 | agree | Six-hour publication plus one-hour polling is a nominal seven-hour path before execution delay, network delay, and retries. It is not a hard bound. |
| 7. Target timing | agree with one correction | Streaming needs no HTTP-wide or request-wide deadline after commitment, but it still needs first-token, stream-idle, and explicit operator cancellation. Current execution applies a hard two-minute context to streams and must change. |
| 8. Anti-herd | agree | Current fixed tickers do not provide a stable fleet phase. The target needs deterministic phase, bounded full-jitter retry, rate-limit handling, and one lease owner for shared state. |
| 9. Status and timestamps | agree | Current generation metadata does not express channel heartbeat, operation state, provider outcome, or direct versus upstream-reported health. |
| 10. Starport API and UI | agree with one correction | The current admin refresh is synchronous. The current console does not contain the reported hard-coded seven-day stale badge. Add server-evaluated freshness; do not describe the change as removal of a current badge. |
| 11. Deployment examples | agree | Current Starport validation prevents the required remote-source plus local-acquisition composition. The target cases prove that source selection and acquisition are independent. |
| 12. Repository gaps | agree with one correction | All listed gaps exist except the current seven-day badge. Historical release tags remain a read compatibility requirement. Runtime environment aliases do not. |
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
| `CATALOG_SOURCE_MAX_AGE` | `8h` | Source freshness warning budget |
| `CATALOG_SOURCE_MAX_HOPS` | `8` | Maximum accepted Starmap source-chain depth |
| `CATALOG_ACQUISITION_ENABLED` | `true` | Pending CAT-D11: enables automatic startup and interval work |
| `CATALOG_ACQUISITION_INTERVAL` | `4h` | Normal cadence; `0s` means startup only while enabled |
| `CATALOG_WORKSPACE_PATH` | empty | Reviewed operator catalog input |
| `CATALOG_REFRESH_TIMEOUT` | `5m` | Complete manual or automatic refresh deadline |

There is no `CATALOG_ACQUISITION`, `CATALOG_ACQUISITION_MODE`,
`CATALOG_ACQUISITION_ON_START`, or acquisition schedule expression in the
recommended contract.

| Enabled | Interval | Automatic behavior |
| --- | --- | --- |
| `true` | `4h` | automatic startup work and periodic work; default |
| `true` | `0s` | one startup pass |
| `false` | any value | manual only; `Sync` remains available |

Automatic startup work does not always mean an immediate provider request. A
runtime with fresh durable observations can wait for its stable phase. A new
runtime, a new credential, or stale evidence can run after a short jitter.

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

The proposed option is `WithAcquisitionEnabled(bool)`. Do not keep both it and
`WithAcquisitionOnStart`. Source and acquisition options remain independent.

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
| Publisher cadence | Main runs daily at `03:17` UTC. The plan branch uses minute 17 every six hours. | CAT3 keeps six hours unless CAT-D10 selects four hours. GitHub owns dispatch and can delay or drop a run. |
| Publisher execution | No job timeout; GitHub default is 360 minutes. The latest 20 successes were 153-285 seconds, median 222.5 seconds, p95 284 seconds. | CAT3 sets `timeout-minutes: 20`. GitHub cancels the job at the bound. |
| Complete Starmap sync | `sync.Options` defaults to five minutes. | `Refresh` owns a five-minute child context. Caller cancellation wins. |
| Starport catalog refresh | Two minutes. | Unified runtime uses the five-minute complete-cycle bound. |
| Ordinary provider HTTP | 30-second Starmap default. | Provider adapter owns 30 seconds within the complete-cycle context. |
| Google Vertex list | Two-minute paginated list context. | Keep provider-specific two minutes within the five-minute complete cycle. |
| Provider concurrency | Maximum five. | Keep bounded; add stable provider offsets before admission. |
| Sync cleanup | 30 seconds. | Keep separate from runtime shutdown; cleanup cannot extend the completed operation indefinitely. |
| Catalog projection or repair | One minute. | Remains inside the complete-cycle budget where possible; report a distinct stage timeout. |
| Durable catalog load | 10 seconds. | Construction stage owns 10 seconds; caller cancellation wins. |
| models.dev HTTP and cache | 30-second request and one-hour cache validity. | Keep separate from released-catalog polling policy. |
| Release or GitHub source fetch | Starmap uses 30 seconds. | Source adapter owns 30 seconds and conditional request state. |
| Starport direct Starmap fetch | Two minutes. | Unified source adapter uses 30 seconds unless measured evidence requires an override. |
| Public source poll | No default connected runtime. | One hour at a stable per-instance phase. A manual refresh can run sooner. |
| Local acquisition | Starport defaults off; Starmap has explicit `Sync`. | Enabled by default, four-hour stable phase, with stale or new state startup rules. |
| Starmap SSE | 20-second heartbeat, 60-second liveness, 100 ms to 5-second half-to-full jitter reconnect. | Keep heartbeat and liveness. Use full or decorrelated jitter, cap near 30 seconds, honor `Retry-After`, then use one-minute conditional polling after repeated failure. |
| Remote subscriber shutdown | Five seconds. | Keep five seconds and join owned work. |
| Starmap server shutdown grace | 100 ms. | Increase background-worker join to five seconds. HTTP server shutdown remains independently bounded. |
| Starmap server HTTP | 10-second read, 10-second write, 120-second idle. | Streaming routes clear total write deadlines and own first-event and idle policy. |
| Starmap admin update | Synchronous with a six-minute write deadline. | Return a refresh operation within five seconds; run it for at most five minutes. |
| Starport admin refresh | Synchronous, two-minute work under a global 60-second request context. | Return or join a `202` operation within five seconds; background work owns five minutes. |
| Starport HTTP | 30-second read and write, 120-second idle, global 60-second request context. | Control-plane routes keep 60 seconds. Inference and streaming routes use route-specific policy. |
| Starport non-stream inference | Executor allows two minutes, but the global request context cancels after one minute. | Route context owns two minutes. Provider header wait remains 30 seconds. Caller cancellation wins. |
| Starport streaming | Controller clears the 30-second write deadline, but global middleware cancels at 60 seconds and executor cancels at two minutes. | No total route or write deadline after stream commitment. Use first-token, stream-idle, disconnect, and operator cancellation bounds. |
| Starport retry | Three total attempts, zero same-route retries by default, 100 ms exponential settings unused unless enabled. | Routing policy owns retry eligibility; all attempts remain inside the route budget. |
| Inference transport | 30-second dial, 10-second TLS, 30-second response-header timeout, 90-second idle connection. | Keep as transport-stage bounds; execution context owns total non-stream work. |
| Direct secret cache | Five-minute refresh. | Add stable fleet phase and shared ownership where available. |
| Credential reconciliation | One-minute interval with a 10-second timeout. | Add stable phase; one owner reconciles shared deployment state. |
| Provider status pages | One-minute interval, five-second request, concurrency eight. | One fleet owner or shared cache; replicas consume the shared projection. |

The earliest deadline always wins. A caller cancellation stops its operation.
The five-minute complete-cycle deadline bounds all stages. A stage deadline can
shorten that bound, but it cannot extend it. Runtime background work uses the
same stage limits and stops when `Close` cancels the runtime context.

The publisher run measurements are in
[`cat2-publisher-runs.json`](cat2-publisher-runs.json). The sample supports a
20-minute workflow timeout with more than four times the observed maximum. It
does not establish a worst-case service guarantee.

## CAT-D10 freshness wording

The six-hour publisher plus one-hour poll is a **nominal seven-hour end-to-end
freshness objective**. It is not a bound. Workflow dispatch delay, a three-to-
five-minute run, transport, conditional poll phase, and retry can extend it.

The recommendation is:

- Keep the six-hour publisher at minute 17.
- Warn when `channel_updated_at` is older than eight hours.
- Mark publisher channel health critical after 14 hours.
- Warn when a consumer source check is older than 90 minutes and mark it
  critical after two hours.
- Warn when eligible acquisition success is older than five hours and mark it
  critical after eight hours.

If the product name must say "six-hour end-to-end objective," run the publisher
every four hours and keep one-hour polling. That gives a nominal five-hour path
and one hour of operational margin. Neither cadence is a mathematical
guarantee. GitHub can delay or drop scheduled work.

`channel_updated_at` advances after each successful publisher verification,
including no-change runs. The signed channel sequence advances too. The
immutable `published_at` and `generated_at` do not change. A consumer `304`
advances `checked_at` only.

## Staggering, retry, and fleet coordination

Each periodic controller uses a stable phase:

```text
phase = hash(instance identity + source or provider identity) mod interval
```

The runtime persists or derives a stable instance identity. A restart keeps
the same phase. A process without durable state, a `require_source` start, a
new credential, or stale evidence can run after a short bounded jitter.

Retry uses full jitter and respects provider or GitHub `Retry-After` and rate
reset values. The normal transient sequence is approximately one minute, five
minutes, 15 minutes, one hour, then the normal four-hour cadence. Authentication
failures wait for credential change or the normal cycle. They do not use the
transient retry loop.

One runtime is single-flight. A manual call joins the compatible active run or
returns its operation ID. A complete refresh can subsume a source-only or
acquisition-only request before publication.

Replicas that share durable layer state use a distributed lease and one
refresh owner. Other replicas consume accepted state. Large fleets use a
central Starmap source and SSE. One fleet owner polls provider status pages and
reconciles shared secrets when possible.

Public local setup does not require a GitHub token. Large fleets can use the
optional source token and central Starmap pattern. Conditional requests and a
stable phase also prevent a NAT fleet from consuming the unauthenticated
60-request hourly budget in a burst.

## Deployment journeys

### Local developer

Developers need no settings. Embedded state works immediately. With no network
and no keys, source fallback is visible. Provider skips are neutral, and no
provider request occurs. With network but no keys, only declared public
provider endpoints run. With one explicit acquisition key, only that provider
and public provider endpoints run.

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
accepted changes.

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
prefer SSE and use bounded conditional polling only as fallback.

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

A refresh report contains run ID, operation kind, start, completion, and
duration. It gives prior, current, and upstream generation IDs. It also gives
change and activation flags, source result, provider outcomes, retained state,
and next attempts. Manual start and completion enter the existing admin audit
trail. Overlapping refreshes join one run.

The console shows:

1. Effective catalog identity, digest, generated and activated times, age,
   server-evaluated freshness, and model, provider, and offering counts.
2. Embedded, selected upstream, and operator-acquisition rows with safe
   identity, direct or upstream-reported health, generations, times, and
   fallback.
3. Acquisition policy, eligible, attempted, succeeded, skipped, and failed
   counts, plus last attempt, last success, and next run.
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

## Plan, verifier, and acceptance changes

CAT2 must resolve CAT-D10 and CAT-D11. It then makes the distribution verifier
red. If the owner accepts CAT-D11, the verifier rejects old acquisition mode
and `ON_START` names. It proves the public default, four-hour acquisition,
manual `Sync`, neutral missing credentials, and public endpoint attempts. It
also proves provider retention, retained-layer rebuild, startup policy,
single-flight, async operations, bounded shutdown, channel heartbeat,
source-chain safety, safe status, and historical tag readback.

CAT3 adds the canonical release and channel document, advances the no-change
heartbeat, uses the selected cadence at minute 17, and sets a 20-minute job
timeout.

CAT4 implements conditional GitHub discovery, Sigstore verification, durable
sequence and ETag state, 30-second requests, optional authentication,
rate-limit response handling, stable phase, and bounded full-jitter retry.

CAT5 implements the retained-layer runtime, provider evidence, and credential
outcomes. It adds the five-minute refresh, nested provider timeouts,
single-flight, stable phase, durable state, and five-second join. It also adds
runtime status, historical model lifecycle, and successor metadata.

CAT6 maps the canonical environment into `Open` for the CLI, server, and
container. It proves embedded-first startup, durable restart, default network
behavior, deterministic opt-out, and asynchronous Starmap admin refresh.

CAT7 adapts Starmap HTTP and SSE into the same runtime. It adds safe source
chains, direct versus upstream health, 20-second heartbeat, 60-second liveness,
30-second reconnect cap, `Retry-After`, one-minute polling fallback, and cycle
and hop rejection.

CAT8 replaces Starport's two runtimes with one adapter. It injects the exact
deployment acquisition lookuper and preserves candidate and accepted heads. It
removes source-acquisition exclusion and enables acquisition in development.
It consumes `Updates`, adds async admin operations and audit, and splits route
timeouts. It also adds safe status, provenance, routing reasons, and
lease-based scale-out proof.

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
- Timing: five-minute refresh, 30-second source fetch, and Vertex nested
  timeout. Prove async HTTP, route-specific inference, stream lifetime,
  shutdown, stable phase, and retry headers.

## Remaining owner questions

1. Accept `CATALOG_ACQUISITION_ENABLED=true` plus
   `CATALOG_ACQUISITION_INTERVAL=4h`, with `false` meaning manual-only, and
   remove `CATALOG_ACQUISITION_ON_START`?
2. Keep the six-hour publisher and a nominal seven-hour objective? This choice
   warns after eight hours. Otherwise, use a four-hour publisher for a
   six-hour end-to-end objective.

## Evidence

The review inspected the Starmap workflow, constants, provider clients,
subscriber, server deadlines, model lifecycle, and release names. It also
inspected Starport configuration, validation, catalog runtimes, startup, and
acceptance. The HTTP, execution, routing, usage, status, admin, and console
paths supplied the remaining evidence.

GitHub CLI supplied the latest 20 successful publisher runs. GitHub's primary
documentation supplied scheduled-workflow, job-timeout, rate-limit,
conditional-request, and retry behavior:

- <https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows>
- <https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax>
- <https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api>
- <https://docs.github.com/en/enterprise-cloud@latest/rest/using-the-rest-api/best-practices-for-using-the-rest-api>
