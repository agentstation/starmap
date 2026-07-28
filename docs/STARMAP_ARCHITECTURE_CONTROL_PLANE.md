# Starmap Architecture Execution Control Plane

Last updated: 2026-07-28

Status: `IN_PROGRESS` — P0–P2, the P3.6a/P3.6b/P3.8 publication hotfix, P4,
and P3.1–P3.5/P3.7/P3.9 are complete. P3.10 has implemented explicit
transactional legacy-layout migration, restart recovery, and downgrade
rejection and remains the sole active task through its exact-head local,
hosted, merge, and cleanup gates.

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
7. Treat provider YAML as the single human-facing model representation.
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
- one human-editable provider YAML catalog under ~/.starmap/catalog;
- embedded catalogs are verified, versioned, lowest-authority observations;
- installing or constructing Starmap never silently rewrites an existing
  workspace;
- definitions, offerings, and author membership are derived immutable read
  views, not additional persisted configuration trees;
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
one human workspace: ~/.starmap/catalog/providers/...
            ^            |             ^
            |            |             |
 provider observations   |       models.dev observations
            \             |            /
             one authority + provenance implementation
                              |
                     validate complete candidate
                              |
                    immutable generation
                 store / sole commit CAS
                       |             \
                       v              +--> atomic YAML workspace
             atomic in-memory Catalog     post-commit projection,
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
| Local provider YAML | The one human-editable catalog; semantic edits are supported |
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
| **Provider model** | The one persisted human-readable model record under a provider in the YAML workspace. It contains provider-scoped identity and facts that can be observed or edited together. | A derived definition/offering split, an override fragment, or a remote event payload. |
| **Catalog** | Starmap's concrete immutable in-memory product: a complete validated read model with precomputed indexes. Public consumers retain and share `*catalogs.Catalog`; read methods return caller-owned values. | A mutable builder, a filesystem directory, an acquisition response, a `Snapshot` public API, or a collection returned by reference. |
| **Builder** | A mutable, unpublished construction mechanism used to assemble and validate a candidate catalog off to the side. It has one-way publication into a new immutable catalog. | The consumer-facing catalog, a concurrently shared mutable database, or the durable commit point. |
| **Workspace** | The one human-editable provider-YAML tree rooted at the configured catalog path, normally `~/.starmap/catalog`. Semantic edits are evidence; formatting is not. | The generation store, cache, staging area, exports directory, persisted definitions/offerings/overrides, or an implicitly watched directory. |
| **Source** | A configured acquisition adapter and identity, such as one provider API, models.dev transport, local workspace, or embedded bootstrap. | A field winner, a provider itself, a scheduler, or an anonymous blob with no identity. |
| **Observation** | One source attempt's bounded candidate facts plus source identity, retrieval evidence, status, completeness, issues, and scope. An observation may be rejected, degraded, or reconciled; it is never directly published. | The canonical catalog, proof that absent records were deleted, a generation, or a reusable credential container. |
| **Provenance** | Provider/model/field-scoped evidence explaining which observation or human edit supplied a selected value and why. | A bare model-ID map, one source label for an entire catalog, or persisted secrets/fingerprints. |
| **Definition** | A provider-independent immutable read view derived from canonical provider models for discovery and identity-oriented queries. | A second persisted model tree, an authority source, or a first-wins merge product. |
| **Offering** | A provider-specific immutable read view derived from one canonical provider model, preserving exact provider price, limits, endpoint, availability, and lifecycle knowledge. | A second persisted configuration record, invented availability, or a cross-provider aggregate price. |
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

1. Provider YAML is the only persisted human model representation.
2. Definitions, offerings, and author membership are derived read views from
   the immutable catalog and use the same authority/provenance result.
3. Embedded bytes, local edits, models.dev, and provider APIs enter
   reconciliation as identified observations; none replaces the whole catalog
   merely because it was read later.
4. Valid provider pricing is authoritative for that provider's offering.
   Provider price does not become a provider-independent definition fact.
5. Missing or degraded observations cannot prove deletion. Explicit lifecycle
   evidence is required to retire data.
6. Publication is atomic at generation-store CAS. YAML is a repairable
   post-commit projection and an event is a post-commit hint.
7. Public Go consumers say `catalog`; `snapshot` and `generation` remain
   internal lifecycle/storage vocabulary.
8. A remote subscriber treats stream liveness and catalog freshness as
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
  `/Users/jack/src/github.com/agentstation/starmap-worktrees/catalog-workspace-lifecycle`
- Branch:
  `codex/catalog-workspace-lifecycle`
- Base:
  `origin/main@60f0cd3c6eecf0ecb9be7dc76961abf97919324d`

This worktree contains the remaining P3 human workspace lifecycle work. It was
created from the exact protected main produced by merged PR #51 before the
clean P4 worktree and local branch were removed.

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
6. P5 read views, P6 composition, P7 server/reactive delivery, P8 modularity,
   P9 distribution, P10 verification, and P11 cleanup.

