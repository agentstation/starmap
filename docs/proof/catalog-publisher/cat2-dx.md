# CAT2 runtime and Starport DX review

Date: 2026-09-01

Baseline: `codex/catalog-publisher-six-hour` at `db0c3e73`

## Verdict

Keep `Catalog()` as a non-failing immutable read. Do not return a catalog,
error, and status tuple.

Keep `Open` and `Sync`, but give them separate jobs:

- `Open` creates and starts the composed catalog runtime.
- `Refresh` refreshes every enabled runtime layer once.
- `RefreshPublicCatalog` refreshes only the selected upstream catalog.
- `Sync` gets operator observations and rebuilds the effective catalog.
- `Catalog`, `State`, and `Status` do not read external state.

Replace the proposed `follow` and `author` modes with two independent settings.
One selects the upstream catalog. The other selects operator acquisition.
This model permits both functions in one runtime.

Starport does not implement the target merge itself. Its embedded Starmap
runtime builds one effective catalog. Starport validates and accepts that
catalog as a complete routing candidate.

## Current Starmap behavior

`starmap.New` and `starmap.NewContext` load durable, workspace, or embedded
state. They start no network request or background task.

`acquisition.Syncer.Sync` gets observations from configured acquisition
sources. It reconciles the observations against the current catalog and
publishes a new local generation.

`acquisition.Syncer.ImportRelease` verifies one release and reconciles it as a
`release_artifact` observation. It does not retain public and operator inputs
as separate runtime layers.

`remote.Subscriber` verifies and activates complete server generations. It
does not merge a remote generation with local operator observations.

The authority table orders provider observations above release artifacts and
embedded facts. It does not distinguish a publisher provider observation from
an operator provider observation. Both currently use the `providers` source
ID.

## Current Starport behavior

Starport has two mutually exclusive catalog runtimes.

| Path | Startup input | Refresh behavior | Merge behavior |
| --- | --- | --- | --- |
| local | durable or embedded Starmap state | provider and local acquisition | Starmap merges embedded, provider, and local observations |
| remote | accepted generation or embedded state | subscriber fetch and SSE | no merge; Starport accepts the complete remote generation |

Empty Starport configuration selects the local path. Its defaults set
`RefreshOnStart` to false and `RefreshInterval` to zero. The default therefore
serves durable or embedded state without a refresh.

`STARPORT_CATALOG_REMOTE_URL` selects the remote path. Validation rejects a
remote URL with a workspace, startup acquisition, or scheduled acquisition.

The local `RefreshCandidate` method calls provider acquisition. The remote
method named `Sync` does not send a network request. It only reads the subscriber
state. The shared name hides two different operations.

Starport already has the correct acceptance transaction. It builds connectors,
validates routability, records the accepted head, and publishes one complete
routing snapshot. A failed candidate retains the prior routes and cache
identity.

## Canonical catalog names

The **Starmap public catalog** is the public catalog product. It has two
delivery forms:

| Form | Canonical name | Freshness |
| --- | --- | --- |
| compiled binary payload | embedded public catalog | updated with a Starmap release |
| verified `catalog-latest` generation | released public catalog | published by the six-hour workflow |

The runtime also holds **operator observations**. These observations come from
provider APIs and reviewed local inputs under operator control.

The **effective catalog** is the immutable result that consumers read. It is
not another upstream source.

## Layer contract

The runtime owns this ordered composition:

```text
embedded public catalog
        ↓ fallback
selected upstream catalog
        ↓ public or enterprise baseline
operator observations
        ↓ field-level reconciliation
effective catalog
```

The default selected upstream is the released public catalog. A custom GitHub,
Starmap, or file source replaces that upstream. It never causes an undeclared
fallback request to the public GitHub repository.

The embedded public catalog remains the no-network baseline. A strict source
policy can fail startup or readiness when the selected upstream is unavailable.

The runtime must retain each layer separately. Every public refresh or operator
sync rebuilds the effective catalog from the retained layer states. The runtime
must not merge a new upstream generation into the prior effective result as if
that result were one source.

