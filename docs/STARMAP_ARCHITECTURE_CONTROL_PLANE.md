# Starmap Architecture Execution Control Plane

Last updated: 2026-07-28

Status: `IN_PROGRESS` — P0–P5.12 and P6.1 are complete. P5.13 is the sole
active task after user review reopened the author/model and endpoint identity
contract. P6.2 is paused until the restored catalog DX and corpus completeness
pass their outcome review and exact phase gate.

## Mission

Deliver a smaller, canonical, production-trustworthy Starmap that can be used:

1. directly as an idiomatic Go library with an immutable in-process catalog;
2. as an embeddable Starmap server composed by another Go program; and
3. as a reactive remote catalog source for a Go consumer, with push notification
   as the normal path and polling only as a documented fallback.

The final repository must be ready for an enterprise LLM proxy gateway such as
Starport. Reliability must come from explicit semantics, deep modules, narrow
interfaces, fault isolation, and high-value tests—not from duplicated
representations or speculative abstractions.

## Authoritative Inputs

This plan is the durable execution record.

- Architecture report:
  [`reviews/STARMAP_ARCHITECTURE_REVIEW_2026-07-27.html`](reviews/STARMAP_ARCHITECTURE_REVIEW_2026-07-27.html)
- Architecture report SHA-256:
  `de08e0b3a8e3a22463968f326c4e7659a8f69c04dea166b15ace76e62b0d9235`
- Independent Fable review prompt:
  [`reviews/FABLE_STARMAP_PLAN_REVIEW_PROMPT.md`](reviews/FABLE_STARMAP_PLAN_REVIEW_PROMPT.md)
- Independent Fable review (full report):
  [`reviews/FABLE_STARMAP_PLAN_REVIEW_2026-07-27.md`](reviews/FABLE_STARMAP_PLAN_REVIEW_2026-07-27.md)
- Independent Fable review SHA-256:
  `b2f78de7be15762f9d0425f99a698dc3d63397b3c2e07e553eb16ed51a3495b2`
- Independent Fable review disposition:
  [`reviews/FABLE_STARMAP_PLAN_REVIEW_DISPOSITION_2026-07-27.md`](reviews/FABLE_STARMAP_PLAN_REVIEW_DISPOSITION_2026-07-27.md)
- Independent Fable review disposition SHA-256:
  `df13364d205ca48848841f6aed20888ea0e8baf81e148ea1da73e2bc3406ae86`
- P3 workspace lifecycle outcome review:
  [`reviews/P3_WORKSPACE_LIFECYCLE_OUTCOME_REVIEW_2026-07-28.md`](reviews/P3_WORKSPACE_LIFECYCLE_OUTCOME_REVIEW_2026-07-28.md)
- P3 workspace lifecycle outcome review SHA-256:
  `0bb257ec3cb368b069a5101bf2af436d7ab214cdf28b15c1208a35e7a0ac9c88`
- P5 author-model corpus disposition:
  [`reviews/P5_AUTHOR_MODEL_CORPUS_DISPOSITION_2026-07-28.md`](reviews/P5_AUTHOR_MODEL_CORPUS_DISPOSITION_2026-07-28.md)
- P5 author-model corpus disposition SHA-256:
  `6ffc08afe7eb3e855e90d8a1ab4e43dbcb269d50432acd976df60b9b840f54df`
- P5 catalog read-model outcome review:
  [`reviews/P5_CATALOG_READ_MODEL_OUTCOME_REVIEW_2026-07-28.md`](reviews/P5_CATALOG_READ_MODEL_OUTCOME_REVIEW_2026-07-28.md)
- P5 catalog read-model outcome review SHA-256:
  `974522d8a360302f9abfd12ef01385dc23510b31f86c50141a701f5095f6da1e`
- P6 Go package graph review:
  [`reviews/P6_PACKAGE_GRAPH_2026-07-28.md`](reviews/P6_PACKAGE_GRAPH_2026-07-28.md)
- P6 Go package graph review SHA-256:
  `4207b43d0f828d8d34830314b68512af19e5068beb9a57d90f742f7205085e26`
- Author/slug and endpoint compatibility review:
  [`reviews/AUTHOR_SLUG_ENDPOINT_COMPATIBILITY_REVIEW_2026-07-28.md`](reviews/AUTHOR_SLUG_ENDPOINT_COMPATIBILITY_REVIEW_2026-07-28.md)
- Author/slug and endpoint compatibility review SHA-256:
  `1ab2929954fa0163c731f015334266c48d10bedf565f89564dd773bd916c6e68`
- Exact restored author-model corpus map:
  [`reviews/P5_AUTHOR_MODEL_CORPUS_MAP_2026-07-28.yaml`](reviews/P5_AUTHOR_MODEL_CORPUS_MAP_2026-07-28.yaml)
- Exact restored author-model corpus map SHA-256:
  `230fc29c4e48f7d122e062d8beb691fb2497df0f4d801f00e0fe7f3ca3750840`
- Provider/model identity implementation review:
  [`reviews/P5_PROVIDER_MODEL_IDENTITY_REVIEW_2026-07-28.md`](reviews/P5_PROVIDER_MODEL_IDENTITY_REVIEW_2026-07-28.md)
- Provider/model identity implementation review SHA-256:
  `ad99292bf4298888bb6e481a39da824ea1fef57584eba20b47aa8ba825c3de1a`
- Exact provider/model identity map:
  [`reviews/P5_PROVIDER_MODEL_IDENTITY_MAP_2026-07-28.yaml`](reviews/P5_PROVIDER_MODEL_IDENTITY_MAP_2026-07-28.yaml)
- Exact provider/model identity map SHA-256:
  `6d6fce188901b55bcad12df3fdb5624cda4747ff2802e3cdc3cb4a487e4f136c`
- Reviewed protected-main baseline:
  `9508ee7866e4683e001e7ad153319d348433045d`
- Historical pre-control-plane comparison:
  `3787d7164433f2fcb713a2d81e0cb653f9df6be5`
- Historical production catalog ledger:
  [`STARPORT_CATALOG_CONTROL_PLANE.md`](STARPORT_CATALOG_CONTROL_PLANE.md)

The historical production ledger remains evidence; this plan supersedes its
prescriptive architecture where the new review found a conflict. Historical
claims must not be silently rewritten.

### Historical Supersession Map

| Historical evidence | Earlier decision | Current disposition |
| --- | --- | --- |
| F-095 and P11.15–P11.18 | `~/.starmap/catalog` is a machine generation store; YAML is exported under `~/.starmap/exports/catalog` | Superseded for the target product contract by one human provider-YAML workspace, but existing installations require typed legacy-layout detection and an explicit transactional migration in P3.1/P3.10 |
| P11.14 | Restart gives durable machine `current` precedence over editable YAML | Superseded by the selected human-workspace truth model only after P3 proves safe detection, migration, authority, provenance, restart, and downgrade behavior |
| F-097 | Prelaunch clean break needs no compatibility or migration | Narrowly superseded: no public schema compatibility layer is retained, but a legacy layout detector is mandatory because the same path would otherwise change meaning destructively |
| F-099 / P12.4 | Homebrew release flow remained in progress | Must receive a terminal disposition in P11.9; publishing is not implied |
| F-105 / P12.7 | Version stdout patch release remained in progress | Must receive a terminal disposition in P11.9; publishing is not implied |
| F-106 / P12.8 | Immutable draft-first release flow remained in progress | Must receive a terminal disposition in P11.9; publishing is not implied |
| PR #53 / F-012 / F-052 | Provider model YAML was the sole persisted model truth; the 322 author-model files and all 121 non-exact records were deleted | Superseded by user review in P5.9–P5.13: restore the corpus as authored identity/intrinsic metadata, keep provider records as explicitly linked serving facts, and generate endpoints from the join without restoring the old denormalized-copy semantics |

## Status Legend

- `DONE`: criteria are proven and evidence is recorded.
- `IN_PROGRESS`: exactly one task currently owns execution.
- `PENDING`: ready or waiting on an earlier task.
- `BLOCKED`: repeated external blocker prevents meaningful progress.
- `REJECTED`: user accepted a documented non-action and residual risk.
- `SUPERSEDED`: replaced by a named task with equal or stronger criteria.

`DEFERRED` is not a terminal state. Every phase, task, finding, open pull
request, branch, and worktree must end as `DONE`, `REJECTED`, or `SUPERSEDED`.

## Operating Rules

1. Read this file before every autonomous continuation and after every
   compaction event.
2. Keep at most one task `IN_PROGRESS`.
3. Update the ledgers and evidence log in the same commit as the work they
   describe. Two narrow exceptions apply: third-party PR merges are recorded in
   the next control-plane commit with the exact merge SHA, and the closing
   ledger PR records the exact post-merge machine-gate commands and expected
   output before it merges; that gate then runs after merge without another
   documentation commit.
4. Never mark a task `DONE` from intention, code review, or a narrower test.
5. Preserve immutable catalog publication, compare-and-swap, retained
   generations, rollback, and O(1) catalog access.
6. Do not add persisted `/definitions`, `/offerings`, or `/overrides` trees.
7. Treat authored-model YAML and provider-serving YAML as the two human-facing
   input roles; generated endpoints and immutable read views are not competing
   editable representations.
8. Do not silently mutate an existing local catalog during construction or
   binary installation.
9. Do not infer deletion from source absence or incomplete observation.
10. Never persist credentials, secret values, or reusable secret fingerprints.
11. No hidden long-lived goroutine may be owned by a constructor. Reactive
    synchronization must have explicit context, start, and stop ownership.
12. A failed parse, source read, validation, write, commit, remote fetch, or
    publication must leave the prior catalog usable.
13. Use typed errors. Panic is not an input, source, network, storage, or
    lifecycle error strategy.
14. Prefer deleting a shallow module over preserving an unused seam.
15. One adapter is a hypothetical seam. Retain a seam only when at least two
    real adapters or a concrete test substitution justify it.
16. Never merge a phase PR until its exact head passes required local and hosted
    gates.
17. Do not publish an application or catalog release merely to complete this
    plan. Release publication requires its own explicit authorization.
18. Pause for the user when protected review approval, merge authority, release
    authority, credentials, or a material product decision cannot be exercised
    autonomously. Record the blocker and resume immediately after it is cleared.
19. Run structured autoreview once for a functionally complete, coherent unit of
    value such as a phase or PR candidate, not per change or commit. Repeat only
    when remediation materially changes architecture, public API, concurrency,
    persistence, security, or failure semantics. Evidence-only ledger commits,
    documentation, mechanical cleanup, fixture slimming, CI harness/timeout
    changes, and small product-neutral fixes use ordinary exact gates without
    retriggering structured review.

## `/goal` Prompt

```text
/goal Execute docs/STARMAP_ARCHITECTURE_CONTROL_PLANE.md to completion.

Treat that document as the durable control plane and status ledger. Read it
before acting and after every context-compaction event. Resume from the single
IN_PROGRESS task, or the first PENDING task whose dependencies are DONE. Update
the phase ledger, task ledger, finding ledger, PR ledger, workspace ledger, and
evidence log in the same commit as each material result.

Preserve the reviewed architecture contract:
- one human-editable YAML workspace under ~/.starmap/catalog with authored
  models under authors/{author}/models and serving records under
  providers/{provider}/models;
- embedded catalogs are verified, versioned, lowest-authority observations;
- installing or constructing Starmap never silently rewrites an existing
  workspace;
- author models own canonical intrinsic model facts; provider records own
  provider-serving facts and may retain overlapping upstream claims only as
  evidence that cannot override the linked authored definition;
- definitions, offerings, author membership, and endpoint relationships are
  derived immutable read views over those linked records;
- endpoints.yaml is a deterministic generated projection of authored models
  joined to provider serving records, never an independent authority;
- valid provider pricing wins for that provider offering;
- source absence and degraded observations cannot delete last-known-good data;
- build and validate candidates off to the side, then publish immutable
  generations atomically;
- starmap.Client.Catalog() remains non-failing, non-nil, O(1), allocation-free,
  and returns the concrete immutable catalog;
- the Go library remains independently useful without the CLI, acquisition
  stack, or server;
- an embeddable Starmap server is available to Go programs;
- a remote Go consumer performs an initial verified fetch, then reacts to
  post-commit server events through the sole SSE transport, verifies and
  atomically activates immutable generations, uses flushed heartbeats to
  establish stream health, reconnects with mandatory gap recovery, and polls
  only as an explicit fallback;
- all repository-authored Go files stay below 2,000 lines; files over 1,000
  lines require review, and files over 1,500 require an explicit durable
  modularity justification before they can remain;
- tests must prove user-visible and failure semantics, not inflate counts.

Use a fresh phase branch and worktree from the current protected main for each
coherent phase. Do not use codex/provider-expansion-wave0 as an implementation
base. Treat PR #40 as a read-only donor inventory, salvage only reviewed
provider/connector work that fits the new contract, then close the PR and remove
its branch/worktree. Resolve every open PR as merged or closed. Keep branch
protection active. Do not force-push protected main.

Run the smallest decisive tests while developing, then every task and phase
gate named in the control plane. Record exact commands, commit SHAs, PR heads,
hosted check URLs, benchmark results, and failure-injection evidence. A task is
DONE only when every listed criterion is verified.

Continue autonomously while safe in-scope work remains. Ask only for genuinely
missing external authority or a product decision that materially changes the
contract. Protected review approval and other user-owned GitHub decisions are
explicit pauses, not reasons to weaken or bypass a gate. The goal is complete
only when every phase/task/finding is terminal,
all implementation PRs are merged or intentionally closed, GitHub has no open
Starmap PR, protected main is green, local main exactly tracks origin/main, all
temporary worktrees and obsolete local/remote branches are removed, the working
tree is clean, and the final evidence audit proves Starmap is ready for an
enterprise LLM proxy gateway.
```

## Target Architecture

```text
verified embedded catalog (bootstrap + versioned observation)
                         |
                         v
one human workspace: ~/.starmap/catalog
       |                                  |
       v                                  v
authors/{author}/models             providers/{provider}/models
 authored identity + facts   <----  explicit author/model link +
                                    provider-serving facts
       ^                                  ^
       |                                  |
 models.dev observations            provider observations
       \                                  /
             one authority + provenance implementation
                              |
                     validate complete candidate
                              |
                    immutable generation
                 store / sole commit CAS
                       |             \
                       v              +--> atomic YAML workspace +
             atomic in-memory Catalog     generated endpoints.yaml,
                         /          \      repaired by digest
                Go library       Starmap server
                                      |
                       manifest + immutable payload
                                      |
                     post-commit SSE notification
                                      |
                      reactive Go remote consumer
```

### Catalog roles

| Role | Contract |
| --- | --- |
| Embedded catalog | Verified offline bootstrap and lowest-authority versioned observation; never a wholesale overlay |
| Local author-model YAML | Human-editable canonical author/model identity and intrinsic facts; never provider price, limits, availability, or endpoint behavior |
| Local provider-model YAML | Human-editable provider-serving record with exact provider model ID, explicit author/model link, and provider-scoped facts |
| Generated endpoint YAML | Deterministic digest-bound join of authored models and provider serving records; inspectable/exportable but never an independent authority |
| Provenance and generation metadata | Machine-owned evidence and lifecycle state, not competing configuration |
| Immutable generation | Compiled, validated publication product; safe to retain and share |
| Catalog store | Durable generation history, CAS, retention, and rollback |
| Starmap server | Optional public Go module and binary composition over immutable generations |
| Remote consumer | Verified initial fetch plus push-triggered immutable generation activation |

### Canonical domain vocabulary

These meanings are normative for code, GoDoc, tests, CLI help, APIs, and
architecture documents. Historical evidence may retain earlier terms only when
it is explicitly marked historical or superseded.

| Term | One meaning | Not this |
| --- | --- | --- |
| **Author model** | One persisted human-readable model under `authors/{author}/models`, identified by canonical `{author}/{slug}` and owning intrinsic model facts. | A provider endpoint, a denormalized copy of provider price/limits, or an attribution-only membership entry. |
| **Provider model** | One persisted human-readable serving record under a provider. It contains the exact provider model ID, an explicit author/model link, and provider-scoped facts. | The model author inferred from the provider, a duplicate authored payload, an override fragment, or a remote event payload. |
| **Endpoint projection** | A generated, digest-bound join from one author model to one linked provider model, suitable for inspection, export, and OpenRouter-compatible server adaptation. | An independently editable authority, invented runtime telemetry, or a provider inferred from author identity. |
| **Catalog** | Starmap's concrete immutable in-memory product: a complete validated read model with precomputed indexes. Public consumers retain and share `*catalogs.Catalog`; read methods return caller-owned values. | A mutable builder, a filesystem directory, an acquisition response, a `Snapshot` public API, or a collection returned by reference. |
| **Builder** | A mutable, unpublished construction mechanism used to assemble and validate a candidate catalog off to the side. It has one-way publication into a new immutable catalog. | The consumer-facing catalog, a concurrently shared mutable database, or the durable commit point. |
| **Workspace** | The one human-editable YAML tree rooted at the configured catalog path, normally `~/.starmap/catalog`, containing disjoint author-model and provider-model records plus generated projections. Semantic edits to source records are evidence; formatting is not. | The generation store, cache, staging area, exports directory, persisted definitions/offerings/overrides, or an implicitly watched directory. |
| **Source** | A configured acquisition adapter and identity, such as one provider API, models.dev transport, local workspace, or embedded bootstrap. | A field winner, a provider itself, a scheduler, or an anonymous blob with no identity. |
| **Observation** | One source attempt's bounded candidate facts plus source identity, retrieval evidence, status, completeness, issues, and scope. An observation may be rejected, degraded, or reconciled; it is never directly published. | The canonical catalog, proof that absent records were deleted, a generation, or a reusable credential container. |
| **Provenance** | Provider/model/field-scoped evidence explaining which observation or human edit supplied a selected value and why. | A bare model-ID map, one source label for an entire catalog, or persisted secrets/fingerprints. |
| **Definition** | A provider-independent immutable read view built from one canonical author model plus reconciled intrinsic observations for discovery and identity-oriented queries. | The author-model YAML record itself, a provider serving record, or a first-wins merge product. |
| **Offering** | A provider-specific immutable read view derived from one linked provider model, preserving exact provider price, limits, endpoint, availability, and lifecycle knowledge. | A second persisted configuration tree, invented availability, or a cross-provider aggregate price. |
| **Candidate** | A complete off-side catalog plus evidence prepared by reconciliation and validation before commit. Failure discards it without changing the active generation or workspace. | The active catalog or a partially mutated shared object. |
| **Generation** | An immutable, schema-identified, digest-verified catalog publication unit retained in the machine store with its evidence. Generation is lifecycle/storage vocabulary, not the normal consumer type name. | A timestamp-only event, mutable "current" data, a YAML directory, or the public `Catalog()` return vocabulary. |
| **Catalog store** | The machine-owned durable history of immutable generations, compare-and-swap current identity, retention, and rollback. Its CAS is the sole durable commit point. | The human YAML workspace, a remote cache with no verification, or a second source of editable configuration. |
| **YAML projection** | The atomic, digest-repairable post-commit materialization of the committed generation into the human workspace. | The durable commit point, a pre-commit gate, or an unconditional overwrite during construction/install. |
| **Publication** | The successful ordered transition whose durable point is generation-store CAS, followed by in-memory activation, manifest visibility, repairable YAML projection, and post-commit notification according to the tested order. | Fetch completion, candidate validation alone, a callback before commit, or writing an SSE event. |
| **Manifest** | Small verified metadata naming the current immutable generation, schema, digest, size, and publisher/trust information needed to fetch and verify it. | Mutable catalog data, an unverified redirect target, or a substitute for the generation payload. |
| **Publication event** | A post-commit SSE hint containing committed generation identity and monotonic sequence. It triggers verified fetch/catch-up and contains no mutable catalog payload. | Exactly-once delivery, proof of freshness, a heartbeat, or the catalog itself. |
| **Remote subscriber** | An explicitly started, caller-context-owned Go client lifecycle that performs verified initial fetch, consumes SSE hints, catches up after every reconnect, atomically activates newer generations, and joins on shutdown. | A constructor-owned hidden goroutine, an SSE connection mistaken for catalog freshness, or a normal polling loop. |
| **Stream liveness** | Evidence from timely flushed SSE comments/events that the notification path is alive. | Evidence that the active catalog is current. |
| **Catalog freshness** | Age/state derived from the last successfully verified and activated generation plus the applicable source/publication policy. | TCP/SSE connection state or heartbeat recency alone. |

Normative decision consequences:

1. Author-model YAML is the persisted home and executable authority for
   intrinsic authored facts. Provider-model YAML is the persisted home for
   serving facts and references its author model explicitly. Provider source
   records may retain overlapping upstream claims as evidence, but those claims
   cannot override the linked authored definition.
2. Definitions, offerings, author membership, and endpoint relationships are
   derived read views from the immutable catalog and use the same
   authority/provenance result.
3. `endpoints.yaml` is generated from those explicit links and is never an
   independent authority.
4. Embedded bytes, local edits, models.dev, and provider APIs enter
   reconciliation as identified observations; none replaces the whole catalog
   merely because it was read later.
5. Valid provider pricing is authoritative for that provider's offering.
   Provider price does not become a provider-independent definition fact.
6. Missing or degraded observations cannot prove deletion. Explicit lifecycle
   evidence is required to retire data.
7. Publication is atomic at generation-store CAS. YAML is a repairable
   post-commit projection and an event is a post-commit hint.
8. Public Go consumers say `catalog`; `snapshot` and `generation` remain
   internal lifecycle/storage vocabulary.
9. A remote subscriber treats stream liveness and catalog freshness as
   independent states, always catches up after reconnect, and polls only under
   an explicit fallback policy.

### Human edit contract

- Editing a semantic value creates local evidence for that field.
- Formatting, comments, quoting, and key order are not durable data.
- Generated values keep their original source when YAML still equals the last
  materialized provenance value.
- Valid current dynamic facts normally outrank manual fallback.
- Manual fields survive when dynamic sources do not supply them.
- Removing a field withdraws the local claim and permits source refill.
- Removing a local-only model deletes it.
- Removing an upstream-backed model does not permanently suppress
  rediscovery. Persistent suppression must use an explicit in-record lifecycle
  or exclusion value, not an override file.
- Author-model edits change intrinsic model facts. Provider-model edits change
  only that provider's serving facts or its explicit author/model link.
- `endpoints.yaml` is generated. Direct edits are diagnosed as projection drift
  and are never silently overwritten or accepted as authority.
- A running process does not watch files implicitly. A successful explicit
  reload or update publishes one new immutable generation.

## Go Composition Contract

### In-process library

The normal consumer path remains:

```go
sm, err := starmap.New(...)
if err != nil {
    return err
}

catalog := sm.Catalog()
model, err := catalog.FindModel("gpt-4o")
```

Success requires:

- initialization errors occur in `New`;
- `Catalog()` has no error result;
- the returned static type is the concrete immutable catalog;
- read methods return caller-owned values;
- the root read path does not require server, scheduler, CLI, or provider
  acquisition implementations;
- provider-specific price and limits remain available through an unambiguous
  provider/model query.

### Embeddable server

