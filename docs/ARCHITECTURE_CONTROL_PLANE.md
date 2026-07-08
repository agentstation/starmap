# Starmap Architecture Control Plane

Last updated: 2026-07-08

This document is the durable control plane for the Starmap architecture hardening work. It is written so a future agent can resume after compaction without relying on chat history.

## Mission

Make Starmap best architected, structured, and implemented for its purpose: a unified AI model catalog available as a Go package, CLI, and server. The architectural direction is modularity through deep modules, quality through clear interfaces, testability through stable seams, and robustness through explicit concurrency, persistence, and failure semantics.

## Current Work

Repository: `/Users/jack/src/github.com/agentstation/starmap`

Current baseline already includes fixes from the prior architecture review:

- `CLAUDE.md` is a symlink to `AGENTS.md`.
- `Client.Save` no longer returns an error after successful catalog saves.
- Sync save updates in-memory catalog state only after persistence succeeds.
- `internal/transport.Client.Do` preserves request context.
- Server stats use server start time wired through handlers.
- Reconciliation conflict resolution is resource-aware for provider authority rules.
- Catalog provider/author/model copy boundaries were strengthened.

Known unrelated dirty files that predated this architecture pass and must not be reverted without explicit user approval:

- `cmd/starmap/app/execute.go`
- `cmd/starmap/main.go`
- `docs/CLI.md`
- `internal/cmd/globals/globals.go`

Architecture review report generated outside the repo:

- `/var/folders/kw/d608x5pn4cq73rz78ztl92cw0000gn/T/architecture-review-starmap-20260708-092014.html`

## Status Legend

- `DONE`: Acceptance criteria satisfied and verified.
- `IN_PROGRESS`: Active implementation or review is underway.
- `PENDING`: Not started.
- `BLOCKED`: Cannot proceed without user input or external state.
- `DEFERRED`: Explicitly postponed because a higher-priority phase must land first.

## Ledger

| ID | Phase | Status | Scope | Acceptance Gate |
| --- | --- | --- | --- | --- |
| P0 | Control plane and goal setup | DONE | Create this document, review it, set active goal, start execution | This file exists, has a ledger, execution log, acceptance criteria, and `/goal` prompt |
| P1 | Deepen the catalog sync pipeline | DONE | Concentrate sync ordering, dependency resolution, source fetch, reconcile, persistence, and hook delivery behind one deep module | Focused sync tests plus `go test ./...`, `go test ./... -race -short`, `go vet ./...` |
| P2 | Put provider fetching behind one source seam | DONE | Move provider client construction, credential loading, bounded concurrency, and provider fetch error policy behind a single provider source module | Provider source has unit tests with fake adapters, no unbounded goroutine fan-out, and public `pkg/sources.ProviderFetcher` retains default provider clients while exposing injectable seams |
| P3 | Make catalog ownership explicit | DONE | Separate immutable catalog snapshot behavior from mutable catalog store behavior | Copy semantics are centralized, collection mutation/read tests pass under `-race`, public contracts document ownership |
| P4 | Extract catalog query modules for CLI and HTTP | DONE | Share list/detail/search/sort/pagination behavior between CLI and server adapters | Query module tests cover behavior; handlers and commands are thin adapters; HTTP handler coverage materially improves |
| P5 | Make reconciliation field rules first-class | DONE | Replace scattered string lists/reflection authority drift with one field rule catalog | Rule table drives merge iteration, authority lookup, and provenance names; existing reconciler tests pass |
| P6 | Unify real-time event fan-out | DONE | Put broker, SSE, and WebSocket fan-out behind one event stream module with explicit backpressure policy | Delivery adapters share policy, metrics are comparable, no unbounded per-event goroutine growth |
| P7 | Documentation and enterprise gates | DONE | Update architecture docs and validate final state | `docs/ARCHITECTURE.md` reflects implemented architecture and all final verification commands pass |

## Execution Log

