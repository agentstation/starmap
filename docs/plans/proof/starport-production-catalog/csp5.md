# CSP5 update controls and retained state

CSP5 remains in progress. The local implementation starts from Starmap `f9951ee6` on branch `codex/catalog-update-controls`.
Its worktree is `/Users/jack/src/github.com/agentstation/starmap-catalog-update-controls`.
Checkpoint `eeeba376` holds the local controls. It has no PR or merge. Starport remains at the merged CSP4 consumer source.

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
No passing package skips a test. Both original and corrected results retain their exact source snapshots. All recorded sessions are terminal.

## Remaining work

The [task contract](../../starport-production-catalog-plan.html#task-CSP5) owns all acceptance requirements.
The [registry baseline](csp5/baseline-2026-09-11/verification.json) reports twelve unverified subcases because it contains no checks for them.
The focused results above do not complete those full subcases.

Verify automatic cascade shutdown ownership before implementing generation pins.
The current remote source detaches its stream context, while runtime shutdown does not close an injected source.
Composition and runtime ownership need a defined cancellation and join contract for replacement.

Complete pin and rollback behavior without bypassing permission withdrawal.
Complete owned-stage recovery, bounded history compaction, and ambiguous publication recovery.
Preserve original receipts, omitted offerings, active writers, unknown files, and required generations.
Full task checks, repository verification, review, native CI, and merges remain open.
