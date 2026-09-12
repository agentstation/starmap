# CSP5 update controls and retained state

CSP5 remains in progress. The local implementation starts from Starmap `f9951ee6` on branch `codex/catalog-update-controls`.
Its worktree is `/Users/jack/src/github.com/agentstation/starmap-catalog-update-controls`.

Checkpoint `eeeba376` holds the local controls. Checkpoint `48a9a6e5` adds cascade lifecycle handling.
Checkpoint `52c84e28` corrects source-close timeout ownership. Checkpoint `b3cf9b68` rechecks queued publication guards.

Checkpoint `32951a7b` adds configured pins and consistent unpin startup. Checkpoint `39922ecf` adds durable pin acceptance and origin rollback issuance.
Checkpoint `de8b5abe` adds explicit durability outcomes and pending-pin confirmation.

Commit `ceef5480` corrects the retained record inventory. Commit `55c8bc19` adds baseline stage recovery.
Commit `9e875a35` adds migration initialization, partial ownership, scan limits, and the reopen correction.
Commit `536791a0` checks candidate cleanup and reuses validated repair candidates.

Commit `a05ca541` records private preparation writes and bounds assembly reads.
Commit `fcfda255` checks legacy rollback ownership and rejects replacement assembly files.
Commit `9d8b4b8b` bounds filesystem records and legacy preflight.
Commit `64f905db` preserves workspace record ownership and bounds marker reads.

Commit `e2cbbc6b` binds journal completion to accepted file state.
Commit `e7bfdf67` persists replacement child identities and preserves legacy journals.
Commit `092bf7ce` binds publication and recovery to the held writer.
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

## Retained record inventory

The [inventory proof](csp5/file-inventory-2026-09-11/verification.json) binds commit `ceef5480` to its exact commands and source snapshots.
The original regression found two runtime files absent from the manifest and inspection output: `removals.json` and `generation-pin.json`.
Both the canonical runtime directory and an explicit override reproduced the omission.

The runtime evidence inventory now names both files and applies its existing owner-only policy.
The engineering path table also identifies the 32 KiB pin receipt and its recovery role.
Configuration authority still owns the pin setting. The receipt does not replace configuration.

The regression creates the files through real application startup, operator removal, catalog publication, and pinned restart.
Inspection reports their paths and private-file policy without changing their bytes or exposing the receipt contents.
The inspection application stays passive. Unrecognized runtime files remain outside the managed inventory.

Corrected checks pass 8 application and 86 product-path race events without failures or skips.
The ago check reports no findings, stale ignores, or incomplete errors. The prose check passes 1,534 files with no diagnostics.
All commands select Go 1.26.6 and `GOWORK=off`. All recorded checks are terminal.

Both regression runs record optional YAML workspace repair timeouts after durable catalog activation.
The inventory correction does not change or qualify workspace projection. Full CSP5 verification remains required.
Baseline export still has process-local cleanup only. Recovery must verify persistent ownership before it removes stages from exited writers.

## Baseline stage recovery

The [recovery proof](csp5/baseline-recovery-2026-09-11/verification.json) binds commit `55c8bc19` to sixteen command records and their source snapshots.
The original process-interruption regression fails before journal recovery. A second regression fails when recovery deletes a stage after an actor replaces the retained writer lock.
Both corrected regressions pass.

Baseline export now retains `.starmap-baseline/.owner.lock` and private versioned operation journals beside its published generations.
Each journal binds the lock identity, stage identity, expected file identities, metadata, and content digests.
One writer lock excludes active exporters during recovery. Recovery preserves the stage if another actor replaces the lock.

Recovery resumes a verified partial cleanup after process interruption. It never deletes a published baseline.
Unknown journals, changed entries, unrecorded stages, and oversized content remain in place and appear in `ExportResult.Recovery.PreservedPaths`.
The scan limit is 4,096 combined baseline and metadata entries. The content limit is 64 MiB of declared snapshot bytes per pass.

Repeated stability checks can reread content. The snapshot limit does not impose a catalog size limit or a strict physical I/O budget.

Final local race checks pass 46 baseline events, 30 path events, and 7 later policy events without failures or skips.
These runs overlap and do not establish 83 distinct cases. Consumer dependency checks and the ago check pass.
The corrected prose check passes 1,541 files with no diagnostics. The original paragraph-length failure remains in the proof.

Linux AMD64 and Windows AMD64 compilation pass. Native runtime, ACL, filesystem durability, and power-loss qualification remain separate.

The baseline and path checks precede one separately tested POSIX policy case and a documentation paragraph split.
Final ago, prose, and Linux compilation checks cover all sixteen committed files. All recorded checks are terminal.
Other staging roles, published generation retention, history compaction, and full CSP5 qualification remain open.

## Migration stage recovery

The [migration proof](csp5/migration-recovery-2026-09-12/verification.json) binds commit `9e875a35` to twenty-eight command records and their source snapshots.
The original partial-copy regression deletes changed bytes, replacement files, and unrecorded files during restart.
The original initializer deletes unknown operator content on error. Four process-exit cases leave earlier initialization stages behind.
The source scan also accepts 40,001 empty directories despite its separate file limit.

Initialization now retains a private `stage-initialization.json` record in the operation journal.
It binds the manifest, parent and stage identities, and journal lock. Restart resumes that same stage.
Unknown entries, changed intent, replaced directories, and replaced locks cause refusal without recursive deletion.
Publication preserves any later use of the old staging name.

Each `.partial` file has an immutable sibling `.partial.json` ownership record, limited to 16 KiB.
The record binds native stage, work-directory, lock, and file identities, plus mode and the source entry.
Recovery checks the mutable partial bytes against the exact source prefix before removal.
A crash after partial removal retains enough evidence to finish record cleanup. Missing or unsupported records cause preservation and refusal.

Source inventory permits 40,000 entries, including empty directories and metadata.
The stage scan derives its entry limit from the allowed manifest layout. Both scanners use batches of at most 128 entries.
Aggregate path names cannot exceed 4 MiB. Initialization permits only two metadata files and an empty work directory.

Final migration checks pass 150 race events without failures or skips. Baseline and publication checks pass 32 race events.
The native identity code moves into `internal/filepublish` with unchanged identity encodings. Workspace encodings remain separate.
Consumer dependency checks and the final ago check pass. The final source prose check passes 1,544 files with zero diagnostics.

Two earlier prose runs report paragraph errors. The proof preserves both failures and their source snapshots.

