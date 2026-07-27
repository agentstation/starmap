# Starmap Architecture Execution Control Plane

Last updated: 2026-07-27

Status: `IN_PROGRESS` — control plane authored; implementation has not started.

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
- Reviewed protected-main baseline:
  `9508ee7866e4683e001e7ad153319d348433045d`
- Historical pre-control-plane comparison:
  `3787d7164433f2fcb713a2d81e0cb653f9df6be5`
- Historical production catalog ledger:
  [`STARPORT_CATALOG_CONTROL_PLANE.md`](STARPORT_CATALOG_CONTROL_PLANE.md)

The historical production ledger remains evidence; this plan supersedes its
prescriptive architecture where the new review found a conflict. Historical
claims must not be silently rewritten.

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
   describe.
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
  post-commit server events through SSE by default, verifies and atomically
  activates immutable generations, reconnects with gap recovery, and polls only
  as an explicit fallback;
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
contract. The goal is complete only when every phase/task/finding is terminal,
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
                +-------------+--------------+
                |                            |
       atomic YAML workspace         immutable generation
          materialization              store / CAS
                |                            |
                +-------------+--------------+
                              |
                   atomic in-memory Catalog
                         /          \
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

SSE is the canonical remote notification transport because catalog publication
is server-to-client. WebSocket remains only if a named bidirectional consumer
justifies the extra adapter.

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
12. expose explicit health/freshness/degraded state;
13. stop promptly when its caller-owned context is canceled; and
14. poll only when streaming is unsupported or explicitly configured as
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
- disconnect, reconnect, duplicate, stale, skipped, corrupt, incompatible, and
  unauthorized remote events/generations;
- package consumer compile tests for local, server, and remote compositions;
- deterministic artifact/release reconstruction;
- failure injection at parse, fetch, validate, stage, fsync, rename, commit,
  callback, stream, fetch, and publication points; and
- fuzzing for provider envelopes, YAML/JSON manifests, payloads, provenance,
  SSE framing/event IDs, and authority presence semantics.

Do not:

- assert implementation details available through the public interface;
- duplicate the same behavior across many fixtures without a new failure class;
- treat coverage percentage as proof;
- add mocks for seams with no real variation; or
- retain tests for deleted compatibility behavior before launch.

## Worktree and Branch Strategy

### Active control-plane worktree

- Worktree:
  `/Users/jack/src/github.com/agentstation/starmap-worktrees/starmap-architecture-control-plane`
- Branch:
  `codex/starmap-architecture-control-plane`
- Base:
  `origin/main@9508ee7866e4683e001e7ad153319d348433045d`

