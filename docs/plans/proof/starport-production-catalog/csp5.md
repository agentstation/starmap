# CSP5 update controls and retained state

CSP5 remains in progress. The local implementation starts from Starmap `f9951ee6` on branch `codex/catalog-update-controls`.
Its worktree is `/Users/jack/src/github.com/agentstation/starmap-catalog-update-controls`.

Checkpoint `eeeba376` holds the local controls. Checkpoint `48a9a6e5` adds cascade lifecycle handling.
Checkpoint `52c84e28` corrects source-close timeout ownership. Checkpoint `b3cf9b68` rechecks queued publication guards.

Checkpoint `32951a7b` adds configured pins and consistent unpin startup. Checkpoint `39922ecf` adds durable pin acceptance and origin rollback issuance.
Checkpoint `de8b5abe` adds explicit durability outcomes and pending-pin confirmation.
No checkpoint has a PR or merge. Starport remains on its merged CSP4 source.

## Current controls

The [control proof](csp5/update-controls-2026-09-11/verification.json) preserves commands, source snapshots, failures, and partial results.
Each check records its own source hashes. The base commit alone does not identify the tested implementation.

The runtime now accepts `STARMAP_CATALOG_SOURCE_REFRESH_MODE=automatic|manual` and `STARMAP_CATALOG_NETWORK_MODE=configured|offline`.
Manual source mode suppresses startup reads, polling, and watcher subscriptions. Explicit cascade reads use a bounded `ReadOnce` path.
Close and caller cancellation stop manifest, payload, and source-chain requests. Concurrent subscriber starts and manual reads return conflicts.

Offline mode rejects network source reads, scheduled acquisition, explicit acquisition, and network preparation callbacks.
The manual acquisition adapter declares its source selection before preparation. Offline provider acquisition returns before credential resolution.
Concrete verified imports remain available. Declared local preparation validates the sources of its returned observations.

Internal authority rejects local acquisition before preparation. Independent permission reads remain enabled in manual mode and stop in offline mode.

The descriptor schema, generated reference, environment example, and Compose example include both settings.
Starport adoption still needs a module upgrade and consumer qualification under the plan.

## Evidence

The initial configuration tests fail on the base: three explicit offline operations send one, one, and two HTTP requests.
Manual source mode also subscribes to one watcher channel. The later callback tests expose three more offline HTTP requests.
The provider test resolves two credentials during offline manual acquisition and preview.

The cascade fixture first failed because its source policy was incomplete. Its next attempt omitted the required payload media type.
Those fixture failures remain separate from product evidence.
After fixture correction, the base refresh implementation opens one event stream during manual refresh.
The Go overlay changes only `runtime/refresh.go` to the base version. The proof retains the overlay and exact replacement bytes.

| Check | Result |
| --- | --- |
| Offline configuration, manual watcher, and finite cascade reads | 13 race test events pass, no failures or skips |
| Offline callbacks and lazy provider credentials | 8 race test events pass, no failures or skips |
| Verified local import and restart | 1 race test event passes, no failures or skips |
| Runtime observation, startup, and cancellation regressions | 12 race test events pass, no failures or skips |
| Corrected canonical settings package | 22 race test events pass, no failures or skips |
| ago | Exit 0, no findings, stale ignores, or incomplete errors |
| Technical writing | Corrected checks pass. The first failure and source snapshots remain in the proof. |

The broader package run completed with exit 1 because the settings package found three reference gaps.
The corrected settings package passes all 22 events. The original run passes 53 remote, 108 public configuration, and 104 acquisition events.
No passing package skips a test. Both original and corrected results retain their exact source snapshots. All checks in the earlier control proof are terminal.

## Cascade lifecycle checkpoint

The [lifecycle proof](csp5/cascade-lifecycle-2026-09-11/verification.json) binds the local checkpoint to its commands and source snapshots.
The original regression confirms that runtime shutdown left its constructed cascade stream active.
Separate fixture and compile failures remain in the proof. The corrected focused run passes 27 race events without failures or skips.

`WithOwnedSource` transfers the selected source to runtime shutdown. `WithSource` keeps ownership with the caller.
The Starmap composition transfers the cascade source it constructs. Failed startup uses the same cleanup as runtime shutdown.

A canceled initial read now stops without attaching the continuing stream to one request context.
Tests cover manifest, payload, stream-header, and catch-up cancellation, retry, repeated close, failed startup, and manual offline replacement.
Failed startup preserves source-close errors and releases the directory for a later runtime.

ago and vet pass. The prose check passes before a test-only option-name correction that changes no prose.
The full remote package passes 62 race events. The full settings package passes 24 race events. Neither package skips a test.