Final Linux AMD64 and Windows AMD64 compilation includes the directory flush and reopen correction.
Only a test assertion correction follows those builds. The final migration and ago commands cover that correction.
Native platform behavior and physical power-loss recovery remain unqualified for this change. All recorded checks are terminal.

The moved-stage regression also exposed deferred cleanup through a nil handle after a failed directory reopen.
The initializer now keeps the original and reopened handles separate. A moved stage causes a filesystem error and remains preserved.
The first broad rerun returned that expected error but failed an assertion that did not inspect joined errors.
The corrected assertion uses `errors.Is`. Its focused check and the full migration group pass.

Unrecorded stages and interrupted atomic owner-record scratch remain preserved for explicit recovery.
Workspace preparation and exchange cleanup, retained evidence, discovery staging, generation retention, history compaction, and full CSP5 qualification remain open.
At that checkpoint, private workspace preparation and verification still use recursive cleanup. The preparation section below records the later correction.

## Workspace candidate cleanup and repair reuse

The [workspace proof](csp5/workspace-cleanup-2026-09-12/verification.json) binds commit `536791a0` to its command records and source snapshots.
Eight original cases delete operator content before promotion or after native exchange.
They cover unknown files, changed files, same-content replacement files, and replacement roots.
Both original repair cases also build two candidates for one repair.

Candidate cleanup now checks bounded inventories, native identities, content digests, and access metadata before deletion.
It preserves conflicts and returns the cleanup error with the original operation error.
Cancellation starts a separate 30-second cleanup limit. Native exchange permits cleanup of the recorded old tree at the candidate path.
The visible publication receipt survives cleanup failure.

Repair now publishes its validated candidate without a second render and validation pass.
All 149 workspace race events pass without failures or skips.
Final ago, prose, and Linux/Windows compilation checks pass. Compilation does not qualify native runtime behavior.

The canonical-runtime application test passes before and after candidate reuse.
Its elapsed time falls from 117.50 to 74.64 seconds. Sampled cumulative allocation space falls from 18,970.27 to 11,169.65 MB as reported by pprof.
Each version has one local run with race instrumentation and profiling. The earlier optional-workspace timeout does not recur in either run.

Allocation totals are not peak resident memory. These observations do not qualify production startup, inference latency, or the released product pair.

The proof preserves one intermediate compile failure and both prose failures.
The first prose failure identifies an eight-sentence paragraph. The second scans a generated native sample with a text suffix.
That unchanged machine output now uses a log suffix. The corrected document restores migration paragraph ownership before the new workspace section.

The identity map remains in memory and does not change the existing replacement-journal encoding.
At `536791a0`, private preparation and verification cleanup still use recursive deletion. Assembly still has an unbounded directory read.
Persistent stage recovery, retained evidence, discovery staging, generation retention, history compaction, and full CSP5 qualification remain open.

## Workspace preparation ownership

The [preparation proof](csp5/workspace-preparation-2026-09-12/verification.json) binds commit `a05ca541` to fourteen command records and their exact sources.
Four original cases delete operator files from preparation directories. They cover unknown children, changed render files, replacement files, and replacement enclosures.
A separate boundary case creates an unrecorded file before rejecting the path-name limit. Both failures now have passing regression tests.
The proof also retains one intermediate build failure from an unused import.

`Builder.WriteYAML` emits the existing catalog records and logo sidecars through a callback without filesystem access.
Workspace preparation owns file creation and records native identity, access metadata, and actual written bytes.
Partial writes remain identifiable after a write error or cancellation. Unknown or changed entries remain preserved during cleanup.
Verification uses the same writer and checked cleanup. Operator files copy after generated records, without deleting copied model directories.

The writer checks resource limits before creating another entry. Assembly uses bounded directory reads and indexes expected child counts once.
Temporary render and verification files do not require a flush per file. The publishable candidate still flushes its files and directories.
`SaveTo` retains its existing authoring behavior.

All 160 workspace and 1,264 catalog race events pass without failures or skips.
The canonical-runtime application integration test passes, with two test events including its parent.
Final ago, prose, consumer-dependency, and Linux/Windows compilation checks pass.
The serializer test also reloads emitted bytes and verifies that the callback serializer creates no filesystem entries.
Compilation does not qualify native Windows or Linux runtime behavior. The application result does not establish a production latency claim.

Ownership records remain in process memory. Persistent preparation and replacement recovery, including active-writer exclusion, remain open.
Source inspection also found a separate gap in `rollbackLegacyMove`: its semantic checksum excludes operator notes before recursive deletion.
At `a05ca541`, this finding had no failure-test result. The legacy rollback section below records the correction.

## Legacy rollback and assembly ownership

The [rollback proof](csp5/legacy-rollback-2026-09-12/verification.json) binds commit `fcfda255` to its exact sources and command results.
Seven original rollback scenarios fail, producing nine test failure events including their parents.
They cover unknown projected files, changed YAML comments, replacement files, replacement roots, operator sidecars, and a replaced relocated store.
A separate assembly test also fails because assembly accepts a replacement file with identical bytes.

Rollback now receives the candidate inventory captured before publication. It verifies native file identities, content, access, and remaining entries before removal.
It preserves unknown or changed content and reports a conflict with the original failure.
The store has a native directory identity captured before relocation. Both relocation and restoration use open parent directories and refuse an existing destination.

Rollback leaves projection-marker paths and the stable writer-lock file in place.
Two previous tests expected rollback to delete those unowned entries.
The tests now verify that rollback preserves those entries and restores the exact store.
Additional tests cover cancellation, lock identity and exclusion, and occupied move destinations.
Assembly compares its finished tree with identities and bytes recorded during creation. Identical bytes do not establish ownership of a replacement file.

All 175 workspace and two CLI command race events pass without failures or skips.
The CLI cases use the existing adapter fixture. Workspace tests exercise real filesystem stores.
Final ago, prose, and Linux/Windows compilation checks pass. The proof retains an initial eight-sentence paragraph failure and its corrected document.
Native runtime and physical power-loss qualification remain open.

Store identities and candidate inventories remain in memory. They do not establish recovery after process exit.
At `fcfda255`, legacy preflight still used unbounded directory and manifest reads. The next section records their correction.

At `9d8b4b8b`, projection-marker temporary cleanup still lacked a recorded ownership check. The workspace record section below records its correction.
CSP5 owns those paths, persistent recovery, active-writer exclusion, retention, compaction, and full qualification.

## Bounded legacy preflight and filesystem records

The [preflight proof](csp5/legacy-preflight-2026-09-12/verification.json) binds commit `9d8b4b8b` to its exact sources and command results.
The original filesystem tests fail in six reader and writer scenarios, producing eight failure events including parents.
Oversized records previously entered memory or became stored generations. The corrected reader refuses their size before reading the file.