This worktree contains only the durable plan/report archival work. It must
become a small standalone PR.

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
created from the current protected `origin/main`. Suggested branch names:

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
| [#40](https://github.com/agentstation/starmap/pull/40) | `codex/provider-expansion-wave0@a14d2249` | `PENDING` | Supersede and close after donor inventory | Salvage map recorded; no rejected schema work copied; closing comment links this plan; PR closed; remote branch deleted; worktree removed |
| [#43](https://github.com/agentstation/starmap/pull/43) | Dependabot Go modules `@ebd36505` | `PENDING` | Revalidate and merge first | Exact head includes `golang.org/x/text >= v0.39.0`; local and hosted verification/security gates pass; required review satisfied; merged; branch removed |
| [#44](https://github.com/agentstation/starmap/pull/44) | Dependabot Actions `@e1dcd1e6` | `PENDING` | Rebase after #43, re-run, then merge or recreate | Current GO-2026-5970 failure eliminated by rebased dependency graph; workflow structural tests and required hosted checks pass; merged or replaced by one equivalent PR; old PR closed |

Current #44 failure is not caused by the action syntax itself. Both required
jobs ran against `golang.org/x/text v0.38.0`; `govulncheck` reports
GO-2026-5970, fixed in v0.39.0. PR #43 updates it to v0.40.0. The expected
cleanup order is therefore #43, rebase/recreate #44, then #44.

Final PR gate:

```bash
test "$(gh pr list --repo agentstation/starmap --state open --limit 100 \
  --json number --jq 'length')" -eq 0
```

## Phase Ledger

| Phase | Status | Outcome | Gate |
| --- | --- | --- | --- |
| P0 | `IN_PROGRESS` | Durable control plane and architecture report are reviewable | P0 tasks and plan PR green |
| P1 | `PENDING` | Existing PRs and donor work receive terminal dispositions | PR ledger terminal; no lost salvage |
| P2 | `PENDING` | Catalog contract is characterized before structural change | Golden workflows reproduce current behavior and failures |
| P3 | `PENDING` | One human provider-YAML workspace has deterministic lifecycle | Seed/edit/upgrade/restart/rollback suite |
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
| P0.4 | `DONE` | Archive the architecture report in the repository | Repository HTML has a recorded SHA; this file uses a relative link; report parses and renders locally |
| P0.5 | `IN_PROGRESS` | Validate and publish the plan PR | Markdown links, docs check, diff check, required local verification, and hosted checks pass on exact head |

P0 gate:

```bash
test -f docs/STARMAP_ARCHITECTURE_CONTROL_PLANE.md
rg -n '^## `/goal` Prompt|^## Phase Ledger|^## Finding Ledger|^## Evidence Log' \
  docs/STARMAP_ARCHITECTURE_CONTROL_PLANE.md
test -f docs/reviews/STARMAP_ARCHITECTURE_REVIEW_2026-07-27.html
make docs-check
git diff --check
```

## P1 — Reconcile Existing Pull Requests and Donor Work

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P1.1 | `PENDING` | Revalidate PR #43 | Exact-head local verification, race selection, govulncheck, and hosted checks pass |
| P1.2 | `PENDING` | Merge PR #43 | Protected merge succeeds; main contains x/text v0.40.0; PR and branch are closed |
| P1.3 | `PENDING` | Rebase/recreate and verify PR #44 | Exact head is based on post-#43 main; workflow fixture, actionlint, verification, and security checks pass |
| P1.4 | `PENDING` | Merge PR #44 | Protected merge succeeds; old failed head is superseded; PR and branch are closed |
| P1.5 | `PENDING` | Inventory PR #40 | Every changed production module is marked salvage, already-landed, reject, or superseded with rationale |
| P1.6 | `PENDING` | Close PR #40 | Closing note links this plan and inventory; no open review threads are misrepresented as resolved |
| P1.7 | `PENDING` | Remove #40 branch/worktree | Remote branch absent; local branch/worktree absent; any retained evidence is in docs/Git history, not an active worktree |
| P1.8 | `PENDING` | Rebase control-plane/next phase on current main | No dependency/action regression; ledger records final PR SHAs |

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
| P2.1 | `PENDING` | Record domain vocabulary and decisions | Catalog, workspace, observation, generation, offering, definition, publication, and remote subscriber have one documented meaning |
| P2.2 | `PENDING` | Characterize current local/store paths | Tests reproduce store-only sync failure, input/output divergence, embedded write-path leakage, and restart precedence |
| P2.3 | `PENDING` | Characterize reconciliation loss | Tests reproduce local field loss, provider omission pruning, degraded-source replacement, and bare-model provenance collision |
| P2.4 | `PENDING` | Characterize schema resilience | A malformed sibling currently demonstrates whole-collection loss; invalid local YAML demonstrates fail-closed expectation |
| P2.5 | `PENDING` | Characterize server/remote flow | Existing manifest, payload, SSE, WebSocket, callback ordering, drop, and reconnect semantics are recorded by real transport tests |
| P2.6 | `PENDING` | Measure public composition | Record root/catalog/server dependency closures, binary size, allocations, latency, package count, Go LOC, embedded bytes, and file-size inventory |
| P2.7 | `PENDING` | Freeze user journeys | Golden fixtures cover in-process library, CLI workspace, embedded upgrade, embeddable server, and remote reactive consumer |

P2 gate requires red characterization tests for every reproduced defect and
green tests for every invariant already provided by main.

## P3 — Restore One Human Catalog Workspace

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P3.1 | `PENDING` | Restore one catalog path | One configured path names the human YAML tree; normal update reads and writes the same path |
| P3.2 | `PENDING` | Separate machine state | Locks, staging, generations, and caches are machine-owned and cannot be mistaken for override configuration |
| P3.3 | `PENDING` | Make first-run seed atomic | Missing workspace becomes one complete embedded-seeded tree or remains absent after failure |
| P3.4 | `PENDING` | Reconcile embedded E1→E2 | New embedded revision updates unchanged embedded-derived fields, fills gaps, and preserves actual human edits |
| P3.5 | `PENDING` | Detect semantic human edits | Only changed semantic paths become local evidence; formatting-only changes do not |
| P3.6 | `PENDING` | Make materialization atomic | Staged write, validation, fsync, input-digest conflict check, and atomic swap preserve prior workspace on every injected failure |
| P3.7 | `PENDING` | Add multi-process writer control | Two processes cannot interleave; loser receives typed busy/conflict; readers remain available |
| P3.8 | `PENDING` | Preserve store-only use | Catalog-store sync succeeds without a YAML path and performs zero workspace filesystem operations |
| P3.9 | `PENDING` | Define reload and rollback | No implicit watcher; explicit reload publishes once; rollback restores exact YAML semantics, provenance, digest, and reads |
| P3.10 | `PENDING` | Prove restart/downgrade behavior | Restart is identical; unknown newer schema and older binary fail before mutation |

## P4 — Consolidate Authority, Provenance, and Source Resilience

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P4.1 | `PENDING` | Replace duplicate authority policies | One executable field table selects all winners; unused parallel inventory is removed |
| P4.2 | `PENDING` | Enforce provider pricing authority | Valid provider price wins atomically for its offering; invalid price records rejection and falls back |
| P4.3 | `PENDING` | Make local data fallback | Dynamic valid facts beat local for discoverable fields; manual missing data and operator configuration survive |
| P4.4 | `PENDING` | Scope evidence by provider/model | Shared model IDs cannot collide in price, limit, availability, or lifecycle provenance |
| P4.5 | `PENDING` | Preserve source identity through YAML | Reloading generated YAML does not relabel unchanged provider/models.dev/embedded values as local |
| P4.6 | `PENDING` | Model presence explicitly | Missing, explicit false, explicit zero, empty, and unknown round-trip distinctly |
| P4.7 | `PENDING` | Make absence non-authoritative | Complete omission, partial response, timeout, and fetch failure cannot hard-delete or retire |
| P4.8 | `PENDING` | Quarantine records independently | Malformed source record is isolated while valid siblings survive; collection envelope remains bounded |
| P4.9 | `PENDING` | Make strict mode truthful | “Require all” rejects failed, degraded, skipped, or incomplete required observations before publication |
| P4.10 | `PENDING` | Fuzz untrusted policy inputs | Provider envelopes, presence values, provenance keys, and authority selection pass bounded fuzz corpora without panic |

## P5 — Keep One Persisted Model and Derive Read Views

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P5.1 | `PENDING` | Keep provider YAML/payload canonical | No persisted definition/offering tree or top-level duplicate collection exists |
| P5.2 | `PENDING` | Internalize build projection | No exported “legacy migration” vocabulary remains before launch |
| P5.3 | `PENDING` | Derive offerings | Same ID at two providers retains distinct exact price, limits, availability, endpoint, and lifecycle |
| P5.4 | `PENDING` | Derive provider-independent definitions | Conflicts use the same authority/evidence implementation, never map iteration or alphabetical first-wins |
| P5.5 | `PENDING` | Derive author membership | Author queries remain equivalent without loading, writing, embedding, or storing author model copies |
| P5.6 | `PENDING` | Remove invented facts | Unknown availability/lifecycle remains unknown; migration/build does not invent “available” or “active” |
| P5.7 | `PENDING` | Preserve immutable catalog DX | Consumer compile example, mutation isolation, O(1), zero-allocation, and concurrent publication tests pass |
| P5.8 | `PENDING` | Remove prelaunch compatibility | Alias/deprecated types and schema readers with no named external consumer are deleted |

## P6 — Deepen Go Library Composition

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P6.1 | `PENDING` | Map the package graph | Each public package has a named consumer and role; import cycles remain zero; growth from the protected-main baseline of 89 package directories has explicit rationale |
| P6.2 | `PENDING` | Keep read-only consumption small | A program using `starmap.New().Catalog()` does not compile provider cloud SDKs, CLI, scheduler, or server implementations |
| P6.3 | `PENDING` | Move acquisition behind explicit composition | CLI/server that need providers opt in; read-only library behavior remains complete |
| P6.4 | `PENDING` | Narrow interfaces at use sites | Command, source, storage, server, and remote tests use the smallest real seam |
| P6.5 | `PENDING` | Delete hypothetical seams | Unused enhancer, distribution, evidence, registry, and compatibility modules are removed or connected to a named production composition |
| P6.6 | `PENDING` | Validate concrete consumer examples | Separate external test modules compile local library, store-only, server embed, and remote subscriber programs |
| P6.7 | `PENDING` | Measure improvement | Dependency closure, build time, binary size, package count, Go LOC, and public exports are no worse without explicit rationale |

## P7 — Embeddable Server and Reactive Remote Consumer

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P7.1 | `PENDING` | Expose an embeddable server module | External Go fixture constructs, starts, drains, and stops server without importing `internal` or CLI packages |
| P7.2 | `PENDING` | Make publication ordering exact | Durable commit precedes catalog swap, cache invalidation, manifest visibility, and event publication |
| P7.3 | `PENDING` | Select event adapters by use | SSE is canonical; WebSocket remains only with a named bidirectional consumer and independent value |
| P7.4 | `PENDING` | Define stable event identity | Event carries enough generation/sequence identity for dedupe and catch-up but no mutable catalog payload |
| P7.5 | `PENDING` | Implement explicit reactive lifecycle | Caller context owns initial fetch, stream, retry, activation, and shutdown; constructor starts no hidden goroutine |
| P7.6 | `PENDING` | Verify immutable generations | Client rejects wrong media, size, digest, ID, schema, redirect origin, publisher, or stale generation before activation |
| P7.7 | `PENDING` | Recover from stream loss | Reconnect uses bounded backoff/jitter, Last-Event-ID when supported, and mandatory current-manifest catch-up |
| P7.8 | `PENDING` | Make polling last resort | No normal polling occurs while stream is healthy; fallback is explicit, observable, bounded, and tested |
| P7.9 | `PENDING` | Prove concurrent read safety | Readers observe complete old/new generations while stream activates updates under `-race` |
| P7.10 | `PENDING` | Exercise real transport failures | Disconnect, duplicate, out-of-order, skipped, slow, unauthorized, corrupt, incompatible, and shutdown cases pass |
| P7.11 | `PENDING` | Expose production health | Server and subscriber expose generation, freshness, connected/degraded state, retry count, and last error without secrets |

## P8 — Go Modularity, Naming, and Complexity

| Task | Status | Work | Verifiable success criteria |
| --- | --- | --- | --- |
| P8.1 | `PENDING` | Add file-size verification | CI lists >1000, requires reviewed >1500 rationale, and fails every repository-authored file >=2000 |
| P8.2 | `PENDING` | Split current hard-limit test | `pkg/reconciler/merger_test.go` is divided by behavior with no duplicated fixture machinery |
| P8.3 | `PENDING` | Review >1000 production files | Each file is split by concept or receives recorded depth/locality rationale; no unreviewed concern remains |
| P8.4 | `PENDING` | Audit package/file stutter | Every finding is renamed, retained with rationale, or rejected; Go call sites become clearer |
| P8.5 | `PENDING` | Audit pockets of complexity | Cyclomatic/cognitive hot spots are mapped to domain concepts and deepened without pass-through modules |
| P8.6 | `PENDING` | Apply deletion test | Public modules with no production caller and seams with one adapter are removed unless a concrete near-term composition is proven |
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
| P9.6 | `PENDING` | Choose one distribution seam | Remote/server/release flow is wired end-to-end; unused competing protocols are removed |
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
| P11.8 | `PENDING` | Close the ledger | Every phase/task/finding/PR/workspace row is terminal and evidence totals match |

Final machine gate:

```bash
test -z "$(git status --porcelain)"
test "$(git rev-parse main)" = "$(git rev-parse origin/main)"
test "$(git rev-list --left-right --count main...origin/main)" = $'0\t0'
test "$(git worktree list --porcelain | rg '^worktree ' | wc -l | tr -d ' ')" -eq 1
test "$(gh pr list --repo agentstation/starmap --state open --limit 100 \
  --json number --jq 'length')" -eq 0
```

## Finding Ledger

| Finding | Status | Description | Owning task |
| --- | --- | --- | --- |
| F-001 | `PENDING` | Editable YAML is a secondary export after durable current exists | P3.1–P3.5 |
| F-002 | `PENDING` | Store-only sync still attempts YAML save | P3.8 |
| F-003 | `PENDING` | YAML replacement is destructive and non-atomic | P3.6 |
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
| F-026 | `PENDING` | PR #40 implements rejected persisted schema and is too broad | P1.5–P1.7 |
| F-027 | `PENDING` | PR #43 is green but unresolved and contains vulnerability fix | P1.1–P1.2 |
| F-028 | `PENDING` | PR #44 fails on vulnerable pre-#43 dependency graph | P1.3–P1.4 |
| F-029 | `PENDING` | Local main diverges and multiple stale worktrees/branches remain | P11.2–P11.5 |
| F-030 | `PENDING` | Existing architecture docs contain superseded “YAML export” guidance | P10.5 |

## Workspace Ledger

| Workspace/ref | Status | Required terminal state |
| --- | --- | --- |
| Primary `/Users/jack/src/github.com/agentstation/starmap` on divergent `main@3787d716` | `PENDING` | Preserve the unique checkpoint under an explicit archive ref, then leave clean `main == origin/main`; no lost unrecorded work |
| `architecture-review-20260727-clean` detached at `9508ee78` | `PENDING` | Removed after report archival |
| `fresh-catalog-release` on `codex/immutable-release-pipeline@6d4d4c27` | `PENDING` | Verify #39 contains work; remove worktree and obsolete local branch |
| `provider-expansion-wave0` on `a14d2249` | `PENDING` | Inventory, close #40, remove worktree/local/remote branch |
| `starmap-architecture-control-plane` on fresh branch | `IN_PROGRESS` | Plan PR merged, then worktree/branch removed |

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

## Final Definition of Done

The goal is complete only when:

- every phase and task is terminal;
- every finding is `DONE`, user-accepted `REJECTED`, or correctly
  `SUPERSEDED`;
- provider YAML is the only human model representation;
- embedded/local/live/release upgrade semantics pass the lifecycle suite;
- immutable Catalog DX and budgets remain intact;
- provider pricing, source resilience, and provenance pass the authority suite;
- the Go library, embeddable server, and reactive remote consumer compile and
  pass real end-to-end tests;
- SSE push is the normal remote path and polling is only a tested fallback;
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
