# CSP19.1 lead review (2026-10-04)

Branch `csp191-migration` at `1528da52` on Starport main `1f18b216`. Eight commits, 13 files, +1210/-16. PR #412 merged as `149303fc` on 2026-10-04 (42 of 42 checks, reviewed tree equals the merge tree, record in `starport412-merge-2026-10-04/`). The implementer report is in `implementer-report-2026-10-04.md`.

## Review result

- The source changes add typed refusals only: `recovery.ErrConflict` on a deployment mismatch in `internal/app/restore.go`, `blob.ErrImportTargetPopulated`, and `recovery.ErrBackupKey`. No CLI mapping, storage layout, protocol, or public API changed.
- The five `internal/app` tests fence by stopping the processes they own. The procedure keeps the external fence mandatory.
- `TestLocalToSharedMigrationPromotionAfterMove` runs the gateway, because `executePromotions` starts only in `catalog.Runtime.Start`. The three-minute wait and every assertion stayed in place. The implementer reported the fail-before run (265.69 s, "baseline promotion is still pending").
- The catalog test asserts the leader republication over the carried head and promotes with the carried revision as `ExpectedRevision`.
- The docs name the eight steps, the parity check, the second replica, the rollback, and the limits. The lint passes on every touched document except the five pre-existing `docs/TASKS.md` diagnostics.

## Lead verification at `1528da52`

Real backends through the fixture wrapper: PostgreSQL 16.15, Valkey 7.2.14 (plaintext, private network), MinIO. Race detector on.

| Check | Result |
| --- | --- |
| `go test -race -run '^TestLocalToSharedMigration' ./internal/app/` | `ok` 380.8 s, 5 of 5 PASS, 0 skips |
| PopulatedWorkload parity | keys=3 accounts=2 credentials=2/2 files=2 users=2 teams=1 memberships=2 grants=2 budget_windows=2 budget_records=11 usage=4 audit=3 |
| PromotionAfterMove | moved head revision=2, promotion applied previous=2 promoted=4, leader named |
| `go test -race -run '^TestLocalToSharedPromotionAfterMove$|Promot' ./internal/catalog/` | `ok` 1163.9 s |
| `TestLocalToSharedPromotionAfterMove` verbose | PASS 207.02 s |
| Pre-PR roster | 34 checks rc 0, `goago` rc 2 (normal), `dirty=0` |
| Autoreview `--gate pre-pr --mode auto` | Sol 6.1 high, one pass, no findings, "patch is correct (0.99)" |

Durations ran under concurrent load from the roster and the CSP19 image build.

## Implementer evidence (reported, not repeated by the lead)

- Separate fixture run: 5 of 5 PASS in 534.3 s.
- Catalog race run `-run 'Promotion|Promote|Topology|Baseline'`: `ok` 1600.1 s.
- `TestContainerRecipeLocalToShared` PASS with the recipe image (commit `d6399d3c`). Activation stops before the history package.
- Skip proof without the variables: the five app tests and the container test skip with `UNVERIFIED` messages.
- Fail-before: the refusals carried untyped strings, and the promotion test failed without a running gateway.

## Limits recorded

- External fencing of every writer stays mandatory. Nothing retires the closed local source.
- The container test stops before activation. CSP19.2 owns the history package command.
- The fixture team has no budget, so team budget history is not tested.
- CI does not select the local-to-shared tests. The Starmap registry runs them under `A31.upgrade_restore_workload`.
- The procedure is not qualified for production.
