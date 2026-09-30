# Fable continuation prompt and checkpoint report

Use Fable 5.1 as the lead model for this work. Continue the existing Starmap and Starport production catalog plan. Continue implementation, review, and delivery. Read the current ledger before making changes. Do not restart completed work or reopen accepted product decisions.

Checkpoint: September 30, 2026. The owner requested this model transition. Both delegated agents stopped at safe checkpoints. Root's last native check finished. No verification process from this checkpoint remains running. Existing shared Docker fixtures remain available.

## Outcome and current goal

The product must support a local developer, a small team, and a qualified single-region enterprise deployment. Starmap owns catalog facts, acquisition, publication, storage contracts, and catalog authority. Starport owns gateway identity, inference credentials, routing, admission, budgets, execution, operator configuration, and its UI.

The recorded whole-plan goal is:

```text
Execute the Starmap and Starport production catalog plan at /Users/jack/src/github.com/agentstation/starmap-catalog-requirements/docs/plans/starport-production-catalog-plan.html to completion. First resolve CA01–CA07 from the final audit, activate the durable plan, and establish CSP0 fail-before verification. Continue through the ledger across both recorded worktrees, preserving confirmed decisions, historical evidence, and required authorization boundaries. Completion requires all 50 primary acceptance cases and their revised required subcases against the declared released pair, final verification, and plan cleanup after the final implementation PR merges. Record exact progress, blockers, checks, and next action in the plan.
```

The goal API currently reports `blocked`. The durable plan remains active. The latest owner answer resolved the numeric recovery-target question. This checkpoint has executable work and no unanswered question from that exchange. Resume the whole-plan goal through the receiving harness's goal controls. Do not mark it complete after one component or task.

The ledger has **27 of 40 tasks done**. **CSP13** is the current implementation task. Its formal record has one passing required subcase and fifteen UNVERIFIED subcases. Many component tests pass, but they do not establish complete recovery. CSP15 through CSP24 and CSP99 retain their ledger requirements.

The goal's opening CA/CSP0 clauses describe the original activation. Use the ledger to recognize completed work. Do not execute those completed tasks again.

## Read these files first

Use the `plans`, `use-modern-go`, and `technical-writing` skills. Use `gh` for GitHub CLI work. Use `autoreview` before a substantive-code PR, after final checks and commit. Follow repository instructions and their complete required pre-PR roster.

1. `/Users/jack/src/github.com/agentstation/starmap-catalog-requirements/docs/plans/starport-production-catalog-plan.html`
2. `/Users/jack/src/github.com/agentstation/starmap-catalog-requirements/docs/design/catalog-lifecycle/PRD.md`
3. `/Users/jack/src/github.com/agentstation/starmap-catalog-requirements/docs/design/catalog-lifecycle/ENGINEERING_SPEC.md`
4. `/Users/jack/src/github.com/agentstation/starmap-catalog-requirements/docs/design/catalog-lifecycle/REPOSITORY_FINDINGS.md`
5. `/Users/jack/src/github.com/agentstation/starmap-catalog-requirements/docs/plans/proof/starport-production-catalog/acceptance-map.json`
6. `/Users/jack/src/github.com/agentstation/starmap-catalog-requirements/docs/plans/proof/starport-production-catalog/pr-audit-2026-09-08/current-queue/queue-latest.json`
7. The active consumer's `AGENTS.md`, `docs/TASKS.md`, and relevant source, tests, and callers.

The plan is the control plane. Proof files hold detailed evidence. Read historical logs only to answer a specific evidence question.

## Worktrees and source state

