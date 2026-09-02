# CAT2 connected runtime and Starport contract review

Date: 2026-09-01

Baseline: `codex/catalog-publisher-six-hour` at `8e5ddf6a`

The [final runtime, transport, and operations review](cat2-final-review.md)
supersedes this file where it records newer timing evidence or repository
corrections.

## Verdict

The revised framing is correct. Starmap must own one connected runtime that
retains and composes four independent states:

```text
embedded public catalog
        ↓
selected upstream catalog
        ↓
retained operator observations
        ↓
effective catalog
```

`starmap.New` and `starmap.NewContext` stay deterministic and offline.
`starmap.Open` becomes the recommended connected entry point. `Catalog`,
`State`, and `Status` remain non-I/O reads. A catalog read must not return an
error or status tuple.

Provider catalog acquisition is automatic when Starmap finds usable
catalog-acquisition credentials. Missing credentials are normal. The owner
accepted `CATALOG_ACQUISITION_ENABLED` and
`CATALOG_ACQUISITION_INTERVAL`. There is no mode or on-start setting.

Starport must replace its mutually exclusive local and remote runtimes with one
adapter over the Starmap runtime. Starmap produces effective candidates.
Starport keeps its separate candidate and accepted heads and its complete
routing acceptance transaction.

## Verified agreements

The repository supports these conclusions:

1. `Client.Catalog()` already provides a non-failing, immutable, O(1) read.
   `CurrentCatalogState()` correlates the catalog with generation
   identity, checksum, generated time, and local sequence.
2. `New` and `NewContext` load embedded, durable, and workspace state without
   starting network work. They are the correct offline constructors.
3. `RefreshSource` is the correct source-only name. The selected source can be
   public GitHub, custom GitHub, a Starmap server, a file, or embedded state.
4. Source selection and provider acquisition are orthogonal. A custom upstream
   cannot disable local provider acquisition by implication.
5. A custom source replaces the public upstream. It must not trigger a hidden
   public GitHub fallback request.
6. Starmap, not Starport, owns source verification, catalog acquisition,
   retained layers, reconciliation, and the effective generation.
7. Starport already owns the correct final acceptance boundary. It builds
   connectors and routes, validates the complete candidate, moves the accepted
   head, and publishes one complete routable snapshot.
8. An HTTP manual refresh should be asynchronous. Starport allows a two-minute
   catalog refresh but applies a one-minute request timeout to the current
   synchronous handler.
9. Starport is pre-v1 and requires direct breaking changes. The implementation
   must replace old environment names without a runtime alias period. The
   migration guide still names each replacement. GitHub keeps historical
   catalog tags readable because they contain published data, not application
   configuration.

## Verified disagreements and corrections

The earlier CAT2 draft has these incorrect statements:

- It defines `CATALOG_ACQUISITION=disabled|manual|scheduled`. Remove the
  setting and the three policy constructors.
- It defaults startup acquisition to false and the interval to zero. Both
  Starmap and Starport default to startup acquisition and a four-hour interval.
- It says Starport disables local acquisition when it follows a Starmap
  server. The source and acquisition policies must compose.
- It uses `RefreshPublicCatalog`. Rename it to `RefreshSource`.
- It makes `prefer_source` wait for a source operation. The default returns as
  soon as embedded or durable state is usable. It schedules connected work at
  a stable phase and reports `warming`.
- It proposes a configuration alias period for Starport. This conflicts with
  Starport's direct-breaking policy.
- It treats `published_at` as evidence that the scheduled publisher is alive.
  An unchanged catalog can keep one immutable release for weeks. Channel
  health needs a separate heartbeat.
- It cannot support the six-hour consumer freshness objective with a six-hour
  publisher. The owner selected a four-hour publisher and one-hour poll.

## Current Starmap gaps

The current source pipeline does not implement the target runtime:

- `acquisition.Syncer.Sync` reconciles against the current effective catalog.
  It does not retain embedded, upstream, and operator inputs independently.
