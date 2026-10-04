# CSP19.2 implementer report

## Outcome

`starport backup write-history` is in place, with tests and docs, in five commits on branch `csp192-write-history`. The worktree is `/private/tmp/starport-csp192-20261004`.

- The commits are based on `6553b692`. I did not rebase onto main or `9f0ad373`.
- The worktree is clean. I did not push.
- The new fleet harness steps are written, but no test has run them. The lead stopped fixture-backed runs before the fleet run. Of the planned checks, only the fleet run and the full `./internal/app/` run remain.

## Commits

| hash | subject | contents |
| --- | --- | --- |
| `df1da034` | recovery: add the final history package writer (CSP19.2) | `internal/recovery/history_writer.go`: `RestoreSource.WriteFinalHistory` (see the detail below). The unit tests cover the round trip, each refusal, a partial directory with no manifest, and determinism. |
| `66d79c52` | cli: add backup write-history (CSP19.2) | `app.WriteImportedHistory` in `internal/app/write_history.go`, the `Dependencies` wiring, `cmd/starport/run.go`, the `backup write-history` verb, CLI tests, and the verb in `backup_test.go`. |
| `0b7d7c23` | test: build every final-only history package through the shipped writer (CSP19.2) | Changes to the fixture chain (see the detail below) and the new `internal/app/write_history_test.go`. |
| `8693bcfa` | test: write the history package and activate in the fleet recipe (CSP19.2) | Changes to `scripts/test-storage-recipes.py` and `internal/config/recipe_fleet_test.go` (see the detail below). |
| `bb5e2176` | docs: route the bind step through backup write-history (CSP19.2) | The six doc files in the D10 section. |

The commits have no AI attribution or co-author lines. A grep of the commit trailers found no match.

Commit detail:

- `df1da034`, `WriteFinalHistory`:
  - It writes a final-only package: KV final, then SQL final.
  - It derives `backup_sha256`, `deployment_id`, and `prepared_sha256` from the source.
  - It requires all three attestation bools.
  - It refuses a non-empty directory.
  - It writes the payloads first and `history.json` last.
  - It verifies its own output through `VerifyHistoryPackage` before it returns.
- `66d79c52`, `app.WriteImportedHistory`:
  - It inspects the restore source and checks the deployment ID and path separation.
  - It gets the KV value from the backup capture and the SQL stamp from the live target.
  - It recomputes the target digest last and refuses with `ErrConflict` on drift.
- `0b7d7c23`, the fixture chain:
  - `activationHistoryFixture` now builds the package through `app.WriteImportedHistory`, with new helpers: `activationPreparedFixtureWith`, `activationHistoryWith`, `activationHistoryWriter`, `activationHistoryCommand`, and `writeHistoryArguments`.
  - The binary-driven CLI chain calls `backup write-history`.
  - `measurementWriteHistory` replaces the `through_utc` rewrite.
  - `localOperatorDependencies` is complete.
  - The new `write_history_test.go` holds `TestWriteImportedHistoryBindsApplication`, `TestWriteImportedHistoryBindsActivation`, and the `writeHistoryRefusals` table.
- `8693bcfa`, the fleet harness:
  - New `private_archive`: a tar with uid/gid 65532, dirs 0700, and files 0600. `source_archive` now uses it.
  - A new `place()` puts the archives into the backup volume with `compose create` and `docker cp -a -`.
  - After `fleet_restore_import_inspected`, the harness writes the package and activates the restored target. Then it records `fleet_restore_history_written` and `fleet_restore_activated`.
  - The fleet test expects the two new observations.
  - The FLEET_LIMITS and local-to-shared limit text are updated.

## Commands and results

The Go environment is `GOTOOLCHAIN=go1.27.1 GOWORK=off CGO_ENABLED=1` with the shared `GOCACHE`. All logs are in the scratchpad.