This rule prevents three defects:

1. A public refresh cannot erase a private or special-access model.
2. A carried operator value keeps its operator evidence.
3. Repeated refreshes cannot create merge-order drift.

Layer order does not replace field authority. Operator observations lead the
same public observation scope. Missing operator fields fall back through the
public layers. Source-specific field policies still choose provider,
models.dev, local, and embedded facts within each layer.

An operator provider list omission is not entitlement evidence. It must not
delete a public model. Starport owns inference credentials and runtime
availability. Provider failures can make an offering unroutable without
removing the catalog fact.

The implementation needs scoped observation identity. A publisher provider
observation and an operator provider observation cannot collapse into one
`providers` map entry.

## Go package contract

Keep `starmap.New` as the low-level deterministic client. Keep it free of
network and lifecycle work.

Make `starmap.Open` the recommended runtime entry point. The zero-option call
uses the released public catalog and an in-memory store. Applications can
supply a durable store.

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

The target read contract is:

```go
func (r *Runtime) Catalog() *catalogs.Catalog
func (r *Runtime) State() starmap.CatalogState
func (r *Runtime) Status() starmap.RuntimeStatus
```

`Catalog()` stays non-failing, non-nil after successful `Open`, O(1), and safe
to retain. Returning an error would force every hot-path read to handle an
operation that cannot fail.

`Status()` reports public-source and acquisition health. It includes the
active generation ID, so a caller can correlate health with `State()`.
Status changes do not require a new catalog generation.

Do not add `Snapshot()` until one consumer proves that catalog state and
runtime health require one atomic read. If that need appears, return one named
struct instead of a three-value tuple.

I/O methods return errors and operation reports:

```go
func (r *Runtime) Refresh(ctx context.Context) (starmap.RefreshReport, error)
func (r *Runtime) RefreshPublicCatalog(ctx context.Context) (starmap.PublicCatalogRefresh, error)
func (r *Runtime) Sync(ctx context.Context, opts ...sync.Option) (*sync.Result, error)
func (r *Runtime) Close() error
```

`Refresh` runs one complete configured cycle. It is the application-level
operation for Starport and the Starmap server. `Sync` remains the exact name for
operator observation and reconciliation.

`Open` owns background work and bounded shutdown. The low-level `Client` still
owns serialized durable publication. The runtime owns layer refresh order and
is the only publisher that uses that client.

The root package cannot import the current `acquisition` package because that
package imports the root client. CAT5 must repair this dependency direction.
The runtime can own the internal observation pipeline while the public
`acquisition.Syncer` remains an advanced low-level facade.

## Programmatic configuration

Do not expose `follow` and `author` modes. Use orthogonal source and acquisition
policies.

```go
runtime, err := starmap.Open(
	ctx,
	starmap.WithCatalogStore(store),
	starmap.WithCatalogSource(catalogsource.StarmapServer(
		"https://catalog.example.com/api/v1",
		catalogsource.WithAPIKey(apiKey),
	)),
	starmap.WithAcquisition(starmap.AcquisitionScheduled(15*time.Minute)),
)
```

The exact option constructors remain a CAT5 API task. This review fixes the
public behavior.

| Catalog source | Use | Update method |
| --- | --- | --- |
| `public` | AgentStation released public catalog | GitHub conditional polling |
| `github` | operator GitHub catalog | GitHub conditional polling |
| `starmap` | Starmap server | SSE with conditional polling fallback |
| `file` | reviewed local or air-gap release | explicit refresh or file watch |
| `embedded` | embedded public catalog only | no source network request |

| Acquisition policy | Startup | Steady state |
| --- | --- | --- |
| `disabled` | no operator acquisition | no operator acquisition |
| `manual` | optional explicit startup sync | explicit `Sync` only |
| `scheduled` | configured startup sync | configured interval and explicit `Sync` |

## Application configuration

Starmap uses the `STARMAP_` prefix. Starport uses the `STARPORT_` prefix.