Filesystem payloads have a 32 MiB limit. Manifests have a 64 MiB limit, and current pointers and authority records have a 16 KiB limit.
The pointer limit includes its newline. Commit rejects oversized payloads, manifests, and pointers before creating generation state or changing current.
These limits do not configure other adapters.

The first migration test used the wrong expected error field. Its corrected test still shows that an oversized retained manifest reaches JSON parsing.
The passing correction rejects its size before parsing and preserves current, the retained record, and the absent destination parent.
The proof retains both failed attempts and their original source.

Fixed-layout reads inspect at most four entries. Retained generation scans use 128-entry batches and check cancellation before each batch and generation.
They validate every retained generation and preserve the complete count. Tests scan 259 entries and stop at cancellation or a failed generation.
A full migration preserves all 129 generations across a batch boundary and reads every unchanged generation afterward.

All 187 workspace, 101 storage, 60 private-file, and two CLI command race events pass without skips.
The CLI cases use their existing adapter fixture. Workspace and storage tests use real filesystem state.
Final ago, prose, and Linux/Windows compilation checks pass. The initial prose failure and its correction remain in the proof.
Native runtime and physical power-loss qualification remain open.

These changes do not provide persistent workspace or legacy relocation recovery.
The next section records marker cleanup. Active-writer exclusion for persistent stages, retention, compaction, and complete CSP5 qualification remain open.

## Workspace marker and journal ownership

The [record proof](csp5/workspace-records-2026-09-12/verification.json) binds commit `64f905db` to the original failures and final checks.
Twelve cleanup cases fail before the fix, producing thirteen failed events including their parent.
They cover ordinary markers, replacement journals, and journaled markers. The injected hooks expose the existing write and publication boundaries.

The original writers delete changed files, identical-byte replacements, recreated paths, and replacements made after partial writes.
A separate Go overlay restores only the original marker reader. That reader accepts a valid marker larger than 4 MiB.
The proof preserves both overlay inputs and the exact command sources.

The shared record writer captures native identity, access, and the actual written bytes. It keeps the file open through cleanup.
Before publication, it rechecks the candidate, destination snapshot, and cancellation. Journals cannot replace an existing destination.
Cleanup preserves changed files and reports its errors with the original failure. Cancellation retains a separate 30-second cleanup context.

All 213 workspace and two CLI command race events pass without skips. Tests also cover access changes and unchanged partial cleanup.
Exact-size publication, oversized refusal, short writes, cancellation, destination conflicts, and journal collisions pass.
Final ago, prose, and Linux/Windows compilation checks pass. Native runtime and physical power-loss qualification remain open.

The records remain in memory. Persistent workspace and legacy relocation recovery remain incomplete.
At `64f905db`, `finishReplacementRecord` still compared parsed content before removing the completion file.
The next section records its identity binding and regression results.

The [current PR queue](pr-audit-2026-09-08/current-queue/queue-status-2026-09-12.json) has no open PRs.
CSP5 remains unpublished until its full contract and required review pass. Both implementation worktrees are clean.

## Accepted journal completion identity

The [journal identity proof](csp5/journal-identity-2026-09-12/verification.json) binds commit `e2cbbc6b` to six original failures and their corrections.
The cases cover initial replacement and recovery under identical file replacement, equivalent JSON edits, and changed access metadata.
Seven failed events include the parent test. The original cleanup deleted each changed journal.

Publication now returns the original staged file state. Recovery captures journal bytes, native identity, and access metadata from one bounded read.
Completion compares that accepted state before removal. Cancellation retains the journal, and a later recovery attempt validates it again.
Tests also require a receipt and preserve the original publication identity after a publication error.

All 224 workspace and two migration CLI race test events pass without failures or skips.
The focused boundary run passes 27 events, including process exits at six replacement phases.
Final ago, prose, and Linux/Windows compilation checks pass. Native execution remains unverified.

At `e2cbbc6b`, receipts remained in memory. That checkpoint used version 2 journals.

A separate preserved Go overlay proves another defect: recovery deletes an identical replacement backup child.
Version 2 persists root identity and child content and access metadata. It omits native child identities.
The next section records persisted child ownership and legacy-journal compatibility. The original failing probe remains preserved.

## Persisted replacement child identities

The [child identity proof](csp5/backup-children-2026-09-12/verification.json) binds commit `e7bfdf67` to original failures and final checks.
In the red run, recovery deletes identical replacement files and directories. It also accepts a version 2 journal without child identities.
Five failed events include two parent tests. The existing version 1 refusal passes in that same run.

Version 3 persists native identities for every old and new inventory entry. Both maps must cover exactly their inventory paths and match the root identity.
Each identity remains bounded to 128 bytes. Both maps remain within the existing 4 MiB journal limit.
Recovery checks identities, content, and access before moving live or candidate trees.

Backup cleanup checks persisted child identities before cleanup and before each removal. It also verifies the backup root's path binding.
Tests preserve a file replaced during cleanup and reject incomplete, extra, empty, oversized, and root-mismatched identity records.
Version 1 and 2 journals remain unchanged with a `workspace_replacement.version` error. Their workspace, candidate, and backup remain preserved for explicit recovery.

All 243 workspace and two migration CLI race test events pass without skips. The final boundary run passes 19 events.
The earlier 20-event run also passes process-exit recovery at all six replacement phases.
Final ago, prose, and Linux/Windows compilation checks pass. Native runtime qualification remains open.

Journal acceptance receipts remain in memory. Child ownership now persists in version 3 journals.
Preparation and legacy relocation recovery remain incomplete, with evidence and discovery staging, retention, compaction, and full task qualification.

## Workspace writer identity

The [writer identity proof](csp5/workspace-writer-2026-09-12/verification.json) binds commit `092bf7ce` to original failures and final checks.
Both native-directory and journal publication previously continued after another writer acquired a replacement lock file.
Two failing scenarios and their parent produce three failed events. Preparation recovery also needs this writer identity check.

Writers now retain a checked handle and native identity through projection, repair, and legacy layout migration.
The lease checks the existing path, locked file, and current path. Failed attempts close their handles, and release preserves the stable lock file.
Directory moves, record publication, journal phases, backup cleanup, and journal completion recheck the held writer before further changes.

Version 3 replacement journals require `lock_identity` as well as child identities. Recovery preserves records whose writer identity is absent or does not match.
Direct protocol tests hold a real writer lease. Tests also verify exclusive ownership and stable identity across lease release.
Windows tests allow a native permission refusal to rename the open lock and then require normal completion. That branch still requires native CI.

