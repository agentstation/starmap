# CAT2 package and operator design

Date: 2026-09-01

Baseline: `codex/catalog-publisher-six-hour` at `f268b52f`

## Current behavior

`starmap.New` and `starmap.NewContext` load durable, workspace, or embedded
state. They start no network request or background task.

`acquisition.Syncer.Sync` gets catalog facts from provider APIs, models.dev, and
the human catalog workspace. It reconciles those observations and publishes a
new local generation. It does not follow a published catalog.

Starport uses `acquisition.Syncer.Sync` in local acquisition mode. Starport's
remote runtime has a method named `Sync`, but that method only reads the
subscriber's current state. The subscriber sends all remote
requests. This shared name hides two different behaviors.

## Required distinctions

| Concept | Input | Output | Network owner |
| --- | --- | --- | --- |
| catalog source | one complete published generation | one verified generation | connected runtime |
| acquisition source | provider or metadata observations | facts for reconciliation | acquisition syncer |
| bootstrap | bundled fallback generation | immediate initial catalog | binary |
| catalog store | immutable generations and current pointer | durable last-known-good state | caller or application |
| activation | verified generation | current immutable Starmap state | Starmap client |
| Starport acceptance | Starmap state | routable connectors and cache identity | Starport transaction |

The public term `catalog source` always means a source of complete catalog
generations. The term `acquisition source` always means a source of facts for
`Sync`.

## Package contract

Keep `starmap.New` as the deterministic low-level client. Existing tests,
tools, and applications can use it without network access or lifecycle work.

Add `starmap.Open` as the recommended connected package entry point. `Open`
loads a local generation, checks the selected catalog source, and starts the
source lifecycle. It returns a runtime that owns shutdown.

```go
runtime, err := starmap.Open(ctx)
if err != nil {
    return err
}
defer runtime.Close()

catalog := runtime.Catalog()
status := runtime.SourceStatus()
```

The zero-option call uses the AgentStation public catalog source. It uses an
in-memory store unless the caller supplies a store. CLI, server, Docker, and
Starport compositions supply durable stores.

The runtime API should include these operations:

- `Catalog` returns the current immutable catalog without I/O.
- `State` returns the atomic generation state without I/O.
- `Refresh` checks the selected catalog source once.
- `Updates` reports verified state changes for an application acceptance gate.
- `SourceStatus` reports source identity, health, and timestamps.
- `Readiness` combines catalog and source readiness.
- `Close` cancels and joins owned work within the configured timeout.

`Open` does not call `acquisition.Syncer.Sync`. It does not inspect provider API
keys. It does not create a locally derived catalog.

Keep catalog authoring explicit:

```go
client, err := starmap.NewContext(
    ctx,
    starmap.WithCatalogStore(store),
)
if err != nil {
    return err
}

syncer, err := acquisition.New(client)
if err != nil {
    return err
}

_, err = syncer.Sync(ctx)
return err
```

The name `Sync` stays reserved for observation and reconciliation. Catalog
distribution uses `Open`, `Refresh`, and `Updates`.

## Runtime modes

Applications need two explicit modes. They must not run independent source and
acquisition writers against one client.

| Mode | Default | Source behavior | Acquisition behavior |
| --- | --- | --- | --- |
| `follow` | yes | activate the exact verified source generation | disabled |
| `author` | no | use the verified source as an optional baseline observation | explicit manual or scheduled `Sync` |

`follow` mode rejects workspace and acquisition settings. It preserves the
upstream generation identity across Starmap server hops.

`author` mode publishes a new local generation after reconciliation. Its
manifest records the upstream generation as source evidence. A source refresh
and an acquisition pass use one runtime-owned mutation queue.

## Catalog source kinds

| Kind | Use | Trust anchor | Update method |
| --- | --- | --- | --- |
| `public` | AgentStation catalog | pinned repository and workflow identity | GitHub conditional polling |
| `github` | operator catalog repository | explicit repository, workflow, and Sigstore policy | GitHub conditional polling |
| `starmap` | Starmap server | configured HTTPS origin and optional API key | SSE with conditional polling fallback |
| `file` | reviewed local or air-gap artifact | explicit publisher verifier | explicit refresh or file watch |
| `none` | offline or fixed deployment | durable or embedded state | no source network work |

The `public` kind is a safe preset. It fixes the repository, workflow, channel,
and trust policy. A custom GitHub source must not infer publisher trust from a
mutable URL.