| Purpose | Path | Branch and checkpoint |
| --- | --- | --- |
| Canonical plan and documentation | `/Users/jack/src/github.com/agentstation/starmap-catalog-requirements` | `codex/catalog-qualification`. The handoff commit follows `f81d5dd73`. Read actual HEAD. |
| Integrated Starport implementation | `/private/tmp/starport-clock-review-20260926` | `codex/recovery-catalog-activation`, clean `c6b102327482e1a7337ca76720520cc52e28519b` |
| Full embedded race repair | `/private/tmp/starport-recovery-canonical-semantic-20260930` | `codex/recovery-canonical-semantic-proof`, HEAD `4805e6d4091720f43075d71d0136a30a192dd24d`, unfinished source |
| Populated native adoption | `/private/tmp/starport-recovery-restart-epoch-20260930` | `codex/recovery-restart-epoch-20260930`, HEAD `39c7cb9eaad2c38b44c3d6d2196c8ef3a0f8b6a8`, unfinished successor source |
| Fresh-history delivery | `/private/tmp/starport-recovery-fresh-history-20260930` | `codex/recovery-fresh-history-process`, clean `d4f3ff7d`, already integrated |
| CI delivery | `/private/tmp/starport-recovery-application-ci-20260930` | `codex/recovery-application-ci-qualification`, clean `1c95d5d8`, already integrated |
| Acceptance registration draft | `/private/tmp/starmap-recovery-acceptance-registration-20260930` | `codex/recovery-acceptance-registration`, clean `58a157d541e59e2c791530c1c961e3772ff64fb5` |

Inspect `git status` before each edit or commit. Preserve unrelated plan-worktree changes in `AGENTS.md`, `Makefile`, `go.mod`, `go.sum`, and `scripts/verify.sh`. Do not use a shared reset, clean, or stash. Do not cherry-pick an already integrated delivery again.

The consumer pins published Starmap `v0.16.6-0.20260930115749-2bb995712275`. There is no local module replacement. Use exact Go **1.27.1**, `GOWORK=off`, and cache `/private/tmp/csp13-go-build-20260929`. Set `CGO_ENABLED=0` for pure-Go checks and `CGO_ENABLED=1` for race checks. The pinned lint binary is `/private/tmp/csp13-lint-tools/golangci-lint`, version **2.13.2**. The system's older lint binary is unsuitable for this qualification.

## Confirmed product and authorization decisions

- Internal Starmap servers are authoritative. Retain valid accepted authority during outages. A first boot without accepted authority blocks affected inference. Public fallback and local acquisition require explicit selection.
- GitHub publishes the catalog and updates the default branch every four hours. New binaries and module releases embed the accepted baseline. Existing binaries cannot change their embedded bytes.
- Catalog acquisition credentials and paid inference credentials remain separate. STARMAP-prefixed inference fallback is opt-in. Conventional provider environment names remain available.
- Missing provider models remain visible. Exclude only the affected provider/account entry from automatic routing. Operators explicitly remove entries. Canonical ID aliases remain until explicit or baseline removal.
- Full operator documentation ships embedded and on a versioned public site. It must explain local, team, and enterprise setups, update controls, storage, recovery, and configuration authority.
- Local persistence uses Badger, SQLite, and files. The primary replicated recipe uses Valkey, PostgreSQL, and object storage. Redis and MySQL require their own compatibility evidence.
- Shared storage alone does not require qualified local UTC. Authorization contracts determine clock requirements. Warm permission checks read memory.
- Gateway authorization has a 60-second maximum disconnected lifetime and a 2-second normal revocation-propagation target. Known withdrawals take effect immediately when observed. Already admitted streams follow the agreed completion policy.
- Physical suspend testing is a tracked, nonblocking platform qualification. Required expiry, withdrawal, native clock, and deterministic tests remain mandatory.
- Both products support macOS arm64, Linux amd64/arm64, and Windows amd64/arm64. The owner removed Go 1.25/1.26 and Intel Mac support. Preserve their historical evidence.
- **D42:** Measure supported recovery deployments first. Propose numeric RPO and RTO from those results. Get owner approval before production readiness. Do not invent numeric targets or treat component duration as deployment RTO.
- Provider billing uses actual billing units. Derived per-page prices are estimates. Quarantine invalid source records, retain accepted data, and expose actionable source status.

