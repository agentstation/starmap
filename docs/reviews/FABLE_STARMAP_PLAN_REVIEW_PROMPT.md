# Fable Review Prompt — Starmap Architecture Control Plane

Copy the prompt below into Fable from the repository root.

---

You are reviewing the architecture execution plan for Starmap:

<https://github.com/agentstation/starmap>

Starmap is intended to be the authoritative LLM catalog for Starport, an
open-source OpenRouter-style enterprise gateway written in Go. The desired
outcome is a canonical, production-ready Go package that is also composable as
an embeddable server and usable as a verified reactive remote catalog source.

This is a review task, not an implementation task. Be skeptical. Do not assume
the plan is correct merely because it is detailed or because prior work is
marked complete.

## Read these sources completely, in order

1. `AGENTS.md`
2. `docs/STARMAP_ARCHITECTURE_CONTROL_PLANE.md`
3. `docs/reviews/STARMAP_ARCHITECTURE_REVIEW_2026-07-27.html`
4. `docs/ARCHITECTURE.md`
5. `docs/proof/starport-catalog-control-plane/README.md` (archived proof for
   the completed catalog control plane)
6. `README.md`
7. `go.mod`

Then inspect the implementation and tests referenced by the control plane.
Inspect live PRs #40, #43, and #44 if GitHub access is available. If it is not,
use the exact PR metadata recorded in the control plane and state that
limitation.

The reviewed protected-main baseline is:

`9508ee7866e4683e001e7ad153319d348433045d`

Review the plan against that baseline. Do not review only the prose.

## Primary question

If the plan were executed exactly as written, would the result be a simple,
idiomatic, reliable, production-ready Go package that an enterprise LLM proxy
gateway can trust?

Answer this by reviewing every phase, task, finding, success criterion, and
cleanup gate step by step.

## Required architecture outcome

The intended result is:

- one human-editable provider-oriented YAML catalog under
  `~/.starmap/catalog`;
- no persisted `/definitions`, `/offerings`, or `/overrides` trees;
- embedded catalogs treated as verified, versioned, lowest-authority
  observations;
- installation and construction never silently rewriting an existing human
  workspace;
- semantic human edits preserved as local evidence;
- dynamic valid facts normally outranking local fallback;
- source absence and degraded observations unable to delete last-known-good
  data;
- provider price preferred for that provider offering when valid;
- definitions, offerings, and author membership derived as immutable read
  views;
- candidate construction, validation, durable commit, and atomic immutable
  publication;
- concrete immutable `*catalogs.Catalog`;
- non-failing, non-nil, O(1), allocation-free `Client.Catalog()`;
- a useful in-process library that does not require the CLI, server, scheduler,
  or cloud provider acquisition stack;
- a public embeddable Starmap server that another Go program can compose;
- a reactive remote Go consumer that performs a verified initial fetch and
  normally follows post-commit server events through SSE;
- event notifications treated only as hints to fetch and verify immutable
  generations;
- reconnect and missed-event recovery that cannot leave a client permanently
  stale;
- polling used only as an explicit fallback;
- no hidden constructor-owned lifecycle goroutines;
- explicit context, shutdown, timeout, retry, and error ownership;
- a clean final repository, protected `main`, no open PRs, and no obsolete
  worktrees or branches.

Challenge any of these decisions when evidence shows they are wrong, but give a
specific replacement and explain the tradeoff.

## Canonical Go review

Review the proposed and existing Go design against ordinary Go conventions,
including:

- accept interfaces and return concrete types;
- define narrow interfaces at the consuming seam;
- do not introduce an interface with only one real adapter;
- keep constructors explicit and unsurprising;
- keep lifecycle ownership visible to callers;
- make zero values useful where reasonable;
- use typed errors with `errors.Is`/`errors.As` semantics;
- wrap errors with operational context without destroying identity;
- propagate contexts through blocking operations;
- never store contexts in long-lived structs unless the ownership model
  genuinely requires it;
- avoid goroutine leaks, unbounded queues, unbounded retries, and shutdown
  races;
- avoid package globals for mutable registries or configured clients;
- preserve immutable published values across goroutines;
- keep package names short, concrete, and free of import-path stutter;
- keep exported identifiers from repeating their package name unnecessarily;
- avoid `util`, `helpers`, `common`, `manager`, and `service` dumping grounds;
- avoid cyclic dependencies and oversized composition roots;
- avoid reflection where typed code gives clearer semantics;
- keep generated code and compatibility code out of the normal consumer
  interface;
- document every exported identifier and important invariant;
- keep root-package dependency closure appropriate for a library;
- keep server, acquisition, CLI, and optional integrations opt-in where
  practical;