All 255 workspace and two migration CLI race test events pass without skips. The focused boundary run passes 18 events.
The earlier 17-event run includes process-exit recovery at six replacement phases.
Final ago, prose, and Linux/Windows compilation checks pass. The first source prose check failed on one nominalization, with both versions preserved.

The baseline exporter supplies the stable-lock recovery pattern. Workspace preparation still has no durable ownership journal.
Connect that journal to the checked writer identity before qualifying preparation recovery.
Other persistent recovery, retention, compaction, full task checks, review, native CI, and merge remain open.

## Durable private preparation recovery

The [preparation recovery proof](csp5/preparation-journal-2026-09-12/verification.json) binds commit `873a53c5` to the original process-exit failure and final checks.
Repair previously left the abandoned preparation directory after the child process exited during rendering.
The final race suite passes 278 test events without failures or skips.

Private `.preparation.jsonl` records bind the target, enclosure, journal identity, and stable writer-lock identity.
Append records persist entry identities, contents, and access settings without rewriting preceding inventories.
Projection and repair recover recorded preparation trees while holding the checked workspace writer.
Cleanup verifies ownership, excludes active writers, and preserves unknown files or changed entries.

Process-exit tests cover initialized staging, partial writes, and completed render.
Other checks cover changed bytes and identities, unknown files, invalid journals, replaced locks, cancellation, partial cleanup, and scan limits.
Journals have a 32 MiB limit, a 120,000-event limit, and at most four recorded trees.
The parent scan stops at 4,096 entries before cleanup starts. Each tree retains its existing inventory limits.

Final ago and prose checks pass. The first prose check reaches the glossary candidate threshold for CI, and the final text uses verification gate.
Linux and Windows test binaries compile. Those compilation checks do not qualify native execution.

The journal covers entries inside private preparation only. Candidate handoff still needs durable ownership through publication and cleanup.
A crash after journal removal can leave an unrecorded empty enclosure, which remains preserved.
Other staging, retention, compaction, task acceptance, review, native CI, and merge remain open.

## Durable candidate handoff

The [candidate recovery proof](csp5/candidate-journal-2026-09-12/verification.json) binds commit `042b261e` to crash failures, publication failures, and final checks.
The corrected fail-before test uses a Go overlay of `873a53c5`. Both prepared and exchanged candidates survive recovery in that implementation.
The original test draft had one incorrect catalog expectation. Its source and results remain preserved with the corrected failure evidence.

Version 2 preparation journals append a handoff record before exporting the assembled candidate.
The record binds its name, prepared identity, and original workspace inventory with native child identities.
Staging retains the journal after render cleanup. Collection accepts only remaining entries from the prepared or exchanged original inventory.
It never selects the installed workspace path. Version 1 journals remain readable for private preparation cleanup only.

Five of six changed-ownership publication scenarios initially failed. Their parent tests bring the failed-event count to eight, with one passing event.
Publication now verifies the original journal receipt and complete prepared inventory before changing the workspace.
The replacement protocol also checks its candidate against that prepared inventory before recording ownership.
A later check verifies that a journal edit with equivalent JSON meaning prevents publication.

Workspace recovery settles replacement journals before collecting preparation and candidate state.
The first full race run exposed an old-tree recovery path that omitted preparation cleanup.
The shared recovery entry point now handles both protocols. The test retains its exact-tree and completed-cleanup assertions.
A focused follow-up passes ten events. The earlier boundary run passes 81 events, including changed-file and active-writer checks.

The final suite passes 314 race test events without failures or skips.
Eight process-exit cases cover first installation, replacement, prepared and installed candidates, and four replacement-journal phases.
Other cases cover invalid handoffs, partial cleanup, version compatibility, changed candidate files, and changed exchanged backups.
Go policy, source prose, and Linux/Windows compilation checks pass. Compilation does not qualify native execution.

A concurrent migration changed five tooling files after the final race run started. Those edits remain outside this commit.
The additional goago check passes under the migrated tooling. Full task qualification must use the final module graph.

Incomplete or unrecorded ownership remains preserved, including empty enclosures after journal removal.
Temporary records, legacy relocation, other staging, retention, compaction, full task checks, review, native CI, and merge remain open.

## Durable temporary publication records

The [temporary-record proof](csp5/record-journal-2026-09-12/verification.json) binds commit `2a4454fd` to crash failures and final checks.
The original regression leaves temporary files after four crash scenarios. Five failing events include their parent, while two scenarios already pass.
The scenarios cover prepared projection markers, prepared replacement journals, published replacement hard links, and prepared replacement markers.

Version 3 preparation journals record each temporary file's destination, basename, identity, contents, size, and access metadata.
The basename binds to the preparation directory's unique suffix. Record stages contain no catalog trees or candidate handoff.
The publisher records empty files and returned partial or complete writes before publication.
It verifies the original journal receipt before replacing or linking the destination.

Recovery removes only temporary files that match their receipts. Changed files, unknown entries, and incomplete ownership remain preserved.
A missing temporary name permits cleanup after rename or interrupted removal. Cleanup never selects the published destination.
Projection, journaled replacement, replacement recovery, and legacy projection supply their checked workspace writer.
Versions 1 and 2 remain readable within their original contracts.

Twelve process-exit scenarios cover empty, partial, prepared, and published records across all three publication flows.
Additional tests cover changed ownership, malformed receipts, interrupted cleanup, equivalent-JSON journal edits, and older candidate journals.
The focused race run passes 80 events, including existing active-writer, scan-limit, and cancellation contracts.
The final full suite passes 347 race test events without failures or skips.

Go policy, source prose, and Linux/Windows compilation checks pass. Compilation does not qualify native execution.
These checks use the observed concurrent goago module migration. Its five files remain outside the catalog commit, and the proof archives their inputs.
Final task qualification must use the final committed module graph.

Legacy relocation, evidence and discovery staging, retention, compaction, twelve mapped subcases, full verification, review, native CI, and merge remain open.

## Durable legacy relocation recovery

The [relocation proof](csp5/relocation-journal-2026-09-12/verification.json) binds commit `57bb6d17` to crash failures and final checks.
The original retry failed after relocation, before workspace publication, and after workspace installation because the destination already existed.
Four failing test events include those three scenarios and their parent.

