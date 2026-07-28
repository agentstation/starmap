# Starmap Architecture Execution Control Plane

Last updated: 2026-07-27

Status: `IN_PROGRESS` — P0–P2 are complete. P3.6a and P3.8 now establish
generation-store-first publication and store-only operation; P3.6b owns the
atomic, digest-repairable YAML projection before authority/resilience
implementation begins.

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
  `/Users/jack/src/github.com/agentstation/starmap-worktrees/catalog-publication-hotfix`
- Branch:
  `codex/catalog-publication-hotfix`
- Base:
  `origin/main@f8973be3a6f25960efb786b7620a8c7975cfbf1d`

This worktree contains the P3.6a/P3.6b/P3.8 commit-point, atomic-projection,
and store-only hotfix. It was created from the exact protected main produced
by merged PR #49 after the P2 worktree and branches were removed.

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
| P4 | `PENDING` | One authority/provenance implementation is resilient to drift | Authority, presence, quarantine, degradation, and fuzz gates |
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
| P3.1 | `PENDING` | Restore one catalog path safely | One configured path names the human YAML tree; before mutation Starmap detects the pre-plan machine layout (`current`, `generations/`, `.commit.lock`) and returns a typed error that names the required migration unless an explicit transactional migration was selected |
| P3.2 | `PENDING` | Separate machine state | Locks, staging, generations, and caches are machine-owned and cannot be mistaken for override configuration |
| P3.3 | `PENDING` | Make first-run seed atomic | Missing workspace becomes one complete embedded-seeded tree or remains absent after failure |
| P3.4 | `PENDING` | Reconcile embedded E1→E2 after P4 | New embedded revision updates unchanged embedded-derived fields, fills gaps, and preserves actual human edits using the completed P4 authority/provenance model |
| P3.5 | `PENDING` | Detect semantic human edits after P4 | Only changed semantic paths become local evidence under the completed P4 model; formatting-only changes do not |
| P3.6a | `DONE` | Establish one durable commit point before P4 | Generation-store CAS is the sole commit point; post-commit YAML failure returns an observable `pending_repair` projection result without rolling back the store or immutable in-memory catalog; commit failure still publishes neither |
| P3.6b | `IN_PROGRESS` | Make YAML projection atomic and repairable before P4 | YAML is staged, validated, fsynced, input-digest checked, and atomically projected only after commit; startup compares digests and repairs an interrupted or stale projection without republishing the generation |
| P3.7 | `PENDING` | Add multi-process writer control | Two processes cannot interleave; loser receives typed busy/conflict; readers remain available |
| P3.8 | `DONE` | Preserve store-only use | `TestStoreOnlyApplyCommitsWithoutWorkspaceAccess` proves a configured catalog store commits and publishes with a nil projection and no workspace path or filesystem operation |
| P3.9 | `PENDING` | Define reload and rollback | No implicit watcher; explicit reload publishes once; rollback restores exact YAML semantics, provenance, digest, and reads |
| P3.10 | `PENDING` | Prove migration, restart, and downgrade behavior | Existing machine-layout fixtures are detected before mutation and explicitly migrated or rejected transactionally; restart is identical; unknown newer schema and older binary fail before mutation |