| command | exit | result |
| --- | --- | --- |
| `gofmt -l` on the Go files changed since `6553b692` | 0 | No files. The repository-wide `gofmt -l` lists only the existing `internal/recovery/catalog_expiring_preparation_test.go`. I did not touch it. |
| `go vet ./...` | 0 | Clean. |
| `make lint` (`csp192-lint.log`) | 0 | 0 issues. It ran after all edits were final. |
| `make build` (`csp192-build.log`) | 0 | Build complete. |
| `scripts/verify-dependency-direction.sh` | 0 | Pass. |
| `scripts/verify-v1-architecture.sh` (`csp192-v1arch.log`) | 0 | 11 passed, 0 failed. |
| `STARMAP_OWNERSHIP_STARMAP_ROOT=$HOME/src/github.com/agentstation/starmap scripts/verify-starmap-ownership.sh` (`csp192-ownership.log`) | 0 | 12 passed, 0 failed. |
| `scripts/verify-doc-links.sh` | 0 | Pass. |
| Technical-writing lint on each changed doc | see the next column | 0 errors each, except two files at their base counts: `migration.md` (4) and `TASKS.md` (6). The brief expected 0 for `migration.md`. |
| `go test -race -count=1 -timeout 45m ./internal/recovery/` (`csp192-recovery-race.log`) | 0 | `ok` in 960.654s. |
| `go test -race -count=1 -timeout 45m ./internal/cli/` with the fixture variables unset (`csp192-cli-race.log`) | 0 | `ok` in 2.786s. 125 top-level tests passed. |
| `go test -run '^TestDeclaredRecipePages$\|^TestRecipeLatencyProfiles$' ./internal/config/` | 0 | Pass. |
| Fleet skip proof: `env -u STARPORT_RECIPE_IMAGE -u TEST_VALKEY_URL -u TEST_POSTGRES_URL -u TEST_BLOB_S3_ENDPOINT go test -count=1 -run '^TestFleetRecipeContainerRecreation$' ./internal/config/ -v` | 0 | SKIP with the required-variables message. `ok` in 0.706s. |
| `python3 -m py_compile scripts/test-storage-recipes.py` | 0 | Compiles. I removed the generated `__pycache__`. |
| `docker build -q -t starport-storage-recipe:csp192` (`csp192-image-build.log`) | 0 | 50s, `sha256:213fe0caaf41...` |

## Focused fixture runs (through the acceptance wrapper)

These are separate from the full `./internal/app/` run, which is in the next section.

Run 1 (`csp192-focused.log`, start 05:22:37Z, `-timeout 30m`):

- 18 tests passed and 0 failed.
- Then the run reached the 30-minute timeout during `TestRecoveryActivationFullEmbeddedApplication`. The package result is a `FAIL` that the timeout caused, not a test failure.
- The passes:
  - TestApplyImportedHistoryRetainsClosedIdempotentJournal 4.71s
  - LocalToShared PopulatedWorkload 76.69s
  - LocalToShared Refusals 26.51s
  - LocalToShared PromotionAfterMove 91.03s
  - LocalToShared SecondReplicaJoins 73.85s
  - LocalToShared Rollback 52.04s
  - TestRecoveryActivationLocalOperatorCommands 35.26s
  - ExactPendingPhaseRestart 450.74s
  - JournalExactPublicationProcessExit 12.94s
  - ReleasesDecodedCatalogScope 30.58s
  - SealedRetryRequiresOriginalSelector 27.48s
  - FreshProcessNativeBoundaries 189.46s
  - CurrentChoiceOrOriginalEvidenceFailureKeepsRemainingOwnersClosed 211.42s
  - CompletedRetryPreservesLaterWithdrawal 34.97s
  - SQLApprovalRejectsChangedCurrentSource 31.00s
  - SQLApprovalRejectsOtherTopologyBackend 30.44s
  - NativeTopologyRecipes 199.60s
  - BoundedSourceNormalStartup 59.51s

Run 2 (`csp192-focused2.log`, start 05:53:36Z, `-timeout 120m`, `-skip` for the 18 passed tests):