| Time | Entry |
| --- | --- |
| 2026-07-08 09:20 America/Chicago | Full architecture report generated as temp HTML. Top recommendation: deepen the catalog sync pipeline. |
| 2026-07-08 09:30 America/Chicago | Control plane started in `docs/ARCHITECTURE_CONTROL_PLANE.md`. |
| 2026-07-08 09:35 America/Chicago | Control plane reviewed for scope, ordering, acceptance gates, unrelated dirty-file protection, and compaction survivability. |
| 2026-07-08 09:37 America/Chicago | Active goal created from the `/goal` prompt. P1 started with characterization tests before refactoring sync internals. |
| 2026-07-08 09:42 America/Chicago | P1 characterization tests added for source filtering, source config, dependency resolution, fetch/cleanup error joining, context cancellation, and dry-run publication behavior. `go test .` and `go test ./...` passed. |
| 2026-07-08 09:55 America/Chicago | `internal/catalog/pipeline` introduced. `client.Sync` now delegates to the pipeline store adapter; source construction, dependency resolution, cleanup, fetch, reconciliation, dry-run, no-change, and force-save policy are tested at the pipeline seam. `go test ./internal/catalog/pipeline ./pkg/sync .`, `go test ./internal/sources/providers ./pkg/reconciler ./pkg/sync`, and `go test ./...` passed. |
| 2026-07-08 10:04 America/Chicago | P1 documentation updated in `docs/ARCHITECTURE.md`. Acceptance gates passed: `go test .`, `go test ./internal/sources/providers ./pkg/reconciler ./pkg/sync`, `go test ./...`, `go test ./... -race -short`, `go vet ./...`, and `git diff --check`. Public `pkg/*` to `internal/*` imports remain only as known pre-existing seams for P2/P3. |
| 2026-07-08 10:09 America/Chicago | P2 started. Baseline provider source and public provider fetcher coverage is 0.0%. Current design has unbounded provider goroutine fan-out and `pkg/sources` importing `internal/sources/clients`. |
| 2026-07-08 10:25 America/Chicago | P2 provider seam implemented. `internal/sources/providers` now accepts fake client factories and bounds provider fetch concurrency. `pkg/sources.ProviderFetcher` uses public `ProviderClientFactory`/`ProviderRawFetcher` seams. Focused coverage: `internal/sources/providers` 77.7%, `pkg/sources` 36.5%. `go test ./...` and `go vet ./...` passed. |
| 2026-07-08 10:29 America/Chicago | P2 acceptance gates passed: `go test ./internal/sources/providers ./internal/sources/providers/openai ./internal/sources/providers/anthropic`, `go test ./pkg/sources`, `go test ./...`, `go test ./... -race -short`, `go vet ./...`, and `git diff --check`. |
| 2026-07-08 10:39 America/Chicago | P3 endpoint ownership leak fixed. `Endpoints` now copies on map initialization, set/add, batch writes, get, map, and callback iteration. Architecture docs now state the catalog ownership contract. `go test ./pkg/catalogs`, `go test ./pkg/catalogs -race`, and `go test ./...` passed. |
| 2026-07-08 10:42 America/Chicago | P3 acceptance gates passed: `go test ./pkg/catalogs -race`, `go test ./pkg/catalogs`, `go test ./...`, `go vet ./...`, and `git diff --check`. |
| 2026-07-08 10:43 America/Chicago | P4 started with query behavior inventory across HTTP handlers and CLI commands. |
| 2026-07-08 10:55 America/Chicago | P4 shared query module added at `internal/catalog/query`. CLI model/provider/author list commands and HTTP model/provider list handlers now use shared query/pagination behavior. Handler adapter tests added; `internal/server/handlers` coverage increased from 0.0% to 15.0%. Gates passed: `go test ./internal/catalog/query ./internal/server/handlers ./internal/server/filter ./cmd/starmap/cmd/models ./cmd/starmap/cmd/providers ./cmd/starmap/cmd/authors`, `go test ./...`, `go vet ./...`, and `git diff --check`. |
| 2026-07-08 10:56 America/Chicago | P5 started with reconciliation field-rule inventory. |
| 2026-07-08 11:12 America/Chicago | P5 field-rule catalog added in `pkg/reconciler`. Model/provider/author field paths now live in one rule table; model/provider merge loops use rules for reflection, authority lookup, and provenance names. Complex model provenance names now route through model provenance rules. Author authority paths were corrected from stale `URL` entries to current author fields such as `Website`, and provider `Models` authority is explicit. Focused gate passed: `go test ./pkg/authority ./pkg/reconciler`. |
| 2026-07-08 11:14 America/Chicago | P6 started with real-time event fan-out inventory across broker, SSE, and WebSocket packages. |
| 2026-07-08 11:29 America/Chicago | P6 shared event fan-out implemented in `internal/server/events`. Broker, SSE, and WebSocket delivery now use `Fanout` with explicit skip/disconnect policy and cumulative delivery counters. Broker no longer starts one goroutine per subscriber per event. SSE keeps slow clients and records skipped deliveries; WebSocket disconnects slow clients and records disconnects. Admin stats expose broker, SSE, and WebSocket delivery counters. Gates passed: `go test ./internal/server/events ./internal/server/events/adapters ./internal/server/sse ./internal/server/websocket`, `go test ./internal/server -race`, `go test ./...`, and `go vet ./...`. |
| 2026-07-08 11:32 America/Chicago | P7 started with architecture docs updated for reconciler field rules and real-time event fan-out. Final generated-doc, import, and verification gates are underway. |
| 2026-07-08 11:49 America/Chicago | P7 complete. Generated package docs were refreshed and `make docs-check` passes, including a newly generated `internal/embedded/openapi/README.md` because docs-check expects every package with `generate.go` to have one. Final gates passed: `go test ./...`, `go test ./... -race -short`, `go vet ./...`, `git diff --check`, and `make docs-check`. |
| 2026-07-08 12:08 America/Chicago | Autoreview found two valid issues: public `pkg/sources.ProviderFetcher` lost its default clients, and broker fan-out could be stalled by one blocking subscriber. Fix in progress: restore default provider hooks and move broker subscribers behind bounded per-subscriber queues. |
| 2026-07-08 12:24 America/Chicago | Autoreview fixes implemented. `pkg/sources.ProviderFetcher` now preserves default provider client/raw-fetch behavior while retaining injectable test seams. Broker subscribers now use bounded per-subscriber queues/workers so one slow subscriber cannot stall broker control-plane operations. Generated docs were normalized for idempotent `make generate`/`make docs-check` behavior. Gates passed: `go test ./pkg/sources`, `go test ./internal/server/events`, `go test ./internal/server/events -race`, `go test ./...`, `go vet ./...`, `go test ./... -race -short`, `git diff --check`, and `make docs-check`. Second autoreview run is pending. |
| 2026-07-08 12:38 America/Chicago | Second autoreview found one valid issue: shared model query accepted `ModelOptions.Provider` but did not apply it. Fix implemented with `query.ProviderModelIndex` built from provider model membership and aliases; CLI model listing and HTTP model list/search now prefilter by provider before applying flattened model filters. Regression coverage added for provider IDs, aliases, unknown providers, missing index fail-closed behavior, and HTTP provider filtering. Gates passed: `go test ./internal/catalog/query ./internal/server/handlers ./cmd/starmap/cmd/models`, `go test ./...`, `go vet ./...`, `make docs-check`, `git diff --check`, and `go test ./... -race -short`. Final autoreview rerun is pending. |
| 2026-07-08 12:43 America/Chicago | Final autoreview rerun passed cleanly with no accepted/actionable findings. Autoreview parallel gate `go test ./... && go vet ./...` exited 0. |
| 2026-07-08 12:50 America/Chicago | Naming cleanup completed after architecture review. `internal/catalogquery` moved to `internal/catalog/query` with package name `query`. `internal/syncpipeline` moved to `internal/catalog/pipeline` with package name `pipeline`, `Pipeline.Sync`, and root `pipelineStore`. Architecture docs now show catalog-scoped internal query and pipeline packages. Verification is in progress. |