Version 4 preparation journals record both parent identities, the store metadata, every retained generation, and the optional Windows lock alias.
The workspace inventory flushes before workspace publication. Explicit migration retry validates retained state and both advisory locks before restoring the store and retrying migration.
Ordinary projection and repair refuse a pending relocation. Recovery preserves changed files, unknown entries, incomplete receipts, and conflicting destinations.

Five process-exit scenarios cover inventory completion, relocation, candidate preparation, workspace installation, and completed projection.
Separate tests simulate interrupted workspace cleanup and a restored store. Fourteen changed-state scenarios verify preservation.
Other tests cover active workspace and commit writers, cancellation, parent-scan limits, lock aliases, older journals, and the original receipt before publication.
The final full suite passes 384 race test events without failures or skips.

The first integration attempt lacked a test import. A broader run exposed an incorrect error classification for a replaced store.
The first full suite also exposed incorrect writer-conflict classification. A separate test reproduced open handles after setup cleanup failed.
The final code restores the retryable conflict and closes those handles while preserving operator files. All original failures remain preserved.

The first prose check found a seven-sentence paragraph, which the final text splits.
Go policy, prose, and Linux/Windows compilation checks pass. Compilation does not qualify native execution.
The proof archives concurrent tooling inputs outside the catalog commit. Full task qualification must use the final committed module graph.

Recovery keeps the existing journal-size, tree-inventory, and parent-scan limits. Generation scans use batches of 128 entries.
A completed migration removes its journal, and a repeated completed command reports the existing destination.
Crashes with incomplete ownership or an unrecorded empty enclosure require explicit recovery.
Evidence and discovery staging, retention, compaction, twelve mapped subcases, full verification, review, native CI, and merge remain open.

## Private record publication recovery

The [private record proof](csp5/private-record-recovery-2026-09-12/verification.json) binds commit `9f82730e` to its checks and source snapshots.
The fail-before API bridge called the existing writer and did not recover temporary files.
Its prepared process-exit scenario retained an abandoned file. Two failing events include that scenario and its parent.

The new opt-in API records native identities, access snapshots, mode, modification time, size, and content digest.
A private `.record-publications` directory holds its stable `.owner.lock` and pending JSONL receipts.
The writer validates the original destination and ownership receipt before publication. Recovery excludes active writers and never removes the accepted destination.

Nine process-exit scenarios cover complete headers, empty files, partial writes, incomplete ownership, and publication to new or existing destinations.
Other tests cover changed files, receipts, metadata directories, writer locks, cancellation, scan limits, ambiguous flush, and retry.
Unknown files and incomplete receipts remain preserved. All 502 final race test events pass without failures or skips.

Recovery scans at most 4,096 metadata entries in batches of 128. Each receipt permits three events within 65,536 bytes.
Each record permits at most 64 MiB. Linux and Windows compilation passes but does not qualify native execution.
Consumer dependencies stay within their existing limits. Go policy and final prose checks pass.
Two earlier prose failures remain in the proof, with corrections for a glossary threshold and one passive sentence.

The ordinary writer and its runtime and discovery callers remain unchanged. Product adoption is still required.
Integrate recovery before retained-state startup, with the publication-input and generation-pin paths included.
Update the canonical file manifest and prove provider binding directories retain their intended record inventory.
Migration must preserve receipt identity and passive inspection. Pending receipts cannot become valid recovery evidence through a copied native file identity.

`Source.mu` serializes one GitHub source instance. Before adopting the writer, verify concurrent source instances cannot lower the retained replay floor.
The new per-write lock alone does not prove that domain contract. Preserve conditional-request and accepted-release state when adding its publication guard.
Generation retention, history compaction, twelve mapped subcases, full verification, required review, native CI, and merge remain open.

## Runtime and discovery record recovery

The [integration proof](csp5/record-integration-checkpoint-2026-09-12/verification.json) binds implementation `d0009a18` and fixture fix `989310c7` to 22 source files and completed checks.
Runtime startup recovers source, provider, binding, publication-input, and pin records before loading retained layers.
GitHub construction recovers local discovery records without network acquisition.
Verified channel updates compare the previously read state under the shared writer lock. A stale writer returns a conflict and preserves the newer replay floor.
Retry uses current state and preserves the accepted release reference and ETag.

Runtime migration refuses pending receipts because their native identities cannot survive a copy.
Migration can copy inactive metadata after recovery in the original directory. Inspection preserves source bytes and creates no recovery metadata.
Only declared runtime and discovery paths carry this check. Unrelated directories with the same metadata name remain operator-owned input.
File inspection reports private stages, journals, and locks without exposing contents or claiming unrelated files.

The initial runtime fixture omitted required runtime ownership. Its five crash cases proved no recovery defect.
Its separate GitHub test reproduced replay-floor regression. The corrected fixture reproduced all five abandoned runtime stages, with seven failed events including its parent and GitHub.

An old-source overlay reproduced migration accepting pending receipts. A separate corrected probe reproduced migration claiming an unrelated metadata directory.
The first scope probe did not compile because it referenced the wrong preparation result field. Both fixture versions remain preserved.

The expanded race run passes 19 test events. The final migration run passes five events, including three unrelated directory layouts.
The file and retry run passes five events. The counts overlap. Do not sum them.

Policy, dependency, final prose, and Linux/Windows compilation checks pass. Compilation does not qualify native execution.
An earlier prose check found an eight-sentence paragraph. The final contract splits that paragraph without changing its meaning.

The first broad run stopped after the separate migration scope defect reproduced. Its process interruption record and partial output remain preserved.
The corrected broad run remains active in session `51528` against implementation `d0009a18`. It found one existing privacy fixture that read the new metadata directory as a file.

The current resume state owns the command path and next action. No broad-suite or mapped-task acceptance credit follows from this partial result.
Concurrent tooling edits remain outside the catalog commit, with their tested inputs archived. Full qualification must use the final committed module graph.

Follow-up `989310c7` scans all nested files and keeps the diagnostic-sentinel assertion. Its focused race test, policy check, and Linux/Windows compilation pass.
The active broad binary retains the original fixture failure. Its terminal result cannot count as a passing task gate.

Generation retention, history compaction, twelve mapped subcases, full task verification, review, native CI, and merge remain open.

## Discovery construction cancellation

The [cancellation proof](csp5/discovery-context-2026-09-12/verification.json) binds commit `91b4fbc6` to nine files.
Runtime construction passes its caller context to `github.NewContext` and local record recovery.
A nil or canceled context cannot create discovery state. Cancellation during recovery preserves accepted records and unfinished receipts.
A later construction attempt completes recovery. Failed runtime construction releases ownership before retry.

The legacy `github.New` constructor retains its background context and existing API.