| Setting | Default | Values or meaning |
| --- | --- | --- |
| `CATALOG_SOURCE` | `public` | `public`, `github`, `starmap`, `file`, or `embedded` |
| `CATALOG_SOURCE_URL` | empty | Starmap API root or artifact location |
| `CATALOG_SOURCE_API_KEY` | empty | Starmap server credential |
| `CATALOG_SOURCE_REPOSITORY` | preset | custom GitHub `owner/repository` |
| `CATALOG_SOURCE_CHANNEL` | `catalog-latest` | mutable discovery channel |
| `CATALOG_SOURCE_SIGNER_WORKFLOW` | preset | expected GitHub workflow identity |
| `CATALOG_SOURCE_TOKEN` | empty | optional GitHub API credential |
| `CATALOG_SOURCE_POLL_INTERVAL` | `1h` | conditional polling interval |
| `CATALOG_SOURCE_STARTUP_POLICY` | `prefer_source` | `prefer_source` or `require_source` |
| `CATALOG_SOURCE_MAX_AGE` | disabled | readiness age budget |
| `CATALOG_ACQUISITION` | `disabled` | `disabled`, `manual`, or `scheduled` |
| `CATALOG_ACQUISITION_ON_START` | `false` | run one operator sync during startup |
| `CATALOG_ACQUISITION_INTERVAL` | `0s` | operator sync schedule |
| `CATALOG_WORKSPACE_PATH` | empty | reviewed operator catalog input |
| `CATALOG_REFRESH_TIMEOUT` | `2m` | one source or acquisition operation bound |

The old Starport `CATALOG_REMOTE_*` and ambiguous `CATALOG_REFRESH_*` settings
need a documented migration period. Configuration inspection redacts every
token, API key, and credential-bearing URL.

Validation applies these rules:

1. Reject source-specific fields that do not match `CATALOG_SOURCE`.
2. Reject scheduled acquisition without a positive interval.
3. Reject acquisition settings when the configuration disables acquisition.
4. Reject a custom GitHub source without an explicit signer policy.
5. Reject non-loopback plain HTTP Starmap sources.
6. Reject URL credentials and cross-origin redirects.
7. Reject a Starmap source that resolves to the serving instance itself.

## Startup and steady state

`prefer_source` is the default startup policy:

1. Load the last effective durable generation.
2. Use the embedded public catalog when no durable generation exists.
3. Check the selected upstream within the startup timeout.
4. Rebuild the effective catalog from all retained layers.
5. Run startup acquisition only when configured.
6. Return the best verified state when an optional operation fails.
7. Report degradation and continue configured background retries.

`require_source` fails `Open` when the selected upstream cannot establish an
acceptable generation. A maximum-age policy can fail readiness while the
runtime retains last-known-good state.

GitHub sources use ETags and process jitter. Starmap sources use SSE and close
the fetch-to-subscribe gap. Bounded conditional polling starts after repeated
SSE failures.

## Starport target

Replace Starport's local and remote catalog runtimes with one adapter over the
Starmap runtime.

```text
Starmap runtime refreshes configured layers
    -> Starmap publishes one effective candidate head
    -> Starport builds the complete connector candidate
    -> Starport validates routes and inference configuration
    -> Starport records the accepted head
    -> Starport atomically publishes routes and cache identity
```

Starport must preserve its separate candidate and accepted heads. It must also
preserve the complete acceptance transaction.

Remove the remote no-I/O `Sync` compatibility method. Starport's manual
`RefreshCatalog` operation should call the Starmap runtime `Refresh` method.
Background effective-catalog updates should use one `Updates` stream.

Starport's empty configuration should select the released public catalog and
disable operator acquisition. This default needs no provider catalog keys.

A Starport instance can enable operator acquisition when it needs private or
special-access models. A central Starmap server is the preferred enterprise
owner of that work. Starport replicas then select the server and disable local
acquisition.

## Deployment examples

### Individual developer

`starmap.Open(ctx)` starts with the embedded public catalog and checks the
released public catalog. `starmap.New()` stays deterministic and offline.

### Standalone Starport with operator discovery