## Global Constraints

- Preserve public Go package compatibility unless a breaking change is explicitly justified and approved.
- Keep edits scoped to the active phase.
- Do not revert unrelated dirty files.
- Prefer idiomatic Go: small concrete types, interfaces at use sites, explicit errors, context propagation, bounded concurrency, and table-driven tests.
- Use structured tests at module seams. Avoid tests that only assert implementation details unless the module is intentionally internal.
- Run focused tests after each phase and the full gates before marking a phase `DONE`.
- Keep `docs/ARCHITECTURE_CONTROL_PLANE.md` current after every meaningful change.

## Phase P0: Control Plane and Goal Setup

### Tasks

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P0.1 | Write control-plane document | DONE | This document includes mission, current work, ledger, execution log, phase acceptance criteria, and `/goal` prompt |
| P0.2 | Review control-plane scope | DONE | Plan explicitly preserves unrelated dirty files, orders high-leverage work first, and has verification gates |
| P0.3 | Create active goal | DONE | Active goal exists and references this file as the source of truth |
| P0.4 | Start implementation | DONE | First implementation phase has at least one scoped code/test change or a deliberate narrower design step logged |

### Acceptance Gate

- `docs/ARCHITECTURE_CONTROL_PLANE.md` exists.
- Active `/goal` has been created from the prompt in this document.
- P1 has been started or explicitly prepared with a concrete design step.