The earlier lifecycle run completed against `48a9a6e5` with 989 passes and two explicit skips.
Runtime passes 903 events, remote passes 62, and settings passes 24.
The two public-catalog tests skipped because their immutable fixture was absent. The latest proof retains their exact reasons.
Its command was `go test -race -count=1 -timeout 30m -json ./runtime ./remote ./internal/catalog/settings`.
These focused and package results overlap. They do not complete any mapped CSP5 subcase.

## Source-close timeout correction

The [shutdown proof](csp5/source-shutdown-2026-09-11/verification.json) binds checkpoint `52c84e28` to exact commands, snapshots, and results.
The regression stalls a real subscriber reconnect through an injected transport that delays cancellation.
The runtime previously returned a close timeout and released its directory while that source worker remained active.

`OwnedSource` now requires `Shutdown(context.Context)`. Runtime cleanup supplies a live context and retains directory ownership until the source joins.
The caller still receives a typed timeout after five seconds. `Source.Close` keeps its bounded standalone behavior.
Failed startup uses the same bounded cleanup. It preserves the original startup error and retains the directory through delayed shutdown.

The corrected focused run passes 36 race events without failures or skips.
It covers the stalled worker, context deadline, failed startup, lease release, late publication, and clock cleanup.
Complete remote and settings packages pass 63 and 24 race events without failures or skips. These results overlap the focused run.
ago and vet pass. The prose check precedes a nil-receiver guard and two test additions that change no source comments.

The earlier broad run in session `13494` covers checkpoint `48a9a6e5` only.
Its result cannot qualify the timeout correction. The latest pin proof records its completion and separate source identity.
Starport still owns its cascade separately. CSP8 must adopt coordinated source shutdown before consumer qualification.

## Configured generation pins

The [pin proof](csp5/generation-pins-2026-09-11/verification.json) binds checkpoints `b3cf9b68` and `32951a7b` to their exact checks.
The queued-mutation regression shows update, activation, reload, and rollback bypassing a guard changed while they waited.
The corrected guard runs again after entry to the mutation transaction. All 24 selected guard, reload, and rollback events pass.

`STARMAP_CATALOG_GENERATION_PIN` and `WithGenerationPin` select one retained artifact before startup can rebuild from retained inputs.
The host persists this value in its selected configuration authority. An explicit empty value clears the pin during runtime replacement.
The descriptor has deployment scope and the `runtime-replacement` change class. No separate pin configuration file exists.
The generated reference, descriptor schema, environment example, and Compose example include the setting.

Pins block source reads, scheduled acquisition, watcher subscriptions, imports, removals, and direct client mutations.
Independent permission observation continues. Permission enforcement uses the selected artifact's exact manifest and payload identity.
A permission withdrawal blocks new attempts while retaining pinned metadata. An incompatible authority cannot accept the pin.
Invalid identity, payload, schema, or missing artifacts fail startup and leave accepted storage unchanged.

The first pin configuration test shows ignored selection, network requests, candidate work, and direct rollback across two starts.
The first runtime test had two fixture errors: it removed a persistent alias and passed an empty import.
After fixture correction, unpin exposed a runtime/client mismatch. Startup now restores the same verified retained generation in both views.
The filesystem test reopens the store twice before clearing the pin and restoring the latest retained inputs.

Corrected focused checks pass 86 race events without failures or skips.
All 111 public configuration events pass. The initial internal settings run missed one sample value. Its corrected run passes all 24 events.

ago and vet pass. The prose check passes before that test-only sample correction, which changes no comments.

Full runtime/storage session `36556` starts from clean `32951a7b`.
Its command is `env STARMAP_PUBLIC_FIXTURE_REQUIRED=1 go test -race -count=1 -timeout 30m -json ./runtime ./pkg/catalogs/storage`.
The preparation command downloaded and verified the immutable public fixture before this run.
The original proof preserves a running snapshot. The latest acceptance proof records the terminal timeout after 969 passing events.
That command does not pass and does not cover the later acceptance changes.

Checkpoint `32951a7b` has no durable acceptance record and cannot roll an origin back to older content.
The acceptance work below replaces those limits. Complete pin recovery, generation retention, and full task acceptance remain open.

## Durable pin acceptance

The [acceptance proof](csp5/pin-receipts-2026-09-11/verification.json) binds checkpoint `39922ecf` to its commands, snapshots, and terminal results.
The original regressions show a missing acceptance file and refusal to restore an older origin generation.