Another Go program must be able to compose the Starmap server without importing
an `internal` package or invoking the CLI. The public server module must:

- accept an already constructed Starmap/catalog dependency;
- expose health/readiness, manifest, immutable generation, query, and event
  endpoints;
- publish events only after durable generation commit;
- have explicit lifecycle and context ownership;
- validate authentication, body limits, media types, origins, timeouts, and
  streaming compatibility during construction;
- avoid acquiring provider credentials unless the embedding program explicitly
  composes acquisition.

### Reactive remote library

SSE is the sole remote notification transport because catalog publication is
server-to-client. The existing WebSocket path is deleted; a future
reintroduction requires a named bidirectional consumer and a new reviewed
decision. SSE runs through the normal HTTP router, authentication, CORS,
timeouts, proxies, and load balancers without a protocol upgrade.

Publication events are hints that identify a committed immutable generation.
They are not catalog data, and correctness never depends on replay or an
unbroken stream. The server emits flushed SSE comment heartbeats on the same
serialized writer as publication events. The default heartbeat interval is 20
seconds and the default client liveness timeout is 60 seconds; both are
configurable with validation that preserves a useful timeout margin. A
heartbeat does not carry an event ID, advance publication sequence, or trigger
a catalog fetch.

Stream liveness and catalog freshness are separate states. If a publication
cannot be written within its deadline, the server must coalesce toward the
newest generation or terminate the connection so reconnect catch-up runs. It
must never silently discard a generation hint while continuing to report a
healthy stream.

The remote consumer must:

1. fetch and verify the current manifest and immutable payload;
2. atomically publish the initial compatible generation;
3. subscribe to post-commit SSE events;
4. treat an event as a hint, never as catalog data;
5. fetch the addressed immutable generation;
6. verify schema compatibility, identity, size, digest, and publisher policy;
7. deduplicate by generation ID and payload digest;
8. atomically activate only a newer valid generation;
9. reconnect with bounded exponential backoff and jitter;
10. use `Last-Event-ID` where supported;
11. refetch current state after every reconnect so dropped events cannot cause
    permanent staleness;
12. treat absence of an event or heartbeat before the liveness deadline as a
    degraded stream that is canceled and reconnected;
13. expose stream liveness, catalog freshness, and catch-up state separately;
14. stop promptly when its caller-owned context is canceled; and
15. poll only when streaming is unsupported or explicitly configured as
    fallback.

Delivery is at-least-once. Correctness must not depend on exactly-once event
delivery or an unbroken connection.

## Go Modularity and File-Size Policy

The limit applies to every repository-authored `.go` file, including tests.
Vendored code is not repository-authored. Generated files must be split at the
generator when they exceed the hard limit.

| Lines | Policy |
| --- | --- |
| `0–1000` | Normal |
| `1001–1500` | Review required; record whether the module remains deep and conceptually local |
| `1501–1999` | Durable justification required showing why splitting would reduce locality, testability, or leverage |
| `>=2000` | Hard failure; split before merge |

Protected-main baseline findings:

| File | Lines | Required disposition |
| --- | ---: | --- |
| `pkg/reconciler/merger_test.go` | 2059 | Split; hard-limit violation |
| `internal/providers/google/client.go` | 1206 | Review and extract wire, normalization, and error concepts if that improves locality |
| `internal/providers/openai/client.go` | 1183 | Review and extract wire, normalization, and error concepts if that improves locality |
| `pkg/reconciler/merger.go` | 1134 | Deepen reconciliation around one authority implementation; split by concept only |

PR #40 also contains a 2044-line reconciler test and a 1565-line OpenAI
connector. Salvaged work must satisfy this policy before entering a new phase
branch.

Naming review must cover:

- package/import and exported-identifier stutter;
- file names that repeat their package without adding a concept;
- generic `util`, `helpers`, `common`, `manager`, or `service` names;
- public packages with one production caller;
- interfaces with only one real adapter;
- tiny pass-through modules that fail the deletion test; and
- broad interfaces used by commands or tests that should be defined at the use
  site.

Do not split a cohesive deep module only to reduce line count. Extract a named
concept with its own invariant and test surface.

## Test Value Policy

Tests exist to prove contracts and prevent credible regressions.

Priority order:

1. end-to-end user and process workflows;
2. integration tests across real seams;
3. focused unit tests for deterministic policy;
4. fuzzing for untrusted parsing and state-machine inputs;
5. benchmarks for stated performance budgets.

Required high-value suites:

- first-run seed, manual edit, embedded E1→E2 upgrade, live source conflict,
  degraded source, restart, and rollback;
- store-only update with zero filesystem access;
- multi-process workspace writer exclusion and CAS;
- concurrent readers during publication;
- provider-first atomic pricing with rejection evidence;
- provider/model-scoped provenance;
- per-record schema quarantine with valid sibling preservation;
- server publication through manifest/payload/SSE to reactive Go consumer;
- disconnect, reconnect, duplicate, stale, skipped, corrupt, incompatible,
  unauthorized, missing-heartbeat, half-open, slow-consumer, and write-failure
  remote events/generations;
- package consumer compile tests for local, server, and remote compositions;
- deterministic artifact/release reconstruction;
- failure injection at parse, fetch, validate, stage, fsync, rename, commit,
  callback, stream, fetch, and publication points; and
- table/property tests for deterministic authority selection and presence
  semantics;
- fuzzing for untrusted provider envelopes, YAML/JSON
  manifests/payloads/provenance, and SSE framing/event IDs.

Do not:

- assert implementation details available through the public interface;
- duplicate the same behavior across many fixtures without a new failure class;
- treat coverage percentage as proof;
- add mocks for seams with no real variation; or
- retain tests for deleted compatibility behavior before launch.

## Worktree and Branch Strategy

### Active phase worktree

- Worktree:
  `/Users/jack/src/github.com/agentstation/starmap-worktrees/starmap-library-composition`
- Branch:
  `codex/starmap-library-composition`
- Base:
  `origin/main@76dd317810815604b6c796814bce5b8887aaadd0`

This worktree contains the P6 Go-library composition work. It was created from
the exact protected main produced by merged PR #53 before the clean P5 worktree
and local branch were removed.

### Provider expansion worktree

Do **not** reuse:

- Worktree:
  `/Users/jack/src/github.com/agentstation/starmap-worktrees/provider-expansion-wave0`
- Branch:
  `codex/provider-expansion-wave0`