The `starmap` kind trusts the configured server boundary. A non-loopback source
requires HTTPS. The client rejects cross-origin redirects. A key never belongs
in a URL.

The `none` kind disables source network access. It does not disable an explicit
authoring `Sync` unless the application also selects `follow` mode.

## Programmatic source selection

The exact option names remain subject to the Go API review. The target call
shape is:

```go
runtime, err := starmap.Open(
    ctx,
    starmap.WithStore(store),
    starmap.WithSource(catalogsource.StarmapServer(
        "https://catalog.example.com/api/v1",
        catalogsource.WithAPIKey(apiKey),
    )),
)
```

Offline and deterministic calls stay short:

```go
runtime, err := starmap.Open(
    ctx,
    starmap.WithSource(catalogsource.None()),
)
```

Tests and tools that need no lifecycle should continue to use `starmap.New`.
The Go package must not read environment variables. Application composition
maps files, flags, and environment values to these options.

## Application configuration

Use the same values in Starmap and Starport. Each application keeps its own
environment prefix.

| Setting | Default | Values or meaning |
| --- | --- | --- |
| `CATALOG_MODE` | `follow` | `follow` or `author` |
| `CATALOG_SOURCE` | `public` | `public`, `github`, `starmap`, `file`, or `none` |
| `CATALOG_SOURCE_URL` | empty | Starmap API root or configured artifact location |
| `CATALOG_SOURCE_API_KEY` | empty | Starmap server credential |
| `CATALOG_SOURCE_REPOSITORY` | preset | custom GitHub `owner/repository` |
| `CATALOG_SOURCE_CHANNEL` | `catalog-latest` | mutable discovery channel |
| `CATALOG_SOURCE_SIGNER_WORKFLOW` | preset | expected GitHub workflow identity |
| `CATALOG_SOURCE_TOKEN` | empty | optional GitHub API credential |
| `CATALOG_SOURCE_POLL_INTERVAL` | `1h` | GitHub or polling-fallback interval |
| `CATALOG_SOURCE_STARTUP_POLICY` | `prefer_source` | `prefer_source` or `require_source` |
| `CATALOG_SOURCE_MAX_AGE` | disabled | readiness age budget |
| `CATALOG_WORKSPACE_PATH` | empty | author-mode human catalog workspace |
| `CATALOG_REFRESH_ON_START` | `false` | author-mode acquisition at startup |
| `CATALOG_REFRESH_INTERVAL` | `0s` | author-mode acquisition schedule |
| `CATALOG_REFRESH_TIMEOUT` | `2m` | source or acquisition request bound |

Starmap uses the `STARMAP_` prefix. Starport uses the `STARPORT_` prefix. The
configuration loader redacts tokens, API keys, and credential-bearing URLs.

Validation applies these rules:

1. Reject source-specific fields that do not match `CATALOG_SOURCE`.
2. Reject workspace and acquisition settings in `follow` mode.
3. Reject `CATALOG_SOURCE=none` with `require_source`.
4. Reject a custom GitHub source without an explicit signer policy.
5. Reject non-loopback plain HTTP Starmap sources.
6. Reject URL credentials and cross-origin redirects.
7. Reject a Starmap source that resolves to the serving instance itself.

## Startup and steady state

`prefer_source` is the default startup policy:

1. Load a verified durable generation.
2. Use the embedded bootstrap when the store is empty.
3. Check the configured source within the startup timeout.
4. Activate a newer verified generation when available.
5. Return the best verified state when the source is unavailable.
6. Report source degradation and continue background retries.

`require_source` fails startup when the source check cannot establish an
acceptable generation. A maximum-age policy can also fail readiness while the
process retains last-known-good state.

GitHub sources poll with ETags and process jitter. Starmap sources use SSE and
close the fetch-to-subscribe gap. Conditional polling starts after repeated SSE
failures. A source change never causes fallback to the public source.

## Starport target flow

Starport should use the connected Starmap runtime for all follow-mode sources.
It should not call provider acquisition in its default configuration.

```text
catalog source
    -> Starmap verifies and stores remote head
    -> Starport builds complete connector candidate
    -> Starport validates routability and credentials
    -> Starport records accepted head
    -> Starport atomically publishes routes and cache identity
```

Remove the no-I/O remote `Sync` method from Starport's internal contract.
Starport should consume `CurrentCandidate` and `Updates` from the connected
runtime. Preserve the separate remote and accepted current pointers.

Starport local acquisition becomes explicit `author` mode. It keeps the current
provider and workspace `Sync` path. Empty configuration selects `follow` mode
with the public source.