Suggested branch names:

- `codex/catalog-workspace-lifecycle`
- `codex/catalog-authority-resilience`
- `codex/catalog-read-model-simplification`
- `codex/starmap-library-composition`
- `codex/starmap-reactive-server`
- `codex/starmap-go-modularity`
- `codex/starmap-production-closeout`

Do not run overlapping implementation phases against the same files. Each phase
PR updates this control plane and lands before the next dependent phase starts.

## Live Pull Request Ledger

Live state inspected 2026-07-27.

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
| [#52](https://github.com/agentstation/starmap/pull/52) | `codex/catalog-workspace-lifecycle@35ff2f6a` | `IN_PROGRESS` | P3 human-workspace lifecycle and explicit legacy-layout migration | Parent passed local verification and hosted Security & Reliability; hosted Verification Gate exposed F-049, whose repair commit passes exact local verification and must still pass the final ledger-head local gate plus both hosted gates before the protected merge pause |

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
| P3 | `IN_PROGRESS` | One human provider-YAML workspace has deterministic lifecycle | P3.6a/P3.6b/P3.8 first establish one durable commit point, atomic repairable projection, and store-only operation |
| P4 | `DONE` | One authority/provenance implementation is resilient to drift | Authority, presence, quarantine, degradation, and fuzz gates |
| P5 | `PENDING` | One persisted provider model produces immutable read views | No persisted duplicate schema; read DX and benchmarks green |
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
| P3.10 | `IN_PROGRESS` | Prove migration, restart, and downgrade behavior | Existing machine-layout fixtures are detected before mutation and explicitly migrated or rejected transactionally; restart is identical; unknown newer schema and older binary fail before mutation |

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
| P5.1 | `PENDING` | Keep provider YAML/payload canonical | No persisted definition/offering tree or top-level duplicate collection exists |
| P5.2 | `PENDING` | Internalize build projection | No exported “legacy migration” vocabulary remains before launch |
| P5.3 | `PENDING` | Derive offerings | Same ID at two providers retains distinct exact price, limits, availability, endpoint, and lifecycle |
| P5.4 | `PENDING` | Derive provider-independent definitions | Conflicts use the same authority/evidence implementation, never map iteration or alphabetical first-wins |
| P5.5 | `PENDING` | Derive author membership | Author queries remain equivalent without loading, writing, embedding, or storing author model copies |
| P5.6 | `PENDING` | Remove invented facts | Unknown availability/lifecycle remains unknown; migration/build does not invent “available” or “active” |
| P5.7 | `PENDING` | Preserve immutable catalog DX | Consumer compile example, mutation isolation, concurrent publication, and `BenchmarkClientCatalog` at 0 allocs/op and no more than 10 µs/op pass |
| P5.8 | `PENDING` | Remove prelaunch compatibility | Alias/deprecated types and schema readers with no named external consumer are deleted |

## P6 — Deepen Go Library Composition

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P6.1 | `PENDING` | Map the package graph | Each public package has a named consumer and role; import cycles remain zero; growth from the protected-main baseline of 89 package directories has explicit rationale |
| P6.2 | `PENDING` | Keep read-only consumption small | Invert the `pkg/sources` → internal provider-client edge behind an injected factory; a `starmap.New().Catalog()` consumer stays within the numeric P2.6 dependency budget and its compile closure contains no GenAI, gRPC, OpenTelemetry, WebSocket, SQLite, Cobra, scheduler, or server implementation; a CI dependency-closure assertion enforces the budget so regression fails the verification gate |
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
| F-011 | `PENDING` | Definitions/offerings are layered as runtime legacy migration vocabulary | P5.1–P5.4 |
| F-012 | `PENDING` | Author model tree and payload duplicate provider models | P5.5 |
| F-013 | `PENDING` | Derived offerings invent available/active state | P5.6 |
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
| F-032 | `PENDING` | Reassigning `~/.starmap/catalog` from machine generations to human YAML lacks safe legacy-layout detection and migration | P3.1, P3.10 |
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
| F-049 | `IN_PROGRESS` | Hosted race verification exhausted Go's implicit 10-minute per-package timeout while unrelated migration/rollback fixtures repeatedly decoded and projected the full embedded catalog; focused fixtures now use the smallest catalog that proves their contract, tests that do not exercise construction use direct clients, and repository race verification has an explicit 20-minute package ceiling within the unchanged 45-minute job | P3.10 |

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
| `catalog-workspace-lifecycle` on `codex/catalog-workspace-lifecycle@60f0cd3c` | `IN_PROGRESS` | Complete the remaining P3 lifecycle on a fresh phase branch, then pass exact local/hosted gates and remove the worktree/branch |

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

## Final Definition of Done

The goal is complete only when:

- every phase and task is terminal;
- every finding is `DONE`, user-accepted `REJECTED`, or correctly
  `SUPERSEDED`;
- provider YAML is the only human model representation;
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