- Wrapper exit 0, native test exit 0. `ok` in 821.355s, 13m50s wall time.
- The passes:
  - TestRecoveryActivationFullEmbeddedApplication 323.34s
  - TestRecoveryPopulatedOperatorCommandsAcrossNativePhaseCut 67.25s
  - TestPopulatedAdoptionOperatorCommands 114.60s
  - TestPopulatedAdoptionProcess 234.76s
  - TestRecoveryActivationReplaysActualPostBackupRevocationSpendAndUncertainDispatch 38.86s
  - TestWriteImportedHistoryBindsApplication 6.15s
  - TestWriteImportedHistoryBindsActivation 34.01s

Total: 25 focused tests passed and 0 failed.

Development iterations (`csp192-app-iter1..3.log`):

- Iterations 1 and 2 failed in the earlier combined test `TestWriteImportedHistoryBindsApplicationAndActivation`.
- I split it into the application and activation tests and replaced `assertInspectionStillClosed` in the activation path.
- Iteration 3 passed in 51.369s.

## Checks not run

- `TestFleetRecipeContainerRecreation` with `STARPORT_RECIPE_IMAGE=starport-storage-recipe:csp192`. The lead stopped fixture-backed runs. The new `fleet_restore_history_written` and `fleet_restore_activated` steps have not run. The docs already say that the fleet recipe test writes the package and activates. If the fleet run fails, `bb5e2176` must be adjusted.
  - Likely risks: activation in fleet mode (the catalog source), `compose create` with the volume override, the `docker cp -a` ownership on the distroless image, and the 900s activation timeout.
  - The earlier fleet run took 106s. The test context is 10 minutes.
- The full `go test ./internal/app/` through the wrapper, which was planned once at the end. It is not run, for the same reason.
- `TestRecoveryMeasurement` is gated by `STARPORT_RECOVERY_MEASUREMENT_DIR` and did not run. It now calls `measurementWriteHistory`.

## Deviations

- `-timeout 45m` for the race test of `./internal/recovery/`, as the lead allowed.
- The focused run was split into two runs, and the second used `-timeout 120m`.
- `app.WriteImportedHistory` recaptures the SQL stamp after it writes, before the final target-digest recheck. This goes beyond D4.

## D1 to D10 status

- D1 (name `backup write-history`): done (`66d79c52`).
- D2 (writer in `internal/recovery/history_writer.go`, all listed refusals, payloads first and `history.json` last, self-verification): done (`df1da034`).
- D3 (final-only scope, replace the fixture writers, keep the negative and unit packages): done (`0b7d7c23`).
  - Replaced: the body of `activationHistoryFixture`, and the `through_utc` rewrite in `recovery_measurement_test.go` (now `measurementWriteHistory`).
  - Kept by hand, because they are deliberate prefix, zero-step, or malformed packages:
    - `postBackupHistory`
    - `populatedPrefixHistory`
    - the zero-step `remain_restricted` package in `apply_history_test.go`
    - the `internal/recovery` unit-test packages
    - the negative rename tests
  - The proof must record this reading of "no test builds the package by hand".
- D4 (KV from the backup, SQL from the live target, final digest recheck with `ErrConflict`): done. See the SQL recapture in the deviations.
- D5 (no manifest format change, attestation required and not stored): done.
- D6 (`--evidence-file id=path[=reference]` and `--epoch-evidence`): done.
- D7 (flags, required bools, `--through` required, output lines): done.
- D8 (app composition through `Dependencies` and `cmd/starport/run.go`): done.
- D9 (tests):
  - The recovery unit tests, the CLI tests, the app tests, and the fixture chain are done.
  - The fleet harness is written but not run.
- D10 (docs): done, with two exceptions.
  - `docs/OPERATOR-GUIDE.md` does not list the backup verbs, so it is unchanged.
  - The DECISIONS.md 19-9 amendment is not applied. See the open questions.

## Fail-before evidence

`csp192-fail-before.txt` is a 35-line grep at HEAD `6553b692`.