RPO defines maximum acknowledged data loss. RTO defines maximum restoration time. Supported deployment measurements must state their topology, hardware, load, backup age, history interval, outage, and restored readiness.

Existing authority permits reviewed commits, pushes, draft PRs, native CI, and protected exact-head merges without repeated permission requests. The owner authorized safe merged branch/worktree cleanup. Preserve branch protection. Record actual merge commits before task completion. Releases, new paid actions, and external review comments retain their separate authorization boundaries. Reuse existing PRs and verify current GitHub state before publication.

The last queue snapshot had no open PRs in either product. Root did not create a Starport PR for this integrated successor. This is historical state, not a substitute for a fresh queue check.

## Verified implementation and evidence

The following proof paths are relative to:

`/Users/jack/src/github.com/agentstation/starmap-catalog-requirements/docs/plans/proof/starport-production-catalog/`

| Delivery | Evidence and limits |
| --- | --- |
| Starmap #205 | Merged as `2bb99571227518efcb67aa64f9b78bf9079fab27` after 62 successful checks. Exact reviewed tree and unchanged protection recorded in `csp13/starmap205-merge-2026-09-30/verification.json`. Cleanup has its own proof. |
| Checked original sources | Consumer `2a7cd1a1` retains checked original artifact references with complete hash, length, and key checks. Restart keeps full inspection. Canonical checks passed 49 results per mode. See `csp13/checked-source-integration-2026-09-30/verification.json`. |
| Gateway permission repair | Consumer `cee3f1ae` latches observed closure/conflict/incarnation changes and checks them in memory before dispatch. Missing native budget approval is checked before concept startup. Gateway results: 24 per mode. Final authority results: seven per mode. See `csp13/gateway-integration-2026-09-30/verification.json`. |
| Maximum catalog capacity | Clean `4805e6d4` passed one required test with no skips or failures. Workload: 96 entries, 2,143,303,963 serialized bytes, 32 rollback entries, two exact lost-response retries. Wall time: 786.175 seconds. Largest process RSS: 7,771,095,040 bytes. Original bounds stayed intact. See `csp13/maximum-capacity-qualification-2026-09-30/verification.json`. This is a catalog component result. |
| Fresh replica history barriers | Delivery `d4f3ff7d`, integration `da58e084`. One parent plus nine native refusal scenarios passed in each explicitly recorded pure-Go/race mode. Checks compare complete bounded KV, SQL, object, and selected-file state before and after fresh-process normal construction. See `csp13/fresh-history-qualification-2026-09-30/verification.json`. This is separate from primary-file CLI proof. |
| Application CI | Delivery `1c95d5d8`, integration `5dde9e99`, review fix `8740bb71`. Five isolated groups times two modes require 29 active parents and 71 named subcase suffixes. Sixteen new verifier regressions passed. Existing native platforms remain. An idle child dispatcher was removed from counts. Hosted qualification remains UNVERIFIED. See `csp13/application-ci-integration-2026-09-30/verification.json`. |
| Combined native application groups | Clean `c6b10232` passed all thirteen namespace/operator/gateway parents in explicit pure-Go mode: 38 test results, zero failures/skips, 275.332 seconds. Actual shipping CLI bytes bind to that source. Combined race is UNVERIFIED. See `csp13/combined-application-retry-2026-09-30/verification.json`. |
| Independent history projection | Delivery `39c7cb9e`, integration `c6b10232`. Offline expected KV/SQL/blob state derives from the original backup and complete independently supplied typed history. Delivered pure/race: 57 results, twelve parents, four packages, zero failures/skips. Root's integrated pure run also passed 57. It grants no native approval. Consumer proof: `docs/proof/recovery-activation-20260930/history-projection.json`. |

