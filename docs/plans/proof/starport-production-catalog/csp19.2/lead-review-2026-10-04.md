# CSP19.2 lead review (2026-10-04)

Branch `csp192-write-history` at `9511a683` on Starport main `9f0ad373`. Six commits after the rebase onto the merged #413 with `git rebase --onto main 6553b692`. The implementer report is in `implementer-report-2026-10-04.md`. The design brief is in `design-brief-2026-10-04.md`.

## Review result

- `starport backup write-history` writes the final-only history package through `RestoreSource.WriteFinalHistory` in `internal/recovery/history_writer.go`. No shipped command writes prefix steps for activity after the backup.
- `app.WriteImportedHistory` validates the request, inspects the restore source, and checks the deployment identity. It captures the target digest and the SQL stamp, writes the package, and rechecks both. Drift after the write returns `recovery.ErrConflict`.
- The command refuses an unfenced attestation before it loads configuration. The CLI test proves the usage exit and zero configuration loads.
- `writeHistoryRefusals` covers four refusals: unfenced attestation, wrong manifest digest, wrong target digest, and populated directory. No refusal leaves a package behind, and a retained operator file survives.
- The lead asked for one more refusal test. `TestWriteImportedHistoryRefusesAnotherDeployment` in commit `9511a683` loads the same target paths under `STARPORT_DEPLOYMENT_ID=another-deployment`. The write refuses with `recovery.ErrConflict`, the directory stays empty, and the configured deployment then succeeds.
- The fleet harness gains the steps `fleet_restore_history_written` and `fleet_restore_activated`. The local-to-shared harness mode still stops before activation, which the docs state.
- The implementer open question 4 names two step orders for adoption. The target digest does not cover the catalog state directory, so both orders produce the same digest. The docs order stays.
- No protocol, storage layout, or public API changed outside the new command and the `recovery.WriteHistoryRequest` and `HistoryWriteReport` types.

## Lead verification at `9511a683` (gate 1)

| Check | Result |
| --- | --- |
| Six `internal/app` tests with the fixtures and the race detector | 6 PASS, 0 FAIL, `ok internal/app 474.6 s` |
| `TestRecoveryActivationLocalOperatorCommands`, `TestRecoveryActivationFullEmbeddedApplication`, `TestPopulatedAdoptionOperatorCommands` | PASS in 46.5 s, 280.6 s, 103.8 s |
| `TestWriteImportedHistoryBindsApplication`, `RefusesAnotherDeployment`, `BindsActivation` | PASS in 6.2 s, 3.1 s, 32.0 s |
| Recipe image tests, image `starport-storage-recipe:csp192` id `sha256:f71a5470…` | Compose, Persistence, LocalToShared, ReadOnlyMounts PASS. Fleet FAIL in 83.75 s |
| Pre-PR roster at `9511a683` | all rc 0 (`goago` exits 2 by design), head `9511a683`, dirty 0 |

The fleet failure came from `backup activate` inside the restored fleet container. The harness hid the command stderr, so the lead sent the diagnosis to the implementer.

## Fleet fix review at `2b775ea6`

Three commits after `9511a683`: `1027efd1` (build), `145ce2a4` (test), `2b775ea6` (docs). No Go source changed, so the gate 1 app results stand.

- Cause 1: the restored target had no current local admin token. The staged token from the backup stays inactive by design, and activation reads the current token. The shipped step is `starport auth rotate --no-secret`. The test `restore_local_access_test.go` already proves it after a prepare. The harness now runs it after `backup prepare`. The three restore documents name the step before activation.
- Cause 2: the image created `/var/lib/starport/{config,data,state,cache}` with mode `0755`. Activation inspects the data root through `productfiles.ExistingDirectory`, which refuses group and other bits. The Dockerfile now copies the four roots with `--chmod=0700`, and `recipes.md` states the mode. CI builds the image with buildx, which supports `--chmod`.
- The harness error now carries the command stderr for the six backup commands. The text redacts URL credentials and stops at 4000 bytes. Stdout stays out of the error.
- Implementer evidence: `TestFleetRecipeContainerRecreation` PASS in 206.79 s with all 12 observations, image `starport-storage-recipe:csp192-modes` id `sha256:ffa3f755…` from the clean tree at `2b775ea6`.

## Lead verification at `2b775ea6` (gate 2)

| Check | Result |
| --- | --- |
| Five recipe image tests with image `csp192-modes` id `sha256:ffa3f755…` | 5 PASS, 0 FAIL, `ok 247.9 s`. Compose 0.20 s, Persistence 14.28 s, LocalToShared 15.61 s, Fleet 209.66 s, ReadOnlyMounts 7.40 s |
| Pre-PR roster at `2b775ea6` | all 36 checks rc 0 (`goago` exits 2 by design), `go test ./...` 832 s, head `2b775ea6`, dirty 0 |
| Autoreview `--gate pre-pr --mode auto` | Sol 6.1 high, one pass, no findings, "patch is correct (0.97)" |

## Addendum: rebase and pull request (2026-10-04)

The lead rebased the branch onto Starport main `e17ea25a` with `git rebase --onto origin/main 9f0ad373`. The only overlap with the merged #414 was `docs/TASKS.md`, and the resolution keeps both Active Work entries. The branch is now nine commits at `71c2a651`. The diff against main outside `docs/TASKS.md` is identical to the reviewed diff at `2b775ea6`, so the fixture-backed evidence above binds the rebased tree.

Focused checks at `71c2a651`: `go build ./...` and `go vet ./...` rc 0. The packages `internal/recovery`, `internal/cli`, `internal/config`, and `cmd/starport` pass without the fixtures. `verify-doc-links.sh` PASS. `docs/TASKS.md` keeps its 6 base diagnostics. Autoreview reused the clean attestation, because the substantive diff did not change.

Draft PR #415 opened at `71c2a651` with the title `recovery: add backup write-history and activate the fleet restore (CSP19.2)`.

## Limits recorded

- The post-write drift recheck in `internal/app/write_history.go` lines 98 to 106 has no direct test. No hook lets a test change the target between the write and the recheck.
- The local-to-shared harness mode stops before activation. The fleet mode activates but starts no gateway after activation.
- The `docs/site/storage/migration.md` lint base is 4 diagnostics. The CSP19.2 edit adds none.
- The DECISIONS.md 19-9 resolution paragraph waits for the fleet run to pass.
- A volume created from an older image keeps mode `0755` on the state roots, and activation into it refuses. The documented restore uses fresh targets.