## P4 — Consolidate Authority, Provenance, and Source Resilience

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P4.1 | `PENDING` | Replace duplicate authority policies | One executable field table selects all winners; the shadow `complexModelStructures` policy and unused parallel inventory are deleted |
| P4.2 | `PENDING` | Enforce provider pricing authority | Valid provider price wins atomically for its offering; invalid price records durable rejection evidence and falls back; rejection evidence remains when every candidate is invalid |
| P4.3 | `PENDING` | Make local data fallback | Dynamic valid facts beat local for discoverable fields; manual missing data and operator configuration survive |
| P4.4 | `PENDING` | Scope evidence by provider/model | Shared model IDs cannot collide in price, limit, availability, or lifecycle provenance |
| P4.5 | `PENDING` | Preserve source identity through YAML | Reloading generated YAML does not relabel unchanged provider/models.dev/embedded values as local |
| P4.6 | `PENDING` | Model presence explicitly | Tri-state or equivalent typed representation makes missing, explicit false, explicit zero, empty, and unknown round-trip distinctly for limits, features, and other affected fields |
| P4.7 | `PENDING` | Consume observation health and make absence non-authoritative | Reconciliation consumes source status, completeness, issues, and volume history; complete omission, partial response, timeout, fetch failure, and suspicious volume collapse cannot hard-delete or retire |
| P4.8 | `PENDING` | Quarantine records independently | Every P2.4-characterized whole-collection decode site (models.dev envelope, provider list responses, local YAML walk, stored payload) isolates a malformed record while valid siblings survive; collection envelope remains bounded |
| P4.9 | `PENDING` | Make strict mode truthful | Every required source must be `Complete` and `Succeeded`; missing credentials, degraded/skipped state, stale fallback, or empty results without explicit issues fail before publication |
| P4.10 | `PENDING` | Test policy and fuzz untrusted decoders | Authority/presence pass deterministic table and property tests; provider envelopes and provenance decoding pass bounded fuzz corpora without panic |

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
| F-001 | `PENDING` | Current behavior treats editable YAML as a secondary export after durable current exists, contrary to the selected human-workspace contract | P3.1–P3.5 |
| F-002 | `DONE` | Store-only apply skips YAML entirely, commits the generation, and publishes the immutable catalog | P3.8 |
| F-003 | `PENDING` | YAML replacement is destructive and non-atomic | P3.6b |
| F-004 | `PENDING` | Embedded/local structural merge loses or rejects valid manual data | P3.4–P3.5 |
| F-005 | `PENDING` | Provider omission/degradation can remove last-known-good models | P4.7 |
| F-006 | `PENDING` | Two authority policy implementations disagree | P4.1–P4.3 |
| F-007 | `PENDING` | Provider-scoped provenance can collide on bare model ID | P4.4 |
| F-008 | `PENDING` | Zero/false/unknown/absent are not consistently distinguishable | P4.6 |
| F-009 | `PENDING` | Whole-collection decode defeats record quarantine | P4.8 |
| F-010 | `PENDING` | “Require all sources” does not require successful complete sources | P4.9 |
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
| F-031 | `PENDING` | Sync saves YAML before the durable generation commit, making a fragile projection gate the durable product | P3.6a–P3.6b |
| F-032 | `PENDING` | Reassigning `~/.starmap/catalog` from machine generations to human YAML lacks safe legacy-layout detection and migration | P3.1, P3.10 |
| F-033 | `PENDING` | Reconciliation/pipeline paths do not consistently consume observation status, completeness, and issues | P4.7, P4.9 |
| F-034 | `PENDING` | Publication generations can reorder; current event identity is timestamp-based or provider-ambiguous | P7.2, P7.4 |
| F-035 | `PENDING` | Server/background shutdown lacks owned joins; stopped or blocked subscriptions can hang | P7.5, P7.10 |
| F-036 | `PENDING` | Hook overload can drop a whole generation and the counter is not an adequate operational contract | P7.2, P7.11 |
| F-037 | `PENDING` | Historical F-099/F-105/F-106 release findings lack terminal mapping in the new plan | P0.5, P11.9 |
| F-038 | `PENDING` | Hour-scale publication gaps require SSE heartbeats; without them intermediaries reap idle streams, half-open clients linger, and polling/stream health cannot be determined reliably | P7.3, P7.5, P7.8, P7.10–P7.11 |
| F-039 | `DONE` | The plan PR's original dependency graph had reachable GO-2026-5970 (`x/text v0.38.0`) and GO-2026-6061 (`grpc v1.82.0`); replacement #46 resolved both and rebased PR #45 passed exact local and hosted proof | P0.5, P1.1–P1.2 |
| F-040 | `DONE` | Dependabot PR #44 updated reviewed action pins without updating the exact structural assertions in `internal/ciworkflow`; replacement PR #47 updated both and passed exact local and hosted verification | P1.3–P1.4 |
| F-041 | `PENDING` | `make verify` labels its provider listing credential-free but inherits ambient cloud SDK state; the P1 run reported Google Vertex `Configured` from the developer's ADC despite unset provider API-key variables | P10.1, P10.6 |

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
| `catalog-publication-hotfix` on `codex/catalog-publication-hotfix@f8973be3` | `IN_PROGRESS` | Complete P3.6a/P3.6b/P3.8, merge its exact green phase PR, then remove the worktree/branch |

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
