# CSP5 update controls and retained state

CSP5 remains in progress. The local implementation starts from Starmap `f9951ee6` on branch `codex/catalog-update-controls`.
Its worktree is `/Users/jack/src/github.com/agentstation/starmap-catalog-update-controls`.

Checkpoint `eeeba376` holds the local controls. Checkpoint `48a9a6e5` adds cascade lifecycle handling.
Checkpoint `52c84e28` corrects source-close timeout ownership. No checkpoint has a PR or merge. Starport remains at the merged CSP4 consumer source.

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

The earlier runtime package remains active in session `13494`. Its source is `48a9a6e5`. Its command is `go test -race -count=1 -timeout 30m -json ./runtime ./remote ./internal/catalog/settings`.
Read `.tmp/csp5-update-controls/lifecycle-packages/verification.json` and resume the existing session before starting another run.
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
Its result cannot qualify the timeout correction. The proof preserves its separate source identity and current status.
Starport still owns its cascade separately. CSP8 must adopt coordinated source shutdown before consumer qualification.

## Remaining work

The [task contract](../../starport-production-catalog-plan.html#task-CSP5) owns all acceptance requirements.
The [registry baseline](csp5/baseline-2026-09-11/verification.json) reports twelve unverified subcases because it contains no checks for them.
The focused results above do not complete those full subcases.

Inspect the existing earlier-source race session before starting another broad runtime run.
Do not replace an observation timeout with a new test process.
Continue generation pins and rollback under the task contract.

Complete pin and rollback behavior without bypassing permission withdrawal.
Complete owned-stage recovery, bounded history compaction, and ambiguous publication recovery.
Preserve original receipts, omitted offerings, active writers, unknown files, and required generations.
Full task checks, repository verification, review, native CI, and merges remain open.