The original constructor ignored cancellation. Five failed test events reproduce that behavior, including parent events.
The package race run passes 141 events. A separate runtime selection passes 12 events.
These disjoint runs total 153 passing events, with no failures or skips. The earlier focused run overlaps and adds no event count.

Policy, prose, dependency, and Linux/Windows compilation checks pass. Compilation does not qualify native execution.

The earlier broad suite tests `d0009a18` and retains its privacy-fixture failure.
Its running process remains separate from this cancellation proof. Its terminal result cannot qualify the newer source or count as a passing gate.
Generation retention, history compaction, twelve mapped subcases, full qualification, review, native CI, and merge remain open.

## Repeated provider history compaction

The [history proof](csp5/repeated-history-2026-09-12/verification.json) binds commit `cdcf4fff` to four files and 57 passing race test events.
Before another acquisition exceeds a history limit, the runtime compacts histories that contain only provider observations without resets.
It groups original observations with identical payloads and complete binding declarations.
The first and latest successful inventories remain. Intermediate copies no longer consume retained history capacity.

Distinct inventories preserve omitted offerings. Partial observations and equal-time evidence keep their original receipts.
Compaction preserves accepted history until catalog publication succeeds. The compacted history survives stage, save, load, and catalog rebuild without an identity change.
It does not collect immutable observation files or catalog generations.

The first fixture did not compile because Model has no Copy method. The corrected fixture reproduced capacity refusal at 4,096 batches.
The expanded boundary fixture omitted a required issue subject. Correcting that fixture preserved receipt validation.
A separate equivalence test reproduced a timestamp defect when compaction retained only the latest inventory.
The final algorithm also retains the first inventory, preserving the original model change time.

The final selected suite passes 57 events, with no failures or skips. It covers the batch and byte limits with valid observations.
Other cases cover original receipts, omitted models, account scopes, partial evidence, equal times, cancellation, metadata boundaries, resets, and reload.
Policy, prose, and Linux/Windows compilation checks pass. Native execution remains subject to the required platform checks.

The earlier integration suite finished at 07:24 UTC against `d0009a18`. It recorded 1,151 passing test events and the known privacy-fixture failure.
The runtime package reached its aggregate 30-minute timeout. The other four packages passed.
The current selected suite passes both the corrected privacy fixture and the test active at timeout.
This result does not establish that the full runtime suite meets its required deadline.

Metadata histories, existing resets, incoming reset operations, and other differing provider inventories retain their current boundaries.
Their compaction remains required before CSP5 can finish. Collection must preserve accepted references, pending publication, baselines, pins, and active readers.
Full verification, twelve mapped subcases, required review, native CI, and merge remain open.
Concurrent tooling inputs remain archived separately. Full qualification must use the final committed module graph.

## Bounded history checkpoints: 2026-09-12

Local commit `964f1c36` adds version 4 checkpoints for metadata, provider, and mixed histories, including ordered reset operations.
The [checkpoint proof](csp5/checkpoint-history-2026-09-12/verification.json) records 166 passing race test events across two disjoint selections.
The original batch-limit test reproduced capacity refusal before implementation.
An intermediate selection matched only the source-version tests. It supplies no checkpoint or provider-version coverage.
The final selections include both version matrices, every checkpoint case, preview, pins, and observation updates.

Checkpoints store distinct payloads once and retain original receipts, ordered batch references, and reset scopes.
Recovery validates every reference and receipt before applying retained inputs. Invalid records preserve the accepted head and pending journal.
Tests preserve catalog identity through mixed-source replay, replacement baselines, reset exclusions, reload, and a later append that reaccepts an original receipt.
The publication recovery cases use durable journals in one process. They do not qualify process-crash recovery for the complete product.

Each checkpoint permits 64 MiB of encoded data and 65,536 ordered observation references.
The history permits at most 4,096 linked records between checkpoints. Later linked records also count toward the encoded byte limit.
Original versions 1, 2, and 3 remain readable. Older readers reject version 4 heads.

Required distinct data can still exceed capacity. That refusal preserves the accepted catalog.

Final policy, prose, and Linux/Windows compilation checks pass. The proof preserves the original prose diagnostic and its correction.
All race tests ran on macOS. Native platform execution and the required broad task suite remain open.
Five concurrent tooling edits remain outside this commit. Final task qualification must use the committed module graph.

Checkpoints do not retire distinct superseded inventories or collect predecessor files and catalog generations.
CSP5 retains those requirements, twelve mapped subcases, full verification, review, native CI, and merge.

## Catalog serialization and immutable provenance: 2026-09-12

Local commit `c8ec797a` caches validated immutable payloads and removes repeated normalization for built-in JSON values.
The [runtime cost proof](csp5/runtime-cost-2026-09-12/verification.json) records 2,120 passing race test events across four packages, with no failures or skips.
The separate final import/restart profile passes. Policy and prose checks pass.

Each catalog retains at most one validated payload within the existing 32 MiB bound.
Warm encoding returns one caller-owned byte slice. Mutable builders encode their current state.
Tests cover simultaneous first encodings, returned-byte mutation, schema preservation, exact numeric values, unsupported data, and cycles.

Cache review exposed shared nested provenance values. Immutable construction now snapshots those values and rejection records.
Reads return independent nested values. The private construction builder supplies independent entry slices, which avoid two redundant table copies.

Producer changes and caller changes cannot alter published provenance or make it disagree with the cached payload.
Mutable builder provenance retains its prior mutation semantics. These checks prove the immutable catalog boundary.

The first snapshot change exposed inconsistent typed YAML and restored JSON comparisons for reasoning and verbosity records.
The focused semantic test also reproduced metadata and pricing differences. Canonical comparison now uses JSON and retains field-scoped YAML aliases.
Control-record tests preserve original receipts and enforce refusal after both JSON payload and YAML workspace reloads.
The proof retains every failed allocation, ownership, receipt, and prose check with its exact source.

The import/restart profiles estimate 6,092,786,599 allocated bytes before the change and 5,066,953,937 bytes afterward.
These sampled totals measure cumulative allocation during one test. They do not measure resident memory, production request allocations, or Starport latency.
Intermediate profiles remain separate from the final source. Some runs overlapped unrelated checks, so their wall times do not establish a speed guarantee.

The minimum-toolchain selection passes 49 events before the final private table-copy removal.
Full native and minimum-toolchain qualification remains open. Five concurrent tooling changes remain outside this commit.
Final task qualification must use the committed module graph.