## User and deployment workflows

### Individual Go developer

`starmap.Open(ctx)` gets the public catalog and follows updates. An unavailable
network returns embedded or in-memory state with degraded source status.

Use `starmap.New()` for a deterministic test or offline tool. Use a `none`
source when the application needs the runtime API without network access.

### Starmap CLI user

Read commands use the connected runtime by default. `starmap update` remains an
explicit authoring command that calls acquisition `Sync`.

Add `starmap catalog refresh` for one source check. Add `starmap catalog status`
to show source, active generation, `generated_at`, `published_at`, last check,
last success, next check, and degradation.

### Startup team

Each Starport instance can use the default public source. Starport persists the
last accepted generation and needs no catalog-acquisition provider keys.

A team can deploy one Starmap server when replica count or GitHub API limits
make direct pulls undesirable. Starport instances then select that server.

### Enterprise with public upstream

The central Starmap server keeps its default public source. Starport instances
select the central server and make no public GitHub requests.

```bash
# Central Starmap server
STARMAP_CATALOG_MODE=follow
STARMAP_CATALOG_SOURCE=public

# Each Starport instance
STARPORT_CATALOG_MODE=follow
STARPORT_CATALOG_SOURCE=starmap
STARPORT_CATALOG_SOURCE_URL=https://catalog.corp.example/api/v1
STARPORT_CATALOG_SOURCE_API_KEY=secret-reference-or-value
```

### Enterprise without public upstream

Set the central server source to `none`. The server starts from its durable
generation or embedded bootstrap. An operator can import a reviewed file
artifact or run explicit authoring acquisition.

```bash
STARMAP_CATALOG_MODE=follow
STARMAP_CATALOG_SOURCE=none
```

An air-gap transfer can import the artifact and attestation into the central
server. Downstream Starport configuration does not change.

### Enterprise authoring server

Select `author` mode for local provider discovery or a human catalog workspace.
The public source remains the baseline unless the operator selects `none`.

```bash
STARMAP_CATALOG_MODE=author
STARMAP_CATALOG_SOURCE=public
STARMAP_CATALOG_WORKSPACE_PATH=/etc/starmap/catalog
STARMAP_CATALOG_REFRESH_INTERVAL=6h
```

This server publishes a derived enterprise generation. Its source evidence
links the public generation and local observations.

## Channel and runtime timestamps

The immutable generation keeps `generated_at`. The mutable channel adds
`published_at` and a monotonic publication sequence. A runtime records
`checked_at`, `last_success_at`, and `activated_at` as local health state.

Only `generated_at` belongs to the generation identity. Transport and local
timestamps must not alter catalog bytes or create a new generation in follow
mode.

## Edge-case policy

| Case | Required behavior |
| --- | --- |
| first boot without network | use embedded state, report degradation, and retry under `prefer_source` |
| first boot under `require_source` | fail without an acceptable source generation |
| restart during source outage | use the durable last-known-good generation |
| invalid signature or signer | reject before storage or activation |
| wrong repository or workflow | reject before storage or activation |
| source returns an older generation | reject unless an explicit rollback operation authorizes it |
| equal time with different bytes | report a conflict and retain current state |
| channel points to missing assets | retain current state and retry as a transient publication race |
| unsupported schema | retain current state and report incompatibility |
| HTTP 401 or 403 from Starmap | stop automatic retries and report configuration failure |
| GitHub rate limit | honor retry metadata, retain current state, and report the next attempt |
| store commit failure | do not swap in-memory state |
| concurrent store writers | use compare-and-swap, reload the winner, and re-evaluate the candidate |
| custom source outage | do not fall back to public GitHub |
| source configuration change | bind health and accepted state to the new source identity |
| source points to self | reject configuration before the lifecycle starts |
| two Starmap servers form a cycle | detect the source chain and stop propagation |
| SSE blocked by a proxy | use bounded conditional polling after repeated failures |
| process shutdown | cancel and join source work within the configured timeout |
| Starport candidate failure | retain the accepted head, routes, connectors, and cache identity |
| author and source update race | serialize both through one runtime-owned mutation queue |

## Owner decisions

1. Approve `New` for offline state and `Open` for connected distribution.
2. Approve `follow` and `author` as the two application modes.
3. Approve `public` as the empty-configuration source and `none` as opt-out.
4. Approve `prefer_source` startup with last-known-good fallback.
5. Approve explicit custom-source replacement without public fallback.
6. Approve one-hour GitHub polling with jitter and conditional requests.