## Phase P1: Deepen the Catalog Sync Pipeline

### Problem

`Client.Sync` currently exposes a small method but hides correctness across many root-package functions: option validation, source construction, dependency resolution, concurrent fetch, reconciliation, save, logo copying, in-memory publication, and hook delivery. Coverage confirms this seam is weak: `sync.go`, `update.go`, and `lifecycle.go` showed 0% coverage for the main orchestration path in the architecture review coverage run.

### Target Shape

Create a deep sync pipeline module behind `starmap.Client.Sync`. The root client should become an adapter that provides catalog access, state publication, and hooks, while the pipeline owns execution ordering and failure semantics.

Candidate module shape:

- `internal/catalog/pipeline` for implementation details that may import internal dependency/source helpers.
- A small pipeline execution type used by `client.Sync`.
- Test fakes for sources, dependency resolution, reconciliation, and persistence decisions.

### Tasks

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P1.1 | Characterize current sync behavior | DONE | Tests capture option validation, source fetch error handling, dry-run behavior, save ordering, and forced save behavior |
| P1.2 | Introduce pipeline execution type | DONE | `client.Sync` delegates to a catalog pipeline without changing public method signature |
| P1.3 | Move fetch/cleanup/dependency orchestration | DONE | Pipeline owns context timeout, dependency filtering, fetch fan-out, cleanup, and error joining |
| P1.4 | Move persistence decision policy | DONE | Save decision and publication ordering are testable without hitting real provider APIs |
| P1.5 | Update tests and docs | DONE | Focused sync tests exist; architecture doc notes the pipeline module |

### Acceptance Gate

- `go test .`
- `go test ./internal/sources/providers ./pkg/reconciler ./pkg/sync`
- `go test ./...`
- `go test ./... -race -short`
- `go vet ./...`
- No new public import from `pkg/*` to `internal/*` is introduced.

## Phase P2: Put Provider Fetching Behind One Source Seam

### Problem

Provider fetching is split across public `pkg/sources`, internal client factory, provider source concurrency, and provider-specific clients. Public `pkg/sources` currently imports `internal/sources/clients`, which weakens the advertised package layering. Provider source fetch uses one goroutine per provider and has no direct package tests in the coverage run.