Distinct-inventory retirement, generation and observation-file collection, twelve mapped subcases, full verification, review, native CI, and merge remain open.
The older required runtime suite remains failed historical evidence. CSP5 remains unpublished until its complete contract passes.

## Required runtime suite: 2026-09-12

The [completed suite record](csp5/runtime-required-c8ec797a-2026-09-12.json) binds the required runtime and storage command to clean source `c8ec797a`.
The command passed within its 30-minute timeout, using the committed module graph.
It records 1,111 passing test events and two skips. Both skipped public-catalog tests lacked the generated public catalog fixture.
The exact skipped output and terminal command output remain archived. The temporary checkout no longer exists.

This result predates memory and filesystem retention. It does not qualify the final CSP5 source or the twelve mapped subcases.
Prepare the public catalog fixture before the final runtime suite with `python3 scripts/prepare_public_catalog_fixture.py`.
The prior failed suite remains historical evidence under the repeated-history proof.

## Explicit memory retention: 2026-09-12

Local commit `78102bca` adds `RetainingStore` with optional `AcquireGeneration` and `Collect` methods.
The existing `Store` interface remains unchanged. Memory implements the new contract. No automatic collection starts.

The [retention proof](csp5/memory-retention-2026-09-12/verification.json) records 112 passing storage race test events on each toolchain. The runs use Go 1.26.6 and 1.25.12.
Both suites contain the same cases, including parent events. Neither suite reports failures or skips.
Policy, package lint, and final prose checks pass.
The proof preserves two original capability failures and the first prose diagnostic without changing the glossary policy.

Each collection request requires positive count and byte limits and the expected current generation.
Current content, caller-required generations, and active read leases survive collection, even if they exceed the limits.
Every required generation must exist. Stale heads, missing requirements, canceled requests, and incomplete scans stop collection before deletion.
The default scan limit is 4,096 entries. Each explicit pass permits at most 100,000 entries.

Collection removes the oldest unprotected content first, using generation time and then ID for equal times.
Byte totals count encoded manifests and payloads. They exclude filesystem overhead, journals, and replication.
Dry runs preserve content and report projected capacity. The report identifies protected content that exceeds capacity.

Read leases return independent generation data. Repeated release calls cannot end another caller lease.
Release remains available after context cancellation. Memory serializes collection with publication under its existing lock.
Tests exercise independent leases, simultaneous releases, concurrent publication, count and byte limits, and refusal without deletion.

These macOS checks qualify the memory implementation only. Persistent adapters and runtime adoption remain open.
The separate clean `c8ec797a` runtime suite excludes this change. It retains its own command and source record above.
Five concurrent tooling edits remain outside the commit. Final task qualification must use the committed module graph.

Persistent generation collection, observation-file collection, distinct-inventory retirement, twelve mapped subcases, full verification, review, native CI, and merge remain open.
CSP5 remains unpublished until its complete contract passes.

## Recoverable filesystem retention: 2026-09-12

Local commit `199cdaaf` adds native generation leases and recoverable filesystem collection.
The [retention proof](csp5/filesystem-retention-2026-09-12/verification.json) records 250 passing storage and private-file race test events on each Go toolchain.
The runs use Go 1.26.6 and 1.25.12. They repeat the same cases and include parent events.

Neither suite reports failures or skips. Policy, package lint, and source prose pass.
Linux and Windows test binaries compile. These checks do not qualify native filesystem behavior on those platforms.

The existing publication lock coordinates collection, publishers, ordinary generation reads, and authority-head reads across processes.
Each explicit reader owns an independent native generation lock. Its idempotent release survives context cancellation.
Current content, caller-required IDs, and active read leases remain protected, including when they exceed capacity.
Dry scans do not create generation lease files. Pending record recovery or retirement requires a normal pass.

A bounded scan validates generation identities, payload digests, optional authority bytes, and recognized records.
Unknown names, changed content, missing requirements, stale heads, and incomplete scans cause refusal.
Before retirement, a journal records native identities, access policy, file metadata, and content digests.
The directory then moves atomically before any file deletion. Checked cleanup synchronizes directory metadata and removes its journal last.

Recovery cancels preparations that never moved the directory. A new reader or pin can protect that generation before a fresh decision.
Recovery resumes partial deletion only for remaining records that match their receipts.
Unknown files, changed records, and replaced parent or writer identities preserve the retired directory and journal.
Retirement journals use the existing private-record publisher and its owned staging recovery.

Tests cover four process-exit points, another process reader, independent leases, ordinary reads, concurrent publication, later pin requirements, malformed journals, and changed files.
The original tests prove that filesystem retention was absent. They do not prove a prior deletion defect.

The first lint check also found existing private-publication complexity and naming diagnostics.
Smaller functions preserve journal decoding, stage writes, and publication outcomes. A platform file owns the Linux and Windows ACL bound.
Final lint and policy checks pass without suppressions or policy changes. The proof retains the intermediate receiver-name diagnostic.

Capacity totals count encoded manifests and payloads. They exclude journals, lock files, and filesystem overhead.
No automatic collection starts. Object storage, runtime adoption, observation-file collection, and distinct-inventory retirement remain open.

Five concurrent tooling edits remain outside this commit. Final qualification must use the committed module graph.
CSP5 retains all twelve mapped subcases, full verification, required review, native CI, and merge.

## Empty filesystem stores: 2026-09-12

Follow-up commit `b10721b8` corrects retention before the first catalog publication.
The [empty-store proof](csp5/filesystem-empty-store-2026-09-12/verification.json) retains the direct probe and failing regression matrix.
The probe returned raw filesystem absence for missing acquisition and collection against an existing empty directory.
Four subcases and their parent failed. Two empty-store subcases already passed.

Missing generation leases and required IDs now return the catalog not-found error.
A nonempty expected head conflicts with an absent store. Collection succeeds with zero usage for empty filesystem stores.
The six-case matrix covers these boundaries, including an existing empty generations directory.

Both complete storage and private-file suites pass 257 test events, including parents, without failures or skips.
The Go 1.26.6 and 1.25.12 runs repeat the same cases. Policy, package lint, and source prose pass.
Linux and Windows test binaries compile. Native platform qualification remains open.
The retirement format, native lease protocol, and publication synchronization remain unchanged.

## Remaining work