- PR:
  [#40](https://github.com/agentstation/starmap/pull/40)

Reason: the branch makes persisted definitions/offerings the sole schema, which
contradicts the selected provider-YAML architecture. It also changes 576 files
with `+45,142/-10,593`, preventing reliable conceptual review.

The branch is a read-only donor until P1 records a file/commit inventory.
Nothing is merged or cherry-picked wholesale. Each salvage candidate must:

- serve the selected architecture;
- have a named production composition;
- satisfy current file-size and package rules;
- receive focused tests;
- be reapplied on a fresh phase branch; and
- pass the full phase gate.

### Phase worktrees

After the control-plane PR merges, use one fresh worktree per coherent phase,
created from the current protected `origin/main`. Phase IDs are stable ledger
identifiers, not an instruction to execute strictly in numeric order. The
required dependency order is:

1. P1 existing-PR reconciliation;
2. P2 green characterization and product decisions;
3. P3.6a, P3.6b, and P3.8 as the narrow store-only/commit-point/YAML
   atomicity hotfix;
4. P4 authority, provenance, completeness, and resilience;
5. the remaining P3 workspace lifecycle, including legacy-layout migration;
6. P5 read views;
7. the user-steered P5.9–P5.13 author-model restoration, explicit serving
   links, endpoint projection, and immutable identity indexes;
8. P6 composition, P7 server/reactive delivery, P8 modularity,
   P9 distribution, P10 verification, and P11 cleanup.

Suggested branch names:

- `codex/catalog-workspace-lifecycle`
- `codex/catalog-authority-resilience`
- `codex/catalog-read-model-simplification`
- `codex/catalog-author-endpoint-restoration`
- `codex/starmap-library-composition`
- `codex/starmap-reactive-server`
- `codex/starmap-go-modularity`
- `codex/starmap-production-closeout`

Do not run overlapping implementation phases against the same files. Each phase
PR updates this control plane and lands before the next dependent phase starts.

## Live Pull Request Ledger

Live state inspected 2026-07-28.

| PR | Head | Status | Disposition | Verifiable terminal criteria |
| --- | --- | --- | --- | --- |
| [#40](https://github.com/agentstation/starmap/pull/40) | `codex/provider-expansion-wave0@a14d2249` | `DONE` | Superseded after read-only donor inventory | All 66 changed production Go modules and non-Go areas classified; no rejected schema work copied; [closing comment](https://github.com/agentstation/starmap/pull/40#issuecomment-5099100581) links the plan and immutable inventory; zero review threads; PR closed; remote/local branch and worktree removed |
| [#43](https://github.com/agentstation/starmap/pull/43) | Dependabot Go modules `@5f5e54dd` | `SUPERSEDED` | Closed in favor of #46 | Recreated head retained vulnerable `grpc v1.82.0`; replacement exists with the regenerated dependency group plus the security patch; #43 is closed with an exact explanation |
| [#44](https://github.com/agentstation/starmap/pull/44) | Dependabot Actions `@1edb7172` | `SUPERSEDED` | Closed in favor of #47 | Rebased action-only head resolved the vulnerable graph but left structural pin assertions stale; #47 carries equivalent action updates plus reviewed test expectations; #44 and its remote branch are closed |
| [#45](https://github.com/agentstation/starmap/pull/45) | `codex/starmap-architecture-control-plane@662d5714` | `DONE` | Merged as `2561456e` | Exact rebased head passed Verification Gate and Security & Reliability; protection required zero approvals and had no review threads; merged; remote/local branch and worktree removed |
| [#46](https://github.com/agentstation/starmap/pull/46) | `codex/dependency-security-prerequisite@2fbd4c6d` | `DONE` | Replaced #43 and merged as `53285f13` | Exact head contained regenerated direct updates, `x/text v0.40.0`, and `grpc v1.82.1`; current govulncheck, local verification, both hosted gates, and branch-protection readback passed; merged; remote branch removed |
| [#47](https://github.com/agentstation/starmap/pull/47) | `codex/starmap-pr-reconciliation@650f5406` | `DONE` | Replaced #44 and merged as `a87b6425` | Exact head contained only P0/P1 ledger evidence, the three reviewed action-pin updates, and matching structural assertions; actionlint, race fixture, current govulncheck, full local verification, both hosted gates, protection, and review-thread checks passed; merged; remote branch removed |
| [#48](https://github.com/agentstation/starmap/pull/48) | `codex/provider-donor-inventory@b7afa2df` | `DONE` | Closed P1 reconciliation and merged as `08f51ca9` | Exact head contained only the control-plane ledger and exhaustive #40 donor inventory; exact local verification, both hosted gates, protection, and zero review threads passed; merged; remote/local branch and worktree removed |
| [#49](https://github.com/agentstation/starmap/pull/49) | `codex/catalog-contract-characterization@39b08d6d` | `DONE` | P2 characterization and production composition decisions merged as `f8973be3` | Exact head passed the complete P2 affected-package race run, current govulncheck, exact `make verify`, Verification Gate, Security & Reliability, strict protection readback, and zero review threads; merged; remote/local branch and worktree removed |
| [#50](https://github.com/agentstation/starmap/pull/50) | `codex/catalog-publication-hotfix@4f7756e0` | `DONE` | P3.6a/P3.6b/P3.8 commit-point and atomic-projection hotfix merged as `1dc811b5` | Final ledger head passed exact local verification, Verification Gate, Security & Reliability, strict protection readback, and zero review threads; F-003/F-031 and P3.6b are closed; remote/local branch and worktree are removed |
| [#51](https://github.com/agentstation/starmap/pull/51) | `codex/catalog-authority-resilience@7454e3b8` | `DONE` | P4 authority, provenance, presence, and source-resilience phase merged as `60f0cd3c` | Exact head passed local verification, current govulncheck, Verification Gate, Security & Reliability, strict protection readback, and zero review threads; protected squash merge completed; remote/local branch and worktree were removed before the remaining P3 lifecycle started |
| [#52](https://github.com/agentstation/starmap/pull/52) | `codex/catalog-workspace-lifecycle@42295e4e` | `DONE` | P3 human-workspace lifecycle and explicit legacy-layout migration merged as `9609f4f4` | Exact final head passed uninterrupted local verification, Verification Gate, Security & Reliability, strict protection readback, and zero review threads; protected squash merge completed; remote/local branch and worktree removed safely |
| [#53](https://github.com/agentstation/starmap/pull/53) | `codex/catalog-read-model-simplification@94157b42` | `DONE` | P5 single persisted model and derived immutable read views merged as `76dd3178` | Exact head passed local verification, current govulncheck, Verification Gate, Security & Reliability, strict protection readback, and zero review threads; protected squash merge completed; remote/local branch and worktree removed safely before P6 |

Current #44 failure is not caused by the action syntax itself. Both required
jobs ran against `golang.org/x/text v0.38.0`; `govulncheck` reports
GO-2026-5970, fixed in v0.39.0. Replacement PR #46 updated it to v0.40.0 and
merged the additional grpc security fix. The remaining cleanup order is
therefore rebase/recreate #44, then merge its replacement.

Final PR gate:

```bash
test "$(gh pr list --repo agentstation/starmap --state open --limit 100 \
  --json number --jq 'length')" -eq 0
```

## Phase Ledger

| Phase | Status | Outcome | Gate |
| --- | --- | --- | --- |
| P0 | `DONE` | Durable control plane and architecture report are reviewable | P0 tasks and plan PR green after the authorized dependency prerequisite |
| P1 | `DONE` | Existing PRs and donor work receive terminal dispositions | PR ledger terminal; no lost salvage |
| P2 | `DONE` | Catalog contract and keep/delete decisions are characterized before structural change | Green characterization workflows pin current behavior and known defects |
| P3 | `DONE` | One human provider-YAML workspace has deterministic lifecycle | P3.6a/P3.6b/P3.8 first establish one durable commit point, atomic repairable projection, and store-only operation |
| P4 | `DONE` | One authority/provenance implementation is resilient to drift | Authority, presence, quarantine, degradation, and fuzz gates |
| P5 | `IN_PROGRESS` | Linked authored models and provider serving records produce immutable read views and generated endpoints | Disjoint schema ownership; complete corpus disposition; identity, endpoint, DX, and benchmark gates green |
| P6 | `PENDING` | Go library composition is small and canonical | Consumer compile and dependency-closure gates |
| P7 | `PENDING` | Embeddable server and reactive remote consumer are reliable | Real SSE end-to-end and recovery suite |
| P8 | `PENDING` | Go modules have depth, locality, and compliant file sizes | No hard-limit file; every concern dispositioned |
| P9 | `PENDING` | Distribution and embedded upgrade paths preserve exact evidence | Artifact/import/upgrade/reproducibility gates |
| P10 | `PENDING` | Production verification and documentation inspire trust | Full local/hosted/security/docs gates |
| P11 | `PENDING` | GitHub and local machine end clean | No open PRs; clean protected main; obsolete work removed |

Every phase has the following additional exit criteria:

1. every task in the phase is terminal;
2. every finding owned by the phase is terminal or explicitly remapped;
3. the phase evidence names the exact commit and test commands;
4. local verification passes on the exact phase head;
5. required hosted checks pass on that same head;
6. the phase PR is merged or intentionally closed;
7. this ledger is updated on protected main; and
8. no newly discovered finding is left outside the Finding Ledger.

## P0 — Establish the Control Plane

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P0.1 | `DONE` | Inspect protected main, live PRs, branches, and worktrees | Baseline SHA, three open PRs, branch divergence, and worktree inventory are recorded |
| P0.2 | `DONE` | Decide the implementation base | Fresh control-plane worktree exists from exact protected main; PR #40 is explicitly rejected as a base |
| P0.3 | `DONE` | Write the durable plan and `/goal` prompt | Mission, invariants, phase/task/finding/PR/workspace ledgers, gates, and continuation rules exist |
| P0.4 | `DONE` | Archive the architecture report in the repository | Repository HTML has a recorded SHA; this file uses a relative link; HTML parses locally; CDN-backed visual enhancement is not claimed to work offline |
| P0.5 | `DONE` | Reconcile independent review and publish the plan PR | The full Fable review is archived with a recorded SHA-256 and every finding has an explicit disposition; historical supersessions are mapped; Markdown links, docs check, diff check, required local verification, and hosted checks pass on exact head; user authorized running #43 first because #45 inherits two reachable vulnerabilities |

P0 gate:

```bash
test -f docs/STARMAP_ARCHITECTURE_CONTROL_PLANE.md
rg -n '^## `/goal` Prompt|^## Phase Ledger|^## Finding Ledger|^## Evidence Log' \
  docs/STARMAP_ARCHITECTURE_CONTROL_PLANE.md
test -f docs/reviews/STARMAP_ARCHITECTURE_REVIEW_2026-07-27.html
test -f docs/reviews/FABLE_STARMAP_PLAN_REVIEW_2026-07-27.md
test -f docs/reviews/FABLE_STARMAP_PLAN_REVIEW_DISPOSITION_2026-07-27.md
make docs-check
git diff --check
```

## P1 — Reconcile Existing Pull Requests and Donor Work

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P1.1 | `DONE` | Revalidate replacement PR #46 | Current exact head contains the regenerated #43 group, `x/text v0.40.0`, and `grpc >= v1.82.1`; exact-head local verification, race selection, current govulncheck, and hosted checks pass |
| P1.2 | `DONE` | Merge replacement PR #46 | Protected merge succeeds; main contains x/text v0.40.0 and grpc >= v1.82.1; #43 is superseded; #46 and its branch are closed |
| P1.3 | `DONE` | Rebase/recreate and verify PR #44 through replacement #47 | Exact head is based on post-#46 main; workflow fixture, actionlint, verification, and security checks pass |
| P1.4 | `DONE` | Merge replacement PR #47 | Protected merge succeeds; old failed #44 head is superseded; PRs and remote branches are closed |
| P1.5 | `DONE` | Inventory PR #40 | Every changed production module is marked salvage, already-landed, reject, or superseded with rationale in [`reviews/PR40_DONOR_INVENTORY_2026-07-27.md`](reviews/PR40_DONOR_INVENTORY_2026-07-27.md) |
| P1.6 | `DONE` | Close PR #40 | Closing note links this plan and inventory; no open review threads are misrepresented as resolved |
| P1.7 | `DONE` | Remove #40 worktree and branches in safe order | Worktree is removed before its checked-out local branch; remote and local branches are absent; retained evidence lives in docs/Git history |
| P1.8 | `DONE` | Rebase control-plane/next phase on current main | No dependency/action regression; ledger records final PR SHAs |

P1 gate:

```bash
gh pr list --repo agentstation/starmap --state open --limit 100
git worktree list
git branch -vv --all
govulncheck ./...
```

No implementation task starts while #43/#44 are unresolved or #40 remains an
active alternative architecture.

## P2 — Characterize the Product Contract

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P2.1 | `DONE` | Record domain vocabulary and decisions | Catalog, workspace, observation, generation, offering, definition, publication, and remote subscriber have one documented meaning |
| P2.2 | `DONE` | Characterize current local/store paths | Green F-001/F-002 tests pin store-only failure before commit, input/output divergence, embedded write-path leakage, and durable-store restart precedence at their current call sites |
| P2.3 | `DONE` | Characterize reconciliation loss | Green F-004/F-005/F-007/F-008 tests pin manual-model drop at `pkg/catalogs/catalog.go:483-491`, non-boolean zero-value clearing asymmetry in `pkg/catalogs/merge.go`, provider omission pruning through wholesale `SetProvider` replacement, degraded-source replacement, and the bare-model-ID provenance tracker keying (`pkg/reconciler/merger.go:202`, `pkg/provenance/provenance.go:147-149`) whose report winner is timestamp-order-dependent |
| P2.4 | `DONE` | Characterize schema resilience | Green F-009 tests prove malformed-sibling whole-collection loss at the monolithic models.dev unmarshal (`internal/sources/modelsdev/parser.go:241`), single provider-response decode (`internal/transport/request.go:88`), Google multi-page loop, local YAML walk (`pkg/catalogs/load.go`), and strict stored-payload decode (`pkg/catalogstore/payload.go:37-39`); invalid configured YAML remains typed fail-closed; `cmd/starmap/cmd/providers/fetch.go:460-478` is the existing raw-message skip reference |
| P2.5 | `DONE` | Characterize server/remote flow | Green real-transport and callback tests record verified manifest/payload behavior, ignored Last-Event-ID, duplicate second-resolution SSE IDs, opposite SSE/WebSocket backpressure, within-generation callback order, cross-generation overtaking, whole-generation hook drop, and one-shot remote fetch with no reconnect lifecycle |
| P2.6 | `DONE` | Measure public composition after #46 | [`reviews/P2_COMPOSITION_BASELINE_2026-07-27.md`](reviews/P2_COMPOSITION_BASELINE_2026-07-27.md) records exact root/catalog/server/Google closures and attribution, frozen numeric ceilings, binary sizes, accessor allocations/latency, package/file/LOC totals, embedded bytes, and every >1000-line file |
| P2.7 | `DONE` | Freeze user journeys | [`reviews/P2_USER_JOURNEYS_2026-07-27.md`](reviews/P2_USER_JOURNEYS_2026-07-27.md) and five parsed golden fixtures cover the in-process library, CLI workspace, embedded upgrade, public `server` package, and opt-in public `remote` subscriber |
| P2.8 | `DONE` | Decide production compositions before deletion | [`reviews/P2_PRODUCTION_COMPOSITION_DECISIONS_2026-07-27.md`](reviews/P2_PRODUCTION_COMPOSITION_DECISIONS_2026-07-27.md) retains one server manifest/payload/SSE plus public `remote` flow and the immutable artifact format; it records deletion of the unused hosted distribution protocol, scheduler subsystem, and WebSocket path before P6.5/P7/P9.6 |
| P2.9 | `DONE` | Close the characterization phase | Exact head `39b08d6d` passed `make verify`, current `govulncheck`, documentation/diff checks, and the complete P2 affected-package race suite; protected PR #49 passed Verification Gate and Security & Reliability and merged as `f8973be3` before the P3.6a/P3.6b/P3.8 hotfix began |

P2 never merges a failing test. Each known defect receives a green
characterization test that pins the currently observed defective behavior,
names its finding ID, and explains the intended correction. The fixing PR
rewrites or inverts that expectation and proves the desired behavior. Consumer
compile and performance baselines must also be green.

## P3 — Restore One Human Catalog Workspace

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P3.1 | `DONE` | Restore one catalog path safely | One configured path names the human YAML tree; before mutation Starmap detects the pre-plan machine layout (`current`, `generations/`, `.commit.lock`) and returns a typed error that names the required migration unless an explicit transactional migration was selected |
| P3.2 | `DONE` | Separate machine state | Locks, staging, generations, and caches are machine-owned and cannot be mistaken for override configuration |
| P3.3 | `DONE` | Make first-run seed atomic | Missing workspace becomes one complete embedded-seeded tree or remains absent after failure |
| P3.4 | `DONE` | Reconcile embedded E1→E2 after P4 | New embedded revision updates unchanged embedded-derived fields, fills gaps, and preserves actual human edits using the completed P4 authority/provenance model |
| P3.5 | `DONE` | Detect semantic human edits after P4 | Only changed semantic paths become local evidence under the completed P4 model; formatting-only changes do not |
| P3.6a | `DONE` | Establish one durable commit point before P4 | Generation-store CAS is the sole commit point; post-commit YAML failure returns an observable `pending_repair` projection result without rolling back the store or immutable in-memory catalog; commit failure still publishes neither |
| P3.6b | `DONE` | Make YAML projection atomic and repairable before P4 | YAML is staged, validated, fsynced, input-digest checked, and atomically projected only after commit; startup compares digests and repairs an interrupted or stale projection without republishing the generation |
| P3.7 | `DONE` | Add multi-process writer control | Two processes cannot interleave; loser receives typed busy/conflict; readers remain available |
| P3.8 | `DONE` | Preserve store-only use | `TestStoreOnlyApplyCommitsWithoutWorkspaceAccess` proves a configured catalog store commits and publishes with a nil projection and no workspace path or filesystem operation |
| P3.9 | `DONE` | Define reload and rollback | No implicit watcher; explicit reload publishes once; rollback restores exact YAML semantics, provenance, digest, and reads |
| P3.10 | `DONE` | Prove migration, restart, and downgrade behavior | Existing machine-layout fixtures are detected before mutation and explicitly migrated or rejected transactionally; restart is identical; unknown newer schema and older binary fail before mutation |

## P4 — Consolidate Authority, Provenance, and Source Resilience

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P4.1 | `DONE` | Replace duplicate authority policies | One executable field table selects all winners; the shadow `complexModelStructures` policy and unused parallel inventory are deleted |
| P4.2 | `DONE` | Enforce provider pricing authority | Valid provider price wins atomically for its offering; invalid price records durable rejection evidence and falls back; rejection evidence remains when every candidate is invalid |
| P4.3 | `DONE` | Make local data fallback | Dynamic valid facts beat local for discoverable fields; manual missing data and operator configuration survive |
| P4.4 | `DONE` | Scope evidence by provider/model | Shared model IDs cannot collide in price, limit, availability, or lifecycle provenance |
| P4.5 | `DONE` | Preserve source identity through YAML | Reloading generated YAML does not relabel unchanged provider/models.dev/embedded values as local |
| P4.6 | `DONE` | Model presence explicitly | Tri-state or equivalent typed representation makes missing, explicit false, explicit zero, empty, and unknown round-trip distinctly for limits, features, and other affected fields |
| P4.7 | `DONE` | Consume observation health and make absence non-authoritative | Reconciliation consumes source status, completeness, issues, and volume history; complete omission, partial response, timeout, fetch failure, and suspicious volume collapse cannot hard-delete or retire |
| P4.8 | `DONE` | Quarantine records independently | Every P2.4-characterized whole-collection decode site (models.dev envelope, provider list responses, local YAML walk, stored payload) isolates a malformed record while valid siblings survive; collection envelope remains bounded |
| P4.9 | `DONE` | Make strict mode truthful | Every required source must be `Complete` and `Succeeded`; missing credentials, degraded/skipped state, stale fallback, or empty results without explicit issues fail before publication |
| P4.10 | `DONE` | Test policy and fuzz untrusted decoders | Authority/presence pass deterministic table and property tests; provider envelopes and provenance decoding pass bounded fuzz corpora without panic |

## P5 — Keep One Persisted Model and Derive Read Views

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P5.1 | `DONE` | Keep provider YAML/payload canonical | No persisted definition/offering tree or top-level duplicate collection exists |
| P5.2 | `DONE` | Internalize build projection | No exported “legacy migration” vocabulary remains before launch |
| P5.3 | `DONE` | Derive offerings | Same ID at two providers retains distinct exact price, limits, availability, endpoint, and lifecycle |
| P5.4 | `DONE` | Derive provider-independent definitions | Conflicts use the same authority/evidence implementation, never map iteration or alphabetical first-wins |
| P5.5 | `DONE` | Derive author membership | Provider-backed author attribution is deterministic without author model copies; the 121 non-provider records receive the reviewed 32/25/64 terminal disposition in `docs/reviews/P5_AUTHOR_MODEL_CORPUS_DISPOSITION_2026-07-28.md` |
| P5.6 | `DONE` | Remove invented facts | Unknown availability/lifecycle remains unknown; migration/build does not invent “available” or “active” |
| P5.7 | `DONE` | Preserve immutable catalog DX | Consumer compile example, mutation isolation, concurrent publication, and `BenchmarkClientCatalog` at 0 allocs/op and no more than 10 µs/op pass |
| P5.8 | `DONE` | Remove prelaunch compatibility | Alias/deprecated types and schema readers with no named external consumer are deleted |
| P5.9 | `DONE` | Restore and disposition the authored-model corpus | All 322 files deleted by PR #53 are restored and retained as authored/history records in an exact machine-checkable map; 197 retain an exact current provider ID and four provider-artifact IDs are canonicalized while remaining explicitly served; 30 missing primary authors are repaired, lab authorship is independent from the serving provider, and provider status, price, limits, modes, and provider extensions are absent; ordinary and race-enabled exact corpus/bootstrap tests pass |
| P5.10 | `DONE` | Separate author and provider record ownership | Author models are the sole executable authority for intrinsic definition facts; provider records own serving facts and may retain overlapping upstream observations only as non-overriding evidence; every provider serving record has an explicit `model: author/slug` reference; provider identity is never authorship evidence; load/save/projection/payload/provenance round-trip both roles without first-wins or duplicate-field authority |
| P5.11 | `DONE` | Generate and validate endpoints.yaml | A versioned deterministic digest-bound endpoint projection joins each provider serving record to its author model; projection is built off-side and atomically written post-commit; drift is detected and never silently overwritten; no row exists without a valid provider/model link and provider price remains exact |
| P5.12 | `DONE` | Build canonical identity and offering indexes | Immutable publication precomputes canonical author/slug, author alias, model alias, provider offering, and model-to-offerings indexes; cross-provider IDs for one model produce one definition with distinct exact offerings; ambiguity/collisions return typed errors and cannot replace the current generation |
| P5.13 | `IN_PROGRESS` | Prove restored catalog DX and corpus completeness | Tests prove Alibaba-served Moonshot/Zhipu models resolve to their labs, author and provider edits round-trip independently, all retained author records and provider links load, endpoint generation is deterministic, malformed siblings quarantine safely, strict/release coverage has no unresolved public identity, docs explain the two source roles and generated projection, and an outcome autoreview plus exact phase gate pass |

## P6 — Deepen Go Library Composition

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P6.1 | `DONE` | Map the package graph | [`reviews/P6_PACKAGE_GRAPH_2026-07-28.md`](reviews/P6_PACKAGE_GRAPH_2026-07-28.md) names the role, production consumer, and disposition of every public/importable package; `go list` proves zero cycles; the 89→90 package change is exactly explained |
| P6.2 | `PENDING` | Keep read-only consumption small | After P5.9–P5.13, invert the `pkg/sources` → internal provider-client edge behind an injected factory and remove all remaining pipeline, models.dev, remote HTTP, and concrete acquisition imports from the root package; a real external `starmap.New().Catalog()` consumer stays within the numeric P2.6 dependency budget and its compile closure contains no GenAI, gRPC, OpenTelemetry, WebSocket, SQLite, Cobra, scheduler, server, or acquisition implementation; a CI dependency-closure assertion enforces the budget so regression fails the verification gate |
| P6.3 | `PENDING` | Move acquisition behind explicit composition | A named opt-in provider-client composition path serves CLI/server acquisition; read-only library behavior remains complete without importing it |
| P6.4 | `PENDING` | Narrow interfaces at use sites | Command, source, storage, server, and remote consumers define the smallest real role interfaces; the broad `internal/application.Application` interface is split by consumer |
| P6.5 | `PENDING` | Delete hypothetical seams after P2.8 | Unused enhancer wiring, `catalogdistribution`, `catalogscheduler` (including the inert operations projection), `sourceevidence`, registry, compatibility aliases, `internal/utils/ptr`, and pass-through save modules are removed; real operational health moves to the production-owned state named by P7.11 |
| P6.6 | `PENDING` | Validate concrete consumer examples | Separate external test modules compile local library, store-only, server embed, and remote subscriber programs |
| P6.7 | `PENDING` | Measure improvement | Dependency closure, build time, binary size, package count, Go LOC, and public exports are no worse without explicit rationale |
| P6.8 | `PENDING` | Make construction and access canonical | `Catalog()` has documented nil-receiver behavior consistent with neighboring methods; storage-backed construction has a caller-owned context/cancellation path; cancellation, timeout, successful construction, and O(1) non-failing access pass external consumer tests |

## P7 — Embeddable Server and Reactive Remote Consumer

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P7.1 | `PENDING` | Expose an embeddable server module | A public server package accepts `*starmap.Client` or a narrower catalog/events role; an external Go fixture constructs, starts, drains, and stops it without importing `internal` or CLI packages |
| P7.2 | `PENDING` | Make publication ordering exact | Generation CAS is the commit point; catalog swap, cache activation, manifest visibility, and event publication have one tested order; concurrent generations cannot reorder; overload coalesces to the newest generation or closes the stream and cannot silently drop a generation while the connection remains healthy |
| P7.3 | `PENDING` | Keep one heartbeat-enabled reactive transport | SSE is canonical; delete WebSocket transport, hub, adapter, dependency, tests, and documentation; one serialized per-connection writer emits and flushes publication events plus comment-line heartbeats, defaults to a 20-second interval, applies write deadlines, and promptly cleans up a failed/dead connection |
| P7.4 | `PENDING` | Define one stable publication event | The only event is catalog publication with generation ID and monotonic per-stream sequence; no per-model event or mutable catalog payload exists; heartbeat comments carry no ID, do not advance sequence, and do not trigger fetch; dedupe and mandatory catch-up survive reconnect |
| P7.5 | `PENDING` | Implement explicit reactive lifecycle | Caller context owns initial fetch, stream, retry, activation, and shutdown; constructor starts no hidden goroutine; every owned loop is joinable within a bounded shutdown; absence of an event or heartbeat for the default 60-second liveness timeout marks the stream degraded, cancels it, and starts reconnect plus catch-up |
| P7.6 | `PENDING` | Verify immutable generations | Client rejects wrong media, size, digest, ID, schema, redirect origin, publisher, or stale generation before activation |
| P7.7 | `PENDING` | Recover from stream loss | Reconnect uses bounded backoff/jitter, Last-Event-ID when supported, and mandatory current-manifest catch-up; replay remains an optimization rather than a correctness dependency |
| P7.8 | `PENDING` | Make polling last resort | No normal polling occurs while recent heartbeat/event activity establishes a healthy stream; fallback begins only through an explicit bounded streaming-failure policy, is observable, uses conditional manifest requests, and stops before streaming is declared healthy again |
| P7.9 | `PENDING` | Prove concurrent read safety | Readers observe complete old/new generations while stream activates updates under `-race` |
| P7.10 | `PENDING` | Exercise real transport failures | Tests prove heartbeats flush and are ignored as events; interval/timeout validation, missing-heartbeat, half-open connection, write failure, slow consumer, disconnect, duplicate, out-of-order, skipped, unauthorized, corrupt, incompatible, subscribe-after-stop, blocked subscriber, reconnect/catch-up, non-concurrent polling, cleanup, and shutdown/join cases pass |
| P7.11 | `PENDING` | Expose distinct production health | Server and subscriber separately expose stream state, last heartbeat/event, last successful catch-up, active generation, catalog freshness, retry count, last error, and any coalesced/terminated publication delivery without secrets; heartbeat activity cannot falsely refresh catalog age and silent whole-generation loss is impossible |
| P7.12 | `PENDING` | Expose OpenRouter model and endpoint compatibility | The public server implements exact `GET /api/v1/model/{author}/{slug}` and `GET /api/v1/models/{author}/{slug}/endpoints` route shapes through server-owned DTOs; aliases resolve deterministically; endpoint rows preserve provider identity, exact model ID, price, limits, parameters, and status; optional runtime metrics come from a separate role and are never invented or persisted into catalog YAML |

## P8 — Go Modularity, Naming, and Complexity

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P8.1 | `PENDING` | Add file-size verification | CI lists >1000, requires reviewed >1500 rationale, and fails every repository-authored file >=2000 |
| P8.2 | `PENDING` | Split current hard-limit test | `pkg/reconciler/merger_test.go` is divided by behavior with no duplicated fixture machinery |
| P8.3 | `PENDING` | Review >1000 production files | Each file is split by concept or receives recorded depth/locality rationale; `pkg/differ/differ.go` (exactly 1000 lines at baseline) joins the review set; no unreviewed concern remains |
| P8.4 | `PENDING` | Audit package/file stutter | Every finding is renamed, consolidated, deleted, retained with rationale, or rejected; explicitly review `provenance.ProvenanceFile`, `format.Formatter`, `internal/utils/ptr`, and the seven-package `catalog*` family |
| P8.5 | `PENDING` | Audit pockets of complexity | Cyclomatic/cognitive hot spots are mapped to domain concepts and deepened without pass-through modules |
| P8.6 | `PENDING` | Apply the residual deletion test | After P6.5, remaining public modules with no production caller and seams with one adapter are removed unless a concrete near-term composition is proven; dead exported behavior with zero callers, including `differ.Changeset.Filter` and the inert `ApplyAdditive` strategy path, receives the same disposition |
| P8.7 | `PENDING` | Keep tests modular | No test file exceeds 1999 lines; shared fixtures hide setup, not assertions or behavior |
| P8.8 | `PENDING` | Preserve canonical Go | `go vet`, lint, race, error, context, cleanup, documentation, and package naming reviews pass |

## P9 — Distribution and Embedded Upgrade Integrity

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P9.1 | `PENDING` | Publish exact committed generation | Release staging never reconstructs source lineage as one local observation |
| P9.2 | `PENDING` | Separate semantic and evidence digests | Identical catalog facts do not create catalog releases solely from observation timestamps |
| P9.3 | `PENDING` | Verify embedded manifest | Binary bootstrap bytes, generation ID, schema, digest, and size agree deterministically |
| P9.4 | `PENDING` | Verify E1→E2 workspace upgrade | New embedded data merges by provenance/authority and never wholesale-overwrites human data |
| P9.5 | `PENDING` | Verify release import | Checksum, detached statement, publisher identity, compatibility, reconciliation, and rollback pass |
| P9.6 | `PENDING` | Implement the P2.8 distribution decision | The selected remote/server/release flow is wired end-to-end; unused competing protocols and packages are removed |
| P9.7 | `PENDING` | Preserve offline/air-gap behavior | Embedded and pinned artifact startup work without network or provider credentials |

## P10 — Production Verification and Documentation

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P10.1 | `PENDING` | Run exact toolchain verification | `make verify`, race-short suite, vet, lint, actionlint, docs, and diff checks pass |
| P10.2 | `PENDING` | Run high-value integration suite | All P3–P9 end-to-end workflows pass repeatedly under race detection |
| P10.3 | `PENDING` | Run fuzz and fault campaigns | Required fuzz targets and every named failure injection complete with no crash or state corruption |
| P10.4 | `PENDING` | Re-measure budgets | Catalog access, publication, remote activation, dependency closure, binary, embedded size, packages, LOC, and file lengths are recorded |
| P10.5 | `PENDING` | Align documentation | README, GoDoc, architecture, CLI/server/remote docs, examples, and control plane describe the same behavior |
| P10.6 | `PENDING` | Run security review | Credentials, redirects, SSRF, auth, origins, body limits, decompression, symlinks, permissions, and supply chain pass |
| P10.7 | `PENDING` | Run structured final review | Independent review has no unresolved P0/P1 finding; lower findings receive terminal disposition |
| P10.8 | `PENDING` | Verify exact hosted PR head | Verification Gate and Security & Reliability pass; protection readback remains strict |
| P10.9 | `PENDING` | Audit catalog fact consistency | Review authored and serving records for contradictory capability flags/modalities, impossible release/knowledge dates, generation defaults outside known ranges, and ambiguous timestamp semantics; verify corrections against authoritative sources, record deliberate unknown/sentinel semantics, and add only high-value regression checks |

## P11 — Final Repository and Machine Cleanup

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P11.1 | `PENDING` | Merge or close every plan PR | GitHub open PR count is zero; merged commits are on protected main |
| P11.2 | `PENDING` | Remove remote topic branches | Every plan-created/superseded branch is deleted after reachability/evidence check |
| P11.3 | `PENDING` | Remove temporary worktrees | Only the intended primary checkout remains; `git worktree prune` is clean |
| P11.4 | `PENDING` | Reconcile divergent local main | Historical `3787d716` is first made reachable through a deliberate archive branch or annotated tag and verified; local main is then safely realigned to origin/main without losing unrecorded work |
| P11.5 | `PENDING` | Remove obsolete local branches | Gone/merged/superseded branches are deleted only after reachability and worktree checks |
| P11.6 | `PENDING` | Verify protected main | Local and remote main SHAs match; required checks, reviews, conversation resolution, admin enforcement, and no force-push/deletion remain configured |
| P11.7 | `PENDING` | Run clean clone proof | Fresh clone passes documented library, server, reactive consumer, verification, and docs workflows |
| P11.8 | `PENDING` | Close the ledger and define the post-merge gate | The closing ledger PR is the final plan commit; every phase/task/finding/PR/workspace row is terminal, evidence totals match, and the exact post-merge machine-gate commands/expected output are recorded |
| P11.9 | `PENDING` | Resolve historical release work | Historical F-099/F-105/F-106 and P12.4/P12.7/P12.8 are completed, explicitly superseded, or user-accepted as rejected with residual risk; no release is published without separate authority |

Final machine gate:

```bash
test -z "$(git status --porcelain)"
test "$(git rev-parse main)" = "$(git rev-parse origin/main)"
test "$(git rev-list --left-right --count main...origin/main)" = $'0\t0'
test "$(git worktree list --porcelain | rg '^worktree ' | wc -l | tr -d ' ')" -eq 1
test "$(gh pr list --repo agentstation/starmap --state open --limit 100 \
  --json number --jq 'length')" -eq 0
```

P11.8 records this command block and the expected clean values in the closing
ledger PR. Run it after that PR merges; the successful readback is the terminal
machine evidence and does not require a follow-up documentation commit.

## Finding Ledger

| Finding | Status | Description | Owning task |
| --- | --- | --- | --- |
| F-001 | `DONE` | One selected provider-YAML workspace is read exactly, observed independently, and atomically projected after commit; explicit reload/update reconciles semantic edits while construction never silently rewrites it | P3.1–P3.5 |
| F-002 | `DONE` | Store-only apply skips YAML entirely, commits the generation, and publishes the immutable catalog | P3.8 |
| F-003 | `DONE` | YAML replacement is destructive and non-atomic | P3.6b |
| F-004 | `DONE` | Embedded and human catalogs enter reconciliation as separate observations; embedded revision upgrades preserve semantic human fields and formatting does not become local evidence | P3.4–P3.5 |
| F-005 | `DONE` | Complete, partial, degraded, failed, and volume-regressed source attempts retain baseline-only models and exact provenance; stale fallback cannot regress known facts | P4.7 |
| F-006 | `DONE` | One executable field table now selects dynamic facts, local fallback, and operator configuration without a second complex-field policy | P4.1–P4.3 |
| F-007 | `DONE` | Provider/model identity is now part of every durable model-provenance key, report, payload round trip, and normal catalog lookup | P4.4 |
| F-008 | `DONE` | Compact typed presence now distinguishes missing, explicit false/zero/empty, and unknown through source decode, YAML/JSON, copy, merge, reconciliation, and change reporting | P4.6 |
| F-009 | `DONE` | Provider APIs, models.dev, local YAML, and stored payloads now quarantine malformed model records independently with bounded typed evidence; valid siblings survive while malformed envelopes, identity graphs, and manifest-bound partial payloads remain fail-closed | P4.8 |
| F-010 | `DONE` | `RequireAllSources` now rejects unavailable, missing, duplicate, degraded, partial, fallback, volume-regressed, quarantined, and unexplained empty observations before reconciliation; only exactly one complete, succeeded, nonempty observation per configured source passes | P4.9 |
| F-011 | `DONE` | Definitions and offerings are immutable build-time read views; the exported runtime legacy migration layer is deleted | P5.1–P5.4 |
| F-012 | `SUPERSEDED` | P5 deleted the 322-file author-model mirror and derived membership from provider models; user review found the corpus also carried authored identity/metadata needed for author/slug compatibility, so P5.9–P5.13 restores it with disjoint ownership instead of the old denormalized-copy semantics | P5.5, P5.9–P5.13 |
| F-013 | `DONE` | Derived offerings and definitions preserve unknown availability, lifecycle, and open-weight claims without inventing available/active/false | P5.6 |
| F-014 | `PENDING` | Prelaunch compatibility and unused public surfaces add shallow modules | P5.8, P6.5 |
| F-015 | `PENDING` | Root library import pulls provider/cloud acquisition stack | P6.2–P6.3 |
| F-016 | `PENDING` | Server is internal and cannot be cleanly embedded by another Go program | P7.1 |
| F-017 | `PENDING` | Remote package consumer has no push-driven update lifecycle | P7.4–P7.10 |
| F-018 | `PENDING` | SSE and WebSocket exist without one canonical remote client contract | P7.3–P7.10 |
| F-019 | `PENDING` | Hook/event delivery is lossy, provider-ambiguous, and unordered | P7.2–P7.4 |
| F-020 | `PENDING` | Current main has a 2059-line Go test file | P8.1–P8.2 |
| F-021 | `PENDING` | >1000-line production files lack explicit modularity review | P8.3 |
| F-022 | `PENDING` | Package/file stutter and unused seams have no terminal audit | P8.4–P8.6 |
| F-023 | `PENDING` | Release staging can erase real source lineage | P9.1 |
| F-024 | `PENDING` | Observation timestamps churn semantic catalog generations | P9.2 |
| F-025 | `PENDING` | Multiple distribution implementations are not wired to one consumer | P9.6 |
| F-026 | `DONE` | PR #40's rejected persisted schema was not copied; every changed module was inventoried and the draft/worktree/branches were closed and removed safely | P1.5–P1.7 |
| F-027 | `DONE` | Stale PR #43 was superseded by merged #46 after Dependabot recreation retained vulnerable grpc v1.82.0 | P1.1–P1.2 |
| F-028 | `DONE` | PR #44 failed on the vulnerable pre-#43 dependency graph; rebased head `1edb7172` has zero reachable vulnerabilities | P1.3–P1.4 |
| F-029 | `PENDING` | Local main diverges and multiple stale worktrees/branches remain | P11.2–P11.5 |
| F-030 | `PENDING` | Existing architecture docs contain superseded “YAML export” guidance | P10.5 |
| F-031 | `DONE` | Sync saves YAML before the durable generation commit, making a fragile projection gate the durable product | P3.6a–P3.6b |
| F-032 | `DONE` | Legacy machine layouts at `~/.starmap/catalog` are detected before mutation and require the explicit transactional migration, whose restart, rollback, concurrent-recreation, and downgrade behavior is proven | P3.1, P3.10 |
| F-033 | `DONE` | Reconciliation consumes observation status, completeness, counts, issues, and volume history; strict mode additionally requires one complete, succeeded, nonempty observation for every configured source before reconciliation | P4.7, P4.9 |
| F-034 | `PENDING` | Publication generations can reorder; current event identity is timestamp-based or provider-ambiguous | P7.2, P7.4 |
| F-035 | `PENDING` | Server/background shutdown lacks owned joins; stopped or blocked subscriptions can hang | P7.5, P7.10 |
| F-036 | `PENDING` | Hook overload can drop a whole generation and the counter is not an adequate operational contract | P7.2, P7.11 |
| F-037 | `PENDING` | Historical F-099/F-105/F-106 release findings lack terminal mapping in the new plan | P0.5, P11.9 |
| F-038 | `PENDING` | Hour-scale publication gaps require SSE heartbeats; without them intermediaries reap idle streams, half-open clients linger, and polling/stream health cannot be determined reliably | P7.3, P7.5, P7.8, P7.10–P7.11 |
| F-039 | `DONE` | The plan PR's original dependency graph had reachable GO-2026-5970 (`x/text v0.38.0`) and GO-2026-6061 (`grpc v1.82.0`); replacement #46 resolved both and rebased PR #45 passed exact local and hosted proof | P0.5, P1.1–P1.2 |
| F-040 | `DONE` | Dependabot PR #44 updated reviewed action pins without updating the exact structural assertions in `internal/ciworkflow`; replacement PR #47 updated both and passed exact local and hosted verification | P1.3–P1.4 |
| F-041 | `PENDING` | `make verify` labels its provider listing credential-free but inherits ambient cloud SDK state; the P1 run reported Google Vertex `Configured` from the developer's ADC despite unset provider API-key variables | P10.1, P10.6 |
| F-042 | `DONE` | The comment-aware YAML literal encoder doubled backslashes on every save/load cycle, so an embedded model description could not reproduce its committed semantic digest | P3.6b |
| F-043 | `DONE` | Local and hosted lint were non-hermetic: Devbox supplied Go 1.25.1 plus golangci-lint v2.5.0 built with Go 1.25, `scripts/verify.sh` preferred any host version, and workflows installed v2.5.0. Devbox now pins Go 1.26.5; Devbox, Make, the required verification script, PR/release workflows, and structural tests all pin golangci-lint v2.12.2; verification rejects absence/version drift instead of silently skipping lint | P3.6b |
| F-044 | `DONE` | The first root gate after P4.6 found that canonical presence encoding required an embedded-manifest refresh and that a generic YAML map round trip collapsed a one-author slice into a mapping; direct typed YAML assembly plus a regenerated verified manifest repaired both before P4 continued | P4.7 |
| F-045 | `DONE` | The first P4.10 repository gate found the authority package below its enforced coverage floor because stable provenance-path selection was not exercised; the exhaustive policy invariant now verifies both explicit evidence paths and path fallback, raising statement coverage from 88.9% to 95.6% | P4.10 |
| F-046 | `DONE` | The P4 hosted race gate exposed a transport-test lifecycle race: its SSE request context and WebSocket read deadline expired while a valid full-catalog update was still computing on a slower runner; connection establishment is now separately bounded, stream lifetime is caller-owned, the read deadline starts only after publication, and readiness includes both broker adapters | P4.10 |
| F-047 | `DONE` | YAML projection previously bound only workspace path/presence, so a semantic human edit after candidate construction but before post-commit projection could be overwritten; candidate input now carries the loaded semantic digest and projection returns a typed conflict while preserving the edit | P3.9 |
| F-048 | `DONE` | Repository verification found the P3.1 `LegacyCatalogLayoutError` formatting contract untested, lowering `pkg/errors` below its enforced coverage floor; direct target/no-target and identified/fallback-entry tests raise coverage to 84.3% without weakening the gate | P3.10 |
| F-049 | `DONE` | Hosted race verification exhausted Go's implicit 10-minute per-package timeout while unrelated migration/rollback fixtures repeatedly decoded and projected the full embedded catalog; focused fixtures now use the smallest catalog that proves their contract, tests that do not exercise construction use direct clients, and repository race verification has an explicit 20-minute package ceiling within the unchanged 45-minute job | P3.10 |
| F-050 | `DONE` | Outcome review found that migration rollback could unconditionally delete a path recreated after store relocation; rollback now removes only its exact checksum-bound YAML projection, preserves unexpected concurrent data plus relocated state, and returns a typed conflict | P3.10 |
| F-051 | `DONE` | Deleted inert `WithEmbeddedCatalog` and `use_embedded_catalog`; verified embedded data remains the unconditional lowest-authority observation | P5.8, P6.5 |
| F-052 | `SUPERSEDED` | The earlier 32 alias/content, 25 models.dev-only, and 64 presumed-orphan disposition rejected a second provider-model truth; P5.9 now reviews all 121 as authored-model identity/metadata while preserving the rule that none may manufacture a provider endpoint | P5.5, P5.9 |
| F-053 | `DONE` | Indistinguishable conflicting definition values now remain unknown or use the stable identity fallback; only matching provider/model-scoped authority evidence may select a winner | P5.4 |
| F-054 | `DONE` | Removed duplicate `Precision`/`Quantization` and nested/flat cache-pricing spellings so source payloads and human YAML cannot project different semantics | P5.1, P5.8 |
| F-055 | `DONE` | Same-ID provider records remain distinct and carry canonical provider identity through offering indexes, hooks, queries, HTTP, CLI table/JSON/YAML, history aliases, and export policy | P5.3, P5.7 |
| F-056 | `DONE` | Partial or null metadata provenance cannot nil-dereference architecture extraction during build/startup/decode; the exact persisted `metadata` evidence path has race-enabled regression proof | P5.4 |
| F-057 | `DONE` | Structured review corrected stale CLI/GoDoc text, a digest-stale fixture ID, and full-corpus proof; the disproven package-example claim is explicitly dispositioned rather than silently accepted | P5.7, P5.8 |
| F-058 | `DONE` | The first exact P5 lint gate found an unused immutable model-reader wrapper and a 36-complexity definition assembler; the wrapper is deleted and identity, lineage, capabilities, and timestamps are cohesive tested helpers with zero lint issues | P5.2, P5.8, P8.5 |
| F-059 | `DONE` | The second exact P5 verification run reached coverage after all tests, race, vet, performance, and lint gates passed, then found obsolete thresholds for the deleted attribution packages; verification and maintained testing guidance now follow the surviving catalog-derivation seam without weakening any live package threshold | P5.8, P10.1 |
| F-060 | `DONE` | The final public-surface audit found generated OpenAPI still advertising the deleted `precision` field even though ordinary docs checks passed; the schemas are regenerated and `make docs-check` now reproducibly compares both embedded OpenAPI files with current Go types | P5.8, P10.1, P10.5 |
| F-061 | `DONE` | PR #53’s first Verification Gate passed all product, race, performance, lint, and coverage checks but exposed that OpenAPI reproduction depended on a developer-only ambient `swag` binary; generation and checking now invoke the repository-pinned Swag module through Go on every environment, with a structural regression test | P5.8, P10.1 |
| F-062 | `DONE` | Post-merge review questioned whether P5 deleted the embedded catalog; exact tree comparison proves all 611 canonical provider-model YAML files are unchanged while only the 322 duplicate author-model files were removed, and bootstrap now asserts every embedded provider YAML becomes exactly one published offering | P5.8, P6.1 |
| F-063 | `PENDING` | Inverting only `pkg/sources` → provider clients leaves a 244-package root closure because root still compiles pipeline, models.dev, remote HTTP, and `net/http`; P6.2/P6.3 require one complete opt-in acquisition boundary after the reopened catalog identity work | P6.2–P6.3 |
| F-064 | `PENDING` | Existing external journey goldens are parsed but not type-checked and the sketched server/remote import paths do not exist; real `GOWORK=off` consumer modules must replace this false compile evidence | P6.6, P7.1, P7.9 |
| F-065 | `PENDING` | `(*Client).Catalog()` panics on a nil receiver while neighboring accessors define nil behavior, and storage-backed construction roots work in `context.Background`; construction needs documented nil semantics and a caller-owned cancellation path | P6.8 |
| F-066 | `PENDING` | The 10-method `internal/application.Application` interface forces unrelated consumers and test doubles to implement dead build metadata and operational capabilities; consumer-local roles must replace the omnibus interface | P6.4 |
| F-067 | `DONE` | Provider identity is not author identity: every retained provider record now carries an explicit validated `model: author/slug` link; authored records alone build canonical definitions, provider records build exact offerings, and reconciliation preserves the link through an executable authority rule rather than inferring authorship from the serving provider | P5.9–P5.12 |
| F-068 | `DONE` | PR #53 removed the only persisted author/model corpus; all 322 records are restored and normalized as authored/history records, including 197 exact provider-ID matches and four provider-artifact IDs explicitly linked to canonical slugs, with exact-map corpus invariants under ordinary and race tests | P5.9 |
| F-069 | `PENDING` | `endpoints.yaml` is empty, the generic Endpoint type is not an OpenRouter endpoint row, and no generator or compatibility route exists; the endpoint projection must join authored models to exact provider serving records | P5.11, P7.12 |
| F-070 | `DONE` | The pre-#53 `Author.Models` copy semantics remain deleted: dedicated authored construction records own intrinsic facts, explicitly linked provider records own serving facts, and generated endpoints join both identities without a nested mutable author-model mirror | P5.9–P5.11 |
| F-071 | `DONE` | The first P5.13 structured review found provider-as-author mistakes for OpenAI, Canopy Labs, Meta, and Qwen plus Vertex deployment suffixes promoted into canonical identity; records now preserve exact opaque provider IDs while linking to independently reviewed lab-owned definitions | P5.10–P5.13 |
| F-072 | `DONE` | The first P5.13 structured review found Groq prices understated by 10^6, provider float artifacts, and request overrides encoded as YAML byte arrays; provider prices are corrected and tolerance-normalized without changing units, native YAML values round-trip, and generated endpoint regressions cover both | P5.11, P5.13 |
| F-073 | `DONE` | The first P5.13 structured review found Builder alias storage asymmetry, author deletion orphaning authored records, and an empty concurrency-test branch; canonical author storage, typed delete conflict, mutation isolation, and race-exercised authored reads now enforce the construction boundary | P5.10–P5.13 |
| F-074 | `PENDING` | Restored historical source observations contain candidate capability/modality, range/default, release/knowledge-date, and timestamp-semantic contradictions that do not break the validated identity/endpoint join but require authoritative fact review rather than speculative bulk edits | P10.9 |

## Workspace Ledger

| Workspace/ref | Status | Required terminal state |
| --- | --- | --- |
| Primary `/Users/jack/src/github.com/agentstation/starmap` on divergent `main@3787d716` | `PENDING` | Preserve the unique checkpoint under an explicit archive ref, then leave clean `main == origin/main`; no lost unrecorded work |
| `architecture-review-20260727-clean` detached at `9508ee78` | `PENDING` | Removed after report archival |
| `fresh-catalog-release` on `codex/immutable-release-pipeline@6d4d4c27` | `PENDING` | Verify #39 contains work; remove worktree and obsolete local branch |
| `provider-expansion-wave0` on `a14d2249` | `DONE` | Inventory committed; #40 closed; worktree removed before local branch; local and remote branches absent |
| `starmap-architecture-control-plane` on fresh branch | `DONE` | PR #45 merged; remote/local branch and worktree removed |
| `starmap-pr-reconciliation` on `codex/starmap-pr-reconciliation@650f5406` | `DONE` | PR #47 merged; remote branch removed; local branch/worktree removed after the successor P1 workspace was created |
| `provider-donor-inventory` on `codex/provider-donor-inventory@b7afa2df` | `DONE` | PR #48 merged; remote branch removed; local branch/worktree removed after the successor P2 workspace was created |
| `catalog-contract-characterization` on `codex/catalog-contract-characterization@39b08d6d` | `DONE` | PR #49 merged as `f8973be3`; remote/local branch and worktree removed after the successor hotfix workspace was created |
| `catalog-publication-hotfix` on `codex/catalog-publication-hotfix@4f7756e0` | `DONE` | PR #50 merged as `1dc811b5`; remote branch absent; zero-diff squash evidence recorded; clean worktree removed before the local topic branch |
| `catalog-authority-resilience` on `codex/catalog-authority-resilience@7454e3b8` | `DONE` | PR #51 merged as `60f0cd3c`; remote branch absent; zero-diff squash evidence recorded; clean worktree removed before the local topic branch |
| `catalog-workspace-lifecycle` on `codex/catalog-workspace-lifecycle@42295e4e` | `DONE` | PR #52 merged as `9609f4f4`; remote branch absent; zero-diff squash evidence recorded; clean worktree removed before the local topic branch |
| `catalog-read-model-simplification` on `codex/catalog-read-model-simplification@94157b42` | `DONE` | PR #53 merged as `76dd3178`; remote branch absent; zero-diff squash evidence recorded; clean worktree removed before the local topic branch |
| `catalog-author-endpoint-restoration` on `codex/catalog-author-endpoint-restoration@85e79cb9` | `IN_PROGRESS` | Complete P5.9–P5.13 on the user-steered successor branch, pass exact local/hosted gates, then remove the worktree/branch; P6.1 evidence remains committed and P6.2 resumes on a later fresh branch |

## Evidence Log

Append evidence; do not rewrite historical entries.

| Date | Task | Evidence |
| --- | --- | --- |
| 2026-07-27 | P0.1 | Protected main is `9508ee7866e4683e001e7ad153319d348433045d`. Open PRs are #40, #43, and #44. Local primary main is `1` ahead/`11` behind origin and not an ancestor of protected main. Four worktrees exist. |
| 2026-07-27 | P0.1 | PR #40 is draft, mergeable, 576 files, `+45,142/-10,593`, exact head `a14d22497479cb944932274088cf806cb25e993b`, and explicitly persists definitions/offerings as schema v2. |
| 2026-07-27 | P0.1 | PR #43 exact head `ebd36505216af8ba4def089dfbba2addb65849f3` changes only `go.mod`/`go.sum`, updates x/text from v0.38.0 to v0.40.0, and has green Verification Gate and Security & Reliability checks. |
| 2026-07-27 | P0.1 | PR #44 exact head `e1dcd1e6168dc3777e5780c949fa15e09af4b0c2` changes three workflow files. Both required checks fail because `govulncheck` finds GO-2026-5970 in x/text v0.38.0, fixed in v0.39.0. |
| 2026-07-27 | P0.1 | Protected main has `pkg/reconciler/merger_test.go` at 2059 lines and three production files above 1000 lines: Google client 1206, OpenAI client 1183, reconciler merger 1134. |
| 2026-07-27 | P0.2 | Created `/Users/jack/src/github.com/agentstation/starmap-worktrees/starmap-architecture-control-plane` on `codex/starmap-architecture-control-plane` from exact protected main. |
| 2026-07-27 | P0.1 | All four pre-plan worktrees were clean. `fresh-catalog-release@6d4d4c27` has a tree identical to protected main and is a verified cleanup candidate. |
| 2026-07-27 | P0.1 | PR #40 retains the store-only empty-path save defect and invalid `WithAuthorities` option condition while expanding package directories from 89 to 109. Its overlapping acquisition/source/provider vocabularies require concept-by-concept salvage rather than branch reuse. |
| 2026-07-27 | P0.4 | Archived and parsed `docs/reviews/STARMAP_ARCHITECTURE_REVIEW_2026-07-27.html`; SHA-256 `de08e0b3a8e3a22463968f326c4e7659a8f69c04dea166b15ace76e62b0d9235`. |
| 2026-07-27 | P0.5 | Fable independently returned `GO WITH REQUIRED REVISIONS`. B-01 through B-13 and the canonical Go findings were dispositioned in `docs/reviews/FABLE_STARMAP_PLAN_REVIEW_DISPOSITION_2026-07-27.md` (SHA-256 `df13364d205ca48848841f6aed20888ea0e8baf81e148ea1da73e2bc3406ae86`); accepted changes added the green-characterization rule, legacy-layout protection, one durable commit point, corrected phase order, dependency budget mechanism, event/shutdown contracts, historical supersession map, and explicit approval pauses. |
| 2026-07-27 | P0.5 | Archived the full Fable review as `docs/reviews/FABLE_STARMAP_PLAN_REVIEW_2026-07-27.md` (SHA-256 `b2f78de7be15762f9d0425f99a698dc3d63397b3c2e07e553eb16ed51a3495b2`) so its file/line defect evidence is durable. Follow-up review pass pinned exact defect sites into P2.3/P2.4/P4.8, made the P6.2 dependency budget CI-enforced, added `pkg/differ/differ.go` and dead `Changeset.Filter`/`ApplyAdditive` behavior to the P8.3/P8.6 scope, and added `format.Formatter` to the P8.4 stutter audit. |
| 2026-07-27 | P0.5 | User steering made SSE heartbeat behavior load-bearing: comment-line heartbeats establish stream liveness across intermediary idle timeouts, heartbeat absence drives degraded/reconnect/catch-up behavior, liveness remains distinct from catalog freshness, and delivery failure must coalesce or disconnect rather than silently lose a generation on a healthy stream. Recorded as F-038 and strengthened P7.2–P7.5/P7.7–P7.8/P7.10–P7.11. |
| 2026-07-27 | P0.5 | PR #45 exact head `b1548a20359d2611f5e1e35ec435f36aba1c2168` failed [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30317369158/job/90145745252): current `govulncheck v1.6.0` found reachable GO-2026-5970 in `golang.org/x/text v0.38.0` (fixed in v0.39.0) and GO-2026-6061 in `google.golang.org/grpc v1.82.0` (fixed in v1.82.1). Its [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30317369158/job/90145745302) remained in progress at the final blocker readback. |
| 2026-07-27 | P0.5 / P1.1 | A detached audit of #43 exact head `ebd36505216af8ba4def089dfbba2addb65849f3` changed only `google.golang.org/grpc v1.82.0` to `v1.82.1` plus its two `go.sum` records. Go 1.26.5 `go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...` reported zero reachable vulnerabilities, and exact `make verify` passed ordinary tests, `-race -short`, vet, the 0-allocation catalog benchmark (8.060–8.181 ns/op), lint, coverage gates, docs, diff, build, catalog validation, and isolated credential-free CLI checks. The temporary worktree/ref were removed. |
| 2026-07-27 | P0.5 | After three consecutive goal turns, no user authorization, regenerated Dependabot head, replacement security PR, or passing #45 security check existed. Rule 18 prevents silently reordering the protected merge sequence; P0.5 and F-039 are therefore recorded `BLOCKED` pending the user's authorization to recreate/revalidate/approve #43 before rebasing #45. |
| 2026-07-27 | P0.5 / P1.1 | User explicitly authorized fixing rather than retaining the Rule 18 pause. Requested `@dependabot recreate` on PR #43 in [comment 5098519498](https://github.com/agentstation/starmap/pull/43#issuecomment-5098519498); P0.5 and F-039 resumed `IN_PROGRESS`. |
| 2026-07-27 | P0.5 / P1.1 | Dependabot recreated #43 as `5f5e54dd12a983d4dbbcd393e55e414ba1e93526`, updating the direct group but retaining vulnerable `grpc v1.82.0`; its new Security & Reliability run failed. Created replacement PR [#46](https://github.com/agentstation/starmap/pull/46) at exact head `2fbd4c6dea333ec575287a28e76233e8e148d224`, adding only the required grpc v1.82.1 module and checksum change on top of that regenerated head, then closed #43 as superseded. |
| 2026-07-27 | P0.5 / P1.1 | On #46 exact head `2fbd4c6dea333ec575287a28e76233e8e148d224`, Go 1.26.5 current `govulncheck v1.6.0` reported zero reachable vulnerabilities and `make verify` passed tests, `-race -short`, vet, catalog benchmark (9.251–9.422 ns/op, 0 B/op, 0 allocs/op), lint, coverage gates, docs, diff, build, catalog validation, and isolated CLI checks. Hosted [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30318460306/job/90149107028) and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30318460306/job/90149106992) started on that exact head. |
| 2026-07-27 | P0.5 / P1.1–P1.2 | PR #46 exact head `2fbd4c6dea333ec575287a28e76233e8e148d224` passed [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30318460306/job/90149107028) and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30318460306/job/90149106992). Protection readback required both exact contexts with strict checking, admin enforcement, conversation resolution, zero required approvals, and no force-push/deletion; the PR had zero review threads and merged to protected main as `53285f13ac9b97e7fa06d40ba2839507a2368e16`. The remote replacement branch was deleted. |
| 2026-07-27 | P0.5 | Rebased the six control-plane commits onto secure protected main `53285f13ac9b97e7fa06d40ba2839507a2368e16` without conflict. On rebased head `ad682e4cd49f99baeac045869d7f70ff20891e92`, Go 1.26.5 `govulncheck v1.6.0` reported zero reachable vulnerabilities and `make verify` passed ordinary tests, `-race -short`, vet, the catalog benchmark (10.47–11.09 ns/op, 0 B/op, 0 allocs/op), lint, every coverage gate, docs, diff, build, catalog validation, and isolated CLI checks. |
| 2026-07-27 | P0.5 | PR #45 final head `662d57143e7cadb5af2ba741b3980ab68fc905ad` passed Go 1.26.5 `govulncheck v1.6.0` with zero reachable vulnerabilities and exact `make verify`, including race, lint, coverage, docs, catalog/CLI, and catalog performance (10.68–11.43 ns/op, 0 B/op, 0 allocs/op). The same head passed hosted [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30319727717/job/90152880876) and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30319727717/job/90152880904); protection remained strict with both contexts, admin enforcement, conversation resolution, zero required approvals, and no force-push/deletion. The PR had zero review threads and merged as `2561456eee236faa739669d0739aa2a7c75a8272`. |
| 2026-07-27 | P0.5 / P1.3 | Created fresh P1 worktree `/Users/jack/src/github.com/agentstation/starmap-worktrees/starmap-pr-reconciliation` on `codex/starmap-pr-reconciliation` from exact protected main `2561456eee236faa739669d0739aa2a7c75a8272`. Removed the clean merged control-plane worktree, its local branch, and the local replacement-#46 branch after confirming both remote topic branches were absent. |
| 2026-07-27 | P1.3 | Requested Dependabot rebase in [comment 5098836095](https://github.com/agentstation/starmap/pull/44#issuecomment-5098836095), producing exact head `1edb71728e5ffba48d389defc4b431fc376e4099` on protected main `2561456eee236faa739669d0739aa2a7c75a8272`. The diff remained exactly three workflow files and 12 pin replacements; `actionlint` and `govulncheck v1.6.0` passed, but `go test -race ./internal/ciworkflow` and therefore `make verify` failed because the action allowlist plus PR/release exact-pin fixtures still required checkout 7.0.0, setup-go 6.5.0, and docker/login-action 4.4.0. Recorded as F-040; the P1 replacement carries the equivalent workflow changes and updates those exact structural expectations. |
| 2026-07-27 | P1.3 | On replacement head `2092ec8a0414374fe3abd29814446de0986c8ea1`, `actionlint`, `go test -race ./internal/ciworkflow`, Go 1.26.5 current `govulncheck v1.6.0`, and `make verify` passed. Full verification included ordinary tests, `-race -short`, vet, catalog benchmark (10.62–11.30 ns/op, 0 B/op, 0 allocs/op), lint, all coverage gates, docs, diff, build, catalog validation, and isolated CLI checks. Opened replacement PR [#47](https://github.com/agentstation/starmap/pull/47), then closed #44 with an explicit supersession explanation and deleted its remote Dependabot branch; its temporary exact-head worktree/ref were removed cleanly. |
| 2026-07-27 | P1.3–P1.4 | PR #47 final exact head `650f5406728e86e97c11ce92368c49e1bb4ad5fd` passed `actionlint`, current `govulncheck v1.6.0` with zero reachable vulnerabilities, and exact `make verify`; the catalog accessor benchmark measured 7.942–8.555 ns/op, 0 B/op, and 0 allocs/op. The same head passed hosted [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30321210906/job/90157323234) and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30321210906/job/90157323226). Protection required both exact contexts with strict checking, admin enforcement, conversation resolution, zero required approvals, and no force-push/deletion; the PR had zero review threads and merged as `a87b64252f022c398589c5aad8652357ba8a174a`. Its remote branch was deleted. |
| 2026-07-27 | P1.5 | Created fresh worktree `/Users/jack/src/github.com/agentstation/starmap-worktrees/provider-donor-inventory` on `codex/provider-donor-inventory` from exact protected main `a87b64252f022c398589c5aad8652357ba8a174a` for the read-only PR #40 inventory and cleanup. |
| 2026-07-27 | P1.5 | Compared PR #40 exact donor head `a14d22497479cb944932274088cf806cb25e993b` with merge base `9508ee7866e4683e001e7ad153319d348433045d` and current protected main `a87b64252f022c398589c5aad8652357ba8a174a`. [`reviews/PR40_DONOR_INVENTORY_2026-07-27.md`](reviews/PR40_DONOR_INVENTORY_2026-07-27.md) classifies all 66 changed production Go modules exactly once: 38 limited `SALVAGE`, 20 `SUPERSEDED`, and 8 `REJECT`; its non-Go inventory separately records newer workflow pins as already landed and bounds provider research, fixtures, credential-free verification, and cache ownership as evidence-only salvage. A generated module list and the table both contained 66 rows with an empty `comm -3` result; no donor code was copied. |
| 2026-07-27 | P1.6–P1.7 | PR #40 had zero review threads. Added [closing comment 5099100581](https://github.com/agentstation/starmap/pull/40#issuecomment-5099100581) linking the current plan and immutable inventory commit `3fff4465e1db94e95975c8ead57377dbcc3a2c55`, then closed the draft. Verified its donor worktree was clean at exact head `a14d22497479cb944932274088cf806cb25e993b`; removed the worktree before deleting the local branch, deleted the remote branch, verified local/remote refs and the worktree path were absent, and pruned the worktree registry. |
| 2026-07-27 | P1.8 | After #40 cleanup, GitHub reported zero open Starmap PRs. The P1 ledger branch is a descendant of and has no code changes from current protected main `a87b64252f022c398589c5aad8652357ba8a174a`; only this control plane and the donor inventory differ. |
| 2026-07-27 | P1.8 / F-041 | Exact head `c330b131ba5cf67d30b81c0beac5f8db39293779` passed `actionlint`, `go test -race ./internal/ciworkflow`, current `govulncheck v1.6.0` with zero reachable vulnerabilities, and full `make verify`: ordinary tests, repository race-short, vet, catalog benchmark (10.65–11.92 ns/op, 0 B/op, 0 allocs/op), lint, every coverage floor, docs, diff, build, catalog validation, and CLI smokes. The nominally “isolated credential-free provider listing” nevertheless reported Google Vertex `Configured` from ambient developer ADC; this pre-existing non-hermetic verification behavior is recorded as F-041 for P10.1/P10.6 rather than treated as a P1 regression. |
| 2026-07-27 | P1.8 | PR #48 final exact head `b7afa2df93359db7dd420c83fccbb0085a6154d7` passed `actionlint`, `go test -race ./internal/ciworkflow`, current `govulncheck v1.6.0` with zero reachable vulnerabilities, and exact `make verify`; the catalog benchmark measured 8.473–10.43 ns/op, 0 B/op, and 0 allocs/op. The same head passed hosted [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30322875677/job/90162203582) and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30322875677/job/90162203514). Protection required both exact contexts with strict checking, admin enforcement, conversation resolution, zero required approvals, and no force-push/deletion; the PR had zero review threads and merged as `08f51ca9d1d1c924a6637dc70d7f5b89944ed98a`. Its remote branch was deleted and GitHub again reported zero open Starmap PRs. |
| 2026-07-27 | P2.1 | Created fresh worktree `/Users/jack/src/github.com/agentstation/starmap-worktrees/catalog-contract-characterization` on `codex/catalog-contract-characterization` from exact protected main `08f51ca9d1d1c924a6637dc70d7f5b89944ed98a`. Removed the clean merged P1 worktree and its local branch after confirming the remote branch was absent. |
| 2026-07-27 | P2.1 | Added the normative canonical-domain table and decision consequences to this control plane. It gives one positive meaning and explicit non-meanings for provider model, catalog, builder, workspace, source, observation, provenance, definition, offering, candidate, generation, store, YAML projection, publication, manifest, event, remote subscriber, stream liveness, and catalog freshness. `rg` found every P2.1 required term in the table; `make docs-check` and `git diff --check` passed. |
| 2026-07-27 | P2.2 | Added four green tests that name F-001/F-002 and pin the current defects at real call sites without changing production behavior: `TestF002CharacterizationStoreOnlyApplyFailsBeforeGenerationCommit` proves YAML failure precedes generation-store commit and in-memory publication; `TestF001CharacterizationPipelineLoadsLocalFromOutputPath` proves sync reuses the projection path as local input; `TestF001CharacterizationEmbeddedBuilderCarriesRepositoryWritePath` proves embedded construction carries `internal/embedded/catalog` as a write destination; and `TestF001CharacterizationNewPrefersDurableCurrentOverValidLocalWorkspace` proves restart ignores a valid human workspace when durable current exists. `go test -race . ./internal/catalog/pipeline ./pkg/catalogs -run 'F00[12]Characterization' -count=1` passed, followed by the complete affected-package command `go test -race . ./internal/catalog/pipeline ./pkg/catalogs -count=1` (`119.597s`, `1.898s`, and `44.872s` respectively); `git diff --check` passed. The fixing phases must invert these assertions while retaining failure atomicity. |
| 2026-07-27 | P2.3 | Added five green characterization tests without production changes. `TestF004CharacterizationEnrichMergeDropsManualModelWithoutPricingOrLimits` pins the `MergeEnrichEmpty` metadata-only model drop; `TestF008CharacterizationMergeModelsClearsFalseButKeepsOtherZeroValues` pins false clearing while empty string and numeric zero cannot clear; `TestF005CharacterizationPrimaryOmissionPrunesBaselineModel` pins primary omission plus wholesale provider replacement deleting a baseline model; `TestF005CharacterizationDegradedObservationStillPrunesBaselineModel` proves partial/degraded status, rejection count, and issue metadata do not prevent the same deletion; and `TestF007CharacterizationPersistedProvenanceCollidesAcrossProviders` proves two providers' shared model ID persists under one `model:shared:Name` tracker key and `GenerateReport` selects current by reconciliation timestamp. The focused `go test -race ./pkg/catalogs ./pkg/reconciler -run 'F00[4578]Characterization' -count=1` passed, then the full affected-package race suites passed (`44.414s` and `1.430s`); the later fixing phases must invert the loss expectations and make provenance provider-scoped durably. |
| 2026-07-27 | P2.4 | Added six green F-009 characterization tests without production changes. A valid models.dev sibling is unavailable when another model has a drifted integer; generic `DecodeResponse` returns an error even after parsing a valid sibling; Google returns zero models when a malformed second page follows a valid first page; a valid local model becomes unavailable when a later YAML file is malformed; a present invalid workspace returns typed `*errors.ParseError` rather than optional absence; and one malformed stored model rejects the whole generation payload with typed parse failure. The focused `go test -race ./internal/sources/modelsdev ./internal/transport ./internal/providers/google ./pkg/catalogs ./pkg/catalogstore -run 'F009Characterization' -count=1` passed. Full affected-package race suites then passed (`35.686s`, `1.547s`, `1.309s`, `47.245s`, and `2.534s`). The bounded `json.RawMessage` loop in `cmd/starmap/cmd/providers/fetch.go:460-478`, which skips one invalid record while retaining siblings, is the existing reference pattern; P4.8 must generalize that behavior while keeping envelope, resource-budget, and configured-workspace structural failures fail-closed. |
| 2026-07-27 | P2.5 | Preserved the existing real manifest/payload proof (`TestRemoteCatalogClientAndServerShareVersionedManifestSnapshotContract` and `TestRemoteCatalogFetchValidatesManifestSnapshotChecksumAndCompatibility`) and added/renamed green finding-labelled characterizations. A real SSE connection proves `Last-Event-ID` does not change the connection-only handshake and two publications in one second both receive ID `1800000000`; slow SSE delivery is skipped while the client stays connected, whereas slow WebSocket delivery disconnects; publication callbacks complete before model-diff callbacks within one generation, but a blocked generation 1 is overtaken by generation 2; the 17th concurrent hook dispatch is silently dropped after the fixed 16-slot limit; and the remote client performs only manifest plus payload GETs with no event-stream or reconnect lifecycle. Focused `go test -race . ./internal/server/events/adapters ./internal/server/sse ./internal/server/websocket ./pkg/catalogremote -run 'F0(17|18|19|34|36)Characterization|F017F034Characterization' -count=1` passed. Full root/server/adapter/SSE/WebSocket/remote race suites passed (`115.703s`, `60.575s`, `1.382s`, `2.148s`, `3.901s`, and `2.033s`), including the existing post-commit HTTP+SSE+WebSocket correspondence test. P7 must retain verified immutable fetch and post-commit atomicity while replacing these notification/lifecycle defects. |
| 2026-07-27 | P2.6 | Measured code tree `fa088d97d639d716f0593bbdff140d0def255718` with Go 1.26.5 darwin/arm64 and recorded the reproducible baseline in [`reviews/P2_COMPOSITION_BASELINE_2026-07-27.md`](reviews/P2_COMPOSITION_BASELINE_2026-07-27.md). Root/catalog/server/Google compile closures are 472/145/488/448 packages; the Google closure is a strict 448-package subset and 94.9% of root. Root attribution is 214 standard, 33 local, and 225 external packages; the current regression ceiling is 472 plus the P6.2 banned-implementation gate. The CLI is 37,552,946 bytes with `-trimpath` and 27,687,346 bytes stripped. Five `BenchmarkClientCatalog` runs measured 9.159–10.75 ns/op, 0 B/op, and 0 allocs/op. The repository has 89 Go packages, 466 Go files, 86,051 Go lines (47,900 non-test; 38,151 test), and 2,514,088 embedded catalog bytes across 966 files. The only >1000-line files are `pkg/reconciler/merger_test.go` 2,059; Google client 1,206; OpenAI client 1,183; and reconciler merger 1,134. |
| 2026-07-27 | P2.6 | Follow-up closure measurement found the intended local core union, `pkg/catalogs` plus `pkg/catalogstore`, is 149 packages. The P6.2 read-only root acceptance budget is therefore frozen at no more than 160 packages, leaving at most 11 for the root/bootstrap façade; 472 remains only the pre-refactor regression ceiling. Meeting 160 does not waive the explicit GenAI/gRPC/OpenTelemetry/WebSocket/SQLite/Cobra/scheduler/server exclusion. The opt-in verified HTTP remote package currently has a separate 225-package closure and cannot be pulled into every local consumer merely to meet the remote journey. |
| 2026-07-27 | P2.7 | Added [`reviews/P2_USER_JOURNEYS_2026-07-27.md`](reviews/P2_USER_JOURNEYS_2026-07-27.md) and five golden fixtures under `testdata/journeys`. The Go fixtures freeze the canonical root library DX, the public `github.com/agentstation/starmap/server` composition with caller-owned `Serve(ctx, listener)`, and the opt-in `github.com/agentstation/starmap/remote` lifecycle whose constructor is inert and whose `Start` performs verified initial fetch before reactive service. Machine-readable CLI-workspace and embedded E1→E2 fixtures freeze atomic seed, semantic edits, provider-price authority, explicit update, no install-time rewrite, restart, rollback, and the absence of persisted definitions/offerings/overrides. `TestP2UserJourneyGoldenFixtures` parses every fixture, rejects internal/CLI imports, and validates the required contract fields; later phases must promote the same artifacts into external compile and runtime suites. |
| 2026-07-27 | P2.8 | On code tree `892589f790f4a7b3b9c88d913924486017854fed`, production-import queries found zero importer of `pkg/catalogdistribution` and zero caller of `NewRunner`, `NewInitialRunController`, the lease/ledger/freshness constructors, or their `Operations` wiring options. [`reviews/P2_PRODUCTION_COMPOSITION_DECISIONS_2026-07-27.md`](reviews/P2_PRODUCTION_COMPOSITION_DECISIONS_2026-07-27.md) therefore retains one server manifest/immutable-payload/SSE protocol consumed by public `remote`, retains `catalogartifact` for independently versioned GitHub Release and offline artifacts, and directs deletion of the 767-production-line hosted protocol, the 2,314-production-line scheduler subsystem, and WebSocket. Cadence is owned by the embedding deployment over explicit `Sync`; a future `starmap.agentstation.ai` deployment uses the same server contract rather than a competing protocol. `make docs-check`, `git diff --check`, and shell assertions for the zero production callers passed. |
| 2026-07-27 | P2.9 | Exact code-and-decision head `64177404c453a9e695be3e43a9d35d0f8108aa3b` passed the complete P2 affected-package `go test -race` command across root, pipeline, catalogs, reconciliation, source decoders, store, server, both characterized transports, and remote client. Current Go 1.26.5 `govulncheck v1.6.0` reported zero reachable and zero imported-package vulnerabilities. Exact `make verify` passed ordinary tests, repository-wide `-race -short`, vet, lint with zero issues, all coverage floors, docs, diff, build, 933-model catalog validation, and isolated CLI checks; `BenchmarkClientCatalog` measured 10.57–11.00 ns/op, 0 B/op, and 0 allocs/op. Opened protected phase PR [#49](https://github.com/agentstation/starmap/pull/49); its final ledger head still requires the same exact local and hosted proof. |
| 2026-07-27 | P2.9 / P3.6a | PR #49 final exact head `39b08d6d898ce69de7c36e3d13abfb468137e43d` passed the complete affected-package race command, current `govulncheck v1.6.0` with zero reachable/imported-package vulnerabilities, and exact `make verify`; its catalog benchmark measured 11.32–14.61 ns/op, 0 B/op, and 0 allocs/op. The same head passed hosted [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30325378975/job/90169625012) and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30325378975/job/90169625097). Protection required both exact contexts with strict checking, admin enforcement, conversation resolution, zero approvals, and no force-push/deletion; the PR had zero review threads and merged as `f8973be3a6f25960efb786b7620a8c7975cfbf1d`. Its remote/local branch and worktree were removed after creating fresh `/Users/jack/src/github.com/agentstation/starmap-worktrees/catalog-publication-hotfix` on `codex/catalog-publication-hotfix` at that protected-main SHA. |
| 2026-07-27 | P3.6a / P3.8 | Inverted the green F-002 characterization into the desired contract. `Client.save` now builds and validates the immutable candidate, commits it through generation-store CAS, atomically activates it in memory, and only then attempts an optional YAML projection. A failed projection cannot veto or corrupt the committed generation and is returned as `ProjectionStatusPendingRepair` with stable issue code `workspace_projection_failed`; a successful projection is `applied`; a store-only apply has a nil projection and performs no save. `TestProjectionFailureLeavesCommittedGenerationActiveAndReportsRepair` proves the store/current catalog survive a blocking filesystem path, and `TestStoreOnlyApplyCommitsWithoutWorkspaceAccess` proves no-path success. Focused and complete affected-package `go test -race . ./internal/catalog/pipeline ./pkg/sync -count=1` passed (`125.074s`, `1.744s`, `2.026s`); `git diff --check` passed. |
| 2026-07-27 | P3.6b | Added the deep `internal/catalog/workspace` projection module. It stages beside the configured workspace on the same filesystem, copies unmanaged files, serializes only the current human-YAML representation, validates the full committed payload identity, verifies provider/author/provenance entity coverage, proves the separate projected semantic digest is stable over a second save/load cycle, fsyncs regular files and directories, rechecks the input digest, and publishes by Darwin `RENAME_SWAP` or Linux `RENAME_EXCHANGE`. The sibling marker binds generation ID, payload checksum, and workspace checksum; startup repairs a missing/stale unchanged projection or a post-swap marker failure without republishing, while malformed, symlinked, concurrently changed, or semantically dirty workspaces are not overwritten. Root `Client.Save` now uses the same atomic path and update coordinator; sync exposes applied/pending status plus generation/workspace identity; store-only behavior remains nil-projection. |
| 2026-07-27 | P3.6b / F-042 | Digest validation found a real pre-existing serializer defect in `Qwen/Qwen3-TTS`: goccy/go-yaml's comment-aware literal block doubled `\\` on every cycle. A narrow single-quoted encoding path plus `TestModelEncodeYAMLRoundTripsBackslashesWithoutSemanticDrift` makes repeated encode/decode stable. `TestSaveReturnsNilAfterSuccessfulCatalogSave` now loads the complete 933-model embedded projection and proves its semantic checksum equals the active committed catalog. |
| 2026-07-27 | P3.6b | Failure and lifecycle tests cover validation mismatch, pre-promotion failure, concurrent semantic edit conflict, direct symlink rejection, unmanaged-file preservation, old/new replacement, separate generation/workspace digests for non-persisted views, stale repair, dirty-workspace refusal, post-swap marker repair, successful observable identity, post-commit projection failure, store-only mutation, and startup repair without generation or sequence publication. `go test -race . ./pkg/catalogs ./internal/catalog/workspace ./internal/catalog/pipeline ./pkg/sync -count=1` passed in `158.466s`, `47.027s`, `2.533s`, `1.349s`, and `2.128s`; compile-only Linux/amd64 and Windows/amd64 workspace builds produced the expected ELF and PE binaries; workspace statement coverage is 66.6%; current `govulncheck v1.6.0` found zero reachable or imported-package vulnerabilities; `go vet ./...`, generated GoDoc, `make docs-check`, and `git diff --check` passed. |
| 2026-07-27 | P3.6b / F-043 | The first full `make verify` attempt exposed a 10-second full-catalog projection timeout under race load; the catalog-specific budget is now one minute. The separate lint mismatch was fixed rather than treated as an external blocker: Devbox now uses exact Go 1.26.5 and golangci-lint v2.12.2, and Make, `scripts/verify.sh`, PR/release workflows, and structural tests pin the same linter. The required script no longer silently skips lint or accepts an arbitrary host version. The v2.12.2 upgrade exposed three real new-code deprecations, which were fixed, plus 50 mechanical `goconst` suggestions across pre-existing error/resource literals; `goconst` was deliberately disabled rather than degrading local readability with constants created only for a heuristic. `actionlint`, `bash -n scripts/verify.sh`, and `go test -race ./internal/ciworkflow -count=1` passed. Final exact-tree `make verify` passed ordinary tests, repository race-short (root `171.428s`), vet, pinned Devbox lint with zero issues, every coverage floor, current docs/diff, build, 933-model validation, and CLI smokes; `BenchmarkClientCatalog` measured `10.46–11.21 ns/op`, `0 B/op`, and `0 allocs/op`. |
| 2026-07-27 | P3.6b | Committed the implementation and evidence as `2bfe7010a153a4111c2706bde291f45f1b0d3c1e`, pushed `codex/catalog-publication-hotfix`, and opened draft protected phase PR [#50](https://github.com/agentstation/starmap/pull/50). The GitHub connector returned `403 Resource not accessible by integration`, so the authenticated `gh` fallback created the PR without changing repository permissions. This ledger follow-up becomes the final candidate head and must repeat exact local verification before hosted evidence can close P3.6b. |
| 2026-07-27 | P3.6b / P4.1 | PR #50 final exact head `4f7756e0f8e950715df68fedbbb4fbc96cc80018` passed exact `make verify`: ordinary tests, repository race-short (root `166.614s`), vet, pinned golangci-lint v2.12.2 with zero issues, every coverage floor, docs/diff, build, 933-model validation, and CLI smokes; `BenchmarkClientCatalog` measured `9.880–10.66 ns/op`, `0 B/op`, and `0 allocs/op`. The same head passed hosted [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30329654618/job/90181984296) in `12m40s` and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30329654618/job/90181984330) in `2m52s`. Protection required exactly both contexts with strict checking, admin enforcement, conversation resolution, zero required approvals, and no force-push/deletion; the PR was ready, mergeable/clean, and had zero review threads. It squash-merged as `1dc811b5ca9034c5c6ecf1a4b7a93786e7ecf08f`; GitHub reported zero open PRs and the remote branch absent. Created fresh `/Users/jack/src/github.com/agentstation/starmap-worktrees/catalog-authority-resilience` on `codex/catalog-authority-resilience` at that exact main SHA. The clean hotfix worktree was removed first; because squash ancestry makes ordinary `branch -d` insufficient, a zero `git diff origin/main codex/catalog-publication-hotfix` proved identical tree content before deleting the local branch and pruning worktrees. P3.6b, F-003, and F-031 are DONE; P4.1 is the sole active task. |
| 2026-07-28 | P4.1 | Replaced the numeric `Field` inventory and unused definition/offering `CanonicalPolicies` inventory with one immutable concrete `authority.Table`; the algorithm accepts its narrow `authority.Reader`. Every model/provider/author policy now owns source order, merge/empty semantics, evidence path, and rationale. Reconciliation iterates that table directly, and structured model/provider executors receive the same policy instead of owning priority slices. Deleted `pkg/reconciler/field_rules.go` and the 125-line `complexModelStructures` shadow; `pkg/reconciler/merger.go` fell from the 1,134-line baseline to 860 lines. A custom-reader integration test proves injected order controls limits and explicit-false features, so no hidden complex-field order survives. Updated the current authority policy and architecture docs; historical ledgers remain unchanged. Exact focused `go test -race ./pkg/authority ./pkg/reconciler -count=1` passed (`1.290s`, `1.545s`); the broader root/authority/reconciler/pipeline/sync race suite passed (`156.882s`, `1.285s`, `1.546s`, `2.350s`, `2.045s`) before the final provider structured-policy extraction, followed by the exact focused race rerun. Exact `go vet ./pkg/authority ./pkg/reconciler`, pinned golangci-lint v2.12.2 with zero issues, `make docs-check`, and `git diff --check` passed. P4.1 is DONE; P4.2 is the sole active task. |
| 2026-07-28 | P4.2 | The single model `Pricing` policy now selects one complete valid/effective price in provider-first order, keeps ordered rejection reasons on fallback, retains a last-known-good baseline rather than contaminating it with an invalid current candidate, and emits winner-less rejection evidence when every candidate fails. New tests prove atomic provider selection, invalid-provider fallback, two-invalid-candidate evidence, last-known-good retention, and encode/decode survival through the canonical catalog payload. `go test -race ./pkg/reconciler ./pkg/catalogstore -count=1` passed (`1.314s`, `1.713s`); `go vet` passed. The first focused lint invocation read obsolete entries for the already-removed P3 worktree from golangci-lint's machine cache; `golangci-lint cache clean` removed that non-repository state and the exact pinned v2.12.2 package lint rerun passed with zero issues. P4.2 is DONE; P4.3 is the sole active task. |
| 2026-07-28 | P4.3 / F-006 | Commit `6fe9083a` made discoverable model/provider facts dynamic-first and operator connection fields local-first in the one table; commit `de103fce` completed durable pricing rejection evidence. Dedicated integration tests now prove provider name/context limits beat stale YAML, models.dev description/input limits beat stale YAML, local output limits fill a dynamic gap, a human-only missing description survives, discovered provider identity wins while local API-key and catalog endpoint configuration remain authoritative, and a custom reader controls structured-field order. Exact `go test -race ./pkg/authority ./pkg/reconciler -count=1` passed (`1.269s`, `1.576s`); `go vet`, pinned golangci-lint v2.12.2 with zero issues, and `git diff --check` passed. P4.3 and F-006 are DONE; P4.4 is the sole active task. |
| 2026-07-28 | P4.4 / F-007 | Durable model provenance now uses an escaped provider/model resource identity, so opaque slashes, colons, and percent signs cannot collide with one another or the tracker key format. The catalog exposes provider-aware `FindModel` and `FindModelField` reads; sync indexing understands the same durable identity; CLI history infers a unique provider and requires `--provider` for ambiguous shared IDs instead of combining evidence. The inverted F-007 integration test proves independent name, price, per-dimension limit, and lifecycle/availability status evidence for two providers sharing one model ID, independent report resources, and catalog payload encode/decode survival. Exact `go test -race ./pkg/provenance ./pkg/catalogs ./pkg/reconciler ./pkg/sync ./cmd/starmap/cmd/models -count=1` passed (`1.212s`, `46.179s`, `1.312s`, `1.507s`, `1.735s`); focused `go vet`, pinned golangci-lint v2.12.2 with zero issues, generated GoDoc, `make docs-check`, and `git diff --check` passed. P4.4 and F-007 are DONE; P4.5 is the sole active task. |
| 2026-07-28 | P4.5 | Added one reconciliation-owned source-identity module rather than a second override representation. On local observation, it compares the parsed provider/model field with the projected provenance value using schema-tag-aware normalized YAML semantics. Equal values re-enter authority at their original source and retain the exact observation ID, revision, checksum, timestamp, and rejection evidence; mismatches become local claims. Actual current source observations replace unchanged projections, including generated provider catalog configuration that would otherwise incorrectly occupy a local-first slot. Provider fields now receive durable provenance too, excluding the separately tracked `Models` collection and all runtime secret values. A real filesystem test reconciles provider plus models.dev observations, saves provider YAML/provenance, reloads unchanged YAML, proves original source identities for provider config, name, description, composed features, pricing/limits/lifecycle evidence, proves a newer provider endpoint and limit displace unchanged generated values, directly edits provider and model YAML, and proves only those semantic edits become local. Bare bootstrap evidence is used only for model IDs unique across providers, so it cannot revive F-007. Exact final-tree `go test -race ./pkg/reconciler ./pkg/catalogs ./internal/sources/local ./internal/catalog/pipeline ./pkg/sync -count=1` passed (`2.357s`, `46.177s`, `2.055s`, `1.795s`, `1.487s`); focused `go vet`, pinned golangci-lint v2.12.2 with zero issues, `make docs-check`, and `git diff --check` passed. P4.5 is DONE; P4.6 is the sole active task. |
| 2026-07-28 | P4.6 / F-008 | Added compact typed presence without replacing ergonomic scalar fields or adding pointer-heavy consumer records. `ValueMissing`, `ValueUnknown`, and `ValueKnown` are retained for description, all 45 feature booleans, all three limits, and open-weights; two `uint64` feature bitsets and two `uint8` limit bitsets avoid per-model presence maps. Direct non-zero literals infer known values, while source authors use typed setters only for explicit `false`, `0`, `""`, `null`, or withdrawal. Human YAML and immutable JSON now preserve omission, `null`, and explicit zero values distinctly; missing feature keys remain omitted; deep copies, merge, baseline equality, authority selection, and change reporting retain the same semantics. OpenAI-compatible mappings/rules, Google rules, and models.dev raw-key decoders preserve upstream zero/null presence; provider pointer booleans retain explicit false. The green F-008 characterization was inverted, and tests prove YAML/JSON round trips, deep-copy isolation, compact capacity, explicit-empty local fallback, zero/unknown/missing limit authority, provider mappings, models.dev decode, and visible presence transitions. Extraction kept models.dev parsing at 895 lines and `pkg/differ/differ.go` at 978 lines. The exact final-tree affected race command `go test -race ./pkg/authority ./pkg/catalogs ./pkg/differ ./pkg/reconciler ./pkg/catalogstore ./internal/sources/local ./internal/sources/modelsdev ./internal/providers/openai ./internal/providers/google ./internal/providers/anthropic ./internal/catalog/pipeline ./pkg/sync ./cmd/starmap/cmd/models -count=1` passed (`1.288s`, `51.083s`, `1.266s`, `1.516s`, `2.037s`, `1.945s`, `49.344s`, `6.859s`, `2.940s`, `2.102s`, `2.690s`, `3.255s`, `2.830s`). Focused `go vet` passed; pinned golangci-lint v2.12.2 reported zero issues; generated GoDoc, `make docs-check`, and `git diff --check` passed. A final-gate `go-cmp` panic on new unexported limit state and a production-dead comparison helper were fixed in test composition rather than ignored or retained as unused code. P4.6 and F-008 are DONE; P4.7 is the sole active task. |
| 2026-07-28 | P4.7 / F-005 / F-033 | Reconciliation now seeds every provider/model identity from the immutable last-known-good baseline, so source absence never becomes lifecycle evidence. Complete omission and partial/degraded record rejection retain the exact model and provenance; fresh present fields still use normal authority. Baseline provenance is replaced per freshly observed field rather than globally cleared, preserving baseline-only evidence without duplicate append histories. Observation status, completeness, accepted/rejected counts, and stable issue codes are included in durable winner reasons. Stale-cache/bootstrap fallback can fill a missing fact but cannot regress a known model or provider fact. The pipeline converts a failed source with no catalog into a bounded partial/degraded empty observation, continues healthy siblings only in non-strict mode, always honors caller cancellation, and rejects a degraded `Fresh` attempt before empty-baseline reconciliation/publication. Source-attributed model-count regression adds a provider-scoped `volume_collapse` issue and partial/degraded identity; local/manual-only records are excluded from that comparison. The inverted F-005 tests prove complete and degraded omission retention, exact provenance, health evidence, and stale-fallback non-regression; pipeline tests prove non-strict failure continuation, required-all failure, fresh fail-closed behavior, and volume classification. |
| 2026-07-28 | P4.7 / F-044 | The first root integration gate exposed two P4.6 follow-through defects rather than an external blocker. Canonical presence JSON changed the embedded payload digest while the checked-in bootstrap manifest still named `bc35069f054f`; `starmap-bootstrap-manifest` regenerated it as `catalog-20260728T062837Z-463edcfd386c`, checksum `sha256:463edcfd386ce2d108b7b577a60dcb35f880d967673451f60cd690c7c584065f`, size `2241095`. Separately, model-level YAML marshaling through a generic `yaml.MapSlice` round trip collapsed a one-author slice into a mapping during full projection. Direct typed field assembly preserves collection shapes and explicit description presence; a dedicated sequence round-trip test and the real manual-sync projection pass. No failed candidate displaced the durable catalog: the original projection correctly remained `pending_repair`. |
| 2026-07-28 | P4.7 | Exact final-tree `go test -race . ./pkg/reconciler ./internal/catalog/pipeline ./pkg/sources ./pkg/catalogs ./pkg/catalogstore -count=1` passed (`246.951s`, `2.695s`, `1.661s`, `2.404s`, `53.206s`, `2.438s`). Source boundary coverage `go test -race ./internal/sources/providers ./internal/sources/modelsdev ./internal/sources/local ./pkg/sourceevidence -count=1` passed (`1.422s`, `49.881s`, `1.606s`, `2.167s`). Focused `go vet` passed; pinned golangci-lint v2.12.2 reported zero issues; generated GoDoc, `make docs-check`, and `git diff --check` passed. `pkg/reconciler/merger.go` is 925 lines and the isolated observation-health module is 137 lines. P4.7, F-005, and F-044 are DONE; F-033 remains open only for P4.9 strict-mode truthfulness; P4.8 is the sole active task. |
| 2026-07-28 | P4.8 / F-009 | Added one bounded `pkg/sourcepayload` record decoder and typed quarantine report rather than separate ad hoc loops or a new persisted representation. OpenAI-compatible, Anthropic, and Google AI Studio adapters now retain valid response siblings; Google also retains valid prior-page/current-page records, bounds the aggregate, and rejects repeating page tokens. models.dev decodes provider model maps deterministically under global provider/model/byte/nesting limits and preserves reviewable schema-drift evidence. Provider and models.dev observations carry accepted/rejected counts plus stable degraded issues instead of converting a partial response into complete success. |
| 2026-07-28 | P4.8 / F-009 | Local YAML model files now quarantine independently into a caller-owned `LoadReport`; structural provider/author/provenance YAML and filesystem failures remain fatal. The pipeline preserves that report when it prebuilds the local catalog, while embedded bootstrap, bootstrap-manifest generation, legacy store migration, and atomic workspace validation explicitly require an empty report. Stored schema-v1 payloads decode model records independently for diagnostics, but byte/nesting/count limits, required collection identity, unknown/duplicate provider-author-endpoint identity, malformed collection envelopes, and partial manifest-bound activation remain fail-closed. The report implementation stays internal to `catalogstore`, avoiding a new public payload API. |
| 2026-07-28 | P4.8 | Exact final-tree affected boundary gate `go test -race ./pkg/sourcepayload ./internal/transport ./internal/providers/openai ./internal/providers/anthropic ./internal/providers/google ./internal/sources/providers ./internal/sources/modelsdev ./internal/sources/local ./pkg/sources ./pkg/catalogs ./pkg/catalogstore ./internal/catalog/pipeline ./internal/catalog/workspace ./cmd/starmap-bootstrap-manifest -count=1` passed (`4.693s`, `4.818s`, `10.783s`, `5.001s`, `4.720s`, `5.072s`, `83.360s`, `11.730s`, `5.527s`, `67.814s`, `7.434s`, `6.373s`, `3.979s`, `13.317s`). The broader composition gate `go test -race . ./pkg/reconciler ./pkg/sync ./internal/catalog/pipeline ./pkg/sources ./pkg/catalogs ./pkg/catalogstore -count=1` passed (`279.325s`, `1.586s`, `2.366s`, `1.838s`, `2.132s`, `55.843s`, `3.147s`). Focused `go vet` passed; pinned golangci-lint v2.12.2 reported zero issues after extracting payload-envelope validation from catalog construction; generated GoDoc, `make docs-check`, and `git diff --check` passed. `internal/sources/modelsdev/parser.go` remains below review threshold at 967 lines; new record and payload modules are 141 and 271 lines. P4.8 and F-009 are DONE; P4.9 is the sole active task. |
| 2026-07-28 | P4.9 / F-010 / F-033 | `RequireAllSources` is now an explicit pre-reconciliation health gate rather than only a dependency/transport-error check. It verifies that the resolved configured source set and returned observation set match one-to-one, then requires every observation to be `Succeeded`, `Complete`, and contain at least one canonical model definition. Typed sync/validation errors identify the source and failed condition. Missing credentials, stale fallback, record quarantine, volume collapse, missing or duplicate observations, skipped optional dependencies, and a complete/succeeded but unexplained empty result all fail before reconciliation or publication; a healthy nonempty source proceeds. Non-strict synchronization retains its prior degraded-evidence and last-known-good behavior. README, CLI help, option GoDoc, and architecture policy now state the same contract. |
| 2026-07-28 | P4.9 | Exact focused `go test -race ./internal/catalog/pipeline ./pkg/sync ./cmd/starmap/cmd/update -count=1` passed (`1.896s`, `1.554s`, `1.306s`). The exact broader final-tree gate `go test -race . ./internal/catalog/pipeline ./pkg/sync ./cmd/starmap/cmd/update ./internal/sources/providers ./internal/sources/modelsdev ./internal/sources/local -count=1` passed (`323.174s`, `1.747s`, `3.202s`, `2.631s`, `2.464s`, `84.082s`, `8.613s`). Focused `go vet` passed; pinned golangci-lint v2.12.2 reported zero issues after replacing the legacy bare-ID model reader with the canonical immutable definitions view; generated GoDoc, `make docs-check`, and `git diff --check` passed. P4.9, F-010, and F-033 are DONE; P4.10 is the sole active task. |
| 2026-07-28 | P4.10 | Added deterministic policy and presence properties rather than duplicate fixtures. Every model/provider/author policy now proves unique strictly descending authority ranks, deterministic selection from every available rank, its empty-value contract, and stable provenance-path selection. Every one of the 45 model features proves missing, unknown, explicit false, and explicit true through the original value, deep copy, immutable JSON, and human YAML; all three limits prove missing, unknown, explicit zero, and positive through the same surfaces. The ordinary affected-package suite passed (`pkg/authority 0.281s`, `pkg/reconciler 0.546s`, `pkg/catalogs 22.575s`, `pkg/provenance 1.406s`, OpenAI `3.216s`, Anthropic `1.253s`, Google `0.924s`, models.dev `9.700s`). The exact affected race gate `go test -race ./pkg/authority ./pkg/reconciler ./pkg/catalogs ./pkg/provenance ./internal/providers/openai ./internal/providers/anthropic ./internal/providers/google ./internal/sources/modelsdev -count=1` passed (`1.280s`, `1.489s`, `52.696s`, `1.451s`, `6.224s`, `1.670s`, `2.176s`, `64.636s`). |
| 2026-07-28 | P4.10 | Bounded 10-second, four-worker fuzz campaigns completed without panic or failure against the actual quarantining OpenAI-compatible envelope (`40,823` executions; `11.302s`), Anthropic envelope (`287,167`; `11.252s`), Google AI Studio envelope (`254,115`; `10.397s`), models.dev provider/model object decoder (`248,613`; `10.428s`), and provenance YAML decode/report/re-encode path (`565,677`; `11.218s`). Seeds include a valid record beside a drifted sibling, null collections, malformed provenance, and normal payloads. Inputs are bounded by the production source-payload limit and production JSON nesting validator where applicable; no corpus file or new production decoding API was added. |
| 2026-07-28 | P4.10 / F-045 | Focused `go vet`, pinned golangci-lint v2.12.2 with zero issues, generated GoDoc, `make docs-check`, and `git diff --check` passed. The first full `make verify` reached coverage after ordinary tests, repository race-short (root `267.987s`), vet, zero-allocation catalog performance (`7.963–9.515 ns/op`), and zero-issue lint, then correctly failed because `pkg/authority` was `88.9%` against its `90%` floor. The missing behavior was the stable evidence-path contract, not production code: the exhaustive policy test now verifies explicit `EvidencePath` and `Path` fallback for every policy, raising coverage to `95.6%`. The uninterrupted final-tree `make verify` then passed ordinary tests, repository race-short (root `310.266s`), vet, pinned lint, every coverage floor, current docs/diff, build, 933-model validation, and CLI smokes; `BenchmarkClientCatalog` measured `10.18–10.66 ns/op`, `0 B/op`, and `0 allocs/op`. P4.10 remains the sole active task only to carry this exact phase tree through commit, hosted checks, protected merge, and workspace cleanup. |
| 2026-07-28 | P4.10 | Committed the policy/property/fuzz work and evidence as `6c5c5fb7c75c40a496f0ef1e37adcb3c71169899`. That exact commit passed uninterrupted `make verify`: ordinary tests, repository race-short (root `319.476s`), vet, pinned golangci-lint v2.12.2 with zero issues, every coverage floor including authority at `95.6%`, current docs/diff, build, 933-model validation, and CLI smokes; `BenchmarkClientCatalog` measured `10.26–11.41 ns/op`, `0 B/op`, and `0 allocs/op`. Exact-head `go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...` found zero reachable and zero imported-package vulnerabilities. This evidence-only ledger follow-up becomes the final P4 candidate head and must repeat exact local verification before hosted evidence can close P4.10. |
| 2026-07-28 | P4.10 | Ledger head `204b2c79b6e0d50c0621fb1552316d5eef36c34c` passed exact uninterrupted `make verify`: ordinary tests, repository race-short (root `295.168s`), vet, zero-issue pinned lint, every coverage floor, docs/diff, build, 933-model validation, and CLI smokes; `BenchmarkClientCatalog` measured `10.47–11.07 ns/op`, `0 B/op`, and `0 allocs/op`. Current exact-head govulncheck again found zero reachable/imported-package vulnerabilities. A normal push was safely rejected because the local branch inherited `origin/main` as its upstream and global `push.default=upstream`; no protected ref changed. An explicit `HEAD:refs/heads/codex/catalog-authority-resilience` refspec pushed only the topic branch and its upstream was corrected. Opened ready protected phase PR [#51](https://github.com/agentstation/starmap/pull/51); initial [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30340996361/job/90216315200) and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30340996361/job/90216315266) runs started on `204b2c79`. This live-PR ledger commit becomes the final candidate head and must pass exact local and hosted gates before merge. |
| 2026-07-28 | P4.10 / F-046 | PR #51 head `13431170` passed exact local `make verify` (race-short root `266.901s`; catalog access `8.958–11.31 ns/op`, `0 B/op`, `0 allocs/op`) and current govulncheck. Hosted [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30341046147/job/90216528917) passed in `2m43s`, but [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30341046147/job/90216528833) failed after `17m29s`: every ordinary package passed and every race package except `internal/server` passed; `TestPostCommitNotificationCorrespondsAcrossHTTPWebSocketSSEAndCacheDespiteHookFaults` timed out reading WebSocket publication after its own five-second deadline expired during the slower full-catalog update. The test no longer binds the SSE stream lifetime to a connection timeout, bounds only WebSocket establishment, waits for both broker adapters plus both transport clients, and starts the read deadline after the committed generation is observable. Ten consecutive focused real-transport race runs passed (`89.995s`), followed by the complete `go test -race ./internal/server -count=1` (`165.560s`), focused vet, zero-issue pinned lint, docs check, and diff check. No production timeout, retry, or transport behavior changed. |
| 2026-07-28 | P4.10 / F-046 | Final PR #51 head `7454e3b86ac27ee001752c2ff371677116e0f66a` passed exact uninterrupted `make verify`: ordinary tests, repository race-short (root `281.487s`; `internal/server` `171.345s`), vet, pinned golangci-lint v2.12.2 with zero issues, every coverage floor, current docs/diff, build, 933-model validation, and CLI smokes; `BenchmarkClientCatalog` measured `10.36–12.24 ns/op`, `0 B/op`, and `0 allocs/op`. Current exact-head govulncheck found zero reachable and zero imported-package vulnerabilities. |
| 2026-07-28 | P4.10 / P3.1 | PR #51 exact head `7454e3b86ac27ee001752c2ff371677116e0f66a` passed hosted [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30342739533/job/90221817277) in `20m30s` and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30342739533/job/90221817378) in `2m37s`. Protection required exactly both contexts with strict checking, admin enforcement, conversation resolution, zero required approvals, stale-review dismissal, and no force-push/deletion; the PR was ready, mergeable/clean, and had zero reviews or review threads. It squash-merged as `60f0cd3c6eecf0ecb9be7dc76961abf97919324d`; the remote branch was absent. Created fresh `/Users/jack/src/github.com/agentstation/starmap-worktrees/catalog-workspace-lifecycle` on `codex/catalog-workspace-lifecycle` from that exact protected-main SHA. The P4 worktree was clean and its tree matched `origin/main`; it was removed before deleting the local topic branch. P4 and P4.10 are DONE; P3.1 is the sole active task. |
| 2026-07-28 | P3.1 | Replaced the split export/database vocabulary before launch with one human workspace across `catalog_path`, root `WithCatalogPath`, sync `WithCatalogPath`, result `CatalogPath`, and CLI `--catalog-path`; removed `catalog_export_path`, `WithCatalogExportPath`, `WithOutputPath`, `--input-dir`, and `--output-dir` rather than retaining aliases. The CLI's passive immutable generation store is now a separate machine lifecycle at `~/.starmap/state/catalog`; the human provider-YAML workspace remains `~/.starmap/catalog`. Explicit paths bypass default workspace discovery, and the embedded catalog remains the verified fallback only when the selected workspace is absent. README, architecture, CLI/testing guidance, changelog migration note, AGENTS examples, generated root API, constants, and error GoDoc use the same vocabulary. |
| 2026-07-28 | P3.1 / F-032 | Added read-only `workspace.ValidateHumanLayout` and typed `errors.LegacyCatalogLayoutError`. A selected human path containing any of `current`, `generations/`, or `.commit.lock` fails before embedded construction, source fetch, staging, repair, projection, or generation commit and names the legacy path, detected entries, and configured migration target. Missing paths and normal provider YAML pass; symlink/non-directory workspaces fail typed validation. Tests preserve an exact recursive legacy fixture, prove the new machine state root stays absent after constructor rejection, prove an explicit sync path cannot commit before rejection, and prove `workspace.Project` creates no candidate directory before rejection. The explicit transactional migration remains owned by P3.10; no automatic move, compatibility alias, or real-user-path mutation was added. |
| 2026-07-28 | P3.1 | Exact affected ordinary suites passed across root (`101.238s`), `pkg/sync` (`0.966s`), pipeline (`0.787s`), workspace (`0.923s`), CLI app (`28.044s`), update command (`1.385s`), models.dev generation tooling (`12.635s`), and server (`38.045s`). The exact affected race command passed root (`260.766s`), sync (`1.612s`), pipeline (`1.919s`), workspace (`3.234s`), CLI app (`108.096s`), update (`2.440s`), models.dev (`68.351s`), and server (`146.108s`). Focused `go vet`, generated GoDoc, `make docs-check`, and `git diff --check` passed. The first pinned golangci-lint v2.12.2 invocation referenced removed P4 worktree files from its machine cache; `golangci-lint cache clean` removed only that stale non-repository state and the exact rerun passed with zero issues. P3.1 is DONE; P3.2 is the sole active task. |
| 2026-07-28 | P3.2 | Centralized symlink-aware lifecycle-root validation in `internal/catalog/workspace`: the human YAML workspace cannot equal, contain, or sit beneath the filesystem generation store, an explicit source-state root, the default models.dev HTTP cache root, or the default Git checkout root. Root construction, explicit save/sync, and sync option validation use the same typed `catalog filesystem layout` failure. Pipeline preflight now rejects source/cache overlap before it reads the workspace or creates source state. Canonical defaults for state, cache, source checkout, and logs are proven disjoint. The CLI's remaining unexported `catalogDatabasePath` vocabulary was renamed `catalogStatePath`; no compatibility alias was retained. |
| 2026-07-28 | P3.2 | Machine-artifact boundary tests place valid model-shaped YAML in sibling catalog state, HTTP cache, Git checkout, and interrupted projection-candidate trees and prove the human loader sees only its selected workspace. Filesystem store tests prove `.commit.lock`, `current`, retained `generations/`, and all store candidates remain under the state root and successful commit leaves no candidate/current temporary. Projection tests prove its same-filesystem staging and verification directories are hidden siblings, are removed after success and pre-promotion failure, and its repair marker is a sibling outside the provider-YAML loader root. README and architecture documentation state the same separation and explain the same-filesystem staging exception. |
| 2026-07-28 | P3.2 | Exact final-tree `go test -race . ./internal/catalog/workspace ./internal/catalog/pipeline ./pkg/sync ./pkg/catalogstore ./cmd/starmap/app ./internal/sources/modelsdev -count=1` passed (`250.763s`, `1.743s`, `2.276s`, `1.679s`, `1.896s`, `100.878s`, and `65.207s`). Focused `go vet`, generated GoDoc, `make docs-check`, and `git diff --check` passed. Pinned golangci-lint v2.12.2 first reported that the new pre-read branch raised the existing pipeline orchestrator from complexity 30 to 31; context/run-ID/timeout setup was extracted as one coherent helper rather than suppressing the gate, the focused pipeline race suite passed, and the exact lint rerun reported zero issues. P3.2 is DONE; P3.3 is the sole active task. |
| 2026-07-28 | P3.3 | A selected workspace is now observed read-only before local loading and candidate construction. An explicit update of an absent path is a required seed publication even when reconciliation finds no semantic changes; dry run remains non-mutating. The verified compiled catalog enters that first run as the complete `embedded_catalog` observation, while an absent path produces no false `local_catalog` observation. Existing workspaces do not yet receive a mislabeled embedded observation from the merged local builder: P3.4 owns loading the embedded revision independently for E1→E2 reconciliation. Embedded is last in every authority order. |
| 2026-07-28 | P3.3 | Generation-store commit remains before YAML projection. Projection accepts the pre-construction input expectation and returns a typed conflict if another process creates the workspace before projection, preserving the operator's files. Failure before first promotion leaves no workspace or candidate staging. The root lifecycle test proves construction creates nothing; an explicit no-change update commits one generation with exactly the embedded observation; its projected provider/model counts, payload checksum, and provenance match the active immutable catalog; no definitions/offerings/overrides tree exists; and historical local evidence already present in the embedded artifact is not expanded. Injected generation commit failure leaves the workspace absent, store empty, and prior embedded catalog active. |
| 2026-07-28 | P3.3 | Exact final-tree ordinary affected suites passed across root (`53.147s`), workspace (`2.466s`), pipeline (`0.320s`), embedded source (`1.174s`), authority (`1.374s`), sources (`2.081s`), reconciler (`0.517s`), sync (`0.959s`), catalog store (`2.115s`), and CLI app (`17.968s`). The exact race command `go test -race . ./internal/catalog/workspace ./internal/catalog/pipeline ./internal/sources/embedded ./pkg/authority ./pkg/sources ./pkg/reconciler ./pkg/sync ./pkg/catalogstore ./cmd/starmap/app -count=1` passed (`333.375s`, `2.828s`, `1.627s`, `3.633s`, `2.861s`, `3.426s`, `2.759s`, `3.103s`, `1.740s`, and `112.801s`). The embedded source count is provider-model-scoped rather than definition-deduplicated and has a shared-ID test. Focused `go vet`, pinned golangci-lint v2.12.2 with zero issues, generated GoDoc, `make docs-check`, and `git diff --check` passed. P3.3 is DONE; P3.4 is the sole active task. |
| 2026-07-28 | P3.4 | The sync pipeline no longer obtains its human observation from `catalogs.NewLocal`, which pre-merged running embedded bytes with the selected YAML. It now loads the human workspace and verified embedded revision independently and constructs a third derived provider-configuration catalog: embedded supplies newly introduced providers while an existing human provider record supplies its connection configuration. Local and embedded remain distinct immutable observations; embedded always participates and remains last in every field authority order. Missing workspaces still seed through embedded only. |
| 2026-07-28 | P3.4 | The E1→E2 lifecycle test reconciles an E1 embedded observation, writes its provenance to real provider YAML, makes a semantic human name edit, injects E2, and runs the production pipeline over local plus embedded. E2 advances an unchanged description and context limit, fills a previously missing output limit, and retains the human name with local evidence. The resulting candidate atomically projects through the production workspace module and reloads with identical values and embedded/local provenance. A separate injected embedded-load failure performs no apply and leaves the existing workspace unchanged. Input tests prove workspace loading does not merge repository embedded data and provider configuration composition preserves a human endpoint while adding a provider introduced by E2. |
| 2026-07-28 | P3.4 | Exact final-tree ordinary suites passed across root (`52.634s`), pipeline (`2.756s`), workspace (`0.632s`), embedded source (`0.815s`), local source (`3.602s`), reconciler (`1.049s`), catalogs (`25.454s`), authority (`2.580s`), sources (`1.732s`), sync (`2.137s`), catalog store (`1.487s`), and CLI app (`26.975s`). The exact race command `go test -race . ./internal/catalog/pipeline ./internal/catalog/workspace ./internal/sources/embedded ./internal/sources/local ./pkg/reconciler ./pkg/catalogs ./pkg/authority ./pkg/sources ./pkg/sync ./pkg/catalogstore ./cmd/starmap/app -count=1` passed (`299.483s`, `7.581s`, `3.363s`, `2.313s`, `9.365s`, `1.942s`, `67.065s`, `2.845s`, `3.892s`, `3.257s`, `2.542s`, and `127.475s`). The first pinned-lint pass reported that the new input branches raised `Pipeline.Sync` complexity from 30 to 32; catalog-input assembly was extracted as one named invariant instead of suppressing the limit, the exact race command was repeated on that final tree, focused `go vet` passed, pinned golangci-lint v2.12.2 reported zero issues, and generated GoDoc, `make docs-check`, and `git diff --check` passed. P3.4 is DONE; P3.5 is the sole active task. |
| 2026-07-28 | P3.5 / F-001 / F-004 | Root construction with no durable generation now loads an existing human workspace exactly through `NewFromPath`; it no longer pre-merges the running embedded revision and therefore cannot erase a semantic human value before explicit reconciliation. A missing workspace still uses the verified in-memory bootstrap and is never created by construction. The local source likewise requires an injected catalog or actual path and cannot masquerade as embedded fallback. `rg` confirms `catalogs.NewLocal` has no production caller; the now-unused exported prelaunch helper remains explicitly owned by P5.8 rather than being silently retained as architecture. |
| 2026-07-28 | P3.5 | The E1 workspace lifecycle now changes comment text, scalar quoting, and top-level key order without changing typed semantics. A production pipeline update over local plus embedded reports no changes, performs no apply or YAML rewrite, and retains embedded provenance for both tested fields. The paired E1→E2 test changes one semantic name: only that path becomes local evidence, while the unchanged description and limits advance or fill from embedded. A root construction test uses a real embedded-colliding OpenAI/GPT-4o identity and proves the exact human name and one-provider workspace load, catching the former pre-merge behavior. |
| 2026-07-28 | P3.5 | Exact final-tree ordinary suites passed across root (`39.669s`), pipeline (`2.074s`), local source (`0.523s`), reconciler (`0.955s`), catalogs (`20.689s`), sync (`0.732s`), catalog store (`1.865s`), and CLI app (`11.944s`). The exact race command `go test -race . ./internal/catalog/pipeline ./internal/sources/local ./pkg/reconciler ./pkg/catalogs ./pkg/sync ./pkg/catalogstore ./cmd/starmap/app -count=1` passed (`249.989s`, `6.392s`, `1.278s`, `2.625s`, `53.218s`, `2.797s`, `2.178s`, and `61.823s`). Focused `go vet`, pinned golangci-lint v2.12.2 with zero issues, generated GoDoc, `make docs-check`, and `git diff --check` passed. P3.5, F-001, and F-004 are DONE; P3.7 is the sole active task. |
| 2026-07-28 | P3.7 | Projection and startup repair now share one nonblocking OS advisory writer lock in a machine-owned sibling file derived from the canonical absolute workspace path. Lock ownership spans input read, off-side staging, digest recheck, atomic directory promotion, and marker publication. Contention returns typed `errors.ConflictError` immediately; a symlinked or non-regular lock path fails typed validation. The persistent empty lock file is not catalog data, is never traversed by the human loader, and OS process exit releases ownership. Generation-store CAS remains the sole durable commit point and its separate store lock is unchanged. |
| 2026-07-28 | P3.7 | A real two-child-process test holds the writer lock immediately before promotion, proves an unlocked reader still sees the complete prior workspace, proves a second projector and concurrent repair both receive typed conflicts, then releases the holder and observes exactly its complete new workspace with no losing model or staged directory. Ten consecutive focused runs passed. A second process test kills the holder while locked and proves the next projector succeeds, so a crash cannot wedge the workspace. A symlink-lock test preserves the referenced operator file and creates no workspace. |
| 2026-07-28 | P3.7 | Exact final-tree ordinary suites passed across root (`39.121s`), workspace (`0.567s`), pipeline (`1.598s`), catalog store (`1.375s`), sync (`0.924s`), and CLI app (`11.143s`). The exact race command `go test -race . ./internal/catalog/workspace ./internal/catalog/pipeline ./pkg/catalogstore ./pkg/sync ./cmd/starmap/app -count=1` passed (`248.605s`, `3.659s`, `6.289s`, `2.652s`, `1.729s`, and `60.143s`). Focused `go vet`, pinned golangci-lint v2.12.2 with zero issues, generated GoDoc, `make docs-check`, and `git diff --check` passed. P3.7 is DONE; P3.9 is the sole active task. |
| 2026-07-28 | P3.9 / F-047 | Review found that the optimistic projection expectation recorded only path/presence. The pipeline now binds the exact semantic digest of the human catalog loaded before candidate construction. A semantic edit made before projection returns typed `errors.ConflictError`, leaves the human edit and prior complete workspace visible, and creates no staging residue. This is a completed internal reliability finding, not an external blocker. |
| 2026-07-28 | P3.9 | Starmap owns no filesystem watcher. `Client.Sync(ctx, sync.WithSources(sources.LocalCatalogID))` is the explicit library reload and `starmap update --source local` is the CLI spelling. A small real-workspace lifecycle test proves the running catalog remains unchanged after an edit, one semantic reload crosses the pipeline publication boundary exactly once, an unchanged repeat publishes zero times, and the existing comment/quote/key-order fixture publishes zero times. |
| 2026-07-28 | P3.9 | Added `Client.Rollback` over the existing retained-generation `Store.Commit` compare-and-swap rather than a second rollback mechanism. The target is validated, identity-bound, decoded, and its workspace input digest captured before commit. Successful rollback restores exact retained generation bytes, in-memory reads, prior deterministic workspace semantic bytes/checksum, and provenance; retains the later generation; increments sequence and emits one event; and is idempotent on repeat. Injected commit failure changes neither store, catalog, nor workspace. A semantic edit injected after commit remains intact while the rolled-back generation stays active and projection reports `pending_repair`. |
| 2026-07-28 | P3.9 | Exact final-tree ordinary suites passed across root (`52.710s`), workspace (`4.530s`), pipeline (`3.539s`), catalog store (`5.568s`), sync (`0.904s`), and update CLI (`1.221s`). The exact race command `go test -race . ./internal/catalog/workspace ./internal/catalog/pipeline ./pkg/catalogstore ./pkg/sync ./cmd/starmap/cmd/update -count=1` passed (`250.655s`, `3.638s`, `6.029s`, `2.575s`, `1.971s`, and `1.441s`). Focused `go vet`, pinned golangci-lint v2.12.2 with zero issues, generated GoDoc, `make docs-check`, and `git diff --check` passed. P3.9 and F-047 are DONE; P3.10 is the sole active task. |
| 2026-07-28 | P3.10 / F-032 | Added the deliberately invoked `starmap migrate catalog` path rather than automatic constructor mutation or a long-lived compatibility parser. It requires the exact former filesystem-store shape, acquires the store commit lock and workspace writer lock, validates the current plus every retained generation and the running binary's schema compatibility before mutation, atomically renames the store to the separate CLI state root, reopens and verifies its exact current bytes, then projects that current generation as the one human provider-YAML workspace. The command accepts no alternate machine destination and ordinary construction continues to return typed `LegacyCatalogLayoutError` before mutation. |
| 2026-07-28 | P3.10 | An injected post-rename failure removes any candidate workspace/marker, moves the byte-identical store back, and leaves no migration-created writer lock. A process exit after the atomic move is recoverable rather than destructive: normal startup loads the relocated durable current and repairs the absent YAML without another commit. Repeated restarts retain the exact generation ID, canonical payload bytes, YAML bytes, projection marker, and retained generations. A manifest produced for a newer schema is rejected by the older binary before the state parent or any other target path is created. The schema-v0 data adapter remains explicitly distinct from this one-time path-meaning migration. |
| 2026-07-28 | P3.10 | Focused ordinary suites passed for workspace (`0.818s`), migration CLI (`0.159s`), and CLI app (`23.939s`). The exact broader ordinary command `go test . ./internal/catalog/workspace ./cmd/starmap/cmd/migrate ./cmd/starmap/app ./pkg/catalogstore -count=1` passed (`45.810s`, `0.778s`, `0.636s`, `27.841s`, `1.062s`). The exact broader race command `go test -race . ./internal/catalog/workspace ./cmd/starmap/cmd/migrate ./cmd/starmap/app ./pkg/catalogstore -count=1` passed (`254.316s`, `4.379s`, `2.135s`, `148.956s`, `2.061s`). Focused `go vet`, pinned golangci-lint v2.12.2 with zero issues, generated GoDoc, `make docs-check`, and `git diff --check` passed. P3.10 remains active for exact committed-head repository verification and protected hosted evidence. |
| 2026-07-28 | P3.10 / F-048 | The first repository-wide `make verify` passed all ordinary tests, the full race-short suite (root `265.795s`, app `166.670s`, workspace `4.273s`, migration CLI `1.737s`), vet, the catalog performance gate (`10.71–11.64 ns/op`, `0 B/op`, `0 allocs/op`), and pinned golangci-lint v2.12.2 with zero issues, then correctly failed at the unchanged `pkg/errors >= 80%` budget: P3.1 had added the typed legacy-layout error without direct package tests, leaving coverage at `77.8%`. Exact message tests now prove identified entries plus migration target and fallback entries without a target; focused ordinary coverage is `84.3%`, race passes, and pinned lint remains zero-issue. The complete gate therefore had to be repeated after the fix. |
| 2026-07-28 | P3.10 / F-048 | The uninterrupted corrected-tree `make verify` passed ordinary tests, repository race-short (root `307.851s`, app `208.209s`, workspace `4.806s`, migration CLI cached), vet, zero-allocation catalog access (`10.99–11.74 ns/op`), pinned zero-issue lint, every coverage floor including `pkg/errors 84.3%`, docs/diff, build, 933-model validation, and CLI smokes. A final review then tightened post-relocation equality from identity/checksum fields to the entire manifest JSON plus payload bytes. Its exact affected race gate passed (workspace `3.986s`, migration CLI `1.175s`, app `147.864s`, store `2.056s`, errors `1.178s`), followed by focused vet, zero-issue pinned lint, docs check, and diff check. The committed candidate must repeat uninterrupted repository verification before hosted evidence. |
| 2026-07-28 | P3.10 | Committed the migration implementation, operator command, restart/downgrade/rollback suite, documentation, and ledger as `c4d7c7575b0808d7c64483188623e464b048bf22`. That exact clean commit passed uninterrupted `make verify`: ordinary tests, repository race-short (root `289.150s`, app `179.054s`, workspace `5.600s`), vet, pinned golangci-lint v2.12.2 with zero issues, every coverage floor including `pkg/errors 84.3%`, generated docs/diff, build, 933-model validation, and CLI smokes; `BenchmarkClientCatalog` measured `7.968–8.574 ns/op`, `0 B/op`, and `0 allocs/op`. Exact-head `govulncheck@v1.6.0 ./...` found zero reachable and zero imported-package vulnerabilities (one required-module advisory is unreachable and unimported). This evidence-only ledger follow-up becomes the protected P3 candidate and must repeat exact local verification before hosted evidence can close P3.10. |
| 2026-07-28 | P3.10 | Ledger candidate `a133e92132788e8e302f61fc45d5f5ec2bf539c5` passed exact uninterrupted `make verify`: ordinary tests, repository race-short (root `276.035s`, app `174.230s`, workspace `5.239s`), vet, zero-issue pinned lint, every coverage floor, docs/diff, build, 933-model validation, and CLI smokes; catalog access measured `10.02–10.73 ns/op`, `0 B/op`, and `0 allocs/op`. Current exact-head govulncheck again found zero reachable/imported-package vulnerabilities. Explicitly pushed only `HEAD:refs/heads/codex/catalog-workspace-lifecycle` and opened ready protected phase PR [#52](https://github.com/agentstation/starmap/pull/52); initial [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30357694041/job/90269629755) and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30357694041/job/90269629701) runs queued on `a133e921`. Protection requires exactly both contexts with strict checking, admin enforcement, conversation resolution, stale-review dismissal, zero required approvals, and no force-push/deletion. This live-PR ledger commit becomes the final candidate and must pass exact local and hosted gates before the explicit protected merge pause. |
| 2026-07-28 | P3.10 / F-049 | PR #52 exact head `ba60db5031c5d984040fb826be3fb283fc49464f` passed an uninterrupted local `make verify`, including repository race-short (root `290.560s`, app `161.742s`, workspace `4.993s`), zero-issue lint, all coverage/docs/build/catalog/smoke gates, and `BenchmarkClientCatalog` at `9.704–10.09 ns/op`, `0 B/op`, and `0 allocs/op`. Hosted [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30357725482/job/90269794067) passed, but [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30357725482/job/90269794013) failed in the race-short command: the root package reached Go's implicit `10m0s` timeout after its queued parallel rollback/layout tests had run for only eight seconds, and the app migration-restart fixture's one-minute production repair context expired while projecting an unnecessary full 933-model embedded generation under runner contention. The repair does not enlarge a production operation timeout: migration fixtures now use one provider/model, rollback and explicit-sync tests construct direct clients because they do not test `New`, and `scripts/verify.sh` explicitly allows 20 minutes per race package within the existing 45-minute hosted job. The exact focused command passed ordinarily (root `0.659s`, app `3.276s`, workflow `0.683s`) and five consecutive race copies passed (root `3.464s`, app `83.197s`, workflow `1.190s`). Full exact-head local and hosted reruns remain required before F-049 is DONE. |
| 2026-07-28 | P3.10 / F-049 | Repair commit `35ff2f6aa94a5ed0456993dd00fb248b42051244` passed uninterrupted `make verify`. Ordinary root and CLI-app suites completed in `42.091s` and `14.567s`; repository race-short with the explicit per-package ceiling passed with root `268.839s`, app `86.255s`, workspace `4.065s`, migration CLI `2.463s`, and workflow fixtures `1.211s`. Vet, pinned golangci-lint v2.12.2 with zero issues, every coverage floor including `pkg/errors 84.3%`, current docs/diff, build, 933-model validation, and CLI smokes passed; `BenchmarkClientCatalog` measured `9.621–10.73 ns/op`, `0 B/op`, and `0 allocs/op`. This evidence-only ledger head must repeat exact local verification before push and hosted proof. |
| 2026-07-28 | P3.10 | User steering makes structured autoreview an outcome-level gate: ordinary self-review and focused/race/lint/docs/diff checks continue throughout implementation; one review runs when a coherent task/phase/PR candidate is functionally complete and repeats only after material architecture/API/concurrency/persistence/security/failure-semantics remediation. F-049's fixture slimming and bounded CI harness require only exact local and hosted gates. Repository/ledger search found no completed structured review of the full P3 workspace-lifecycle unit, so one final branch review is due before the protected merge pause; its evidence-only follow-ups will not retrigger review. |
| 2026-07-28 | P3.10 / F-050 / F-051 | The autoreview helper validated and bundled the complete 81-file P3 branch in one 358,055-byte pass, then enforced its nested-Codex guard. The documented repository-grounded fallback found one P3 blocker: post-relocation rollback unconditionally removed the vacated path, so an obsolete process or operator could recreate data that rollback then deleted. Rollback now deletes only the exact checksum-bound YAML projection created by the migration; a recreated, invalid, machine-layout, symlinked, non-directory, or semantically different path returns a typed conflict and preserves both it and the relocated generation store. Tests prove byte-identical ordinary rollback, safe rollback after post-projection marker failure, and preservation of concurrently recreated data plus exact relocated current. Focused ordinary/race migration suites, vet, pinned zero-issue lint, docs, and diff checks passed. The same review recorded inert `WithEmbeddedCatalog`/`use_embedded_catalog` as F-051 for the already-scoped P5.8/P6.5 public-surface deletion rather than broadening the P3 persistence fix. Full exact-head repository and hosted gates remain pending. |
| 2026-07-28 | P3.10 / F-049 | PR #52 exact head `b3acac860fc28ad901f8050bf02af9c875e20b7e` passed hosted [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30361252089/job/90281308442) in `19m06s` and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30361252089/job/90281308500) in `2m43s`. This exact hosted proof, together with the three uninterrupted exact-head local `make verify` runs already recorded, closes F-049. Later review remediation changes migration failure semantics and therefore requires its own exact final local and hosted proof; it does not reopen the product-neutral harness finding. |
| 2026-07-28 | P3.10 / F-050 | The complete F-050 remediation tree passed uninterrupted `make verify`: ordinary root and CLI-app suites completed in `45.029s` and `15.558s`; repository race-short passed with root `256.483s`, app `74.990s`, workspace `6.036s`, and migration CLI `1.468s`; vet and pinned golangci-lint v2.12.2 passed with zero issues; every coverage floor passed including `pkg/errors 84.3%`; docs/diff, build, 933-model validation, and CLI smokes passed. `BenchmarkClientCatalog` measured `11.26–11.80 ns/op`, `0 B/op`, and `0 allocs/op`. The material remediation and review evidence must now be committed and this complete gate repeated on that exact clean commit before push. |
| 2026-07-28 | P3.10 / F-050 | Committed the migration rollback hardening, failure tests, operator quiescence guidance, outcome review, and ledger as `04a74036633140ac8c3709c83e6adea142b66cb0`. That exact clean commit passed uninterrupted `make verify`: ordinary root and CLI-app suites completed in `42.595s` and `14.546s`; repository race-short passed with root `250.020s`, app `72.096s`, workspace `5.536s`, and migration CLI cached/green; vet, pinned golangci-lint v2.12.2 with zero issues, every coverage floor including `pkg/errors 84.3%`, docs/diff, build, 933-model validation, and CLI smokes passed. `BenchmarkClientCatalog` measured `8.610–8.847 ns/op`, `0 B/op`, and `0 allocs/op`. This evidence-only ledger follow-up becomes the final P3 candidate and requires one final exact local gate before push and hosted proof; Rule 19 does not retrigger autoreview for that evidence-only change. |
| 2026-07-28 | P3.10 / P5.1 | PR #52 exact final head `42295e4e8540a10137ffbbe4bf6b8d11c89b5505` passed uninterrupted local `make verify`: ordinary root `43.780s`, CLI app `14.660s`, race root `249.253s`, race app `73.329s`, and workspace `6.692s`; vet, pinned golangci-lint v2.12.2 with zero issues, every coverage floor including `pkg/errors 84.3%`, docs/diff, build, 933-model validation, and CLI smokes passed. `BenchmarkClientCatalog` measured `11.33–11.61 ns/op`, `0 B/op`, and `0 allocs/op`. The same exact head passed hosted [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30364045985/job/90290602845) in `13m51s` and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30364045985/job/90290602858) in `2m04s`; protection required exactly both contexts with strict checking, admin enforcement, conversation resolution, stale-review dismissal, zero required approvals, and no force-push/deletion; the PR had zero review threads and merged as `9609f4f4a74281a7f9692a97cc4926df5978d754`. The remote topic branch was absent. A zero tree diff against merged `origin/main` and a clean status were proven before removing the P3 worktree and then its local branch. Created fresh `/Users/jack/src/github.com/agentstation/starmap-worktrees/catalog-read-model-simplification` on `codex/catalog-read-model-simplification` from that exact protected-main SHA. P3, P3.10, and F-032 are DONE; P5.1 is the sole active task. |
| 2026-07-28 | P5.1–P5.8 / F-052 | The P5 source audit found 322 unique persisted author-model records. Only 200 had an exact provider-model ID; strict legacy membership equivalence would lose 122 old memberships, add 311 currently derivable memberships, and preserve a second source of truth. The 121 non-provider IDs classify as 32 alias/content overlaps, 25 models.dev-only records, and 64 stale orphans. The reviewed terminal disposition is recorded in `docs/reviews/P5_AUTHOR_MODEL_CORPUS_DISPOSITION_2026-07-28.md`; dynamic sources may add current records through reconciliation, but embedded provider offerings are not manufactured from the obsolete mirror. Exact implementation and gate evidence remain pending. |
| 2026-07-28 | P5.1–P5.6 / F-011–F-013 / F-052–F-054 | Schema v2 persists exactly `schema_version`, `providers`, `authors`, `endpoints`, `provider_models`, and `provenance`. Repository inspection found zero author-model YAML files, zero persisted definition directories, and zero persisted offering directories; 611 provider-model YAML records remain the human/embedded source of truth. The 322 obsolete author records and exported runtime legacy migration/flattening adapters are deleted. Derived definitions use provider/model-scoped authority evidence with conservative unknown/identity fallback; offerings preserve exact provider price, limits, endpoint, modes, lifecycle, and unknown availability; author membership derives deterministically from inline authors plus attribution. Flat cache pricing and `Quantization` are the only schema spellings. |
| 2026-07-28 | P5.3–P5.8 / F-055–F-057 | Structured outcome review ran on complete 1,172,673-byte, 1,200,206-byte, and 1,212,013-byte three-pass bundles because the first two remediations materially changed public response/failure semantics. It found and closed provider-identity loss in hook/query/list/export/YAML paths, partial-metadata provenance nil dereference, canonical history-provider alias resolution, stale CLI/GoDoc text, a digest-stale fixture ID, and explicit embedded-corpus publication proof. One package-example claim was disproven by direct file inspection and recorded as such. The final run found no production defect; its sole finding was a test-only `Metadata`/`metadata` evidence-path mismatch, now corrected with exact lookup assertion and ten race-enabled repetitions. Full disposition is archived at `docs/reviews/P5_CATALOG_READ_MODEL_OUTCOME_REVIEW_2026-07-28.md`. |
| 2026-07-28 | P5.1–P5.8 | After final product remediation, `go test ./... -count=1` passed every package (root `32.280s`, CLI app `12.754s`, catalogs `17.206s`, bootstrap `4.513s`, server `11.787s`). `go test -race ./pkg/catalogs ./internal/catalog/query ./cmd/starmap/cmd/models ./internal/bootstrap ./internal/server/handlers -count=1` passed in `30.751s`, `1.209s`, `1.660s`, `13.060s`, and `4.942s`. The broader affected race gate had already passed root publication `177.458s`, catalogs `31.176s`, models.dev `66.601s`, app `51.392s`, workspace/store, reconciliation/pipeline/query, embedded/OpenAI sources, and update. `BenchmarkClientCatalog` measured `10.49–10.86 ns/op`, `0 B/op`, and `0 allocs/op`; generated docs and diff checks passed before the final review-only proof changes and must be regenerated in the exact repository gate. |
| 2026-07-28 | P5.2 / P5.8 / F-058 | The first uninterrupted `make verify` passed ordinary tests, repository race-short (root `186.805s`, app `57.152s`, catalogs `34.961s`, server `58.140s`, models.dev `69.134s`), vet, and the catalog-access budget (`10.79–11.43 ns/op`, `0 B/op`, `0 allocs/op`) before lint found two real P5 issues: the deleted public reads left an unused `modelsReader`, and the new definition assembler had cyclomatic complexity 36. The wrapper is deleted; identity, lineage, capabilities, and timestamps are extracted as named tested concepts. Golangci-lint also replayed cached diagnostics from the already-removed P3 worktree; clearing only its cache removed those nonexistent paths. Focused suites passed and pinned golangci-lint v2.12.2 now reports `0 issues`. A complete uninterrupted verification rerun remains required. |
| 2026-07-28 | P5.8 / F-059 | The second uninterrupted `make verify` passed ordinary tests (root `32.193s`, CLI app `11.899s`, catalogs `17.333s`), repository race-short (root `186.458s`, app `56.636s`, catalogs `35.217s`, server `57.944s`, models.dev `69.379s`), vet, catalog access (`11.32–11.62 ns/op`, `0 B/op`, `0 allocs/op`), and pinned lint with zero issues. It then correctly failed before coverage because `scripts/verify.sh` still named the now-deleted `internal/attribution` and `internal/attribution/matcher` packages. Those obsolete policy entries and their maintained documentation rows are removed; author membership is exercised as catalog derivation under `pkg/catalogs`, and no live-package threshold is reduced. Exact coverage and full repository verification remain required. |
| 2026-07-28 | P5.8 / F-059 | `make test-critical-coverage` passed every surviving threshold: pipeline `79.7%`, query `77.7%`, provider clients `96.0%`, provider source `86.2%`, events `73.7%`, middleware `97.0%`, params `98.5%`, response `100.0%`, SSE `96.7%`, WebSocket `88.2%`, transport `58.4%`, authority `95.6%`, catalogs `69.9%`, errors `84.3%`, reconciler `80.4%`, and sources `56.1%`. The gate no longer invokes a deleted package; F-059 is DONE. |
| 2026-07-28 | P5.8 | The third uninterrupted `make verify` passed ordinary tests (root `31.014s`, CLI app `10.366s`, catalogs `16.403s`), repository race-short (root `187.007s`, app `55.373s`, catalogs `35.236s`, server `59.305s`, models.dev `69.841s`), vet, pinned lint with zero issues, every surviving coverage floor, generated GoDoc, diff, build, 611-model catalog validation, and CLI smokes. `BenchmarkClientCatalog` measured `10.65–11.17 ns/op`, `0 B/op`, and `0 allocs/op`. A subsequent public-artifact audit found the generated OpenAPI inconsistency recorded as F-060, so the complete gate must run again after that verification-only repair. |
| 2026-07-28 | P5.8 / F-060 | The Go `ModelArchitecture` correctly deleted legacy `Precision`, but the embedded OpenAPI JSON/YAML still advertised `precision`; `make docs-check` regenerated only package READMEs and therefore could report success while the served schema was stale. `make openapi` removed the field from both embedded specifications. New `make openapi-check` generates both schemas off to the side with pinned Swag v2 and compares exact bytes; `docs-check` depends on it, and the focused check passes. An independent read-only generated-surface audit reproduced both specs byte-for-byte, mapped every exposed `catalogs.*` schema to a current Go type, and found none of the deleted legacy APIs in OpenAPI, root API docs, or catalog GoDoc. No runtime API or catalog semantics changed. |
| 2026-07-28 | P5.8 / F-060 | The complete verification-hardened P5 tree passed uninterrupted `make verify`: ordinary root `38.143s`, CLI app `10.301s`, and catalogs `16.230s`; repository race-short root `211.808s`, app `51.541s`, catalogs `32.148s`, server `55.195s`, and models.dev `66.552s`; vet; pinned golangci-lint v2.12.2 with zero issues; every surviving coverage floor including catalogs `69.9%`; exact OpenAPI reproduction plus generated GoDoc; diff; build; 611-model validation; and CLI smokes. `BenchmarkClientCatalog` measured `9.051–10.72 ns/op`, `0 B/op`, and `0 allocs/op`. This material tree is ready to commit; the exact clean commit must repeat repository verification and current vulnerability scanning before push. |
| 2026-07-28 | P5.8 / F-052–F-060 | Exact clean material commit `c3ae6dee0e139cc5ad6f5a86d1ad9efd023bbe23` passed uninterrupted `make verify`: ordinary root `35.007s`, CLI app `12.261s`, catalogs `17.488s`, and server `13.178s`; repository race-short root `200.813s`, app `51.716s`, catalogs `32.017s`, server `55.285s`, and models.dev `67.176s`; vet; pinned lint with zero issues; all coverage floors; exact OpenAPI and generated-GoDoc checks; diff; build; 611-model catalog validation; and CLI smokes. `BenchmarkClientCatalog` measured `10.69–11.35 ns/op`, `0 B/op`, and `0 allocs/op`. Current `govulncheck v1.6.0 ./...` found zero reachable vulnerabilities and zero vulnerabilities in imported packages; one required module contained a vulnerability in code Starmap does not call. The material P5 result is ready for its evidence-only ledger commit and protected PR lifecycle. |
| 2026-07-28 | P5.8 | Evidence head `fadccebfd2cc6339ee949e613c156ae10b30017e` passed uninterrupted `make verify`: ordinary root `30.918s`, CLI app `10.995s`, catalogs `16.900s`, and server `11.865s`; repository race-short root `182.088s`, app `52.357s`, catalogs `32.760s`, server `56.575s`, and models.dev `67.248s`; vet; pinned lint with zero issues; all coverage, generated-schema/docs, diff, build, 611-model validation, and CLI smoke gates. `BenchmarkClientCatalog` measured `10.96–11.33 ns/op`, `0 B/op`, and `0 allocs/op`. Current exact-head `govulncheck v1.6.0 ./...` again found zero reachable vulnerabilities and zero vulnerabilities in imported packages. Explicitly pushed only `HEAD:refs/heads/codex/catalog-read-model-simplification` and opened ready protected phase PR [#53](https://github.com/agentstation/starmap/pull/53); initial [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30418323225/job/90469542726) and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30418323225/job/90469542681) runs queued on that exact head. This live-PR ledger commit becomes the final candidate and must pass exact local and hosted gates before the protected merge pause. |
| 2026-07-28 | P5.8 / F-061 | PR #53 exact head `e3fca420ae170105ae91319d360e861196864aa0` passed uninterrupted local `make verify` (ordinary root `30.698s`, app `10.521s`, catalogs `16.912s`; race root `184.851s`, app `54.286s`, catalogs `33.231s`, server `57.545s`, models.dev `69.158s`; catalog access `11.47–11.83 ns/op`, `0 B/op`, `0 allocs/op`) and current govulncheck. Hosted [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30418354846/job/90469682104) passed in `2m39s`. [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30418354846/job/90469682161) passed minimum-Go tests, ordinary/race tests, vet, performance, zero-issue lint, and every coverage floor, then failed at `make docs-check` because the new OpenAPI check required an ambient `swag` binary absent from CI. `openapi` and `openapi-check` now run `github.com/swaggo/swag/v2/cmd/swag@v2.0.0-rc4` through the selected Go toolchain; the structural workflow fixture rejects ambient lookup. `make HAS_DEVBOX= openapi-check`, `make HAS_DEVBOX= docs-check`, `go test -race ./internal/ciworkflow -count=1`, and `git diff --check` pass without a Devbox-provided Swag binary. No product, schema, or runtime semantics changed. |
| 2026-07-28 | P5.8 / P6.1 / F-061 | PR #53 exact final head `94157b42269bcb91e1cd5577817871a93ba0acd9` passed uninterrupted `make HAS_DEVBOX= verify`: ordinary root `31.010s`, CLI app `10.098s`, catalogs `16.649s`, and server `11.221s`; repository race-short root `183.271s`, app `51.668s`, catalogs `31.786s`, server `55.115s`, and models.dev `66.770s`; vet; pinned lint with zero issues; every coverage floor; self-contained OpenAPI plus generated-GoDoc checks; diff; build; 611-model validation; and CLI smokes. `BenchmarkClientCatalog` measured `10.26–11.28 ns/op`, `0 B/op`, and `0 allocs/op`; current govulncheck found zero reachable/imported-package vulnerabilities. The same exact head passed hosted [Verification Gate](https://github.com/agentstation/starmap/actions/runs/30419380178/job/90472784848) in `14m37s` and [Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30419380178/job/90472784819) in `2m32s`. Protection required exactly both contexts with strict checking, admin enforcement, conversation resolution, stale-review dismissal, zero required approvals, and no force-push/deletion; the PR had zero reviews and zero review threads. After explicit user approval it squash-merged as `76dd317810815604b6c796814bce5b8887aaadd0`. The remote topic branch was absent; a zero tree diff and clean status were proven before removing the P5 worktree and local branch. Created fresh `/Users/jack/src/github.com/agentstation/starmap-worktrees/starmap-library-composition` on `codex/starmap-library-composition` from that exact protected-main SHA. P5 and P5.8 are DONE; P6.1 is the sole active task. |
| 2026-07-28 | P5.8 / P6.1 / F-062 | Exact `git ls-tree` counts show 611 `internal/embedded/catalog/providers/**/models/*.yaml` files both before PR #53 (`9609f4f4`) and after its merge (`76dd3178`); `git diff --name-status` reports no change anywhere under that provider tree. The large catalog diff consists of 322 deleted `authors/*/models/*.yaml` mirror files plus the refreshed generation manifest; root `authors.yaml`, `providers.yaml`, endpoints, provenance, and all provider records remain embedded by `//go:embed catalog sources`. The corrected source and generated GoDoc now name provider-model YAML as canonical, and `make HAS_DEVBOX= docs-check` passes. `TestEmbeddedBootstrapManifestMatchesCanonicalCatalog` walks the actual embedded filesystem and requires its provider-YAML file count to equal the built immutable offering count; it passes ordinarily and under `-race` with exactly 611 of each. |
| 2026-07-28 | P6.1 / F-063–F-066 | Accepted [`reviews/P6_PACKAGE_GRAPH_2026-07-28.md`](reviews/P6_PACKAGE_GRAPH_2026-07-28.md) at SHA-256 `4207b43d0f828d8d34830314b68512af19e5068beb9a57d90f742f7205085e26`, measured from an isolated archive of protected main `76dd3178`. `go list ./...` reports 90 packages versus the P2 baseline 89: migrate/workspace/embedded-source were added and two attribution packages removed; root plus `pkg/*` remains 23 and `go list -deps ./...` proves zero cycles. Root/catalog/core-union closures are 472/147/152; root is 214 standard, 33 local, and 225 external packages and contains 1 GenAI, 64 gRPC, 21 OpenTelemetry, and 1 Gorilla WebSocket packages. A simulated provider-client-edge cut still leaves 244 packages, so P6.2/P6.3 now remove the complete acquisition implementation from root. Every public and importable command package has a named role, current consumer, and keep/delete/internalize disposition. |
| 2026-07-28 | P5.9 / F-067–F-070 | User review reopened the author/model identity contract after PR #53. Exact protected-main inspection proves 539/611 provider-model files carry explicit authors, 187 provider IDs are hierarchical, and Alibaba records explicitly attribute Kimi to Moonshot and GLM to Zhipu, disproving provider→author inference. PR #53 deleted 322 author-model files and the `author_models` payload; its parent implementation described those files as denormalized views, copied provider models into `Author.Models`, skipped hierarchical IDs on save, and retained no provider key, so that exact behavior cannot generate complete endpoint rows. `authors.yaml` remains 761 lines while `endpoints.yaml` is empty; the server implements neither OpenRouter author/slug route. [`reviews/AUTHOR_SLUG_ENDPOINT_COMPATIBILITY_REVIEW_2026-07-28.md`](reviews/AUTHOR_SLUG_ENDPOINT_COMPATIBILITY_REVIEW_2026-07-28.md) records the user-steered replacement: authored model files and provider serving files own disjoint facts, provider records link explicitly to `{author}/{slug}`, and `endpoints.yaml` is generated from the validated join. P5.9 is the sole active task; P6.2 is paused. |
| 2026-07-28 | P5.9 / F-068 | Restored all 322 PR-#53-deleted author-model YAML files from exact pre-PR baseline `9609f4f4`, then normalized them as authored/history records rather than provider copies. The exact disposition map [`reviews/P5_AUTHOR_MODEL_CORPUS_MAP_2026-07-28.yaml`](reviews/P5_AUTHOR_MODEL_CORPUS_MAP_2026-07-28.yaml) has SHA-256 `f31d79125b16ecfe41ae53ebf83fbd53bfb11ac5f3e91382e70dd9afeef4aa24`, 322 unique `keep` entries, 201 with at least one exact current provider ID, and 121 authored-only entries. Thirty records missing inline authors now carry their reviewed path author; the two path/author contradictions moved from Alibaba to DeepSeek and Qwen with zero resulting identity collisions. Exact scans find zero author records containing provider status, pricing, limits, or modes and no extension other than models.dev. `go test ./internal/bootstrap -run 'TestEmbeddedAuthorModelCorpusHasExactReviewedDisposition|TestEmbeddedBootstrapManifestMatchesCanonicalCatalog' -count=1` passed (`0.907s`), as did the exact race command (`5.200s`); `git diff --check` passed. P5.9 and F-068 are DONE; P5.10 is the sole active task. |
| 2026-07-28 | P5.10–P5.12 / F-067 / F-070 | Implemented the reviewed two-role model. Schema v3 round-trips dedicated authored records plus provider serving records; all 610 retained provider YAML files carry an explicit `model: author/slug` link to one of 653 authored records. Authored records alone build canonical definitions; provider records build 610 exact offerings across 533 served definitions, with 120 authored-only definitions and no manufactured endpoint. Reconciliation has an executable `ModelRef` authority rule so source refreshes preserve the author link and never substitute the serving provider. Publication precomputes canonical, author, alias, provider-offering, and definition-offering indexes; dangling links and ambiguous aliases fail with typed errors. The generic unrelated endpoint collection and unused models.dev catalog wrapper are deleted. `internal/embedded/catalog/endpoints.yaml` now deterministically contains 610 provider rows, exact provider prices, generation ID, and payload digest; it is staged and validated off-side, atomically projected after the durable commit, included in projection repair checks, and direct drift is not overwritten. The exact provider identity map is [`reviews/P5_PROVIDER_MODEL_IDENTITY_MAP_2026-07-28.yaml`](reviews/P5_PROVIDER_MODEL_IDENTITY_MAP_2026-07-28.yaml), SHA-256 `ccf76f104d3336e8da951e2650c042758e721cadccf52387d06718565c8eec6f`. Focused ordinary and race suites for catalogs, store, bootstrap, workspace, manifest tooling, artifacts, and remote consumption passed; `go test ./... -count=1` passed every package, including root `43.988s`, CLI app `17.224s`, catalogs `19.340s`, bootstrap `10.165s`, workspace `3.161s`, models.dev `10.134s`, and server `16.369s`. The first-update integration test proves every reconciled/projected provider model still joins an authored definition. P5.10–P5.12, F-067, and F-070 are DONE; P5.13 is the sole active task pending one outcome autoreview and the exact phase gate. |
| 2026-07-29 | P5.13 / F-071–F-074 | The first structured P5.13 review returned 28 findings against committed head `1951e88b`; accepted identity, serialization, price, builder, and concurrency findings were remediated rather than hidden. Canonical ownership now maps Qwen models to Qwen Team even when Alibaba Cloud serves them, GPT OSS Safeguard to OpenAI, Orpheus to Canopy Labs, and BART to Meta. Exact Vertex `@001`/`-maas` provider IDs link to provider-independent slugs. Groq per-million prices are corrected, provider float noise is snapped only within a tight decimal tolerance, and request overrides emit native YAML values rather than byte arrays. Builder alias storage and author deletion enforce canonical ownership; the race suite exercises authored reads. Regenerated generation `catalog-20260729T065706Z-d0d18fa7fe24` has payload `sha256:d0d18fa7fe24fd2a0b25f651c55a2af2eb9baed3505e2a76bb6a8e21b285d2c8` and 610 endpoint rows across 531 served definitions; 649 authored definitions include 118 without an offering. Focused ordinary suites passed; `go test -race ./pkg/catalogs ./internal/bootstrap ./internal/catalog/workspace ./internal/providers/openai ./internal/providers/groq -count=1` passed in `38.598s`, `36.267s`, `4.831s`, `4.952s`, and `5.765s`. The remaining candidate historical fact contradictions are recorded as F-074/P10.9 rather than changed speculatively. An attempted final branch-mode review was stopped after proving it targeted the unremediated committed head; the remediated exact commit must receive the final structured review. |

## Final Definition of Done

The goal is complete only when:

- every phase and task is terminal;
- every finding is `DONE`, user-accepted `REJECTED`, or correctly
  `SUPERSEDED`;
- authored-model and provider-serving YAML are the only human model input
  representations; generated endpoints are never an authority;
- embedded/local/live/release upgrade semantics pass the lifecycle suite;
- existing generation-store layouts are detected before mutation and receive a
  tested explicit migration or typed rejection;
- generation-store CAS is the sole commit point and interrupted YAML
  projections repair by digest;
- immutable Catalog DX and budgets remain intact;
- provider pricing, source resilience, and provenance pass the authority suite;
- the Go library, embeddable server, and reactive remote consumer compile and
  pass real end-to-end tests;
- SSE push is the normal remote path and polling is only a tested fallback;
- flushed SSE heartbeats establish stream health, heartbeat loss forces
  reconnect and manifest catch-up, and catalog freshness is never inferred from
  connection liveness;
- the unused WebSocket path and dependency are absent unless a later named
  bidirectional consumer justified their reintroduction;
- no repository-authored Go file reaches 2000 lines and every >1000 file has a
  terminal review;
- dead modules, duplicate persisted data, and rejected compatibility surfaces
  are gone;
- the exact final protected-main head passes all required local and hosted
  checks;
- GitHub has no open Starmap pull request;
- local main exactly equals origin/main;
- temporary worktrees and obsolete branches are removed;
- a fresh clone reproduces the documented verification; and
- the final evidence audit states why the result is ready for an enterprise LLM
  proxy gateway without relying on an unpublished intention.
