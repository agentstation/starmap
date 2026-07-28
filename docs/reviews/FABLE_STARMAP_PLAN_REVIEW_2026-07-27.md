# Starmap Architecture Execution Plan — Independent Review

Date: 2026-07-27
Reviewer: Claude Fable 5 (independent review; prompt at
[`FABLE_STARMAP_PLAN_REVIEW_PROMPT.md`](FABLE_STARMAP_PLAN_REVIEW_PROMPT.md))
Companion disposition:
[`FABLE_STARMAP_PLAN_REVIEW_DISPOSITION_2026-07-27.md`](FABLE_STARMAP_PLAN_REVIEW_DISPOSITION_2026-07-27.md)

Reviewed: `docs/STARMAP_ARCHITECTURE_CONTROL_PLANE.md` (branch
`codex/starmap-architecture-control-plane`, `b876b966`) against protected-main
baseline `9508ee78`, with live GitHub inspection of PRs #40, #43, #44 (access
was available; no fallback to recorded metadata was needed). Code claims were
verified directly at the baseline worktree, including empirical probes run
out-of-tree.

## 1. Executive verdict

**GO WITH REQUIRED REVISIONS.**

1. **The plan's evidence is trustworthy.** Every P0.1 claim I could check
   verified exactly: file sizes (2059/1206/1183/1134), 89 package directories,
   PR heads (`a14d2249`, `ebd36505`, `e1dcd1e6`), #43 green with
   x/text→v0.40.0, #44 failing on the pre-#43 graph, local main
   1-ahead/11-behind. The 30 findings sampled in code are all real, and several
   are *worse* than recorded (details below). The target architecture — one
   human YAML workspace, derived immutable read views, SSE-as-hint, immutable
   generations — is the right one, and the baseline code independently confirms
   it is needed.
2. **The plan cannot be executed as written because the P2 gate contradicts its
   own phase exit criteria.** "P2 gate requires red characterization tests for
   every reproduced defect" cannot coexist with "required hosted checks pass on
   that same head" for a merged phase PR on a protected branch. This must be
   reworded before the `/goal` loop starts, or execution stalls at the second
   phase.
3. **The plan reassigns `~/.starmap/catalog` from machine generation-store to
   human YAML workspace with no migration or detection task**, and in doing so
   silently reverses the user-approved historical decisions F-095/F-097
   (STARPORT ledger P11.15–P11.19) without the explicit supersession mapping
   that ledger's own constraint 12 requires. Any existing install has an opaque
   generation store at the exact path the new plan calls "the one
   human-editable catalog." This is a data-integrity and ledger-integrity gap.