```bash
STARPORT_CATALOG_SOURCE=public
STARPORT_CATALOG_ACQUISITION=scheduled
STARPORT_CATALOG_ACQUISITION_ON_START=true
STARPORT_CATALOG_ACQUISITION_INTERVAL=15m
```

The embedded Starmap runtime builds the effective catalog. Starport consumes
that result and does not own fact reconciliation.

### Enterprise Starmap source

```bash
# Central Starmap server
STARMAP_CATALOG_SOURCE=public
STARMAP_CATALOG_ACQUISITION=scheduled
STARMAP_CATALOG_ACQUISITION_ON_START=true
STARMAP_CATALOG_ACQUISITION_INTERVAL=15m

# Each Starport instance
STARPORT_CATALOG_SOURCE=starmap
STARPORT_CATALOG_SOURCE_URL=https://catalog.corp.example/api/v1
STARPORT_CATALOG_SOURCE_API_KEY=secret-reference-or-value
STARPORT_CATALOG_ACQUISITION=disabled
```

Only the central server reaches public GitHub and provider catalog APIs.
Starport replicas follow its effective generations.

### Enterprise without public egress

```bash
STARMAP_CATALOG_SOURCE=embedded
STARMAP_CATALOG_ACQUISITION=scheduled
STARMAP_CATALOG_ACQUISITION_INTERVAL=15m
```

An operator can also select a reviewed file or internal Starmap source. A
custom source never triggers a public GitHub fallback request.

## Status and timestamps

`RuntimeStatus` should separate these records:

| Record | Required fields |
| --- | --- |
| effective catalog | generation ID, generated time, activation time, sequence |
| public catalog | form, source identity, generation ID, generated time, published time, checked time, last success |
| operator acquisition | policy, last attempt, last success, provider results, next attempt |

The immutable generation keeps `generated_at`. The mutable channel adds
`published_at` and a monotonic publication sequence. Local health records add
`checked_at`, `last_success_at`, and `activated_at`.

Only generation facts belong to generation identity. Transport and health
timestamps must not alter catalog bytes.

## Edge-case policy

| Case | Required behavior |
| --- | --- |
| first boot without network | use the embedded public catalog and report source degradation |
| restart during source outage | use the durable last-known-good effective generation |
| invalid signature or signer | reject before layer storage or effective publication |
| public refresh after private model discovery | retain the private model and its operator evidence |
| operator source omits a public model | retain the public fact; do not infer entitlement |
| operator sync fails | retain prior operator layer and effective catalog |
| custom source outage | do not request the public GitHub source |
| source returns an older generation | reject unless an explicit rollback authorizes it |
| equal time with different bytes | report a conflict and retain current state |
| source configuration changes | bind health and retained state to the new source identity |
| source points to self | reject before lifecycle start |
| two Starmap servers form a cycle | detect the source chain and stop propagation |
| Starport candidate fails | retain accepted routes, connectors, and cache identity |
| concurrent public and operator updates | serialize layer replacement and effective publication |
| process shutdown | cancel and join source and acquisition work within the bound |

## Owner decisions

1. Approve `Catalog() *catalogs.Catalog`, plus separate `State()` and `Status()` reads.
2. Approve `Open` for the composed runtime and `Sync` for operator acquisition.
3. Approve the embedded public, released public, operator observation, and effective catalog names.
4. Approve orthogonal source and acquisition settings without `follow` or `author` modes.
5. Approve `public` as the source default and `embedded` as the network opt-out.
6. Select whether scheduled operator acquisition is opt-in or key-detected by default.
7. Approve operator observations as additive for model identity, not entitlement evidence.

## Review evidence

- Starmap module: Go 1.25 language floor and Go 1.26.6 toolchain.
- Starport module: Go 1.26 language and Go 1.26.5 toolchain.
- Modern Go guidance ran unfiltered for `client.go` and Starport
  `internal/catalog/runtime.go`.
- The active Starmap `ago` policy uses the built-in v0.2.0 restrictions.
- No Go source changed during this API and architecture review.