- `ImportRelease` adds a `release_artifact` observation. It does not establish
  a replaceable upstream layer.
- `remote.Subscriber` exact-activates a complete upstream generation. It does
  not compose it with local operator observations.
- The provider source enumerates every provider with a catalog endpoint and
  attempts every fetch. A missing credential becomes a
  `missing_credentials` issue and degrades the aggregate observation.
- The provider source produces one `providers` observation. It cannot retain
  one successful provider while carrying the last-known-good observation for a
  different failed provider.
- Publisher provider evidence and local operator provider evidence use the
  same `providers` source identity.
- `GenerationManifest.SourceObservations` binds direct observations, but it
  does not carry a sanitized transitive source chain or runtime instance
  identity for cycle detection.
- The root package cannot import `acquisition` because `acquisition` imports
  the root client. The runtime needs an internal observation and reconciliation
  boundary. The public acquisition facade can remain for advanced callers.

The runtime therefore needs a durable layer store in addition to the immutable
effective catalog store. The layer store keys state by source identity and, for
operator acquisition, provider ID. A source configuration change must not
reuse retained state or health from another identity.

## Current Starport gaps

Starport has two mutually exclusive runtimes:

| Path | Source work | Acquisition work | Acceptance |
| --- | --- | --- | --- |
| local | none | provider and workspace sync | activates the local state |
| remote | Starmap fetch and SSE | none | keeps remote and accepted heads |

The remote method named `Sync` does no I/O. It only returns the current
subscriber state. The local `Sync` queries providers. One adapter
must replace both meanings.

Current configuration also conflicts with the target:

- `RefreshOnStart` defaults to false.
- `RefreshInterval` defaults to `0s`.
- `starport dev` forcibly sets both values to disabled.
- validation rejects a remote source with a workspace or acquisition.
- the runtime constructs Starmap's default `os.LookupEnv` resolver instead of
  the exact lookuper that loaded process variables and `.env` files.

The existing safe `GET /api/v1/catalog` shows generation metadata, a manifest
summary, direct observation links, and age. The current console flags a
catalog older than seven days with a hard-coded badge in `FreshnessBar.tsx`.
The third review confirmed that rule. Neither surface shows source
health, layer identity, provider outcomes, last attempts, next attempts,
fallback, transitive provenance, or a server-evaluated freshness policy.

## Canonical names and layer rules

The **Starmap public catalog** is the public catalog product. It has two
delivery forms:

| Form | Name | Update point |
| --- | --- | --- |
| compiled payload | embedded public catalog | Starmap binary release |
| verified channel selection | released public catalog | successful catalog publisher run |

**Operator observations** are provider or reviewed local evidence obtained by
the running deployment. The **effective catalog** is the immutable result that
consumers read. It is never an input source.

The runtime retains all input layers independently. After any source or
provider update, it rebuilds the effective catalog from retained inputs. It
must not merge a new input into the prior effective result.

The runtime retains operator observations by provider. A successful provider update
can publish while another provider keeps its last-known-good observation. A
provider list omission is discovery evidence. It is not entitlement or
deletion evidence and cannot remove unrelated public or private facts.

## Credential and provider outcome contract

Provider catalog metadata defines catalog authentication requirements. For
example, DeepInfra already declares a `public` profile. The runtime must use
this metadata. It must not probe a credentialed endpoint to discover whether
authentication is optional.

Each cycle resolves credentials again so rotation takes effect. Starport must
pass Starmap a resolver backed by the same raw deployment lookuper used for
process variables, `.env` files, explicit catalog credential references, and
tests. It must not pass inference material, shared deployment credential
records, or account BYOK. Remote source authentication is also a separate
credential plane.

Provider outcomes are:

| Condition | Outcome | Health effect | Request |
| --- | --- | --- | --- |
| required credential absent | `skipped_not_configured` | neutral | none |
| declared public catalog profile | attempted, then succeeded or failed | failure degrades | yes |
| usable credential and success | `succeeded` | healthy | yes |
| configured reference invalid, denied, or unavailable | `failed` | degrades | none or source read only |
| provider rejects credential | `failed` | degrades | yes |
| transport or schema failure | `failed` | degrades | yes |