The runtime stores its latest operation in `catalog-runtime/generation-pin.json` beneath the selected runtime state directory.
The private file has a 32 KiB limit. It records prepared, accepted, or released state and the source or authority binding.
Its receipt identifies the selected generation, accepted generation, predecessor, payload checksum, authority head, operation ID, and local times.
The selected configuration authority still owns the pin setting. This file owns recovery evidence only.

The runtime writes preparation before catalog publication and acceptance before readiness.
A rejected commit, lost reply, or failed acceptance write can resume the same operation after restart.
An accepted restart preserves the operation and file bytes. A changed source binding, malformed record, or unrelated catalog head prevents readiness.
A pending operation requires its original configured selection before configuration can change. Complete operator recovery guidance remains with CSP18.

Clearing an accepted pin records release after startup restores its retained inputs. A later pin creates a new operation.

An origin restores the selected payload under a new authority sequence. It preserves the current sequence on an identical retry.
The test covers restart and a fresh runtime directory against the same memory catalog store.
Clearing the origin pin restores retained inputs under the next sequence. This test does not qualify a production fleet.

`Runtime.PinAcceptance` reads the accepted receipt from memory and reports whether it has durable storage.
This file keeps the latest operation. Deployment audit history and the operator UI remain separate plan work.

The final Go 1.26.6 command passes 47 race events without failures or skips.
It covers ordinary and origin pins, failure recovery, authority binding, receipt schema, permission withdrawal, unpin, and directory ownership.
Separate isolation passes 52 events from the unfinished source, migration, status, and recovery tests.
The ago check reports no findings, stale ignores, or incomplete errors. Vet passes. The prose check passes 1,529 files with no diagnostics.

The earlier full runtime/storage command at `32951a7b` failed at its 30-minute limit.
It recorded 969 passing events and no individual test failure before the timeout. Storage completed, but runtime did not.
The timeout stack identifies Go 1.27.0. Final focused commands explicitly select Go 1.26.6 and `GOWORK=off`.
Passing isolated checks do not replace the required full command. All recorded checks are terminal.

## Filesystem durability and pin recovery

The [durability proof](csp5/filesystem-durability-2026-09-11/verification.json) binds checkpoint `de8b5abe` to its commands, snapshots, and results.
`TestFilesystemCommitAmbiguousFlushOutcome` originally found three defects during first publication and replacement.
The pointer became visible with an ordinary I/O error. An identical retry skipped the failed flush and reported success without confirmation.

Private-file publication now returns `*errors.PublicationError` after publication if synchronization fails.
The filesystem store identifies the current generation in that error. The wrapped cause remains available through `errors.Is`.
Failure before publication preserves the prior pointer and does not claim a published outcome.
An identical retry verifies retained content and synchronizes the generation directory, its parent, and the current directory.
The retry preserves pointer identity and rejects different content under the same generation ID.

The pending-pin regression exposed readiness after restart while the store still reported unconfirmed durability.
Startup now retries the recorded commit before acceptance. It preserves the selected generation and operation ID.
An accepted restart reasserts the same receipt to confirm its persistence. The operation ID and acceptance time remain unchanged.
Additional tests refuse clearing or replacing a pending pin and refuse an unrelated head after preparation or acceptance.

The complete storage, private-file, and error packages pass 236 race events: 92, 60, and 84 respectively.
The final pin check passes 37 events. Activation, rollback, and authority-publication consumers pass 31 events.
None of these checks fail or skip. The ago and vet checks pass.

The first prose check found two complex-tense comments. Its corrected run passes 1,533 files with no diagnostics.
Generated Go documentation now includes the pin, acceptance, and publication-error APIs.

All commands select Go 1.26.6 and `GOWORK=off`. Each result retains its tested source snapshot.
Later changes to tested storage logic are comments and generated documentation only.

The filesystem tests inject a synchronization failure after real link or rename publication.
They verify visible bytes, reopen, continued refusal, successful retry, and retained prior content.
Physical power-loss and native platform qualification remain separate. All recorded checks are terminal.

## Remaining work

The [task contract](../../starport-production-catalog-plan.html#task-CSP5) owns all acceptance requirements.
The [registry baseline](csp5/baseline-2026-09-11/verification.json) reports twelve unverified subcases because it contains no checks for them.
The focused results above do not complete those full subcases.

Implement bounded owned-stage recovery and generation retention under the task contract.
Preserve rollback pins and every other required generation.
Use Go 1.26.6 explicitly for the remaining checks.
Full current-source runtime/storage verification must pass before task completion.

Keep pin and rollback behavior subject to permission withdrawal during remaining changes.
Complete owned-stage recovery and bounded history compaction.
Preserve original receipts, omitted offerings, active writers, unknown files, and required generations.
Full task checks, repository verification, review, native CI, and merges remain open.
