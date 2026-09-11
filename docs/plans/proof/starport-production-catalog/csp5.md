# CSP5 update controls and retained state

CSP5 remains in progress. The local implementation starts from Starmap `f9951ee6` on branch `codex/catalog-update-controls`.
Its worktree is `/Users/jack/src/github.com/agentstation/starmap-catalog-update-controls`.

Checkpoint `eeeba376` holds the local controls. Checkpoint `48a9a6e5` adds cascade lifecycle handling.
Checkpoint `52c84e28` corrects source-close timeout ownership. Checkpoint `b3cf9b68` rechecks queued publication guards.

Checkpoint `32951a7b` adds configured pins and consistent unpin startup. No checkpoint has a PR or merge. Starport remains on its merged CSP4 source.

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
The proof preserves a running snapshot. Read `.tmp/csp5-update-controls/generation-pin-runtime-storage/verification.json` for its final status.

This checkpoint does not record a durable rollback acceptance event or qualify complete pin recovery.
An origin can pin its current authority generation. An origin rollback still requires implementation that issues a new authority revision.
Authority-binding receipts, generation retention, and full task acceptance remain open.

## Remaining work

The [task contract](../../starport-production-catalog-plan.html#task-CSP5) owns all acceptance requirements.
The [registry baseline](csp5/baseline-2026-09-11/verification.json) reports twelve unverified subcases because it contains no checks for them.
The focused results above do not complete those full subcases.

Resume current-source session `36556` before starting another broad runtime run.
Do not replace an observation timeout with a new test process.
Continue acceptance records, origin rollback publication, and complete pin recovery under the task contract.

Complete pin and rollback behavior without bypassing permission withdrawal.
Complete owned-stage recovery, bounded history compaction, and ambiguous publication recovery.
Preserve original receipts, omitted offerings, active writers, unknown files, and required generations.
Full task checks, repository verification, review, native CI, and merges remain open.