`skipped_not_configured` is a runtime acquisition outcome. It is not an
observation issue and does not enter generation evidence because the runtime
observed no provider evidence.

## Go API

The recommended contract is:

```go
runtime, err := starmap.Open(ctx)
if err != nil {
	return err
}
defer runtime.Close()

catalog := runtime.Catalog()
state := runtime.State()
status := runtime.Status()
```

`Open` uses an in-memory writable store when the caller supplies none. Server,
CLI, container, and Starport composition inject durable storage.

Client and runtime options are separate interfaces. Shared option values can
implement both:

```go
runtime, err := starmap.Open(ctx,
	starmap.WithCatalogStore(store),
	starmap.WithCatalogSource(catalogsource.Starmap(
		"https://catalog.example.com/api/v1",
		catalogsource.WithAPIKey(sourceKey),
	)),
	starmap.WithAcquisitionEnabled(false),
	starmap.WithCatalogCredentialResolver(resolver),
)
```

`New` and `NewContext` accept client options. `Open` accepts runtime options.
`WithCatalogStore` and `WithCatalogPath` return shared concrete option values.
The offline constructors reject source, lifecycle, and automatic acquisition
options.

The read methods do not access external systems:

```go
func (r *Runtime) Catalog() *catalogs.Catalog
func (r *Runtime) State() CatalogState
func (r *Runtime) Status() RuntimeStatus
```

`Catalog()` stays non-failing, non-nil after successful `Open`, immutable,
O(1), and safe to retain. `Status()` includes the active effective generation
ID for correlation. Do not add a catalog, error, and status return tuple.

The I/O methods have distinct effects:

```go
func (r *Runtime) Refresh(ctx context.Context) (RefreshReport, error)
func (r *Runtime) RefreshSource(ctx context.Context) (SourceRefreshReport, error)
func (r *Runtime) Sync(ctx context.Context, opts ...SyncOption) (AcquisitionReport, error)
func (r *Runtime) Updates() <-chan CatalogState
func (r *Runtime) Close() error
```

`Refresh` stages one source refresh followed by one provider acquisition pass.
The provider pass uses credential metadata from the new source when the source
succeeds. If the source fails, acquisition can use the retained source layer.
The complete operation publishes one effective candidate when possible.

`RefreshSource` changes only the selected upstream layer. `Sync` changes only
operator observations. Both rebuild against all retained layers. Explicit
calls remain available under every automatic policy.

The runtime owns one mutation coordinator. It serializes source replacement,
provider-layer replacement, effective rebuild, durable commit, and event
publication. Concurrent manual and background requests join the compatible
active run or coalesce into one pending complete refresh. They cannot publish
out of order.

`Updates` carries complete effective candidates for one application
composition consumer. It coalesces to the latest state and includes sequence
and generation identity so a consumer can ignore duplicates. Starport reads
the initial `State`, then consumes `Updates`.

`Close` is idempotent, cancels runtime-owned work, and joins it within five
seconds. `Open` uses its context to bound construction and a
blocking `require_source` startup. The returned resource owns its background
context. It does not retain the caller's context as an operation field.

CAT5 should split the current public functional option type into `ClientOption`
and `RuntimeOption`. This keeps runtime policy out of the offline client.

## Automatic policy and environment

Starmap uses the `STARMAP_` prefix. Starport uses the `STARPORT_` prefix.
The suffixes and defaults are identical.

