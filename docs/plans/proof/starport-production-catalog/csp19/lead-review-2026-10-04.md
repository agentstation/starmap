# CSP19 lead review (Starport branch `csp19-recipes`, 2026-10-04)

Merged as Starport PR #413, squash commit `9f0ad373`, 2026-10-04T16:36:23Z. CI at `4117a4a6`: 42 of 42 SUCCESS. The reviewed tree equals the merge tree. Proof in `starport413-merge-2026-10-04/`.

## Delegated work

The implementer (Opus 5.5, high effort) delivered two commits at the pre-rebase head `284a849c` on `1f18b216`:

- `7b793faa` deploy: run the Compose recipes on a read-only root file system (CSP19). Both Compose files set `read_only` with tmpfs scratch mounts. The harness gains `--mode readonly` and `--mode fleet`. Two new Go tests cover the read-only mounts and the fleet container recreation.
- `284a849c` docs: add deployment recipes for targets T1 to T7 (CSP19). The recipes page, the status cells per decision 19-3, and the egress columns. Also the update policy selector, the backend version table, and two docs tests.

The implementer report lists the fail-before evidence at `491d9281`. Both docs tests FAIL, the Compose files have zero `read_only` lines, none of the four test names exists, and a replica row claimed MySQL. Mutation checks confirmed that each docs assertion fails on a changed status, a renamed section, or a changed latency value.

## Lead changes after the delegated work

- `fcf277a9` (folded into the rebase): the fleet harness prints `ps -a` and the last 200 gateway log lines before the readiness assertion fails. The harness redacts store URLs in the log. The attempt count and the assertion stay the same. Reason: one lead run failed with `fleet replica 0 did not become ready` after 211 s and left no log.
- Rebase onto `149303fc` (#412). The harness conflict kept both designs: `--mode` from CSP19 and `--local-to-shared` from CSP19.1, with `run(command, data=None, timeout=90, refusal=None, check=True)`. The per-volume cold backup from CSP19 stays, because the read-only root refuses `docker cp` of `/var/lib/starport/`.
- `6553b692` docs: align the fleet status text with the T4 qualification (CSP19). The pre-CSP13 fleet status text in `OPERATOR-GUIDE.md`, `FLEET_INITIALIZATION.md`, and `storage-selection.md` now names the populated adoption procedure and the T4 limits. The T3 limit points at the local-to-shared move. The Storage Recipes CI job also runs `TestContainerRecipeReadOnlyMounts`.
- `4117a4a6` test: normalize CRLF in the recipe docs tests (CSP19). The first CI run of #413 failed `Test (windows-11-arm)` in both docs tests. The Windows checkout converts the docs to CRLF, and the heading comparison kept a trailing carriage return. `readRepositoryDoc` now normalizes CRLF. Proof in a throwaway worktree with the four docs converted to CRLF. Both tests FAIL before the fix and PASS after it. Autoreview at `4117a4a6`: clean (Sol 6.1 high, one pass, TruffleHog clean).
- The same CI run lost `Recovery test (windows-2025, 1)` to a 20-minute package timeout in `internal/recovery`. The last running test was `TestBundleIdentityReferencesPreserveGrantsAndRefuseCorruption/deleted-account`. CSP19 does not change that package. The shard passed on the #412 run and on the two main runs before it. The second run at `4117a4a6` decides whether it reproduces.

## Lead verification at `6553b692`

Image `starport-storage-recipe:csp19-final` (`sha256:b859b0429a17…`) built from this head. Clean worktree.

- Docs tests (`TestDeclaredRecipePages`, `TestRecipeLatencyProfiles`): PASS.
- CI pair plus read-only mounts with the image: PASS in 31.4 s. Compose 0.31 s, Persistence 21.87 s, ReadOnlyMounts 7.30 s.
- `TestContainerRecipeLocalToShared`: PASS in 20.1 s.
- `TestFleetRecipeContainerRecreation` against the real Valkey, PostgreSQL, and MinIO fixtures: PASS in 106.4 s, all 10 observations.
- Fleet history on this branch: 1 FAIL (211 s, readiness under host load at `284a849c`). 4 PASS (83 s, 363 s, 367 s, 106 s). The one failure has no identified cause. The harness now keeps the gateway log for the next failure.
- Skip proof without `STARPORT_RECIPE_IMAGE`: the three image tests and the fleet test SKIP and name the missing variables.
- Roster: all 34 steps rc 0 except `goago` (rc 2, its constant exit), head `6553b692`, dirty 0.
- Autoreview `--gate pre-pr --mode auto`: clean (Sol 6.1 high, one pass, P0 threshold, 0.97 correct, TruffleHog clean).

## Implementer open questions and their resolution

1. CI coverage: `TestContainerRecipeReadOnlyMounts` joined the Storage Recipes job. The fleet test needs the TEST_ fixtures. The local-to-shared test needs disposable containers inside the five-minute budget. Both stay registry-only, a documented limit.
2. `TestContainerRecipePersistence` parses `CombinedOutput`: pre-existing, outside CSP19. The gate runs with `PATH=/usr/bin:$PATH`.
3. Harness conflict with CSP19.1: resolved in the rebase (above).
4. Stale `FLEET_INITIALIZATION.md` text: fixed in `6553b692`.
5. No Unreleased CHANGELOG section: the release task owns the changelog.
6. Host-gateway reachability on Linux: not verified. The fleet test is registry-only, and the acceptance run uses macOS.
7. Test imports of the aws, pgx, and valkey clients in `internal/config`: accepted. The tests exercise real backends by design.
8. The MySQL claim line number: a report correction, no change.
9. `storage-selection.md` and `backup-and-restore.md` against the new T4 status: fixed in `6553b692`. The `backup-and-restore.md:114` sentence is a recovery-procedure claim and stays.
10. "One subscription for each replica" in `DEPLOYMENT-TOPOLOGIES.md:175`: that paragraph describes T1. The shared-fleet rule is in `topologies.md:43`. No change.
11. TASKS "Last Updated": updated in the CSP19 entry.
12. No command produces the independent history package: decision 19-9, task CSP19.2.

## Verdict

Accepted for the draft PR. The delegated code needed no correction. The lead added diagnostics, the rebase, and the alignment edits.