4. **The hardest atomicity problem in the design — two durable artifacts (YAML
   workspace and generation store) that must agree — has no named commit point
   or crash-recovery rule.** Today's code orders them backwards (`sync.go:59`
   saves the non-atomic, non-fsynced YAML *before* the fsynced CAS commit, so
   the fragile artifact gates the durable one). Workflow 22 ("process crashes
   between durable steps") has no owning task with a falsifiable criterion.
5. **Two mid-plan underspecifications will force rework**: P3.4/P3.5
   (semantic-edit detection, E1→E2 merge) depend on the single
   authority/provenance implementation P4 builds; and P6.2/P6.3 (library
   slimming) omit the mechanism even though it already exists in the code —
   inverting the static `pkg/sources → internal/providers/clients` import onto
   the existing `WithProviderClientFactory` seam, which alone removes ~448 of
   the root package's 472-package dependency closure.

## 2. Blocking findings

**P0 — fix before execution starts (edits to the plan document itself):**

- **B-01 · P2 gate self-contradiction.** Control plane: "P2 gate requires red
  characterization tests for every reproduced defect" vs. phase exit criteria
  4–5 (local + hosted checks pass on the exact phase head). Failure scenario:
  the P2 PR merges failing tests (impossible under branch protection) or the
  agent quietly weakens the tests to pass, defeating characterization. Fix:
  characterization tests must be **green tests that pin current (defective)
  behavior**, annotated with their finding ID, and flipped in the phase that
  fixes each defect.
- **B-02 · Workspace path reversal without migration.** Baseline:
  `~/.starmap/catalog` = generation store (`.commit.lock`, `current`,
  `generations/<sha>/…`; `pkg/constants/constants.go:179`,
  `pkg/catalogstore/filesystem.go`); `~/.starmap/exports/catalog` = YAML tree.
  The plan makes `~/.starmap/catalog` the human YAML tree with `.state/` for
  machine data. No task detects a pre-existing store layout at that path.
  Failure scenario: a user with an existing install runs the new binary; the
  loader either fails confusingly on opaque JSON or, worse, a future "seed
  absent workspace" path misclassifies the directory. Additionally, historical
  findings F-095/F-097 recorded this layout as a *user-approved clean break*;
  the new plan supersedes it without a mapping row. Fix: add
  layout-detection/migration criteria to P3.1/P3.10 and a supersession-mapping
  section.
- **B-03 · Dual-durable-store commit point unnamed.** `sync.go:59` ("persist
  YAML first, then commit") plus `pkg/catalogs/save.go:33-41`
  (delete-managed-trees-then-rewrite-in-place, no staging, no fsync, no rename)
  means a crash mid-save destroys the workspace *before* the durable commit is
  even attempted, and a successful save + failed commit leaves the workspace
  describing a generation that doesn't exist. The plan's P3.6 covers YAML
  atomicity but never states which artifact is *the* atomicity point or how
  restart repairs divergence. Fix: declare the generation-store CAS commit as
  the sole commit point; workspace materialization is an ordered post-commit
  projection, repaired on next start by digest comparison; add the
  crash-between fault test as a named criterion.

**P1 — fix before the affected phase begins:**

- **B-04 · P3.4/P3.5 depend on P4.** Semantic-edit detection = "compare YAML to
  the last materialized provenance value" — that is exactly the single
  provenance/authority implementation P4.1/P4.5 build. As ordered, P3 would
  implement edit detection against the current *dual* authority tables
  (`pkg/authority/authority.go:118-202` legacy vs `canonical.go:76-110`, which
  disagree on Limits and Metadata precedence, with the canonical table consumed
  at exactly one call site, `pkg/reconciler/merger.go:715`) and then be
  reworked. Fix: split P3.
- **B-05 · P6.2/P6.3 lack the mechanism.** Verified: `go list -deps .` = 472
  packages; `go list -deps ./internal/providers/google` = 448; the root minus
  the Google client closure is 24 packages. The single edge is
  `pkg/sources/providers.go:11 → internal/providers/clients`
  (`provider.go:19-21`). The fix seam already exists:
  `WithProviderClientFactory` (`pkg/sources/providers.go:298`). Name it in the
  criteria, with a numeric closure budget, so the task cannot "pass" through
  weaker measures.
- **B-06 · Orphaned historical open findings.** STARPORT ledger F-099, F-105,
  F-106 (P12.4/P12.7/P12.8: Homebrew tap, version-to-stdout patch release,
  immutable draft-first release flow) are `IN_PROGRESS` and own real risk (a
  shipped v0.1.0 whose `starmap version` writes to stderr). The new plan's
  rule 17 defers release publication but gives these no terminal mapping — the
  plan's own definition of done ("every finding terminal") cannot be satisfied
  for the historical ledger it claims still counts as evidence.

**P2 — strengthen criteria within the phase:**

- **B-07 · P7.2 ordering claims don't match reality.** Actual order at
  baseline: durable commit → in-memory swap → **event dispatch** → cache
  activation *inside the first hook* (`internal/server/server.go:139-141`), on
  a **detached, unowned goroutine** (`hooks.go:111`) whose semaphore drops an
  *entire generation's* events silently when full (`hooks.go:105-110`), and
  with nothing preventing generation N+1's events reaching the broker before
  N's. P7.2/P7.4 criteria must name cross-generation ordering and
  cache-activation placement.
- **B-08 · P7.10 misses shutdown joins.** `Server.Stop` waits a fixed timer
  without joining broker/hub/broadcaster loops (`server.go:207-234`), and
  `Broker.Subscribe` is an unguarded channel send that blocks forever once the
  loop has exited and the 10-slot buffer fills (`broker.go:179`). Add both to
  the failure matrix.
- **B-09 · P9.6 decision gates P6.5 deletions.** `pkg/catalogdistribution`
  (1,307 LOC, complete HTTP handler/client/promotion subsystem) has **zero
  importers anywhere in the repo**; `pkg/catalogremote` is wired on both sides;
  `pkg/catalogscheduler.NewRunner` and its lease/retry machinery have no
  production caller (only the `Operations` read surface is wired). The
  keep-or-delete decision for distribution/scheduler must happen before P6.5's
  deletion pass, not in P9.
- **B-10 · Operating rule 3 and P11.8 are unsatisfiable as written.**
  Dependabot PRs #43/#44 cannot carry ledger updates "in the same commit as the
  work," and post-merge cleanup evidence (worktree removal, branch deletion,
  final machine gate) cannot land on protected main in the same commit as the
  closing ledger PR. Define the exception explicitly.

**P3 — minor:**

- **B-11 · P0.4 "renders locally" requires the network.** The archived report
  loads Tailwind from `cdn.tailwindcss.com` and Mermaid from jsdelivr (HTML
  lines 8–10). Either vendor the assets or weaken the criterion to "parses."
- **B-12 · P4.10 fuzz scope.** Fuzzing "authority selection" and "presence
  values" exercises a deterministic policy table; demote those to
  property/table tests and keep fuzzing where inputs are genuinely untrusted
  (provider envelopes, YAML/JSON manifests, SSE framing).
- **B-13 · Autonomy vs. required reviews.** Branch protection requires a fresh
  approval per PR (F-033). The `/goal` loop will block on the user roughly a
  dozen times; the plan should say so rather than imply uninterrupted autonomy.

## 3. Phase-by-phase audit

| Task | Disposition | Notes |
|---|---|---|
| P0.1 | **ACCEPT** | All recorded evidence independently verified. |
| P0.2 | **ACCEPT** | Worktree exists from exact baseline; #40 correctly rejected as base. |
| P0.3 | **ACCEPT** | |
| P0.4 | **REVISE** | B-11: CDN dependency defeats "renders locally"; vendor assets or reword. |
| P0.5 | **REVISE** | Add criteria: supersession-mapping section for historical F-095/F-097/P11.14–P11.19 and terminal mapping for F-099/F-105/F-106 (B-02, B-06) lands in the plan PR. |
| P1.1–P1.4 | **ACCEPT** | Order (#43 → rebase #44) verified correct against live CI. |
| P1.5, P1.6 | **ACCEPT** | |
| P1.7 | **REVISE** | Add ordering note: remove the `provider-expansion-wave0` worktree *before* deleting the local branch (git refuses to delete a branch checked out in a worktree). |
| P1.8 | **ACCEPT** | |
| P2.1 | **ACCEPT** | |
| P2.2 | **REVISE** | Pin sites: store-only hard failure `sync.go:114-119` (fails before commit — `Client.Update` via scheduler with no output path fails on **every** attempt); restart precedence `client.go:202-236`. Gate wording per B-01. |
| P2.3 | **REVISE** | Pin sites: manual-model drop `pkg/catalogs/catalog.go:483-491` (models without pricing/limits silently vanish — reproduced empirically); zero-value merge asymmetry `pkg/catalogs/merge.go:5-72`; provenance tracker bare-ID keying `merger.go:202` + `pkg/provenance/provenance.go:147-149` (report winner is a scheduling race, `provenance.go:208-219`). |
| P2.4 | **REVISE** | Pin the four whole-collection decode sites: `modelsdev/parser.go:241`, `transport/request.go:88` (+ Google page-loss `google/client.go:398-407`), `catalogs/load.go:37-39,192,225`, `catalogstore/payload.go:37-39`. The resilient RawMessage pattern already exists at `cmd/starmap/cmd/providers/fetch.go:460-478`. |
| P2.5 | **REVISE** | Add to the recorded semantics: no `Last-Event-ID` anywhere (zero grep hits); SSE `id:` is unix-*seconds* (non-unique); opposite backpressure per transport (WS disconnect / SSE skip); four silent drop points; `model.*` events carry no provider or generation identity. |
| P2.6 | **REVISE** | Record the closure attribution as baseline: 472 packages, 448 via `internal/providers/google`; measure *post-#43* since #43 bumps genai/auth/sqlite. |
| P2.7 | **ACCEPT** | |
| P3.1 | **REVISE** | B-02: add legacy-layout detection — a pre-plan generation store at the workspace path fails with a typed error naming the migration path, before any mutation. |
| P3.2, P3.3 | **ACCEPT** | |
| P3.4 | **ADD DEPENDENCY** | Requires P4.1 + P4.5 (one authority table, provenance identity) first. |
| P3.5 | **ADD DEPENDENCY** | Same; "compare to last materialized provenance value" *is* P4 machinery. |
| P3.6 | **REVISE** | B-03: name the commit point (store CAS), invert current `sync.go:59` ordering, add crash-between-artifacts fault test and restart digest-repair criterion. |
| P3.7 | **ACCEPT** | |
| P3.8 | **ACCEPT** | Defect confirmed real and reachable (hard `ConfigError` from `save.go:20-26` via empty write path). |
| P3.9 | **ACCEPT** | |
| P3.10 | **REVISE** | Add "new binary opens old-layout directory" alongside "old binary opens newer workspace." |
| P4.1 | **REVISE** | Strengthen: the hand-written shadow policy `complexModelStructures` (`merger.go:591-712`) with its own hardcoded priority slices must be deleted, not just the unused canonical inventory; a field must be writable by exactly one policy path. |
| P4.2 | **REVISE** | Add: rejection evidence must persist even when *all* candidates fail validation — `merger.go:744-750` currently discards it in exactly that case. |
| P4.3 | **ACCEPT** | |
| P4.4 | **ACCEPT** | Site confirmed: persisted tracker map unscoped; result map already scoped (`reconciler.go:374-386`) — unify on the scoped scheme. |
| P4.5 | **ACCEPT** | |
| P4.6 | **ACCEPT** | Note: requires tri-state types for `ModelLimits` (bare `int64`s) and `ModelFeatures` (bare bools, one-way latches at `merger.go:834-836, 979-984`) — a breaking schema change; plan already accepts pre-1.0 breaks. |
| P4.7 | **REVISE** | Name the mechanism: reconciler/pipeline currently read **zero** fields of `Observation.Status/Completeness/Issues` (grep-verified); baseline models absent from all sources are silently dropped (`merger.go:174-184`) and providers are replaced wholesale (`reconciler.go:279-286`). Criterion: degraded/partial observations must be consumed by the write-decision code, and a volume-retention guard (pattern exists at `modelsdev/http_client.go:484-494`) applies to the final catalog. |
| P4.8 | **REVISE** | Pin the decode sites (see P2.4). |
| P4.9 | **REVISE** | State the semantic: current flag is a *preflight dependency* gate (`lifecycle.go:147-152`) that is a no-op for default sources; `--require-all-sources` with zero API keys exits 0 today. Required = Complete + Succeeded observation. |
| P4.10 | **REVISE** | B-12: fuzz untrusted inputs only; authority/presence become property tests. |
| P5.1–P5.8 | **ACCEPT** ×8 | P5.8's "no named external consumer" guard for `LegacyV0` is the right test; confirm Starport before deletion. |
| P6.1 | **ACCEPT** | |
| P6.2 | **REVISE** | B-05: name the `WithProviderClientFactory` inversion and a numeric budget (root closure contains no genai/grpc/otel/gorilla-websocket; ≈150 packages). |
| P6.3 | **REVISE** | Name the opt-in import path the CLI/server will use to register provider clients. |
| P6.4 | **ACCEPT** | Evidence: `application.Application` is 11 methods bundling four concerns; most consumers use one or two. |
| P6.5 | **REVISE + ADD DEPENDENCY** | Make the list concrete: delete `pkg/types` (165 LOC of deprecated aliases, zero consumers); delete or wire `pkg/catalogdistribution` (zero importers — depends on the P9.6 decision, which must move earlier, B-09) and `pkg/sourceevidence` (zero importers); `pkg/enhancer` wiring is inert (`reconciler.WithEnhancers` has no production caller); fold `pkg/save` into `pkg/catalogs`; inline `internal/utils/ptr` (one importer, three calls); apply the deletion test to `catalogscheduler.NewRunner`. |
| P6.6, P6.7 | **ACCEPT** | |
| P7.1 | **REVISE** | Name the target: a public server package whose constructor accepts a `*starmap.Client` (or narrower catalog+events seam), **not** `internal/application.Application` (`internal/server/server.go:40` — today two internal imports are needed). |
| P7.2 | **REVISE** | B-07: criteria must cover cache-activation placement and cross-generation event ordering. |
| P7.3 | **REVISE** | Default to **delete WebSocket now**. No bidirectional consumer is named anywhere in any of the seven documents; Starport consumes one-way publication. SSE is correct: it's plain HTTP (proxy/LB-friendly, works with the existing auth middleware), has built-in reconnect semantics, and the mandatory catch-up-fetch design makes lossy delivery harmless. WebSocket adds a second framing, an opposite backpressure policy, and a dependency — for nothing. Re-add it when a named consumer appears. |
| P7.4 | **REVISE** | Require a monotonic per-stream sequence bound to generation ID (today's `id:` is unix-seconds and non-unique, `adapters/sse.go:24`); and **delete the `model.*` diff events** — they flatten provider identity (bare model ID, `hooks.go:170-171` over the lossy `Models()` view) and contradict the "events are hints, never data" contract. One event type: `catalog.published{generation_id, sequence}`. |
| P7.5 | **REVISE** | Add: the detached unowned hook-delivery goroutine (`hooks.go:111`) is replaced by context-owned delivery. |
| P7.6, P7.7 | **ACCEPT** | Mandatory post-reconnect catch-up correctly makes `Last-Event-ID` an optimization only. |
| P7.8 | **ACCEPT** | Minor add: manifest polling should send conditional requests (today `FetchCurrent` sends only `Accept`, `catalogremote/remote.go:121`). |
| P7.9 | **ACCEPT** | |
| P7.10 | **REVISE** | B-08: add shutdown-join and subscribe-after-stop cases. |
| P7.11 | **REVISE** | Require the *outermost* drop counter (`Client.HookStats`, `hooks.go:53,108`) on the operational endpoint — today admin stats expose broker/hub/broadcaster counters but not the one that drops whole generations. |
| P8.1, P8.2 | **ACCEPT** | |
| P8.3 | **ACCEPT** | Add `pkg/differ/differ.go` (exactly 1000 lines) to the watch list; note dead `Changeset.Filter` (`changeset.go:369`, zero callers) as a P8.6 candidate. |
| P8.4 | **ACCEPT** | Concrete starters: `provenance.ProvenanceFile`, `format.Formatter`, `catalogstore.Store` is fine, and the seven-package `catalog*` prefix family (three dead/near-dead) is the bigger naming smell. |
| P8.5 | **ACCEPT** | `complexModelStructures` (`//nolint:gocyclo`) is the top hotspot and dies in P4.1. |
| P8.6 | **REVISE** | Scope to residual findings after P6.5 — as written it duplicates P6.5's deletion pass. |
| P8.7, P8.8 | **ACCEPT** | |
| P9.1–P9.5, P9.7 | **ACCEPT** | |
| P9.6 | **MOVE** | Decision to before P6.5 (B-09); implementation wiring may stay in P9. |
| P10.1–P10.8 | **ACCEPT** ×8 | |
| P11.1–P11.3 | **ACCEPT** | |
| P11.4 | **ACCEPT** | Archive-ref-before-realign is exactly right. |
| P11.5–P11.7 | **ACCEPT** | |
| P11.8 | **REVISE** | B-10: define the closing-PR exception — the ledger-closing PR is the final PR; the post-merge machine gate runs without further commits; post-merge cleanup evidence is recorded in that PR as forward-stated commands plus the gate script's output requirement. |

Ledger cross-checks: every finding F-001–F-030 maps to at least one task and
every task supports the mission; I found no circular dependencies beyond
B-04/B-09, no redundant tasks beyond P8.6/P6.5 overlap, and no task that can
falsely pass except P6.2 (before B-05's numeric budget) and P4.9 (before the
Complete+Succeeded semantic wording). Orphans: only the historical
F-099/F-105/F-106 (B-06).

## 4. Canonical Go assessment

**Library interface and DX.** The consumer contract (`starmap.New` →
non-failing, concrete, immutable `sm.Catalog()`; `FindModel` for definitions;
`Offering(provider, model)` for provider facts) is canonical and already real:
verified 13 ns/op, 0 allocs, RLock-pointer-read (`client.go:64-69`). Two
blemishes: `Catalog()` lacks the nil-receiver guard its neighbors have, and
`New` performs a synchronous store read under an internally-created,
uncancellable `context.WithTimeout` (`client.go:203-205`) — `New` should accept
a context or a `NewContext` variant should exist. Neither is in the plan; both
belong in P6.

**Module depth and seams.** The plan's instincts are right and the P2.8
seam-inventory discipline from the historical ledger is genuinely good Go
practice. The remaining problems are the inverse: *built but unwired* surface
(`catalogdistribution`, `sourceevidence`, enhancer wiring, `NewRunner`,
`Changeset.Filter`, `ApplyAdditive`) — deep modules with zero production
callers are still speculative abstractions, just more expensive ones. The
deletion test the plan prescribes is correct; §3 makes the list concrete.

**Context/error/lifecycle/concurrency.** Constructors are clean (refuted the
hidden-goroutine concern at the root — an AST test even enforces it). The real
lifecycle defects are downstream: the detached hook goroutine owned by nothing,
timer-based shutdown without joins, `Subscribe` blocking after loop exit, and
whole-generation silent event drops. Typed errors are used consistently; the
store-only sync failure surfaces as a *misleading* wrapped `ConfigError` ("no
write path configured") from a code path that shouldn't run at all.

**Dependency and package structure.** The single most valuable Go fix in the
whole plan is one line of import inversion: `pkg/sources/providers.go:11` onto
the existing factory seam. After it, the root library closure drops from 472 to
roughly the `pkg/catalogs` weight (~145). The `application.Application`
interface should dissolve into per-consumer role interfaces (`Catalog()` is all
most commands need).

**Naming and file sizes.** Verified inventory matches the plan. Add:
`internal/utils/ptr` (a `utils` grab-bag for three one-liner calls),
`provenance.ProvenanceFile`, and the `catalog*` seven-package prefix family,
which argues for consolidation-by-deletion rather than renaming.

## 5. Catalog lifecycle assessment

The authority contract (provider price wins when valid; absence is not
deletion; embedded is lowest-authority evidence; local is fallback for
discoverable fields, first for operator config) is coherent and the workflow
matrix in the review HTML is close to exhaustive. Confirmed at baseline, the
code violates nearly all of it: restart ignores hand edits (empirically
reproduced — a `hand-edited` provider vanished after restart against a durable
store); manual models without pricing/limits are silently dropped on *every*
sync via the local source; field-clearing is impossible for non-boolean types;
the reconciler never reads the degradation vocabulary it computes; and
`--require-all-sources` passes with zero credentials. The plan's P3–P5 target
the right things.

Three lifecycle gaps to close (all in §2/§3): the dual-store commit point
(B-03), the legacy-layout migration (B-02), and E1→E2 semantic merge ordering
(B-04). Atomicity, restart, rollback, and downgrade are otherwise
well-specified with falsifiable criteria, and the generation store's own
implementation (staging + fsync + rename, CAS, retained history) is the correct
pattern to extend to the workspace — the code already contains its own best
example.

## 6. Server and reactive consumer assessment

**SSE is the correct choice**, and the plan's framing — events are hints;
correctness comes from verify-and-fetch of immutable generations; mandatory
catch-up after reconnect; at-least-once delivery — is exactly right and makes
the transport's weaknesses irrelevant. **WebSocket should be deleted now**
(P7.3 revision): no bidirectional consumer is named, and at baseline it
contributes a second wire format, an opposite backpressure policy, a
provider-ambiguous payload, and a dependency. Polling as explicit bounded
fallback (P7.8) is right; add conditional requests.

Embeddability requires a genuinely new public server package (the current one
takes `internal/application.Application` as its first parameter — structurally
impossible to embed). The reactive consumer is entirely new code (the current
remote client is pull-only with no subscription machinery at all); P7.5–P7.10
specify it well once B-07/B-08 additions land. The event-identity task must
also fix that today's "generation ID" exists only inside an untyped data map on
one event type.

## 7. Test and verification assessment

The test-value policy (prove contracts, reject coverage-count tests) is right,
and the required-suite list is strong. Adjustments:

**High-value additions:** crash-between-YAML-and-commit fault test (B-03);
legacy-layout detection fixture (B-02); cross-generation event-ordering test
under load (B-07); shutdown-join and subscribe-after-stop tests (B-08); a
"required = complete + succeeded" matrix for require-all (P4.9); a
closure-budget compile test (a `go list -deps` assertion in CI keeps P6.2 from
regressing); volume-retention guard test on the final catalog.

**Reduce/redirect:** fuzzing authority selection and presence semantics
(deterministic policy — property tests are the right tool, B-12); "SSE framing"
fuzz is justified only on the *client* parser (untrusted via proxies), not the
server writer. Delete tests along with their code: WebSocket suite (with P7.3),
`model.*` event tests (with P7.4), all `catalogdistribution`/`sourceevidence`/
enhancer tests if deletion wins, `pkg/types/aliases_test.go`.

**Benchmarks:** keep exactly the two with stated budgets
(`BenchmarkClientCatalog` 0 allocs/≤10µs; remote activation latency once P7
defines one). Don't add others.

## 8. PR/worktree cleanup assessment

- **#43: merge first** — verified green on exact head `ebd36505`, contains
  x/text v0.40.0 plus genai/auth/sqlite bumps (note: re-measure the P2.6
  closure baseline *after* this merge). **ACCEPT.**
- **#44: rebase/recreate after #43, then merge** — verified failing on both
  required checks; the recorded cause (govulncheck GO-2026-5970 in x/text
  v0.38.0) matches the live state. **ACCEPT.**
- **#40: donor inventory then close** — verified: draft, 576 files,
  +45,142/−10,593, head `a14d2249`, contains a 2,044-line reconciler test and a
  1,565-line OpenAI connector plus a competing `internal/connectors`
  vocabulary. Treating it as read-only donor is right. **ACCEPT**, with the
  P1.7 worktree-before-branch ordering note.
- **Workspace safety:** the P11.4 archive-ref-first treatment of divergent
  `main@3787d716` is the correct way to avoid losing the only copy of that
  checkpoint. The final machine gate is falsifiable and complete, subject to
  the B-10 self-reference fix. The `fresh-catalog-release` worktree was
  verified tree-identical to protected main — safe to remove as recorded.

## 9. Recommended control-plane edits

Delivered as copy-ready Markdown in the original review response and applied —
with dispositions recorded — in
[`FABLE_STARMAP_PLAN_REVIEW_DISPOSITION_2026-07-27.md`](FABLE_STARMAP_PLAN_REVIEW_DISPOSITION_2026-07-27.md)
and control-plane commit `de6c6fda`. The proposed edits covered: Operating
Rule 3 exceptions; the green-characterization P2 gate; revised rows P3.1,
P3.6 (split), P4.2, P4.7, P4.9, P6.2, P7.3, P7.4, P7.10, P11.8; new findings
F-031–F-037; and the Historical Supersession Map.

## 10. Revised execution order

Dependency-minimal PR sequence (each row = one phase PR unless noted):

1. **Plan PR** (P0.5, carrying the §9 edits).
2. **#43 merge**, then **#44 rebase/merge** (P1.1–P1.4). *(Human approval
   needed at each protected merge.)*
3. **#40 inventory + close + worktree/branch removal** (P1.5–P1.8).
4. **Characterization suite** (P2, green pinned tests).
5. **Decision PR (docs-only):** distribution seam (old P9.6 decision) +
   workspace path/migration policy — unblocks the deletion lists.
6. **Hotfix PR:** store-only sync fix + commit-ordering inversion + atomic YAML
   materialization (P3.6, P3.8) — smallest change with the highest production
   value; independent of the workspace redesign.
7. **Authority consolidation** (P4.1–P4.3, delete `complexModelStructures`).
8. **Provenance scoping + presence types** (P4.4–P4.6; breaking schema change).
9. **Resilience** (P4.7–P4.10: observation-health consumption, quarantine,
   strict mode, fuzz).
10. **Workspace lifecycle** (P3.1–P3.5, P3.7, P3.9, P3.10 — now able to use the
    P4 machinery).
11. **Read views + compat deletion** (P5).
12. **Library composition** (P6: factory inversion, deletions, role
    interfaces).
13. **Embeddable server + event identity** (P7.1–P7.4, including WebSocket
    deletion).
14. **Reactive remote consumer** (P7.5–P7.11).
15. **Modularity/naming** (P8, residuals only), then **distribution integrity**
    (P9), **verification** (P10), **cleanup** (P11).

First three actions: (1) apply the edits to the control plane, push the branch,
open the plan PR; (2) after approval, merge #43 on its exact head and rebase
#44; (3) produce the #40 donor inventory and close it.

## 11. Final readiness checklist

Minimum evidence before claiming "production ready for an enterprise LLM proxy
gateway":

- External-module compile tests pass for all four compositions (in-process
  library, store-only, embedded server, remote subscriber), none importing
  `internal`.
- CI-enforced dependency budget: root closure free of provider SDKs, grpc,
  otel, websocket, sqlite, cobra.
- Lifecycle suite green under `-race`, repeatedly: seed, manual edit, E1→E2,
  legacy-layout detection, conflict, degraded source, all-sources-offline,
  edit-during-update digest conflict, two-process CAS, restart, rollback,
  downgrade refusal, tampered-artifact rejection — each tied to a named
  workflow from the matrix.
- Fault-injection campaign covering every named point *including crash between
  store commit and workspace materialization*, with prior state provably
  readable after each.
- Real-transport SSE end-to-end: publish → event → verify → activate; plus
  disconnect/duplicate/stale/skipped/corrupt/unauthorized/slow/shutdown
  recovery, with shutdown joining all goroutines (goroutine-leak assertion).
- Strict mode fails with zero credentials; a degraded observation cannot shrink
  the catalog past the retention guard.
- `Client.Catalog()` budget re-proven (0 allocs, ≤10µs) on the final head; drop
  counters visible on the operational endpoint.
- No repository-authored Go file ≥2000 lines; every >1000 file has a recorded
  terminal review; deletion list (types, distribution-or-wired, sourceevidence,
  enhancer wiring, WebSocket, model events, utils/ptr, save) fully
  dispositioned.
- Zero open PRs; local `main == origin/main`; one worktree; archive ref for
  `3787d716` reachable; branch protection readback strict; fresh-clone
  verification documented and executed.
- Final evidence audit names each of these results independently — and the
  historical F-099/F-105/F-106 rows carry a terminal disposition.

---

**Bottom line:** this is one of the better-evidenced execution plans I've
reviewed — every factual claim I tested held, and the code audit *strengthened*
its case rather than undermining it. The required revisions are concentrated in
five places: fix the P2 gate contradiction, own the `~/.starmap/catalog`
path-meaning reversal (migration + supersession mapping), name the dual-store
commit point, resequence P3's semantic-merge tasks after P4, and write the
library-slimming mechanism into P6's criteria. With those edits applied to the
plan document before the `/goal` loop starts, the executed result would be a Go
package an enterprise gateway can trust.