| Suffix | Default | Meaning |
| --- | --- | --- |
| `CATALOG_SOURCE` | `public` | `public`, `github`, `starmap`, `file`, or `embedded` |
| `CATALOG_SOURCE_URL` | empty | custom Starmap or file/artifact source location |
| `CATALOG_SOURCE_API_KEY` | empty | remote Starmap protocol credential |
| `CATALOG_SOURCE_REPOSITORY` | `agentstation/starmap` | GitHub source repository |
| `CATALOG_SOURCE_CHANNEL` | `catalog-latest` | stable discovery channel |
| `CATALOG_SOURCE_SIGNER_WORKFLOW` | publisher preset | expected GitHub workflow identity |
| `CATALOG_SOURCE_TOKEN` | empty | optional GitHub API token |
| `CATALOG_SOURCE_POLL_INTERVAL` | `1h` | source check interval |
| `CATALOG_SOURCE_STARTUP_POLICY` | `prefer_source` | `prefer_source` or `require_source` |
| `CATALOG_SOURCE_MAX_AGE` | `6h` | source freshness warning objective; readiness gate only under `require_source` |
| `CATALOG_SOURCE_MAX_HOPS` | `8` | maximum accepted Starmap source-chain depth |
| `CATALOG_ACQUISITION_ENABLED` | `true` | enable automatic startup and interval work |
| `CATALOG_ACQUISITION_INTERVAL` | `4h` | normal cadence; `0s` means startup only while enabled |
| `CATALOG_WORKSPACE_PATH` | empty | reviewed operator catalog input |
| `CATALOG_STARTUP_SPREAD` | `15m` | stable cold automatic-work admission window |
| `CATALOG_TRANSFER_IDLE_TIMEOUT` | `2m` | maximum time without body read or response write progress |
| `CATALOG_TRANSFER_MAX_DURATION` | `60m` | maximum duration of one finite HTTP body transfer, zero is invalid |
| `CATALOG_REFRESH_TIMEOUT` | `0s` | optional complete-operation cap; zero adds no deadline |

There is no `CATALOG_ACQUISITION`, `CATALOG_ACQUISITION_ON_START`, acquisition
schedule, or cron expression in the recommended contract.

The transfer maximum applies to one finite HTTP body, such as a manifest, an
archive, or a provider page. It does not bound an SSE subscription lifetime. A
64 MiB body at the 256 Kbps floor rate takes about 35 minutes, and the
60-minute default provides headroom over that case. A zero value is invalid
and fails startup. An operator on a slower link raises the value or lowers the
size cap, and status reports the observed transfer rate.

| Enabled | Interval | Automatic behavior |
| --- | --- | --- |
| `true` | `4h` | automatic startup and periodic work; default |
| `true` | `0s` | startup only |
| `false` | any value | manual only |

`CATALOG_SOURCE=embedded` disables the network source. A deterministic and
offline runtime also sets acquisition enabled to false.

Normal source and acquisition cycles use stable phases across their full one-
and four-hour intervals. Cold, rotated, or stale work uses stable offsets
across 15 minutes. Transient retry uses bounded decorrelated jitter from one
second to 15 minutes. `Retry-After` and rate-reset values are hard not-before
times with post-boundary jitter. Authentication failures wait for credential
change or the normal phase. A successful acquisition returns to the four-hour
cadence.

## Startup behavior

`prefer_source` is the default:

1. Verify and load the latest durable effective generation.
2. Use the embedded public catalog when no durable generation exists.
3. Return a usable runtime immediately.
4. Schedule the selected source check and eligible provider acquisition at
   their stable automatic phases.
5. Report `warming` until the first configured automatic cycle completes.
6. Keep last-known-good layers and catalog when a connected operation fails.

An offline laptop must not wait for connected work.
`require_source` can block within the caller context. It can fail when the
runtime cannot establish an acceptable selected source. Liveness remains
independent from catalog usability and freshness.

## Deployment examples

### Local developer

Developers need no settings. `starmap.Open(ctx)` serves embedded state and starts
the public source and credential-detected acquisition work. With no network and
no keys, it remains usable, reports source fallback, makes no provider request,
and reports neutral skipped providers.

With one key in the process environment or `.env`, the runtime calls only that
eligible provider and catalog-declared public providers.

Deterministic tests and air-gapped programs use:

```bash
STARMAP_CATALOG_SOURCE=embedded
STARMAP_CATALOG_ACQUISITION_ENABLED=false
```