### Target Shape

Provider fetching should have one deep module that owns:

- Client adapter registry.
- Credential loading policy.
- Bounded concurrency.
- Per-provider error classification.
- Test fakes for adapter behavior.

### Tasks

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P2.1 | Add provider source characterization tests | DONE | Tests cover skipped credentials, config errors, successful fetches, and partial failures |
| P2.2 | Introduce provider client registry seam | DONE | Provider source can use fake adapters in tests without real HTTP or environment credentials |
| P2.3 | Bound provider concurrency | DONE | Fetch concurrency is capped and configurable or constant-backed |
| P2.4 | Preserve public provider fetcher defaults with injectable seams | DONE | `NewProviderFetcher` works with default clients and callers can still override client/raw fetcher factories |
| P2.5 | Update docs | DONE | Provider source seam is described in architecture docs |

### Acceptance Gate

- `go test ./internal/sources/providers ./internal/sources/providers/openai ./internal/sources/providers/anthropic`
- `go test ./pkg/sources`
- `go test ./...`
- `go test ./... -race -short`
- `go vet ./...`
- `go test ./pkg/sources` confirms public provider fetcher defaults and injectable seams.

## Phase P3: Make Catalog Ownership Explicit

### Problem

The catalog module is central and mostly well covered, but ownership semantics have historically depended on each method remembering to deep-copy. This creates a broad interface and many places for pointer alias bugs.

### Target Shape

Separate the catalog store from read snapshots. Reads return immutable snapshots or owned values. Writes enter through explicit mutation methods. Copy semantics live in one place.

### Tasks

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P3.1 | Document current ownership contract | DONE | Public docs state which values are owned by caller vs catalog |
| P3.2 | Centralize copy semantics | DONE | Store/snapshot seam owns copy behavior; collection methods do not duplicate deep-copy rules unnecessarily |
| P3.3 | Strengthen race and alias tests | DONE | Tests mutate returned nested fields and prove catalog internals remain unchanged |
| P3.4 | Audit endpoints/provenance ownership | DONE | Provider, author, model, endpoint, and provenance read/write paths have consistent ownership semantics |

### Acceptance Gate

- `go test ./pkg/catalogs -race`
- `go test ./pkg/catalogs`
- `go test ./...`
- `go vet ./...`

## Phase P4: Extract Catalog Query Modules for CLI and HTTP

### Problem

CLI and HTTP routes both query models/providers/authors, but behavior is split across server filters, command filters, table formatting, and handler map construction. Handler coverage is currently 0%.

### Target Shape

Create catalog query modules for model/provider/author list/detail/search behavior. CLI and HTTP remain adapters for parsing inputs and formatting outputs.

### Tasks

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P4.1 | Inventory duplicate query rules | DONE | Model/provider/author query behavior is mapped across CLI and HTTP |
| P4.2 | Create query result types | DONE | Query results are typed and reusable by HTTP and CLI adapters |
| P4.3 | Move model query behavior | DONE | List/search/sort/pagination tests pass without HTTP or cobra |
| P4.4 | Move provider and author query behavior | DONE | Detail/list behavior is shared by CLI and HTTP where practical |
| P4.5 | Add handler adapter tests | DONE | HTTP handlers test request/response translation and cache use, not query internals |

### Acceptance Gate

- `go test ./internal/server/handlers ./internal/server/filter`
- `go test ./cmd/starmap/cmd/models ./cmd/starmap/cmd/providers ./cmd/starmap/cmd/authors`
- `go test ./...`
- `go vet ./...`

## Phase P5: Make Reconciliation Field Rules First-Class

### Problem

Reconciliation is one of the better-tested modules, but field identity appears in several places: hardcoded merge field lists, authority path strings, reflection get/set paths, and provenance names. That creates drift risk.

### Target Shape