- `ProjectIndependentHistory` (`history_projection.go:81`) has only test callers.
- No non-test code writes `history.json`. The only match is the reader at `history_package.go:131`.
- `git grep write-history` gives exit 1, which means that the verb was absent.

## Open questions

1. The DECISIONS.md 19-9 amendment is not applied.
   - The file is `/Users/jack/src/github.com/agentstation/starmap-catalog-requirements/docs/plans/proof/starport-production-catalog/csp19/design-2026-10-03/DECISIONS.md`, entry 19-9 near line 62.
   - That worktree (branch `codex/catalog-qualification`) has unrelated uncommitted changes. The brief prohibits work in other worktrees.
   - The text is in `csp192-decisions-19-9-amendment.md` in the scratchpad. Append it after the 19-9 "Owner decision on 2026-10-03" line:

   > Correction on 2026-10-04: `ProjectIndependentHistory` in `internal/recovery/history_projection.go` line 81 does not write a history package. It consumes a package and writes a projection workspace. Before CSP19.2, the only writer was the test helper `activationHistoryFixture` in `internal/app/recovery_activation_fixture_test.go`. The `backup project-history` option therefore named the consumer, not a writer.
   >
   > Resolution in CSP19.2: `starport backup write-history` writes the final-only package through `RestoreSource.WriteFinalHistory` in `internal/recovery/history_writer.go`. The fleet harness now writes the package and activates the restored target with the image. No shipped command writes prefix steps for activity after the backup.

   The second paragraph claims that the fleet harness activates. Hold it until the fleet run passes.
2. No direct app test covers the refusal when the post-write target recheck finds drift. No direct app test covers the deployment-ID mismatch refusal. The refusals table covers the other refusals.
3. The local-to-shared harness mode still stops before activation. Only the fleet mode writes history and activates. The fleet mode starts no gateway after activation, and FLEET_LIMITS says so.
4. Order of steps for adoption:
   - The docs put `write-history` before the catalog state directory step.
   - The fixture writes the package after it makes the state directory.
   - The target digest does not cover the catalog state directory, so both orders give the same digest.
5. The `migration.md` technical-writing lint base is 4, not the 0 that the brief expected. My edit adds no finding.
6. `#413` merged at `9f0ad373`. The lead must rebase with `git rebase --onto main 6553b692`.

## Addendum (after the lead rebase to 41e9e9e0)

- The new commit is `9511a683` "test: refuse write-history for another deployment (CSP19.2)". It has no attribution trailer, and the worktree is clean.
- The commit adds `TestWriteImportedHistoryRefusesAnotherDeployment` in `internal/app/write_history_test.go`, beside `TestWriteImportedHistoryBindsApplication`. The test checks the following:
  1. It loads a second configuration with the same target paths and `STARPORT_DEPLOYMENT_ID=another-deployment`.
  2. `WriteImportedHistory` refuses with `recovery.ErrConflict` and returns a zero report.
  3. The history directory stays empty.
  4. The same request then succeeds under the configured deployment.
  5. The import barriers stay closed.
- The fixture is `importedInspectionFixture`, which uses local Badger, SQLite, and filesystem backends. The test needs no `TEST_*` variable and no Docker fixture.
- The test did not run. The `-race` run with every `TEST_*` variable unset failed at link time with `no space left on device`. The free space on `/System/Volumes/Data` was 371 MiB, and the log is `csp192-deployment-refusal.log`.
  - I removed my own git-ignored `make build` binary from the worktree (167 MB). Free space then rose to 518 MiB.
  - I did not start the run again, because another `internal/app` race test binary could take the space that the CSP19 acceptance run needs.
  - I did not delete other scratchpad items (2.9 GB in all), because earlier sessions made them.
- `gofmt -l` on the file and `go vet ./internal/app/` both exit 0.
- To run the test: `go test -race -count=1 -run '^TestWriteImportedHistoryRefusesAnotherDeployment$' ./internal/app/`. No fixture variables are necessary.
- The post-write drift recheck (`write_history.go:98-106`) stays untested, as the lead decided.