Preserve failures. The first combined application run passed twelve parents and failed the namespace test's default evidence-directory fixture. The runtime correctly refused nonprivate permissions. Commit `0a5119a3` creates that private fixture through the productfiles API. Focused pure regression passed, then the full thirteen-parent pure retry passed.

The first fresh-history run without race still had CGO enabled. It is historical evidence, not pure-Go qualification. Final proof explicitly records the corrected toolchain settings. Additional test-enabled scoped lint reports four reviewed file/subprocess false positives and one duplicate sentinel string. Do not claim that optional lint run passed. Repository-policy lint and vet passed.

## Main challenges and unfinished source

### 1. Full embedded race deadline

`TestRecoveryActivationFullEmbeddedApplication` passes pure mode but fails the unchanged **420-second activation deadline** in race mode. The last checked-source race package took 484.958 seconds. Preserve the full catalog, assertions, and deadline.

Evidence: `csp13/full-embedded-checked-source-2026-09-30/verification.json`.

Private original logs, executable, and CPU profile: `/tmp/csp13-full-embedded-checked-source-20260930`.

The profile shows repeated catalog decoding and canonical role checks. External TSan frames consume substantial sampled CPU, but are not fully attributed. Reusing a deterministic semantic result is safe only after complete current byte, identity, access, and selection checks.

The stopped repair worktree has three unfinished files:

- Modified `internal/recovery/canonical_files_inspection.go`
- New `internal/recovery/artifact_identity.go`
- New `internal/recovery/original_artifacts.go`

These files have no formatting, compilation, test, or app-integration qualification. Do not present them as a delivered fix.

The proposed contract retains a private semantic proof after full original and target owner validation. Every reuse still checks complete hashes, lengths, key access, native file identities, parent/root identities, permissions, pending publications, configuration, and current selections. Public/restart inspection fully validates semantic content. It grants no authority or timer-based trust.

Next: review the partial capability code. Add observable counted-validator reuse tests and identical-byte inode replacement, root swap, changed configuration, and native identity refusal tests. Carry the capability through the application coordinator without changing sealed records or current permission checks. Then rerun the unchanged full native pure/race test.

### 2. Complete populated in-place adoption

Root integrated the independent offline projection. Complete native populated adoption is not implemented or qualified.

Contract: `csp13/populated-adoption-contract-2026-09-30/CONTRACT.md`.

The stopped adoption worktree preserves an unfinished SQL census-helper edit in `internal/sqlstore/relational_recovery_census.go`. It references undefined `relationalPopulatedPrefix`. That dirty successor does not compile. Its clean committed parent is the delivered projection boundary.

Two untracked, unrun drafts remain: `internal/app/recovery_restart_container_test.go` and `internal/app/recovery_restart_epoch_test.go`.

The required native design has these boundaries:

1. Each native owner checks exact independently captured preimages and retains an operation-bound restricted closure receipt.
2. Preserve every acknowledged domain byte, expiration, and historical receipt. Retire only owner-known current control roots through their owning contracts.
3. Keep ordinary empty-target `Claim` semantics unchanged.
4. Record the already-present nonfinal history prefix through a distinct checked journal mode. Do not replay acknowledged effects.
5. Bind independent prior approval, the actual closed SQL high-water epoch, the new native identity, and original backup/history evidence.
6. Use a monotonic closed-adoption epoch transition. Do not simulate an imported empty witness.
7. Keep cross-owner crash/retry states restricted. Preserve original operation identity and evidence. Do not recapture or renew authority during a retry.
8. Approve only after catalog, revision, native owner, and current permission checks complete.

External process/network fencing of all writers remains mandatory. A caller string, captured control, projection result, or empty-target restore is insufficient authority. Actual persistent restart, promotion, acknowledged-data loss, restored-SQL refusal, and measured recovery remain required. Aggregate projection scratch capacity also remains unqualified.

### 3. Qualification and production readiness