- use functional options only when they improve clarity and validation;
- ensure options compose independently and are not order-sensitive;
- make cancellation and cleanup deterministic;
- use benchmarks only for stated performance budgets; and
- prefer simple code with strong locality over abstract frameworks.

Specifically review whether the planned module seams are deep: callers should
receive substantial behavior through small interfaces, while implementation
knowledge and failure handling remain local.

## File and package structure review

Apply this repository policy to every repository-authored Go file, including
tests:

- 0–1000 lines: normal;
- 1001–1500 lines: concern requiring explicit review;
- 1501–1999 lines: allowed only with a durable conceptual reason that splitting
  would reduce locality, testability, or leverage;
- 2000 or more lines: must be split before merge.

Do not recommend arbitrary file splitting. Every extraction must have a clear
concept, invariant, and test surface. Flag shallow packages created only to
move lines.

Review:

- current files above the thresholds;
- package/file/import-path stutter;
- public packages with no production caller;
- duplicate source/provider/acquisition vocabulary;
- pass-through modules;
- interfaces with one adapter;
- broad application or command interfaces;
- repeated parsing, validation, reconciliation, or publication logic;
- business rules spread across unrelated packages; and
- concepts whose tests currently require reaching past the public interface.

For each structural concern, say whether to:

1. keep it with a concrete rationale;
2. deepen the module;
3. fold it into another module;
4. split it by named concept; or
5. delete it.

## Library consumer review

Review the plan from the perspective of an external Go consumer.

The ordinary experience should remain:

```go
sm, err := starmap.New(...)
if err != nil {
    return err
}

catalog := sm.Catalog()
model, err := catalog.FindModel("gpt-4o")
```

Determine whether the plan preserves:

- obvious construction and error behavior;
- a small learnable interface;
- immutable values that are safe to retain;
- provider-independent model discovery;
- unambiguous provider-specific price, limits, availability, and endpoint
  lookup;
- predictable local, embedded, store-only, and remote modes;
- no surprising filesystem or network mutation;
- no required background lifecycle for an in-process read-only consumer;
- useful GoDoc and compile-time examples; and
- a dependency graph appropriate for a reusable library.

Identify every point where the caller could reasonably misunderstand which
catalog is authoritative, when an update becomes visible, who owns shutdown, or
what happens after a failure.

## Server and reactive remote review

Review whether SSE is the correct canonical notification transport for this
one-way publication flow. Compare it with WebSocket and polling using actual
product needs, not novelty.

The plan must prove:

- server events occur only after durable commit;
- events contain immutable generation identity rather than model payloads;
- the remote consumer always verifies the addressed manifest and payload;
- checksum, size, schema, media type, origin, redirect, and publisher policy are
  enforced;
- duplicates and stale/out-of-order notifications are harmless;
- disconnection followed by reconnect always performs a current-state catch-up;
- dropped fan-out events cannot cause permanent staleness;
- retries use bounded exponential backoff with jitter;
- authentication failures do not retry forever;
- caller context owns the stream lifecycle;
- shutdown cannot leak goroutines, connections, timers, or callbacks;
- slow consumers have explicit observable policy;
- polling cannot run accidentally alongside a healthy stream;
- polling fallback has freshness and request budgets;
- an external Go program can embed the server without importing `internal`
  packages;
- a server-only program does not need provider acquisition unless it explicitly
  composes it; and
- the root read-only library does not absorb server dependencies.

State whether WebSocket should be deleted unless a concrete bidirectional
consumer is named.

## Data lifecycle and reconciliation review

Walk these workflows end to end:

1. no local workspace and no network;
2. first CLI update;
3. human adds a private provider/model;
4. human fills a missing field;
5. human changes a dynamically discoverable field;
6. human deletes a field;
7. human deletes a local-only model;
8. human deletes an upstream-backed model;
9. binary upgrades from embedded revision E1 to E2;
10. E2 adds, changes, or omits a provider/model/field;
11. provider data conflicts with local and models.dev values;
12. provider price is invalid, incomplete, stale, or differently scoped;
13. provider omits a known model;
14. provider response is partial or malformed;
15. one collection record is malformed but its siblings are valid;
16. every source is offline;
17. local YAML is invalid;
18. a human edits YAML during update;
19. two processes update the same workspace;
20. durable store commit fails;
21. YAML staging, fsync, or rename fails;
22. process crashes between durable steps;
23. process restarts;
24. generation is rolled back;
25. an older binary opens a newer workspace;
26. a verified release artifact is imported; and
27. a tampered or incompatible artifact is offered.

For each workflow, determine whether the plan names:

- the source of truth;
- the winner-selection rule;
- the atomicity point;
- the visible result;
- the failure result;
- the recovery behavior;
- the provenance that survives; and
- a test that can falsify the behavior.

Flag any workflow that still needs an operator decision.

## Test strategy review

Reject tests that merely increase coverage. Require tests that prove important
behavior at the narrowest useful level.

Review whether the plan has the right mix of:

- external consumer compile tests;
- real end-to-end workflows;
- integration tests across storage, publication, HTTP, and streaming seams;
- table-driven authority and presence tests;
- concurrency tests under `-race`;
- multi-process filesystem tests;
- deterministic fake-clock retry tests;
- fault injection;
- fuzzing of untrusted inputs and state transitions;
- golden serialization fixtures;
- compatibility and downgrade fixtures;
- security tests;
- benchmarks with explicit budgets; and
- hosted checks on exact PR heads.

For each proposed fuzz target, confirm that fuzzing is likely to discover
meaningful failures rather than exercise trivial getters or already constrained
types.

Identify missing tests, redundant tests, brittle implementation-detail tests,
and tests that should be deleted along with obsolete code.

## Pull request and workspace review

Review the plan to:

- merge or close every current and newly created Starmap PR;
- merge dependency PR #43 before rebasing/recreating #44;
- treat #40 only as a donor inventory;
- preserve useful #40 behavior without retaining its rejected persisted schema;
- keep implementation PRs small and conceptually coherent;
- avoid overlapping worktrees changing the same files;
- preserve the unique local historical checkpoint before normalizing local
  `main`;
- remove stale local/remote branches only after reachability checks;
- preserve branch protection;
- finish with local `main == origin/main`;
- finish with one intended primary worktree; and
- prove the final state from a fresh clone.

Call out any step that risks losing work, rewriting historical evidence, or
leaving a false completion claim.

## Step-by-step plan audit

For every phase P0–P11 and every task in
`docs/STARMAP_ARCHITECTURE_CONTROL_PLANE.md`, assign exactly one disposition:

- `ACCEPT`
- `REVISE`
- `REMOVE`
- `SPLIT`
- `MOVE`
- `ADD DEPENDENCY`
- `BLOCK`

For every disposition other than `ACCEPT`, provide:

1. the exact phase/task ID;
2. the problem;
3. the risk if unchanged;
4. the proposed replacement wording or task structure;
5. stronger verifiable success criteria; and
6. the correct execution order.

Check that every criterion is:

- observable;
- falsifiable;
- scoped to the task;
- reproducible;
- tied to exact evidence;
- not satisfied by a narrower test; and
- strong enough to justify `DONE`.

Check that every finding maps to a task and every task supports the mission.
Identify orphan findings, unowned risks, redundant tasks, circular
dependencies, premature cleanup, and tasks that can falsely pass.

## Required response format

Return one Markdown report with these sections:

1. **Executive verdict**
   - `GO`, `GO WITH REQUIRED REVISIONS`, or `NO-GO`
   - five most important reasons

2. **Blocking findings**
   - ordered P0, P1, P2, P3
   - file and line references where possible
   - concrete failure scenario

3. **Phase-by-phase audit**
   - every phase P0–P11
   - every task disposition
   - missing or weak success criteria

4. **Canonical Go assessment**
   - library interface and DX
   - module depth and seams
   - context/error/lifecycle/concurrency behavior
   - dependency and package structure
   - naming and file-size findings

5. **Catalog lifecycle assessment**
   - embedded/local/live/store/release authority
   - human editing behavior
   - atomicity, restart, rollback, downgrade, and provenance

6. **Server and reactive consumer assessment**
   - embeddability
   - SSE/WebSocket/polling decision
   - correctness and recovery semantics

7. **Test and verification assessment**
   - high-value additions
   - low-value or duplicate tests to remove
   - fuzz and benchmark recommendations

8. **PR/worktree cleanup assessment**
   - #40, #43, #44 dispositions
   - branch/worktree safety
   - final clean-state proof

9. **Recommended control-plane edits**
   - provide copy-ready replacement Markdown
   - preserve existing task/finding IDs where possible
   - explicitly map superseded IDs

10. **Revised execution order**
    - smallest coherent PR sequence
    - dependency graph
    - first three actions

11. **Final readiness checklist**
    - the minimum evidence needed before claiming the package is production
      ready for an enterprise LLM proxy gateway

Be direct. Prefer deletion and simplification where behavior remains complete.
Do not recommend abstraction merely for symmetry or test mocking. Do not accept
“enterprise-grade” as evidence; require observable behavior and reproducible
proof.

---
