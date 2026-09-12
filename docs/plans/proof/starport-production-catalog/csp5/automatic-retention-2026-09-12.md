# CSP5 automatic retention candidate

This record captures a working-tree candidate on `codex/catalog-update-controls`, based on `292fe261`.
The [committed qualification](qualification-2026-09-12.md) records the later consolidated candidate.
The [verification record](automatic-retention-2026-09-12/verification.json) binds 32 current task files, 34 checks, and 132 deduplicated artifacts.
Five unrelated tooling edits remain outside the task changes. No CSP5 PR or merge exists.

## Verified behavior

The connected runtime now schedules local collection independently of source refresh.
Manual, offline, and pinned runtimes remove eligible input files and old generations without replacing the served catalog.
Tests preserve unknown input files, required generations, and the original configured pin through two authority-origin restarts.
Periodic collection repeats, and shutdown stops its worker before releasing directory ownership.

Six canonical settings control scheduling, interval, generation count, generation bytes, scan entries, and input snapshot bytes.
The defaults enable hourly collection with 32 generations, 512 MiB of generation bytes, 4,096 entries, and 256 MiB of input bytes.
All numeric limits require positive values. Required content can exceed the generation targets.
The settings schema now contains 45 descriptors. Starmap references and the environment example contain the new settings.

Starport adoption remains separate work.

Readiness exposes capacity and maintenance diagnostics from runtime memory.
Pending publication recovery prevents automatic deletion. A failed cleanup does not replace the served catalog.
The client collector serializes with updates and protects its served catalog and stored embedded baseline.
The permission wrapper checks authority identity, supported semantics, and the expected stored generation.

Incoming reset requests now use the compactor before the history byte limit rejects another update.
The corrected fixture starts with 47 observations within the limit and proves that the next observation crosses it.
Ordinary and reset updates both succeed after compaction. Restart reproduces the accepted generation.
The earlier 52-observation fixture did not prove a valid starting state and remains historical evidence.

## Checks

The original 31-file candidate produced the broad results below.
The later runtime snapshot correction has separate source bindings and focused results.

| Check | Result |
| --- | --- |
| Selected runtime, client, permission, settings, and readiness tests | 102 passing race events on Go 1.26.6 and 102 on Go 1.25.12. No skips. |
| Clock and RPC packages | 69 passing race events and one skip on each Go toolchain. |
| CSP5 acceptance verifier | Twelve selected subcases pass, with 42 race events across 21 Go invocations. |
| Recursive lint across affected packages | Pass, zero issues. |
| Go policy across the repository | Pass. |
| Maintained prose and glossary | Pass. |
| Windows amd64 compilation | Seven packages compile. This command executes no Windows tests. |

The clock skip is `TestWindowsMonitorFactoryRequiresSourceBounds` on this macOS host.
The clock correction separates native evidence and security setup from shared validation and transport ownership.
Unsupported hosts still refuse before sending RPC bytes and close their stream.

The record preserves failed attempts. Disconnecting automatic retention produces failures in manual, offline, and pinned modes.
This control changes only the scheduler connection. It does not represent an unchanged historical commit.

The first integration attempt exposed the pending-recovery diagnostic order.
The record also preserves four initial staticcheck warnings, an unsuccessful platform split, and two prose findings.
The final candidate resolves these findings without changing lint policy.

These counts describe test events, including parent tests. They do not count distinct acceptance cases.
The record preserves each command, source digest, result, output, and qualification limit.

## Served snapshot regression

A host can activate the exposed client while the runtime still serves its previous effective catalog.
The earlier candidate protected the client snapshot but omitted the distinct runtime snapshot from its required generation IDs.
`TestRetentionProtectsRuntimeSnapshotAfterDirectClientActivation` failed because cleanup deleted that generation.

The overlay correction passed on both Go toolchains before source changes.
The actual implementation now adds the runtime snapshot to required IDs and includes the permanent regression test.
Both toolchains pass 25 focused race events covering retention and the startup test that the full-suite timeout interrupted.
Runtime lint, repository Go policy, and maintained prose pass on the corrected source.
Full task qualification must still cover the final source before merge.

## Remaining CSP5 work

Shared and object generation coordination remain incomplete. The owner choice between shared coordination and S3-only cleanup remains pending.
A configured shared lease currently reports `shared_coordination_required` and preserves catalog generations.
A store without collection support reports `unsupported`. Checked local input cleanup can still proceed.
These limitations do not qualify the required shared retention contract.

The twelve mapped subcases pass on the earlier candidate. A19 qualifies its product cases at that source.
A22 and A23 still require Starport consumer qualification. The task gate does not qualify the other primary cases.
Source changes require new evidence for the affected checks.

The public catalog fixture passes cache verification.
The [required runtime and storage suite](automatic-retention-required-runtime-20260912.json) failed at its 30-minute limit.
It recorded 1,069 passing test events and no assertion failures before timeout.

The interrupted startup test ran for 14 seconds. It then passed separately on both toolchains in about 15 seconds.
This evidence indicates cumulative suite duration, although it does not qualify tests that the timeout prevented.
Resolve the suite duration without removing required checks. All recorded checks are terminal.

Final repository checks must use the final committed module graph.
Required review, native CI, and merge remain open. CSP5 remains in progress.
