# CSP19.2 design brief (lead decisions, 2026-10-04)

Source: the CSP19.2 design exploration (Opus 5.5 explorer, read-only) on Starport main `149303fc` and the CSP19 worktree at `6553b692`. This file records the lead decisions on the open questions of that exploration. The facts below are the ones the implementer needs.

## Key correction to decision 19-9

`ProjectIndependentHistory` (`internal/recovery/history_projection.go:81`) consumes a history package and writes a projection workspace. It does not write a package. The only writer is the test helper `activationHistoryFixture` (`internal/app/recovery_activation_fixture_test.go:45-129`). The decision record carries the correction.

## Decisions

- D1 Command name: `starport backup write-history`. Not `project-history`, because that verb names the consumer.
- D2 Writer owner: new `internal/recovery/history_writer.go` beside `history_package.go`, which lets it serialize the unexported `historyManifest`. API: `func (s *RestoreSource) WriteFinalHistory(ctx, HistoryWriteRequest) (*VerifiedHistory, error)`.
- D2a Writer contract: it derives `backup_sha256`, `deployment_id`, and `prepared_sha256` from the source. It refuses with `ErrConflict` unless all three attestation values are true. It refuses a non-empty directory (pattern at `history_projection.go:116-120`). It writes the payloads first and `history.json` last. It verifies its own output through `VerifyHistoryPackage` before it returns.
- D3 Scope: final-only packages (KV final, then SQL final). Mode `planned_migration` or `disaster_recovery`. Disposition `replay_complete` or `remain_restricted`. The writer replaces `activationHistoryFixture` and every helper that builds the same final-only package. Deliberate negative tests and the `internal/recovery` unit tests keep their hand-built prefix, zero-step, and malformed packages. The proof records how the lead reads "no test builds the package by hand".
- D4 SQL stamp source: the live target through `CaptureSQLRecovery`, as the fixture and `ApplyImportedHistory` do. The command recomputes the target digest as its last step and refuses with `ErrConflict` on drift. The KV value comes from the backup capture (`OpenCapturedKV` and `CaptureKVRecovery`).
- D5 Attestation: no manifest format change. The writer requires the attestation and does not store it. Digests keep the package bound.
- D6 Evidence: `--evidence-file id=path[=reference]`, repeatable. The command computes the sha256 and the size. `--epoch-evidence <id>` names the evidence file that holds the epoch source. The operator types no digest.
- D7 Flags: the base set of `apply-history` (directory, manifest-sha256, operation, fencing-evidence, scratch, valkey-incarnation, operator, attestation-reference, json). The three attestation flags stay Required bools. New: history-directory, expected-target-sha256 (optional), mode, disposition, through (RFC3339, required, no default), end-reference, highest-epoch, epoch-reference, epoch-operator, evidence-file, epoch-evidence.
- D7a Output: `History SHA-256: … Retain this digest outside the package.` and `Target SHA-256: …`.
- D8 App composition: `app.WriteImportedHistory(ctx, cfg, request) (HistoryWriteReport, error)` in `internal/app/write_history.go`, wired through `Dependencies` (`internal/cli/app.go:87-130`) and `cmd/starport/run.go`. It reuses `backupEncryption`, `InspectRestoreSource` (the backup digest refusal comes with it), the deployment-ID check (`apply_history.go:39`), `validateHistoryTargets` (which keeps the paths separate), `restoreBlobTarget`, and `configuredRecoveryTarget`.
- D9 Tests: recovery unit tests for the round trip through `VerifyHistoryPackage`, each refusal, a partial directory without a manifest, and determinism. A CLI test after `internal/cli/apply_history_test.go` and the verb in `backup_test.go:55`. An `internal/app` test that writes, applies, and activates against the real fixtures. The fixture chain calls the app function, and the CLI tests drive the binary. The fleet harness runs the command after `scripts/test-storage-recipes.py:595` with new observations in `recipe_fleet_test.go`.
- D10 Docs: `RECOVERY.md` (lines 81, 209, 253), `backup-and-restore.md:102`, and `migration.md` (lines 150, 191). Also `recipes.md` (lines 170 to 171, the T4 limit), `FLEET_LIMITS`, `internal/recovery/README.md` (lines 644 to 649), and the `TASKS.md` entry.

## Facts the implementer must respect

- Manifest validation lives at `history_package.go:175-233`. Version 1, digests, `through_utc` not before the backup `FinishedAt`, `highest_epoch` at or above the boundary epoch. The epoch `source_sha256` equals one evidence source. Steps are `payloads/%06d.json`, 1 byte to 8 MiB each, 64 MiB in total.
- Final-step order: KV final, then SQL final, as the last two steps (`history_prefix.go:26-36`, kinds at `history_runner.go:23-24`). Payload shapes: `history_payload_kv.go:204` and `history_payload_sql.go:77`.
- Capture functions: `CaptureKVRecovery` (`internal/authorization/revision/recovery_kv.go:73`) and `CaptureSQLRecovery` (`recovery_sql.go:67`). Check `scripts/verify-dependency-direction.sh` before `internal/recovery` imports `revision`. `history_payload_sql.go` already uses `revision.Stamp`.
- About 20 call sites in `internal/app` reach the fixture through `activationPreparedFixture`, `activationFleetFixture`, `populatedAdoptionFixture`, `measurementAdopt`, and `measurementImport`. `recovery_measurement_test.go:298-315` rewrites `through_utc` to now. The `--through` flag covers it.
- The package binds to the target digest at write time. The command runs after `backup prepare` and `inspect-import`, before apply or activate, against the fenced target.
- The fleet container needs a writable package path under `/home/nonroot`, outside the target paths and the bundle. The implementer verifies it.

## Not yet checked by the exploration

`TASKS.md` conventions for a CSP19.2 entry, `ARCHITECTURE.md`, the binary-driving pattern in `recovery_activation_cli_test.go`, the `ImportInspectionResult` fields, and the dependency-direction rule.