### Standalone Starport

Starport needs no catalog settings. It follows the released public catalog
and automatically uses catalog-acquisition credentials that the Starmap
resolver finds. Manual refresh remains available. Durable layer and effective
state survive restart.

Operators can state the four-hour defaults explicitly:

```bash
STARPORT_CATALOG_ACQUISITION_ENABLED=true
STARPORT_CATALOG_ACQUISITION_INTERVAL=4h
```

### Enterprise source with local replica acquisition

```bash
# Central Starmap server: public source plus automatic acquisition.
STARMAP_CATALOG_ACQUISITION_ENABLED=true
STARMAP_CATALOG_ACQUISITION_INTERVAL=4h

# Each Starport replica follows the central server.
STARPORT_CATALOG_SOURCE=starmap
STARPORT_CATALOG_SOURCE_URL=https://catalog.corp.example/api/v1
```

The replicas still query providers locally by default when their deployment
environment holds eligible catalog credentials. Their effective generation can
therefore differ from the upstream generation. Status shows both IDs.

### Enterprise central-only acquisition

```bash
STARPORT_CATALOG_SOURCE=starmap
STARPORT_CATALOG_SOURCE_URL=https://catalog.corp.example/api/v1
STARPORT_CATALOG_ACQUISITION_ENABLED=false
```

Only the central Starmap server queries providers. A private source
never falls back to public GitHub.

## Public channel and timestamps

New immutable releases use `catalog-<catalog-digest>`. Existing
`catalog-semantic-*` and `catalog-payload-*` releases remain readable.

`catalog-latest` is a mutable discovery ref, not an immutable GitHub release.
Its attested `catalog-latest.json` document points to one immutable verified
release. This distinction is necessary because GitHub locks immutable release
tags and assets.

Every successful four-hour publisher verification advances the channel
document, including a no-change run. The document records a monotonic channel
sequence, `channel_updated_at`, the immutable release identity, generation
identity, archive digest, `published_at`, and the prior channel digest. It does
not create a new catalog release or generation when the catalog has no semantic
change.

Consumers persist the highest verified sequence for each channel identity.
They reject a lower sequence and reject different bytes at the same sequence.
The channel has discovery and liveness authority only. Immutable release,
artifact digest, repository, workflow identity, schema, and generation-order
verification remain activation requirements.

Timestamp ownership is:

| Field | Meaning | Changes on no-change check |
| --- | --- | --- |
| `generated_at` | immutable source or effective generation creation | no |
| `published_at` | immutable release publication | no |
| `channel_updated_at` | publisher completed one scheduled verification | yes |
| `checked_at` | this runtime completed a source check, including `304` | yes |
| `observed_at` | provider or source evidence time | only when observed |
| `last_attempt_at` | local operation began | yes |
| `last_success_at` | local operation last succeeded | on success |
| `activated_at` | local effective generation became active | on activation |
| `next_attempt_at` | planned local source or acquisition attempt | after scheduling |

A status-only check or channel heartbeat does not create an effective catalog
generation.

The four-hour publisher plus one-hour consumer polling gives a nominal
five-hour path. The product uses a six-hour end-to-end freshness objective. It
is not a bound. The six-hour warning makes delay operator-visible without
making a temporarily stale source a liveness failure.

GitHub's unauthenticated REST limit is 60 requests per hour per source IP.
Authenticated requests have a 5,000-request hourly limit. Only correctly
authenticated conditional requests that return `304` avoid the primary limit.
The default source should use cacheable direct downloads where trust permits.
It should make API calls only when needed and accept an optional source token.
Large replica fleets should use a central Starmap source.

## Runtime status contract

`RuntimeStatus` is immutable caller-owned data. It contains no secret values,
raw credential references, credential-bearing URLs, response bodies, or raw
wrapped errors.

Top-level fields:

- `usable`, `phase`, `health`, and active effective generation ID.
- Effective generation digest, generated time, activation time, and sequence.
- Selected source kind and safe identity.
- Direct source health observed by this runtime.
- Upstream-reported sanitized source chain and provenance, labeled as upstream.
- Direct upstream and derived effective generation IDs.
- generation, publication, channel, check, observation, success, activation,
  and next-attempt times.
- Source and acquisition in-progress flags, operation kind, and run ID.
- Current stage, byte or page progress, last progress, configured idle timeout,
  optional total deadline, scheduled reason, and retry-not-before.
- Last-known-good and fallback state.
- Configured freshness budget, current evaluation, and age.
- acquisition counts for eligible, attempted, succeeded,
  `skipped_not_configured`, and failed providers.

Each provider outcome contains provider ID, status, sanitized reason code,
observed time, last attempt, last success, retained-last-known-good flag, and
next attempt. It never includes a secret value or proof that an account can
route inference.

Direct and transitive health are distinct. A Starport replica can say that its
central Starmap source is reachable. It can relay the central server's
sanitized report about GitHub and providers, but it cannot claim that it
observed those systems itself.

Source chains include a stable instance identity, upstream generation ID, and
bounded hop list bound to the served manifest or verified status document.
Starmap rejects its own identity, a repeated identity, and a chain over the
configured maximum before activation. URL comparison alone is not enough
because aliases and load balancers can hide a cycle.

## Starport API and console

Keep `GET /api/v1/catalog` under `models:read`. It remains a safe catalog
identity surface. Keep `GET /api/v1/catalog/changes` for accepted-generation
diffs.

Add admin-only operations:

| Method and path | Contract |
| --- | --- |
| `GET /api/v1/admin/catalog/status` | detailed runtime, source, acquisition, freshness, and provenance status |
| `POST /api/v1/admin/catalog/refresh` | start or join one complete refresh; return `202` and an operation ID |
| `GET /api/v1/admin/catalog/refreshes/{run_id}` | return active or completed refresh report |
| `DELETE /api/v1/admin/catalog/refreshes/{run_id}` | request cancellation and audit the actor |

A refresh report includes run ID, operation kind, start, completion, duration,
and prior and current effective IDs. It also includes the upstream ID, change
and activation states, source result, provider outcomes, retained state,
transfer progress, last progress, and next attempts.
A repeated manual request returns the active run instead of overlapping it.
Manual refresh initiation and completion use Starport's existing admin audit
boundary.

The console uses the server's new freshness evaluation and shows:

1. Effective catalog identity, digest, generated and activated time, age, and
   model, provider, and offering counts.
2. Embedded baseline, selected upstream, and operator acquisition rows. Each
   row shows role, safe identity, direct or upstream-reported provenance,
   upstream and effective IDs, health, timestamps, and fallback.
3. Acquisition counts, last attempt, last success, and next run.
4. Provider details with sanitized status and reason. The UI collapses neutral
   skipped rows by default.
5. Changes between accepted effective generations.
6. Model and offering provenance where the catalog provides field evidence.

Catalog acquisition status must not appear on Starport's inference credential
or BYOK screens.

## Hidden effects

The implementation and verifier must cover these effects:

1. Retained layer state needs its own durable CAS contract. Effective
   generation storage alone cannot reproduce a later merge.
2. Per-provider last-known-good retention requires provider-scoped evidence,
   outcomes, checksums, and garbage collection.
3. A source refresh can change provider credential metadata. A complete
   refresh must stage source state before eligibility and credential resolution.
4. An unchanged catalog can still change channel health. Health persistence
   and generation persistence must be separate.
5. A downstream with local observations derives a new effective identity. It
   cannot reuse the upstream generation ID even when most facts are equal.
6. `Updates` can arrive faster than Starport acceptance. Starport may coalesce
   candidates, but it must never move its accepted head before route and
   connector validation succeeds.
7. A restart must restore retained source identity and per-provider layers. It
   must also restore channel sequence, freshness, and effective state. Old
   health must not become a new successful check.
8. Source reconfiguration must retire the old source layer and reset direct
   source health. It must not silently request the default public source.