Acceptance-registration draft `58a157d5` has ten registered subcases and six missing mappings. Missing mappings include measured recovery, fresh-history, old-primary admission, native restart/epoch, operator reconciliation, and unprefixed migration. Bind mappings to exact qualified behavior and source. Source presence is not PASS.

Combined application race, remaining full application groups, native platform CI, final pre-PR checks, structured review, protected merge, released-pair acceptance, and cleanup remain open. Final production recovery targets need owner approval after measurement.

## Tests, services, and iteration rules

Several tests use the current native fixtures. Do not reset, stop, or remove them while a check uses them:

- `starport-csp13-postgres`
- `csp13-unprefixed-valkey-20260929`
- `starport-csp13-mysql`
- `starport-csp122-retirement-objectstore`
- `csp13-adoption-replacement-20260929`

The older `starport-csp13-valkey` container is absent. Derive fixture endpoints privately from Docker inspection. Never print credentials. Namespace qualification uses dedicated source DB13 and target DB14. Replacement-incarnation tests require an actual distinct native identity.

Private combined qualification artifacts are at `/private/tmp/csp13-combined-application-8740-20260930`. The directory name preserves an earlier source label. `retry-source.json`, build metadata, and proof bind the final run to `c6b10232`. Its race executable is older. Rebuild a race executable from the final clean source before source-bound race qualification.

The owner wants larger coherent iterations and useful tests. Use focused checks while implementing. Complete an operator flow before the full repository roster and review. Avoid repeated broad verification without a new source change or unresolved failure. Parallelize independent worktrees.

Serialize shared fixtures and source mutations. Do not weaken assertions or hide missing tests behind green counts.

## Saved conversation

Thread ID: `01a06e82-a5e3-7370-a2bc-6955da68c236`.

The latest local conversation log is:

`/Users/jack/.codex/sessions/2026/09/09/rollout-2026-09-09T10-40-41-01a06e82-a5e3-7370-a2bc-6955da68c236_01a086d4-14eb-74c0-8eff-950f7307284e.jsonl`

Root verified this path on September 30. It is approximately 1.30 GB and remains append-only conversation data. Use a bounded streaming reader for named decisions, compaction summaries, or evidence. Do not load the whole file into the model context. The original conversation contains provider credentials. Keep it local and omit credential values from prompts, proof, and publication.

Earlier segments are:

- `/Users/jack/.codex/sessions/2026/09/04/rollout-2026-09-04T17-20-51-01a06e82-a5e3-7370-a2bc-6955da68c236.jsonl`
- `/Users/jack/.codex/sessions/2026/09/05/rollout-2026-09-05T10-22-58-01a06e82-a5e3-7370-a2bc-6955da68c236_01a0722a-6d20-7020-a855-d797dde40d1e.jsonl`
- `/Users/jack/.codex/sessions/2026/09/05/rollout-2026-09-05T11-26-49-01a06e82-a5e3-7370-a2bc-6955da68c236_01a07264-e303-7631-a058-d3171a604dbc.jsonl`

## First actions after transition

1. Read the current resume state, CSP13 contract, this report, and current proof.
2. Verify all worktree heads and attribute unfinished source. Preserve unrelated work.
3. Resume the whole-plan goal in the receiving harness.
4. Continue the semantic timeout repair and native populated adoption in separate owning packages.
5. Review each coherent delivery before integration. Then qualify the combined source with explicit toolchain settings.
6. Measure supported recovery deployments and prepare concrete numeric target proposals for the owner.
7. Complete required acceptance registrations, full checks, structured review, exact-head CI, and protected merges.
8. Record actual merge commits before terminal ledger status. Continue the remaining plan tasks and final cleanup.

Ask only for genuinely missing product decisions or new authority. Explain concrete consequences before asking. Use the question tool when available. The owner repeatedly reported disappearing questions, so retain pending questions and decisions in the durable record.