The [task contract](../../starport-production-catalog-plan.html#task-CSP5) owns all acceptance requirements.
The [registry baseline](csp5/baseline-2026-09-11/verification.json) reports twelve unverified subcases because it contains no checks for them.
The focused results above do not complete those full subcases.

The integration race suite ended with its known fixture failure and an aggregate runtime timeout. Preserve its terminal evidence under the history proof.
Commit `91b4fbc6` now propagates caller cancellation through GitHub construction.

Implement object-storage retention and runtime adoption. Collect observation files and safely retire superseded distinct inventories.
Preserve rollback pins and every other required generation.
Use Go 1.26.6 explicitly for the remaining checks.
Full current-source runtime/storage verification must pass before task completion.

Keep pin and rollback behavior subject to permission withdrawal during remaining changes.
Complete owned-stage recovery and bounded history compaction.
Preserve original receipts, omitted offerings, active writers, unknown files, and required generations.
Full task checks, repository verification, review, native CI, and merges remain open.

## Object inventory and conditional deletion: 2026-09-12

Local commit `8f76544d` adds `ObjectCollectionBackend` to the memory reference backend and the S3 adapter.
The [backend proof](csp5/object-collection-2026-09-12/verification.json) binds all ten committed files to twelve checks.
The original capability tests fail three cases before either backend implements the new interface.

Both final storage race suites pass 223 test events, including parents, without failures or skips.
The Go 1.26.6 and 1.25.12 runs repeat the same cases. Package lint, goago, and source prose pass.
The proof preserves one integer-conversion diagnostic and three prose diagnostics from earlier runs.

Inventory requires a nonempty namespace and permits one through 1,000 entries per page.
Memory tests cover empty stores, page boundaries, complete traversal, independent results, cancellation, and concurrent replacement.
Conditional deletion preserves a replacement when the supplied validator is stale.

S3 tests use the pinned SDK and local HTTP servers. They verify URL decoding, continuation tokens, conditional headers, error classification, and rejection before network access.
Malformed metadata, oversized XML, duplicate keys, missing validators, and inconsistent page counts cannot produce a usable partial page.
The adapter limits inventory responses to 8 MiB. It sends one exact quoted ETag for deletion without an unconditional fallback.

The new operations do not implement object generation collection. Pages do not form a snapshot, and validators can repeat when bytes repeat.
Coordinated retirement must protect publication, required generations, and active readers before deleting objects.
Bucket versioning can retain historical versions after current-object deletion. Current inventory size does not measure total bucket storage.

Live S3 service compatibility and native Windows or Linux execution remain unverified for these operations.
The HTTP tests establish the adapter wire contract. They do not qualify a hosted service.
Five concurrent tooling edits remain outside the source commit. Final task qualification must use its final committed module graph.

Object generation collection, runtime protection, observation-file collection, distinct-inventory retirement, twelve mapped subcases, full verification, required review, native CI, and merge remain open.
CSP5 remains unpublished until its complete contract passes.

## Runtime pin retention: 2026-09-12

Local commit `03590139` protects the original generation that a running pin selects.
The [pin retention proof](csp5/pin-retention-2026-09-12/verification.json) binds seventeen committed files to twenty-two checks.
The original regression loses that selection when an authority origin republishes its payload under a new generation ID.

The root client exposes an optional generation lease. The authority publisher forwards a private read-only value without exposing its publication store.
The client preserves configured store reads and verifies their manifest and payload against the protected artifact.
Embedded fallback also preserves configured read checks.

A first implementation bypassed a wrapper's read method. Broader tests then accepted invalid identity, payload, and schema cases.
Both toolchains recorded those three subcase failures and their parent. The corrected implementation passes the original tests without weakening their fixtures.

Both final pin suites pass 62 race test events. Both recovery selections pass 240 events.
These counts include parent tests and overlap. The Go 1.26.6 and 1.25.12 runs repeat the same cases.
No selected test fails or skips. The final pin suites also cover a later startup failure after protection starts.

Tests cover independent memory and filesystem results, missing embedded storage, cancellation, invalid artifacts, combined read and release errors, and unsupported stores.
Runtime shutdown keeps the lease until owned work stops. Failed startup releases it, including after a later writer-service error.
Collectors still must retain configured pins and other persistent requirements after shutdown.

The package linter found seven diagnostics in earlier CSP5 code.
Repairs separate checkpoint decoding, migration preparation, partial-removal checks, and retained startup at their owning functions.
Checkpoint indexes use checked conversion. File timestamp checks compare instants. Final lint passes without policy changes or suppressions.

Policy, root dependency checks, catalog dependency checks, and source prose pass.
The proof preserves the intermediate lint diagnostic and the seven-sentence paragraph diagnostic. A paragraph break resolves the prose failure.
Generated API documentation reflects the new capability and current source locations.

Five concurrent tooling edits remain outside the source commit. Final task qualification must use its final committed module graph.
The complete runtime suite still needs final source qualification and its public fixture.
Object generation retirement, automatic runtime collection, observation-file collection, distinct-inventory retirement, twelve mapped subcases, review, native CI, and merge remain open.
CSP5 remains unpublished until its complete contract passes.

## Checked private record removal: 2026-09-12

Local commit `98961b9f` adds removal under the native private-record publication lock.
The [record removal proof](csp5/record-removal-2026-09-12/verification.json) binds three committed files to eleven checks.
The initial capability test fails before the operation exists. It does not identify an existing automatic deletion defect.

Both complete private-file package runs pass 116 race events, including parents, without failures or skips.
The Go 1.26.6 and 1.25.12 runs repeat the same cases. The focused selection passes twelve events.
Checks cover changed content, active publishers, unsafe selections, replaced directories, cancellation, empty records, and removal retries.

Expected bytes and checked file metadata constrain removal. The operation shares ownership with `PublishFileContext` and excludes its concurrent publishers.
A failed directory synchronization after unlink reports visible removal through `PublicationError`.
An absent-file retry verifies directory ownership and synchronizes the directory without recreating data.

Policy, package lint, and prose pass. The prose snapshot precedes two code-only guards on absent-file retries.
Documentation bytes and source comments remain identical. Native Windows and Linux execution remain unverified for this checkpoint.

The runtime must trace accepted manual history and pending publication references before selecting unreachable inputs.
It must exclude input writers and preserve unknown or changed files. Automatic cleanup remains incomplete.

The current enterprise recipe assigns coordination to shared KV and immutable bytes to object storage.
The optional Starmap object catalog adapter remains a separate library extension.
The owner question asks whether object retention should require shared coordination or also support an S3-only coordination and crash-recovery protocol.
The question remains unresolved. The accepted architecture and scope remain unchanged. Local collection work continues independently.

Five concurrent tooling edits remain outside the source commit. Final task qualification must use its final committed module graph.
Object coordination, runtime collection, history retirement, twelve mapped subcases, full verification, required review, native CI, and merge remain open.