Create a field rule catalog that drives merge iteration, authority lookup, provenance naming, and reflect access. Reflection can remain an internal implementation detail.

### Tasks

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P5.1 | Inventory field paths | DONE | Model/provider/author field paths are listed once with resource type and authority path |
| P5.2 | Introduce field rule module | DONE | Merger iterates field rules instead of local string slices |
| P5.3 | Align authority and provenance names | DONE | Tests prove provider/model/author field rules resolve expected authority |
| P5.4 | Preserve existing reconciliation behavior | DONE | Existing reconciler tests pass unchanged or with intentional expectation updates |

### Acceptance Gate

- `go test ./pkg/authority ./pkg/reconciler`
- `go test ./...`
- `go vet ./...`

## Phase P6: Unify Real-Time Event Fan-Out

### Problem

Broker, SSE broadcaster, and WebSocket hub each implement fan-out and backpressure differently. The broker spawns a goroutine per subscriber per event; SSE skips on full client buffer; WebSocket disconnects on full client buffer. Enterprise behavior should be explicit and tested.

### Target Shape

Create one event stream module that owns backpressure policy, counters, and fan-out concurrency. SSE and WebSocket become adapters.

### Tasks

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P6.1 | Document current delivery policies | DONE | Existing drop/skip/disconnect behavior is captured in tests or notes |
| P6.2 | Introduce event stream module | DONE | One module owns fan-out, counters, and backpressure policy |
| P6.3 | Adapt SSE and WebSocket | DONE | SSE and WebSocket delivery use shared policy while preserving endpoint behavior |
| P6.4 | Bound goroutine growth | DONE | Per-event fan-out cannot create unbounded goroutines |

### Acceptance Gate

- `go test ./internal/server/events ./internal/server/events/adapters`
- `go test ./internal/server/sse ./internal/server/websocket`
- `go test ./internal/server -race`
- `go test ./...`
- `go vet ./...`

## Phase P7: Documentation and Enterprise Gates

### Tasks

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P7.1 | Update architecture docs | DONE | `docs/ARCHITECTURE.md` reflects implemented modules and concurrency patterns |
| P7.2 | Update package docs if affected | DONE | Generated or hand-written package docs match new public contracts |
| P7.3 | Final import-direction audit | DONE | Public package imports are intentional and documented |
| P7.4 | Final verification | DONE | Full test, race, vet, and diff checks pass |

### Acceptance Gate

- `go test ./...`
- `go test ./... -race -short`
- `go vet ./...`
- `git diff --check`
- `go list` import audit reviewed.

## `/goal` Prompt

Use this prompt to drive autonomous execution:

```text
/goal Execute /Users/jack/src/github.com/agentstation/starmap/docs/ARCHITECTURE_CONTROL_PLANE.md end to end. Treat the ledger, execution log, constraints, and acceptance criteria in that file as the source of truth. Work phases in order unless a blocker makes a later independent phase clearly safer. After every meaningful implementation step, update the ledger and execution log in the control plane. Do not revert unrelated pre-existing edits in cmd/starmap/app/execute.go, cmd/starmap/main.go, docs/CLI.md, or internal/cmd/globals/globals.go. Preserve public Go compatibility unless explicitly approved. Keep modules idiomatic Go: small interfaces at use sites, concrete types by default, typed errors, context propagation, bounded concurrency, table-driven tests, and race-safe ownership. Continue autonomously through implementation, focused tests, full tests, race tests, vet, docs, and final verification until all phases are DONE or a real blocker is recorded with evidence.
```

## Plan Review Checklist

- Highest-risk, highest-leverage work is first: sync pipeline before provider source, catalog ownership, query adapters, field rules, and event fan-out.
- Every phase has concrete files, tasks, and command gates.
- The plan preserves current uncommitted architecture fixes.
- The plan explicitly preserves unrelated dirty files.
- The plan has compaction-safe current state and execution log.
- The plan has a literal `/goal` prompt and can be used as an autonomous control plane.