9. Provider failures can contain credentials in URLs, bodies, headers, and
   wrapped errors. Public and admin status must use structured reason codes.
10. Metrics use bounded labels such as source kind and outcome. Provider ID,
    generation ID, run ID, URL, and error text are not metric labels.
11. Multi-replica Starport with shared storage needs one accepted-head CAS and
    idempotent candidate handling. It does not need every replica to accept
    every intermediate generation.
12. Public GitHub polling by many replicas behind one NAT can exhaust the
    unauthenticated API budget even with ETags. The central Starmap pattern is
    an operational scaling boundary, not only a convenience.

## CAT2, verifier, and acceptance changes

CAT2 must replace the old mode and default assertions before CAT5 starts.

The red distribution verifier must add fixed conditions for:

- Absence of `CATALOG_ACQUISITION` and `CATALOG_ACQUISITION_SCHEDULE`.
- Enabled and four-hour acquisition defaults in Starmap and Starport.
- Enabled plus interval combinations and manual `Sync` in each.
- Neutral missing credentials with no provider request.
- Public unauthenticated provider attempts.
- Per-provider partial success and last-known-good retention.
- Independent custom source and local acquisition.
- No public fallback from GitHub, Starmap, file, or embedded sources.
- Retained-layer rebuild instead of incremental effective merging.
- Fast `prefer_source`, blocking `require_source`, and bounded `Close`.
- `Refresh`, `RefreshSource`, `Sync`, status, and effective update semantics.
- One staged publication for a complete refresh and no generation on no change.
- Channel heartbeat advance without an immutable release change.
- Direct versus upstream-reported health and provenance.
- Starport `.env` lookuper injection with no inference, shared, or BYOK read.
- Asynchronous manual refresh, run joining, audit, and overlap prevention.
- Server-evaluated freshness and the detailed admin status surface.
- Candidate and accepted head separation under failures.
- Self-reference, two-node cycles, aliased cycles, and excessive hop rejection.
- Slow progressing transfers beyond old elapsed limits, idle and oversize
  rejection, caller cancellation, and slow response writes.
- Full-interval stable phase, 15-minute cold and outage spread, bounded retry,
  retry not-before, source admission, a distributed lease, and bounded metrics.
- Direct Starport configuration replacement with no legacy runtime aliases.
- historical catalog tag readback.

Acceptance tests must prove the four deployment journeys:

| Journey | Required proof |
| --- | --- |
| offline local | embedded usable immediately, no provider requests, visible source fallback |
| connected no-key | public source updates, neutral skips, no credential warnings |
| connected one-key | only the eligible and public providers are called; other layers remain |
| enterprise | central source, optional replica acquisition, central-only opt-out, no public fallback, separate upstream/effective IDs |

The CAT5 tests own runtime composition and concurrency. CAT6 owns CLI, server,
container, and restart defaults. CAT7 owns Starmap chains and source health.
CAT8 owns Starport configuration, lookuper injection, async API, audit,
acceptance, UI, metrics, and three-replica scale-out.

## Owner decisions

The owner accepted `CATALOG_ACQUISITION_ENABLED` and removed `ON_START`. A
false enabled value means manual-only. The owner also selected a four-hour
publisher and one-hour consumer poll for a six-hour freshness objective.

## Review evidence

- Starmap module: Go 1.25 language floor and Go 1.26.6 toolchain.
- Starport module: Go 1.26 language and Go 1.26.5 toolchain.
- Modern Go guidance ran unfiltered for Starmap `client.go` and Starport
  `internal/catalog/runtime.go`.
- Starmap uses the built-in ago v0.2.0 policy. The active restrictions include
  `no-new-expr`, so the Go 1.26 `new(value)` form does not apply to Starmap.
- No production Go source or Starport worktree file changed during this review.
- GitHub documents a 60-request unauthenticated hourly REST limit for each
  source IP. It also documents a 5,000-request authenticated limit,
  authenticated `304` exemptions, and immutable release locking.
