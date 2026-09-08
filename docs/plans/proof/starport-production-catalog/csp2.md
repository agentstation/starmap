# CSP2: Persistent baseline and product paths

Status: native qualification remains incomplete. The owner resolved all three decisions and authorized reviewed publication on 2026-09-06.
See the [decision record](csp2/owner-decisions-2026-09-06.md). Service-managed primary configuration now passes local checks in commit `707d63db`.
The [service configuration evidence](csp2/service-configuration.md) records its scope. Required review and native qualification remain open.

The public accessor, root resolution, and persistent baseline export now pass local checks.
The full A01 through A04 contracts remain incomplete.

## Implemented behavior

`starmap.EmbeddedGeneration` returns a verified manifest and independent payload without application initialization.
Persistent Starmap startup exports that generation under `<data>/catalog/baseline/<generation-id-hash>/`.
The export preserves an existing accepted catalog head and refuses conflicting baseline bytes.
Staged writes include file synchronization, atomic directory promotion, bounded verification, and retry after process interruption.

Interrupted stages remain untouched because their abandoned ownership is not yet established.
These tests prove process-interruption recovery on macOS. They do not qualify power-loss recovery or Windows durability.

The public `pkg/productpaths` resolver defines distinct configuration, data, state, and cache roots for both products.
Native defaults cover Linux XDG paths, macOS application directories, and Windows roaming and local application directories.

Only macOS execution has local evidence. Linux and Windows execution remain UNVERIFIED.
Explicit roots avoid home-directory lookup. Relative leaves use the configuration root and retain their anchor and origin.

Starmap flags, environment, explicit dotenv files, YAML, and Go options now use this resolver.
The selected configuration root stays fixed after file selection. Unknown Go path names fail without exposing their values.
Recognized legacy `.starmap` state causes a migration refusal when new defaults would abandon it.

Persistent runtimes now hold `.owner.lock` under their selected state directory.
Another runtime cannot own that directory, including when its listen port differs.
Shutdown refuses new acquisition and retains ownership until active runs and scheduled work stop.

A timeout retains the lock until the remaining work and shared lease release finish. The kernel releases the lock after process exit.
The lock file stays in place. Its presence alone does not establish an active owner.

`owner.json` now records schema version 1, product, deployment, and instance.
The runtime refuses conflicting or invalid records before catalog access. It never rewrites an existing owner record.
Starmap passes `STARMAP_DEPLOYMENT_ID` and `STARMAP_INSTANCE_ID` through its node configuration layers.
The defaults are `local` and `default`. Starport host integration remains part of CSP8.

New runtime seeds use `R/instance-seed`. Persistent identity binds that seed to the recorded owner.
Host and port changes do not change identity. The owner record also binds an explicit scheduler-identity override through its SHA-256 digest.
Missing, empty, oversized, invalid, or legacy seeds in existing state require explicit recovery or migration.
A process-interruption test verifies recovery between owner-record creation and initial seed creation.

`runtime.PrepareDirectoryMigration` now records a runtime file inventory under an explicit operation ID.
It requires absolute, separate source, target, and journal roots. It resolves existing parent symlinks before it compares these locations.
Operators must stop the source process. Preparation holds its directory lock and rejects an existing target.

Preparation hashes source files, checks seed syntax, and verifies a supplied identity against an existing owner record.
It records legacy identity as unverified when no owner record exists. It cannot reconstruct the old host-dependent identity from a seed alone.
The inventory relocates the legacy seed to `instance-seed` and excludes the process lock and old owner record from copied files.

The immutable manifest retains paths, sizes, checksums, identity, requested ownership, and the old owner-record digest when present.
The bounded journal records ordered events with a digest chain. Changed intent, changed open inputs, and conflicting file paths cause refusal.
Recovery archives a valid partial suffix before truncation. Complete corruption and conflicting archives remain untouched.

The public preparation API creates no destination and does not switch configuration roots.
The journal has no public phase-advance API. The migration engine and CLI must own those transitions after their actual work succeeds.

The `runtime.StageDirectoryMigration` API copies the prepared source into `<target-parent>/.migration-<manifest-sha256>/`.
It retains the source and journal locks and holds a separate staging-directory lock during copy and verification.
The `.migration-pending.json` marker binds that directory to the manifest and blocks runtime initialization.

Copy writes use private `.migration-work/<target-path-sha256>.partial` files and publish complete files without replacement.
Existing staged files must match their expected size and SHA-256 digest. Unknown entries, symlinks, and changed files cause refusal.
Retries can restart operation-owned partial copies. They preserve conflicting complete files and unrelated entries.

Retry removes the scratch name before it creates a new file. It never truncates a retained partial file.

The copy engine records `copied` after file publication and records `verified` after a complete destination check.
Its final target remains absent. The configured roots remain unchanged, and the staged directory cannot start a runtime.
Five child-process cases cover exit after stage preparation, during copy, after file publication, after `copied`, and after `verified`.

Stage initialization uses native directory publication without replacement. Three macOS cases cover an existing empty directory, populated directory, and file.
Linux uses `RENAME_NOREPLACE`, whose support depends on the filesystem. See the [Linux manual](https://man7.org/linux/man-pages/man2/renameat2.2.html).
The initial Windows implementation used `MoveFileEx` without replacement or cross-volume copy flags.
The later [directory publication contract](#directory-publication-contract) replaces that pathname operation with directory handles.

The staging tests prove source-byte preservation and staged recovery. They do not qualify complete migration, power-loss recovery, or Windows durability.
Windows and Linux cross-compilation passed. Both still require native filesystem and permission checks.
Process exit before initial stage publication can leave a private `.migration-build-*` directory. Automatic cleanup of abandoned initial stages remains incomplete.

## Migration publication and configuration

The `runtime.PublishDirectoryMigration` API validates retained catalog layers before it publishes the target without replacement.
It records `promoted`, writes the source retirement record, writes the target receipt, and removes the pending marker in that order.
The source data files remain intact. New runtimes refuse the retired source.
Older binaries must stay stopped because they do not honor retirement records.

Five child-process cases cover exit after rename, promotion, source retirement, receipt publication, and target activation.
Retries verify the immutable receipts, owner record, and seed. They preserve catalog changes made by an active replacement.
Successive moves bind the preceding receipt digest into the next migration manifest.
An existing target without matching operation records causes refusal.

The `Runtime.CompleteDirectoryMigration` method requires an active replacement with the selected directory, owner, and retained identity.
It records `completed` without editing configuration files. Duplicate completion does not append another event.
Unrelated runtimes, changed identities, closed runtimes, and cancelled contexts cause refusal.

The canonical schema already accepts `STARMAP_SCHEDULER_IDENTITY`, YAML `scheduler_identity`, and `--scheduler-identity`.
The path report now includes its value and origin. The existing canonical precedence still applies to this node-scoped setting.
An explicit empty value selects derived identity. Existing owner records still refuse a changed or cleared override.
The application test publishes a real runtime migration, selects it through configuration, completes it, and reopens it from a saved file.

Catalog validation covers retained source and provider layers plus their merge with this binary's catalog.
It does not qualify every host configuration, backend, complete migration procedure, or native filesystem.

## Runtime migration CLI

The CLI now exposes `migrate runtime prepare`, `stage`, `publish`, and `complete`.
Application adapters select the target owner from the configured product, deployment, and instance.
The journal defaults to `<state>/migrations`. An explicit `--journal-root` remains available.

Completion requires matching saved and effective directory, scheduler identity, deployment, and instance values.
The loader records the digest of the exact configuration bytes it parses.
The completion command checks that digest before and after opening its temporary replacement runtime.
Publication verification runs before runtime initialization. Completion closes the temporary runtime and preserves configuration content.

Operators save the new selection and update service definitions through their deployment process.
The CLI does not persist a service's launch arguments or edit configuration files.
The [CLI procedure](../../../CLI.md#runtime-migration) states this boundary and the retry sequence.

Tests exercise the four commands, saved-file selection, duplicate completion, and reopening after command shutdown.
Refusal cases cover flags without a saved selection, changed configuration, missing arguments, relative paths, invalid output, and extra arguments.
Separate tests verify the canonical journal root and target owner.
Publication preflight refuses an unpublished target or conflicting receipt without creating a target or rewriting those records.

A regression replaced the target directory after publication preflight. Ordinary runtime startup initialized that replacement before the fix.
The CLI now supplies `runtime.WithPublishedDirectoryMigration` when it opens the replacement.
This option requires the exact operation and verifies its records under the directory lock before owner, seed, or catalog initialization.
The regression and CLI checks pass with that requirement.


## Evidence

| Check | Result and scope |
| --- | --- |
| [Public accessor regression](csp2/embedded-accessor-red.json) | Missing public accessor before implementation. |
| [Persistent cold-start regression](csp2/persistent-cold-start-red.json) | Missing export before implementation. The current cold-start test passes. |
| [Path regressions](csp2/path-freeze-red.json) | Reproduced configuration-root resolution after selection and ignored unknown names. |
| [Recovery and path fixes](csp2/recovery-and-freeze-checks.json) | 14 test and subtest results passed with the race detector. |
| [Paths and application checks](csp2/baseline-path-final-checks.json) | 188 race test results passed. Go 1.25.12, lint, and ago passed before directory-lock changes. |
| [Ownership regressions](csp2/directory-owner-red.json) | Reproduced concurrent ownership and acquisition after shutdown. |
| [Runtime and application checks](csp2/directory-owner-checks.json) | 178 race test results passed before the additional process-recovery tests. |
| [Ownership and recovery checks](csp2/ownership-final-checks.json) | 239 race test results passed with zero skips. Go 1.25.12, lint, and ago passed. |
| [Shutdown timeout](csp2/ownership-timeout-check.json) | The timed-out runtime retained ownership until its blocked run stopped. |
| [Lease-release regression](csp2/lease-release-red.json) | Reproduced directory release before shared lease release completed. |
| [Lease-release fix](csp2/lease-release-checks.json) | All 67 runtime race test results passed. Go 1.25.12, lint, and ago passed after the ordering fix. |
| [Owner-record regressions](csp2/owner-record-red.json) | Reproduced missing ownership records and acceptance of incompatible owner metadata. |
| [Identity regression](csp2/stable-owner-identity-red.json) | Reproduced a persistent identity change after a listen-port change. |
| [Owner and seed checks](csp2/owner-seed-final-checks.json) | 206 race test results passed. Go 1.25.12, lint, and ago passed. |
| [Owner process recovery](csp2/owner-process-recovery.json) | Two test results passed for process exit between owner and seed initialization. |
| [Scheduler override regression](csp2/owner-override-red.json) | Reproduced an identity change that bypassed the owner record. |
| [Final owner binding](csp2/owner-binding-final.json) | 209 race test results, CLI help, lint, and ago passed after override binding. |
| [Owner binding on Go 1.25](csp2/owner-binding-go125.json) | Owner, override, and process-recovery checks passed on Go 1.25.12. |
| [Journal integrity regressions](csp2/migration-journal-integrity-red.json) | Reproduced append after input changes and acceptance of conflicting inventory paths. |
| [Journal recovery](csp2/migration-journal-recovery.json) | 54 race test and subtest results passed with zero skips. Eight cases exercise process exit. |
| [Preparation regression](csp2/migration-preparation-red.json) | The public preparation API did not exist before implementation. |
| [Preparation and identity checks](csp2/migration-preparation-first-check.json) | 68 race test and subtest results passed. Source inspection and journal recovery passed. |
| [Migration checks on Go 1.25](csp2/migration-preparation-go125.json) | The same 68 results passed on Go 1.25.12 with zero skips. |
| [Migration, runtime, and application checks](csp2/migration-preparation-checks.json) | 321 race test and subtest results passed with zero skips. Runtime lint and whole-module ago passed. |
| [Migration architecture and task gates](csp2/migration-preparation-gates.json) | Go vet and three architecture guards passed. The CSP2 acceptance gate remains incomplete. |
| [Catalog migration help](csp2/migration-cli-help.json) | The existing command describes the resolved store instead of the former hardcoded directory. |
| [Missing staging API](csp2/migration-stage-red.json) | The staging API did not exist before implementation. |
| [Staging recovery](csp2/migration-stage-recovery.json) | 20 race test and subtest results passed with zero skips. Five cases use actual process exit. |
| [Partial hard-link regression](csp2/migration-partial-link-red.json) | Reproduced source truncation when a retained partial file was a hard link to the source. |
| [Partial-file replacement](csp2/migration-partial-link-checks.json) | 21 focused race results passed. The new regression passed on Go 1.25.12. Whole-repository lint and ago passed. |
| [Staging platform checks](csp2/migration-stage-platform-checks.json) | 65 Go 1.25.12 race results passed. Windows and Linux x64 cross-compilation passed. Native execution remains UNVERIFIED. |
| [Cleanup error propagation](csp2/migration-stage-go125-final.json) | All 20 focused staging results passed on Go 1.25.12 after cleanup-error propagation. |
| [Staging architecture and task gates](csp2/migration-stage-gates.json) | Go vet and three architecture guards passed. CSP2 still lacks its complete acceptance evidence. |
| [Staging integration](csp2/migration-stage-checks.json) | 341 runtime, application, and bootstrap race results passed before the additional partial-file fix. |
| [Final runtime checks](csp2/migration-stage-runtime-final.json) | All 177 runtime race results passed with zero skips after the partial-file fix. |
| [Missing publication API](csp2/migration-publication-red.json) | The target publication API did not exist before implementation. |
| [Publication recovery](csp2/migration-publication-recovery.json) | 15 race results passed, including five process-exit cases. |
| [Missing completion API](csp2/migration-completion-red.json) | The runtime completion API did not exist before implementation. |
| [Publication integration](csp2/migration-publication-checks.json) | 358 runtime, application, and bootstrap race results passed with zero skips. Whole-module lint and ago passed. |
| [Final completion and platform checks](csp2/migration-publication-platform-checks.json) | 16 focused race results passed on Go 1.25.12 after the cancellation check. Windows and Linux x64 cross-compilation passed. |
| [Missing identity report](csp2/migration-node-identity-red.json) | The path report lacked the retained identity and its origin. |
| [Canonical identity integration](csp2/migration-identity-final-checks.json) | 126 application race results passed with zero skips. Thirty focused results passed on Go 1.25.12. Lint and ago passed. |
| [Publication architecture gates](csp2/migration-publication-gates.json) | Go vet and three architecture guards passed. All 22 CSP2 component subcases still lack registered acceptance checks. |
| [Missing migration CLI](csp2/migration-cli-red.json) | The command did not recognize runtime migration arguments before implementation. |
| [Initial CLI integration](csp2/migration-cli-first.json) | The full prepare, stage, publish, saved-selection, and complete sequence passed. |
| [Focused CLI and publication checks](csp2/migration-cli-focused.json) | Five race results passed before the additional input and journal-default cases. |
| [CLI platform checks](csp2/migration-cli-platform-checks.json) | 35 focused race results passed on Go 1.25.12. Windows and Linux x64 executable cross-builds passed. |
| [CLI architecture gates](csp2/migration-cli-gates.json) | Go vet, three architecture guards, and CLI help passed. The CSP2 acceptance gate remains incomplete. |
| [CLI integration](csp2/migration-cli-checks.json) | 376 race results passed with zero skips before the startup guard. Whole-module lint and ago passed. |
| [Startup replacement regression](csp2/migration-startup-guard-red.json) | Reproduced initialization of a different directory after successful preflight. |
| [Required migration startup](csp2/migration-startup-guard-checks.json) | Ten focused race results passed after the startup guard. |
| [Guarded CLI integration](csp2/migration-cli-final-checks.json) | 331 race results and ten Go 1.25.12 results passed. Windows and Linux x64 builds passed. Lint found one local-name issue. |
| [Final static checks](csp2/migration-cli-final-static.json) | Lint, ago, and the startup regression passed after the local-name correction. |
| [CSP1 regression](csp2/csp1-regression.json) | All six component subcases passed after the runtime changes. No primary case gained release qualification. |
| [Architecture checks](csp2/architecture-and-admission-checks.json) | Package layout, 13 ownership checks, eight dependency checks, and three application tests passed. |

The [owner architecture checks](csp2/owner-architecture-and-docs.json) passed after owner and seed changes.
The [CSP1 regression](csp2/owner-csp1-regression.json) also passed all six component subcases.
The [CSP2 task gate](csp2/migration-preparation-task.json) still fails: all 22 selected subcases lack registered behavior checks.
Local package results do not replace that acceptance gate.

The [directory-lock input manifest](csp2/ownership-input-manifest.json) preserves 64 source and test hashes from the earlier checkpoint.
The [owner-record input manifest](csp2/owner-record-input-manifest.json) preserves the preceding ownership implementation inputs.
The [preparation input manifest](csp2/migration-preparation-input-manifest.json) preserves the preceding implementation and test hashes.
The [staging input manifest](csp2/migration-stage-input-manifest.json) records 23 source and test hashes after the partial-file fix.
The [publication input manifest](csp2/migration-publication-input-manifest.json) records the preceding migration, ownership, runtime, and application inputs.
The [CLI input manifest](csp2/migration-cli-input-manifest.json) records the preceding inputs after the startup guard and local-name correction.

Whole-repository Go lint passed after the migration CLI and startup guard. The maintained document checks and structure check passed after prose corrections.
The repository writing command still reports three unchanged diagnostics in historical proof files.
The [staging document checks](csp2/migration-stage-documents-final.json) retain the preceding results.
The [publication document checks](csp2/migration-publication-documents-final.json) retain the preceding results.
The [CLI document checks](csp2/migration-cli-documents-final.json) retain the preceding results.

## Completed migration acknowledgement

The [legacy-root regression](csp2/migration-legacy-guard-red.json) reproduced default-root refusal after a completed move.
The [first integration check](csp2/migration-completion-first.json) passed four race results after the correction.
The [focused completion checks](csp2/migration-completion-focused.json) passed 25 race results.
These counts include parent tests and their subtests.

The [integration checks](csp2/migration-completion-checks.json) passed 354 runtime and application race results and Go vet.
The Make lint command reported paths from an absent sibling worktree.
The [isolated static checks](csp2/migration-completion-static.json) passed whole-module lint and ago with no findings, stale ignores, or errors.

The [platform checks](csp2/migration-completion-platform-checks.json) passed 25 focused race results on Go 1.25.12 and three architecture guards.
Windows and Linux x64 executable cross-builds passed. Native filesystem qualification remains UNVERIFIED.
The [task gate](csp2/migration-completion-task.json) still fails. The 22 CSP2 component subcases lack registered acceptance checks.

Completion now writes `R/.migration-completed.json` after the final journal event.

`runtime.ReadDirectoryMigrationCompletion` verifies the preserved source inventory, retirement record, target receipts, owner, and seed without writes.
`runtime.WithCompletedDirectoryMigration` requires the same proof under the runtime-directory lock before persistent initialization.

Default-root startup accepts one matching completed legacy move. Changed source files, wrong identity, or another recognized legacy root cause refusal.
Ordinary catalog reads do not hash the preserved source. Later catalog updates in the target remain intact.
Verification requires the immediate preserved source but does not read the operation journal or earlier migration sources.

Two child-process cases exit after the final journal event and after the completion record.
Retries repair a missing record without duplicating journal events.
Tests also cover changed evidence, a replaced target, journal rollback, missing sources, copied receipts, and successive completed moves.

The [completion input manifest](csp2/migration-completion-input-manifest.json) records 46 source, test, and module hashes for this checkpoint.

## Source cache and checkout roots

The [source-path regression](csp2/source-paths-red.json) reproduced the old `.starmap` defaults before the change.
The [first package checks](csp2/source-paths-first.json) passed after source and host integration.
The [integration checks](csp2/source-paths-integration.json) passed five race results, including both explicit and host-selected HTTP paths.
The [directory contract checks](csp2/source-paths-contracts.json) passed five race results after removal of obsolete default constants.

The [package suite](csp2/source-paths-checks.json) passed 393 race results with zero skips before the import separation guard.
Lint found one slice-capacity issue. The [corrected static checks](csp2/source-paths-final-static.json) passed lint and ago without findings.
The [platform checks](csp2/source-paths-platform-checks.json) passed ten focused Go 1.25.12 race results, Go vet, and three architecture guards.
Windows and Linux x64 executable cross-builds passed before the import separation guard. Native qualification remains UNVERIFIED.

`productpaths.SourceDirectoriesAt` defines source parents under one cache root. Native defaults require an explicit product name.
`acquisition.WithSourceDirectories` accepts host-owned defaults without reading ambient product settings or creating files.
Starmap update and server acquisition use the resolved application cache root.

HTTP acquisition writes `models.dev/api.json` and its metadata under the selected cache parent.
Git acquisition uses `models.dev-git` under the selected checkout parent. Provider and author logos use that same checkout.
`sync.WithSourcesDir` overrides both parents and preserves its existing path semantics.

Invalid native defaults cause refusal before acquisition or cleanup. Explicit paths still work when native settings are unavailable.
The path report exposes the actual HTTP cache and Git checkout directories without creating files.
These changes do not migrate legacy caches or qualify native filesystem behavior.

The [import regression](csp2/source-import-separation-red.json) reproduced publication when the selected checkout overlapped the human catalog workspace.
Sync and release import now check checkout separation before publication. Projection repeats the check before workspace writes.
Workspace operations retain their selected source defaults through projection.

The [guard checks](csp2/source-import-separation-checks.json) passed 22 acquisition race results and five focused Go 1.25.12 results.
Whole-module lint and ago passed after the guard.

The [final build and task checks](csp2/source-paths-final-gates.json) passed Windows and Linux x64 cross-builds after this guard.
The task gate still reports 22 unverified CSP2 component subcases. Package tests do not substitute for those acceptance checks.
The [source input manifest](csp2/source-paths-input-manifest.json) records 51 source, test, and module hashes.

## Passive file inventory checkpoint

The [command regression](csp2/file-manifest-cli-red.json) records the missing `config paths` command before implementation.
The command now uses the application-owned file inventory without opening a runtime or creating product files.
JSON and YAML report selected roots, file patterns, origins, anchors, creation conditions, recovery rules, and external destination roles.
Table and wide output list roots and managed file roles.

The [binary rehearsal](csp2/file-manifest-command-rehearsal.json) passed JSON and table commands with an isolated home and no provider credentials.
The default report contained 26 file entries and seven external roles. The home remained empty after both commands.
Six reserved entries identify unimplemented administration, download staging, trust, and log writers.
File patterns do not prove existence, permissions, active ownership, or full descriptor-policy coverage.

The [corrected checks](csp2/file-manifest-corrected-checks.json) passed 161 package race results and ten focused Go 1.25.12 race results.
No test skipped. Whole-module lint and ago passed.
The tests cover selected origins, relative anchors, credential-value exclusion, disabled workspace siblings, four output formats, and argument refusal.
The inventory covers all nine persistent files observed after startup and an accepted catalog publication.

The [earlier platform checks](csp2/file-manifest-platform-checks.json) passed Go vet, three architecture guards, and Windows and Linux x64 executable cross-builds.
Their focused suite failed because its fixture supplied an internal-source credential while selecting GitHub.
The corrected fixture selects an internal Starmap source and retains the credential-exclusion assertion.
The [earlier full suite](csp2/file-manifest-final-tests.json) preserves that same fixture failure.

The [first inventory run](csp2/file-manifest-contracts.json) expected catalog-store files before any accepted publication.
The corrected coverage test publishes a catalog before it inventories persistent files. Startup alone created five files in that rehearsal.
These fixture failures do not establish product regressions.

The [input manifest](csp2/file-manifest-input-manifest.json) binds the current source and test bytes to this checkpoint.
Native platform execution, optional file flows, and the 22 registered component acceptance checks remain incomplete.

## Relative-anchor migration guard

The [regression run](csp2/relative-anchor-red.json) reproduced two silent anchor changes.
An environment-selected relative runtime path created replacement state while the old seed remained under an unknown prior working directory.
A relative primary filename selected different configuration bytes when both the old working directory and the new configuration root contained that name.

Starmap now refuses ambiguous legacy relative selectors before primary-file selection or catalog initialization.
An absolute selector keeps its selected location. The guard never searches the current directory to infer a previous anchor.
New relative paths require the node-owned `relative_path_base: config` declaration through YAML, environment, explicit dotenv, or a flag.
The declaration follows existing node-setting precedence and appears with its origin in the file report.

Empty declarations restore refusal. Invalid values cause a validation error without value disclosure.
Primary-file selection requires bootstrap intent before that file loads. An absolute primary file can declare intent for its relative leaf settings.
The declaration does not move files or prove migration completion. Operators must resolve old relative values to their previous absolute paths before upgrades.

The guard covers runtime state, canonical and legacy workspace settings, primary configuration, and update workspace and source selectors.
Update resolves selected paths before catalog and credential access. Source environment fallback resolves once at that command boundary.
The new `catalog_store_path` has no prior working-directory contract and retains its configuration-root anchor.
The direct Go `sync.WithSourcesDir` option retains its caller-owned path semantics.

The [first checks](csp2/relative-anchor-first.json) passed ten race results after the guard.
The [expanded checks](csp2/relative-anchor-checks.json) passed 195 package race results and 12 focused Go 1.25.12 results.
The [final checks](csp2/relative-anchor-final-checks.json) include absolute-path restart identity and explicit dotenv bootstrap coverage.
Their focused Go 1.25.12 suite passed 15 race results. Go vet, whole-module lint, and ago passed.

The full final suite passed 458 race results across bootstrap, application, runtime, update commands, config commands, and product paths. No test skipped.

The [binary rehearsal](csp2/relative-anchor-cli-rehearsal.json) passed three CLI cases after a fresh build.
It refused ambiguous primary and source selections and accepted an explicit configuration-root declaration.
The two original input files retained their hashes. No additional files appeared under the isolated product home.

The [platform checks](csp2/relative-anchor-platform-checks.json) passed three architecture guards and Go 1.25.12 Windows and Linux x64 executable cross-builds.
That pre-registration task gate reported 22 missing component checks. Native execution remains UNVERIFIED.
The [input manifest](csp2/relative-anchor-input-manifest.json) records 53 source, test, module, and registry hashes.
Starport must apply the same node-owned semantics through its own namespace in CSP8.

The [registered task gate](csp2/relative-anchor-registered-gate.json) reran nine named tests for `A03.relative_anchor_migration_guard` and passed that component check.
The other 21 CSP2 component checks remain UNVERIFIED. The task gate still fails, and all 50 primary cases remain UNVERIFIED.

The later [file-source regression](csp2/relative-file-source-red.json) reproduced baseline writes with another ambiguous relative selector.
The file catalog source still received its raw filename after the first path guard.
Starmap now resolves `catalog_source_url` for file sources through the same guard before baseline writes.
Runtime composition receives the absolute filename. The file inventory reports that path and anchor in its conditional `source-file` entry.

The [file-source checks](csp2/relative-file-source-first-checks.json) passed both regression and refresh tests, whole-module lint, and ago.
The refresh test reads the configured catalog from two other working directories that contain conflicting filenames.
It verifies healthy refreshes and preserves the operator-owned payload bytes.
The registered anchor guard now includes these two named tests.

The [final file-source checks](csp2/relative-file-source-final-checks.json) passed 460 package race results and 17 focused Go 1.25.12 results without skips.
Go vet and the updated Windows and Linux x64 cross-builds passed.
The registered guard passed all 11 named behavior tests. The other 21 component checks remain UNVERIFIED.
The [final input manifest](csp2/relative-file-source-input-manifest.json) records the 53 source, test, module, and registry hashes for this result.

## Bounded file inspection

The [inspection regression](csp2/file-inspection-red.json) records refusal of the formerly absent `--inspect` flag.
`config paths --inspect` now adds filesystem observations to the passive manifest. Plain `config paths` retains its passive behavior.
The scanner counts unmatched entries within its budget and reads directory names in bounded batches.
It skips observed symbolic-link targets and preserves literal glob characters in workspace staging names.

Linux and macOS report POSIX mode bits and numeric ownership. These values do not prove effective access or ACL safety.
The runtime owner comparison reads at most 4,097 bytes without seed access, file writes, or runtime lock attempts.
A match covers only the canonical record binding. Native Windows permission and owner adapters remain unverified.
The report states its incomplete status when metadata access fails or its entry budget prevents completion.

The [platform checks](csp2/file-inspection-platform-checks.json) passed 30 focused race results on Go 1.25.12 without skips.
They also passed Windows and Linux x64 executable cross-builds. Those builds do not qualify native behavior.
The [static checks](csp2/file-inspection-static.json) passed whole-module lint and ago without findings.

The [binary rehearsal](csp2/file-inspection-cli-rehearsal.json) passed JSON, table, and one-entry-budget cases after a fresh build.
All three commands preserved the isolated configuration file and created no product directories.
The runtime test preserves owner and seed bytes and verifies that the active directory lock still refuses a second runtime.
The application test observes the active owner binding and a staging directory whose workspace name contains literal brackets.

The [first integration run](csp2/file-inspection-final-tests.json) passed 489 race results and failed `TestCloseNeverRacesACallerOwnedRun` with a five-second shutdown timeout.
The [isolated recheck](csp2/file-inspection-close-recheck.json) passed ten repetitions without changing the test or runtime.
The cause of the first timeout remains undetermined. The failed evidence remains part of this proof.
The [integration recheck](csp2/file-inspection-integration-recheck.json) passed all 490 race results without failures or skips under the same command.

The [task checks](csp2/file-inspection-task-checks.json) passed three architecture guards and the registered relative-anchor component.
The other 21 CSP2 component checks remain UNVERIFIED. Bounded inspection alone does not satisfy the complete file-manifest acceptance contract.
The [input manifest](csp2/file-inspection-input-manifest.json) records 31 source, test, module, and registry hashes.

## Runtime permission guard

The [permission regression](csp2/private-runtime-red.json) reproduced runtime writes with group or public directory permissions and a publicly readable lock file.
Linux and macOS now require the effective UID and owner-only mode bits on the runtime directory, lock, owner record, and seed.
The application checks its configured runtime path before baseline export. Runtime ownership repeats the check before identity writes.
The initial guard read mode and ownership metadata without changing existing modes or bytes. The macOS ACL extension appears below.

Other platforms retain file-type checks while native permission adapters remain unverified.

The [first focused run](csp2/private-runtime-focused.json) exposed successful-runtime fixtures that relied on Go's default temporary directory modes.
Successful runtime fixtures now explicitly select `0700`. Rejection tests still set their exposed modes explicitly.
The [corrected checks](csp2/private-runtime-checks.json) passed 11 focused Go 1.25.12 race results without skips.
They also passed whole-module lint and ago. No production permission check or test assertion changed to permit exposed modes.

The application recovery test verifies refusal before baseline export and identity writes.
It verifies that diagnostics remain available after refusal and startup succeeds after an explicit operator mode correction.
Identity-file tests preserve bytes during refusal and reopen successfully after restoring private modes.
The migration path uses the same directory check. Operators must verify the service identity and correct permissions before retrying an affected migration.

The [integration run](csp2/private-runtime-integration.json) passed 496 results and reported three failed results without skips.
The linked-seed subtest and its parent exposed a changed error type. The permission preflight now preserves the existing recovery-conflict error for linked identity records.
The remaining failure repeated the runtime shutdown timeout. The two executable cross-builds passed during that run.

The [late-publication regression](csp2/close-late-publication-red.json) proved that a provider could publish after runtime cancellation.
Publication now checks cancellation before provider retention, source retention, catalog rebuild, and commit.
The close race test now waits for actual runtime cancellation before releasing its blocked provider, as its original scenario requires.
Separate provider and source tests reject late results without changing retained layers or the effective generation.
A canceled provider publication also preserves successful acquisition freshness. Already committed work retains its existing completion semantics.

The [corrected focused checks](csp2/private-runtime-corrected-checks.json) passed 19 Go 1.25.12 race results, lint, and ago before the freshness assertion.
The final checks below include that assertion. Permission, shutdown, and publication time limits remain unchanged.

The [final checks](csp2/private-runtime-final-checks.json) passed 501 integration race results and 19 focused Go 1.25.12 race results without skips.
Whole-module lint, ago, and Windows and Linux x64 executable cross-builds passed.
The [final input manifest](csp2/private-runtime-final-input-manifest.json) records 34 source, test, module, and registry hashes.
The acquisition, public settings, composition, and server fixtures now select private runtime directories too.

The [consumer checks](csp2/private-runtime-consumer-checks.json) passed 22 acquisition and 36 public configuration results without skips.
The server cascade retained one non-private fixture. That fixture now selects a private runtime child directory.
The deployment-name check also confused the implemented node-owned `STARMAP_RELATIVE_PATH_BASE` setting with an unknown catalog setting.
The check now recognizes that exact node setting. Its regression still rejects misspelled node and catalog names.

The [corrected consumer checks](csp2/private-runtime-consumer-corrected.json) passed 38 server and settings race results without skips, followed by lint and ago.

## Native macOS ACL guard

The [native regression](csp2/darwin-acl-red.json) failed for all four runtime identity roles despite private mode bits.
The adapter now inspects native ACL metadata through an open file descriptor.
It refuses non-owner allow entries with nonzero rights, including inherited and inheritance-only grants.
Owner-specific allow entries and deny-only entries remain valid. This rule does not compute effective access from ordered entries.

The adapter validates the descriptor identity before its native read. It does not read identity payloads or change file bytes, modes, or ACLs.
Unsupported, incomplete, or oversized native metadata produces a typed error. The adapter limits the native buffer to 8,192 bytes and 128 entries.
These bounds do not establish a hard elapsed-time limit for native filesystem or identity services.

The adapter uses `fgetattrlist` and `mbr_uid_to_uuid` from `libSystem` through `github.com/ebitengine/purego v0.11.0`.
This dependency supports the module's Go 1.25 floor and avoids a Cgo requirement. Production code does not invoke a shell or use raw syscall traps.
One process-lifetime library reference keeps the cached function pointers valid.
The local SDK headers `sys/attr.h`, `sys/kauth.h`, and `membership.h` define the ABI layouts and limits.

The [Apple source](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/man/man2/getattrlist.2) defines descriptor-based attribute reads.
The [purego source](https://github.com/ebitengine/purego/tree/v0.11.0) defines the native function binding and supported platforms.
The adapter uses the modern SDK's fixed-width attribute reference fields. It does not use the older archived manual's platform-dependent layout.

The [focused checks](csp2/darwin-acl-focused.json) passed 29 race results without failures.
They cover private ACLs, all four public-grant refusals, inherited grants, malformed buffers, and application recovery after explicit ACL removal.
The application test verifies refusal before baseline and identity writes, diagnostic availability, and successful corrected startup.
Fixture ACL changes apply only to test directories.

The [initial lint check](csp2/darwin-acl-lint-initial.json) found two unchecked integer conversions at the native API boundary.
The adapter now checks descriptor and attribute-offset ranges before use.

The [architecture checks](csp2/darwin-acl-architecture.json) passed package layout and dependency direction, but identified the new direct dependency.
The module graph adds only `purego`, with no transitive module requirements. The dependency uses the Apache-2.0 license.
The direct-module inventory now includes this intentional native API dependency. The guard retains its exact-set comparison.
The [corrected ownership check](csp2/darwin-acl-architecture-corrected.json) passed all 13 conditions, including six external consumer compilations.

The [final checks](csp2/darwin-acl-final-checks.json) passed 626 integration race results without failures or skips.
The focused native tests also passed 29 race results on Go 1.25.12 and 29 results with Cgo disabled.
The [input manifest](csp2/darwin-acl-input-manifest.json) records 39 source, test, module, and inventory hashes, plus three native SDK header hashes.

Whole-module lint and ago passed. Go 1.25.12 executable cross-builds passed for macOS, Linux, and Windows x64 with Cgo disabled.
Native Windows and macOS x64 execution remain UNVERIFIED. Cross-compilation cannot qualify their native ACL behavior.

The [evidence harness error](csp2/darwin-acl-harness-error.json) interrupted reporting after lint, before the builds.
The resumed harness repeated the unrecorded ago check and completed all three builds. It did not repeat the completed integration tests.
The [document checks](csp2/darwin-acl-documents.json) passed structure, changed prose, source hashes, and whitespace checks.
The repository-wide writing check retained only its three historical diagnostics. The plan remains at 500 lines with CSP2 in progress.

## Windows ACL implementation

The prior Windows runtime checked file types but not native owners or DACLs. POSIX mode arguments did not define private Windows creation.
Native fail-before execution remains UNVERIFIED because this machine runs macOS. No native Windows result exists for the new adapter either.
The [initial checks](csp2/windows-acl-initial-checks.json) record local policy tests, test-binary compilation, and static-analysis findings.

The adapter reads a security descriptor through an open file handle and verifies descriptor validity before reading basic ACEs.
The descriptor owner must match the process account SID. Grants may name that account, SYSTEM, or Administrators.
Other nonzero grants cause refusal, including inherited and inheritance-only entries. Deny entries remain valid.

Absent, null, and unsupported DACLs cause refusal. An empty DACL remains restrictive but does not prove usable access.

Private creation supplies an explicit owner and protected DACL in the native create call.
The native call uses exclusive creation, one direct child name, an open parent handle, and disabled reparse traversal.
It cannot replace an existing entry. Existing directory security descriptors remain unchanged.
Runtime directories, lock files, staged identity files, and migration staging roots use these primitives.
The runtime fixtures now select newly created private child directories on each platform.

The portable grant policy lives in `internal/runtimeacl/windows`, apart from the Windows native API adapter.
Local tests cover its owner, privileged-account, ordinary-account, empty, null, absent, unsupported, and deny-entry decisions.
These policy tests do not prove native descriptor decoding, filesystem ACL enforcement, or Windows recovery.
Native tests cover all four identity roles, inherited-parent isolation, explicit operator correction, exclusive creation, and descriptor distinctions.

The native SID conversion has one audited `gosec` exception. `GetAce` returns the field inside a validated basic ACE.
The adapter checks the ACE size, SID validity, and SID length. The exception does not suppress other unsafe operations or change linter policy.
The [corrected checks](csp2/windows-acl-corrected-checks.json) passed local policy tests, host lint, Windows runtime lint, and ago.
Whole-module Windows lint still identifies the pre-existing unsupported workspace exchange path. Its initial diagnostic remains in the proof.

The [Microsoft file-security reference](https://learn.microsoft.com/en-us/windows/win32/fileio/file-security-and-access-rights) defines default inheritance and creation-time security descriptors.
The [null-DACL reference](https://learn.microsoft.com/en-us/windows/win32/secauthz/null-dacls-and-empty-dacls) distinguishes unrestricted null DACLs from restrictive empty DACLs.
The [native object-attribute reference](https://learn.microsoft.com/en-us/windows/win32/api/ntdef/ns-ntdef-_object_attributes) defines relative handles and reparse refusal.

The prepared `native-runtime` job in `.github/workflows/pr.yaml` runs the runtime, ACL policy, application, and product-path suites on six native runners.
It uses Go 1.25.12 with Cgo disabled and retains JSON test events and platform metadata even after test failure.
The [workflow checks](csp2/windows-acl-workflow-checks.json) validate workflow syntax, package layout, and dependency direction locally.

The [integration checks](csp2/windows-acl-final-checks.json) passed 641 production race results and 25 CI results, with one CI-layout failure and no skips.
The repository permits one active PR workflow. The native matrix now lives inside that existing workflow, and the guard remains unchanged.
The [corrected workflow checks](csp2/windows-acl-workflow-corrected.json) passed 26 CI race results and validated the final workflow syntax.

Go 1.25.12 passed 44 focused race results. Host lint, Windows runtime lint, and ago passed.
Four native test binaries cross-compiled for Windows x64 and ARM64, covering the runtime and application packages.
The [final input manifest](csp2/windows-acl-final-input-manifest.json) records 48 source, module, test, and workflow hashes.
GitHub publication, dispatch, and native qualification remain pending.

## Remaining work

Complete effective access and native ownership reporting and qualify migration and optional file flows against the inventory.
The CLI verifies operator-saved configuration through the replacement runtime. Complete deployment and recovery procedures remain incomplete.
The former seed path was `R/catalog-runtime/instance-seed`. Startup now refuses that layout until explicit migration.

Directory locking does not establish fleet identity fencing or permit copied instance seeds.

Source acquisition now uses canonical cache defaults. Standalone CLI legacy selectors now have an explicit anchor boundary.

Add service and container recipes and complete native filesystem qualification. Verify Windows directory flushing and resolve unsupported workspace exchange before Windows support.
Run the full CSP2 acceptance gate before changing this task to done.
No commit, CI publication, or released-pair qualification exists for this task.

## Directory publication contract

Baseline export and runtime migration now share `internal/filepublish.DirectoryNoReplace`.
The operation accepts distinct direct child names under an open directory.
Linux uses `RENAME_NOREPLACE`. macOS uses `RENAME_EXCL`.
Windows opens the source relative to the directory handle and calls `NtSetInformationFile` with replacement disabled.

The Windows request uses a relative target and the same directory handle.
It encodes the native structure with bounded byte writes and ABI field offsets, without pointer casts or new lint suppressions.
The preceding Windows implementation rebuilt paths from `os.Root.Name()`.
A parent rename could redirect those paths. Native fail-before and corrected Windows execution remain UNVERIFIED.

The earlier [collision capture](csp2/publication-collision-red.json) exited successfully despite its filename.
It does not establish a fail-before result. Go checks for an existing destination directory before it calls rename.
The replacement requests exclusive publication in the filesystem operation itself.
The final baseline test requires a typed conflict and preserves the competing directory's identity and contents.

Tests cover existing empty directories, populated directories, files, symlinks, invalid names, and sixteen competing publishers.
The renamed-parent test uses Unicode names and creates a replacement directory at the original path.
It verifies publication under the open root and preserves the replacement directory.
Every losing publisher retains its staged payload. Exactly one publisher succeeds.

The native CI job now includes publication and baseline tests alongside runtime, application, path, and ACL tests.
The workflow remains local. It has not run on GitHub.
Successful rename does not establish power-loss durability. Existing synchronization errors still propagate.
Windows directory flushing, workspace exchange, and complete private-file coverage remain open.

[Initial checks](csp2/publication-initial-checks.json) retain three Windows integer-conversion findings.
Explicit conversion bounds resolved these findings. No linter policy changed.
[Platform checks](csp2/publication-final-platform-checks.json) passed 21 Go 1.25.12 race results with no skips.

Six Windows test binaries cross-compiled with Go 1.25.12.
They cover publication, baseline, and runtime for x64 and ARM64.

Windows-targeted lint, ago, dependency direction, and workflow syntax passed.

The initial integration command used an incorrect dependency-check script name.
That exit 127 remains recorded. The repository's `verify-catalog-dependency-direction.sh` passed in the platform checks.
Native Windows qualification and all affected full acceptance cases remain UNVERIFIED.

The [initial integration run](csp2/publication-integration-checks.json) passed 264 runtime and 166 application race results.
It also passed 46 bootstrap results before the five-minute package deadline expired during catalog YAML parsing.
That timeout remains recorded. The command used the nonexistent `internal/ci` package and recorded its setup failure.

The [corrected commands](csp2/publication-integration-corrected.json) passed all 47 bootstrap and 26 CI race results, with no skips.
Bootstrap ran with Go 1.25.12 and a ten-minute package deadline. No test assertions changed.

The final baseline conflict check passed on Go 1.25.12 and requires the typed conflict error.
See [the exact command and events](csp2/publication-baseline-final.json).
The source [input manifest](csp2/publication-input-manifest.json) records the worktree base and affected file digests.
Changes remain uncommitted. This evidence does not complete CSP2 or qualify a release.

The [targeted writing check](csp2/publication-targeted-writing.json) passed all ten changed artifacts.
The [repository writing check](csp2/publication-writing-rechecked.json) retained only the three historical evidence diagnostics.
Host lint, package layout, and the 500-line document structure check passed.
The plan still contains 38 tasks, 50 primary cases, and 324 required subcases.

## Directory flush access

`internal/filepublish.SyncDirectory` now owns platform directory flushes for baseline export, runtime identity, migration journals, catalog storage, and workspace staging.
The Windows adapter reopens the directory under its existing handle with write access, then calls `FlushFileBuffers`.
It uses an empty NT object name, as Go does when it opens the current directory relative to a handle.
The original pathname cannot redirect this operation after a parent rename.

The previous implementations opened directories with read access before flushing.
[Microsoft requires write access for `FlushFileBuffers`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers).
Workspace staging had the same issue for regular files. It now opens stage-owned files with `os.O_RDWR` before flushing.
Migration journal repair already reopens files with `os.O_RDWR` before truncation. That code required no access change.

The shared operation preserves access errors, unsupported-operation errors, and closed-root errors.
It does not change permissions or return success through a no-op fallback.
Tests cover repeated flushes, published bytes, renamed roots, and invalid roots.
A prepared Windows test requires refusal on the old read-only handle and success through the new adapter.
Native fail-before and corrected Windows execution remain UNVERIFIED.

The native CI job now includes catalog storage and workspace packages.
Existing Windows workspace exchange remains unsupported. Its failures must remain visible during qualification.
Directory-flush API success does not establish hardware power-loss recovery or support for untested filesystems.
No primary acceptance case receives credit from cross-compilation.

[Integration checks](csp2/directory-flush-checks.json) passed 543 race results across filesystem publication, runtime, application, catalog storage, workspace, and CI packages.
On Go 1.25.12, the focused baseline recovery suite passed ten race results.
Host lint, targeted Windows lint, ago, dependency direction, and workflow syntax passed.

[Final platform checks](csp2/directory-flush-platform-final.json) passed 24 filesystem publication race results on Go 1.25.12, with no skips.
Six Windows test binaries cross-compiled with that toolchain for x64 and ARM64.
They cover filesystem publication, catalog storage, and workspace code.
The final Windows code uses an empty object name. Earlier compilation also accepted the incorrect dot name, which source review corrected.

The [input manifest](csp2/directory-flush-input-manifest.json) binds the changed files and inspected Go source to this local checkpoint.
The Windows access implementation has no native execution evidence.
Directory durability and supported-filesystem qualification remain open. CSP2 remains in progress.

## Windows workspace decision and cancellation boundary

On 2026-09-06, the user accepted journaled Windows replacement for the optional YAML workspace.
D22 records that decision in the PRD. Section 9.1 of the engineering specification defines the required behavior.
The accepted exception allows the workspace path to disappear between separate rename operations.
Starmap must return a retryable conflict during replacement or recovery. It must never interpret that gap as an empty source.

The journal must precede either move and bind the operation to the expected paths and contents.
Recovery must preserve conflicting operator edits and refuse an unexpected destination.
Reader coordination must cover the complete file read, including readers that start before the journal appears.
Checks only before and after a read cannot exclude a replacement that starts and finishes between those checks.
The accepted in-memory catalog remains independent of workspace repair. Existing inference authority and admission rules remain required.

The journal and reader coordination remain implementation work. Native Windows qualification remains UNVERIFIED.
The current Windows adapter still refuses directory exchange. This decision does not claim a working Windows replacement path.

The [red regression](csp2/workspace-publication-cancellation-red.json) reproduced a separate cancellation defect.
First publication and replacement both returned successful receipts after cancellation at the final publication boundary.
The projector now checks the context immediately before it calls the publication operation.
Cancellation at that point returns no receipt and preserves existing directory identity, model bytes, and marker bytes.
For first publication, the workspace and marker remain absent. Both cases remove their staged candidates.

The [green regression](csp2/workspace-publication-cancellation-green.json) passed three race test results on Go 1.25.12, with no skips.
These are two subcases and their parent. They do not establish atomic cancellation after the last context check or native Windows recovery.
No primary acceptance case changes status. CSP2 remains in progress, and no commit or external write occurred.

[Integration checks](csp2/workspace-publication-cancellation-checks.json) passed 123 race results with no skips: 41 workspace, 60 pipeline, and 22 acquisition results.
Workspace lint and whole-module ago passed. The changed code adds no dependency or suppression.
The documentation check preserves the plan's 38 tasks, 50 primary cases, and 324 required subcases.
The whole-repository writing check retains three existing diagnostics in frozen historical evidence. The updated design documents and proof pass their targeted check.

## Coordinated workspace reads

The [reader regression](csp2/workspace-reader-red.json) failed for present and absent workspace paths while a writer held the lock.
Both calls returned normal input instead of a conflict. This permits the missing-path interval to look like ordinary source absence.

`workspace.Read` now owns complete workspace reads under the existing writer lock protocol.
It opens an existing lock file read-only and takes a shared, nonblocking lock.
An active writer produces a typed conflict before the callback runs.
Other shared readers remain permitted. Callback errors and process exit release the shared lock.

Before any writer creates the lock file, a reader creates no files.
A final check rejects the read if a first writer created its retained lock file, even if that writer already finished.
The reader also verifies workspace directory identity before it accepts the callback result.
This protocol coordinates Starmap writers. It does not exclude external editors or guarantee snapshots of their concurrent file edits.

Startup, local sources, pipeline input, release import, rollback input, and bootstrap-manifest input now use this guard.
Acquisition and rollback compute catalog and endpoint checksums inside the same guarded read.
A rejected read discards the constructed catalog, input expectation, and observation.
Existing accepted-catalog reads in memory take no new filesystem lock.

The [first focused run](csp2/workspace-reader-focused-checks.json) found one test helper that still called the old rollback-input signature.
The helper now passes its test context. Its assertions remain unchanged.

The [corrected floor run](csp2/workspace-reader-focused-final.json) passed 25 race test results on Go 1.25.12, with no skips.
These include two process coordination tests and a completed first publication during a passive read.
The command also selected the existing provider-reader test. The result count includes the helper entry point.

[Platform checks](csp2/workspace-reader-platform-checks.json) passed host lint, package layout, dependency direction, and two Windows workspace cross-builds.
The cross-builds use Go 1.25.12 and cover x64 and ARM64. They are not native execution evidence.
[Whole-module ago](csp2/workspace-reader-ago-final.json) passed without a new suppression or dependency.

The Windows journal and pending-operation read refusal remain open.
They must extend this guard before restart can distinguish an interrupted replacement from an ordinary absent workspace.
Native filesystem and Windows qualification remain UNVERIFIED. CSP2 remains in progress. No primary acceptance case changes status.

The [integration run](csp2/workspace-reader-integration.json) passed five packages but failed the pipeline's existing embedded-load error-order test.
The initial read guard moved catalog validation before that injected embedded-load failure.
The pipeline now preserves its prior validation order inside the complete read guard. The existing assertion remains unchanged.
The [focused regression](csp2/workspace-reader-validation-order.json) passed after that correction.

The [corrected pipeline and acquisition run](csp2/workspace-reader-pipeline-final.json) passed 61 and 23 race results, respectively.
A further pipeline test proves writer exclusion during the loader callback and writer access after the read ends.
The [final pipeline suite](csp2/workspace-reader-pipeline-settled.json) includes that test and passes 62 race results with no skips.
The [final floor suite](csp2/workspace-reader-floor-settled.json) passes 27 race results with no skips on Go 1.25.12.

Final package evidence totals 237 race results across the workspace, pipeline, local source, acquisition, root API, and manifest command suites.
This total selects each package's final successful suite and excludes the earlier failed pipeline suite.
The raw combined command remains failed in its evidence file. The correction preserves all existing assertions.

The [inventory checks](csp2/workspace-reader-inventory-checks.json) passed three additional application race results.
The existing workspace lock entry now describes shared reads and exclusive writes. Inventory paths and entry counts remain unchanged.

The proof still requires Windows journal recovery, native Windows reader execution, and supported-filesystem qualification.
The prepared native workflow includes the workspace reader tests. Publication and dispatch remain unauthorized.

## Windows workspace replacement journal

The Windows projector now replaces an existing YAML workspace through recorded intent, two exclusive directory moves, and verified cleanup.
Linux and macOS retain directory exchange. First publication now uses the shared exclusive directory publication adapter on each supported platform.
The inference catalog remains separate from this optional filesystem projection.

The journal binds the target, candidate, backup, physical root identities, file inventories, and expected projection receipt.
Candidate validation precedes the first move. Recovery verifies catalog and endpoint identity before it accepts the installed tree.
The old directory remains available as a backup until recovery saves the receipt.
Recovery resumes partial cleanup only when every remaining backup entry matches the old inventory.
It refuses copied candidates, unexpected workspace directories, corrupt records, and changed backup files without discarding those files.

The full snapshot bounds each tree to 10,000 entries, 256 MiB of regular file bytes, and 1 MiB of path-name bytes.
The JSON journal limit is 4 MiB. Symlinks and special files cause a conflict.
These limits bound acquisition-side inspection. Inference reads do not walk the workspace or read its journal.

External editors remain outside the advisory lock protocol. These checks do not claim a snapshot of arbitrary concurrent editor writes.

A pending journal now blocks complete workspace reads, including when the workspace path is absent.
Projection and repair resume the journal under the writer lock.
Persistent startup attempts repair when a durable current generation exists.
A constructor without that generation refuses the pending read and does not modify files implicitly.

The [two initial regressions](csp2/workspace-journal-red.json) failed.
The publication collision preserved the competing empty directory but returned a generic I/O error, rather than a typed conflict.
This fixture did not reproduce overwrite or data loss.
The pending-journal read incorrectly returned an ordinary absent workspace. Both regressions now pass.

The [first recovery build](csp2/workspace-journal-focused.json) failed because a new assertion helper used an incompatible test parameter type.
The corrected helper retains the original assertions. The [corrected phase run](csp2/workspace-journal-focused-corrected.json) passed ten test results.
The [expanded recovery run](csp2/workspace-journal-recovery-checks.json) passed 30 floor race results.
That run also exposed a complexity warning. The backup move now has its own operation function, and final lint passes.

Fault injection and separate process exits cover six phases: journal publication, backup move, candidate installation, receipt publication, partial cleanup, and completed backup removal.
Fresh repair completes each interrupted replacement. Repeated repair recognizes the completed state.
Additional checks preserve late operator edits, refuse changed directory identities, restore a missing candidate's old tree, and retain recovery state after cancellation.
These tests exercise process exit. They do not simulate sudden power loss.

The [settled integration run](csp2/workspace-journal-settled-checks.json) passed 90 workspace race results on the Go floor (1.25.12).
It passed 177 consumer race results across the pipeline, acquisition, local source, root API, and bootstrap-manifest command.
Five application inventory results also passed on the Go floor. None of these suites skipped tests.
The [journal binding tests](csp2/workspace-journal-binding-checks.json) add seven floor race results, including the parent case.
They reject unsupported versions, another target, traversing candidate or backup paths, identical directory identities, and changed inventory digests without modifying files.

The inventory now reports the fixed journal, temporary journal writes, and retained backup trees.
Its matching test covers nested backup files and a workspace name with pattern metacharacters.
Disabled workspaces have no active sibling roles. Reporting these roles creates no files.
The CLI guide explains inspection, restart recovery, conflicting files, and the constructor boundary.

The [platform checks](csp2/workspace-journal-platform-docs-checks.json) cross-compiled x64 and ARM64 Windows tests and passed Windows workspace lint.
The native editor-handle test retains a directory handle without delete sharing, then requires recovery after that handle closes.
It remains UNVERIFIED on Windows. The prepared native workflow includes the workspace suite. No publication or dispatch occurred.
The final platform build below includes the additional journal-binding test.

The first documentation check found eight new prose errors and three historical diagnostics.
The final edit corrects the new prose errors. The historical files remain unchanged.
The [checkpoint verification](csp2/workspace-journal-checkpoint.json) includes the final Windows builds and another six prose errors.
The [settled document check](csp2/workspace-journal-final-documents.json) verifies the corrections.

CSP2 remains in progress. Native Windows execution, supported-filesystem durability, orphan staging cleanup, complete access policies, and deployment procedures remain open.
No primary acceptance case receives credit from these component checks. No commit or external publication occurred.

## File policy descriptors

The [file policy regression](csp2/file-policy-red.json) failed because the report omitted structured policies.
Each managed role now declares selectors, applicability, access, retention, and removal conditions.
External destination roles use the same policy type. An unknown role produces a typed configuration error instead of an implicit policy.
Each report owns its selector slices. Caller edits cannot change later reports.

The default inventory has 28 managed entries and seven external roles.
The optional file source adds one managed entry. Six reserved roles still have no implemented writer.
The specification now includes the journal and backup entries added after the historical 26-entry inventory.

Access classes separate owner-only files, explicitly shareable embedded exports, deployment-controlled catalog data, and external systems.
The baseline exporter still creates private files and directories. Its public-read class permits an explicit operator grant without requiring one.
Retention distinguishes operator content, reproducible exports, durable state, identity, recovery records, rebuildable caches, and audit records.
These policies do not enable cleanup or certify effective access.

JSON and YAML expose complete policies. Wide output includes access and retention classes.
The command tests exercise all three formats through the application and confirm that reports create no files.
A separate check verifies that migration selectors name actual flags on the prepare command.

Inspection now reports declared access and a limited assessment for each known file role.
Owner-only POSIX files and directories report a conflict for group or other mode bits, or a different effective owner.
Other present files remain unverified. Absent and inapplicable roles receive no assessment.
The real-file tests verify both the reported conflict and unchanged bytes and permissions.
Classification tests retain uncertainty for Windows metadata, symbolic links, and missing ownership information.

The [initial checks](csp2/file-policy-initial-checks.json) passed the regression on the Go floor, ago, and scoped lint.
The [integration checks](csp2/file-policy-integration-checks.json) passed 216 race results on the Go floor (1.25.12), with no skips.
The total includes 33 product-path results, 173 application results, and ten configuration-command results.
Whole-module lint, ago, package layout, and dependency direction also passed.
Four Windows test binaries cross-compiled across the product-path and application packages on x64 and ARM64.

The [final code checks](csp2/file-policy-final-code-checks.json) cover policy serialization, caller ownership, unknown roles, output formats, and migration flag names.
The [checkpoint verification](csp2/file-policy-checkpoint.json) includes the final platform builds and two prose errors.
The [settled document check](csp2/file-policy-final-documents.json) verifies the corrected prose.
Native Windows execution remains unverified.

This review found a concrete enforcement gap for the next CSP2 step.
The runtime layer writer creates retained evidence with shared catalog modes, subject to the process umask.
The GitHub discovery writer also creates its child directory with shared mode bits.
The enclosing private runtime directory is a separate protection boundary. These child writers still need private creation and existing-file validation.
The policy report makes those requirements explicit without claiming that the writers enforce them.

CSP2 remains in progress. Service procedures, complete access enforcement, native qualification, and release acceptance remain open.
No primary acceptance case receives credit. No commit or external publication occurred.

## Private retained files

The first [retained-file regression run](csp2/retained-private-files-red.json) reproduced three GitHub discovery failures.
The writer created nonprivate files and accepted exposed or linked directories.
The runtime tests initially failed to compile because the new fixture passed a pointer to a value parameter.
The corrected fixture retained its assertions. The [runtime regression run](csp2/retained-private-files-runtime-red.json) then reproduced the same three failures.

Retained runtime layers and GitHub discovery now share the private-file implementation.
POSIX creation uses `0700` directories and `0600` files. Windows creation applies the existing owner and protected DACL contract.
The existing native ACL code moved into this package. Runtime identity checks retain their public entry point and delegate to it.

Native parser tests moved with their owning code. This change adds no dependency or ACL suppression.

Each directory binding compares filesystem identity before use. Exposed or linked directories fail without permission repair.
Records must be private regular files with the expected identity throughout a bounded read.
The layer limit remains 64 MiB. Discovery reads and writes use the 64 KiB state limit.

A missing bound directory reports a conflict. Only an absent record inside a valid directory can represent cold state.
Runtime startup propagates retained-layer access errors instead of silently replacing them with the embedded baseline.

Writes create a unique private temporary file, flush its contents, publish complete bytes, and flush the parent directory.
The temporary file descriptor remains open through publication and cleanup.
Cleanup requires the created file identity. The publication fixtures preserve competing destination and staging files.

Legacy fixed `.tmp` files remain untouched. New evidence staging names use `.layer-` under the layer or provider directory.
The file manifest reports those names. This implementation adds no automatic orphan cleanup.

Migration catalog validation uses passive directory bindings, including when the layer or provider directory is absent.
It creates no missing paths while validating a staged or published migration.
The retained-file store holds no directory handle between operations. Runtime ownership still serializes the connected runtime's work.

The [focused checks](csp2/retained-private-files-focused-checks.json) passed 39 race results on the Go floor (1.25.12).
The [first integration run](csp2/retained-private-files-integration-checks.json) passed 503 race results across runtime, shared files, GitHub source, application, and workflow checks.
The [publication checks](csp2/retained-private-files-publication-checks.json) added lost-directory and competing-file coverage.
The [expanded integration run](csp2/retained-private-files-final-checks.json) passed 507 race results, including the application and migration suites.

The [descriptor lifetime checks](csp2/retained-private-files-descriptor-checks.json) passed 307 race results with the temporary descriptor open through cleanup.
All commands, exit codes, and test events remain in their linked records.

The prepared native workflow now includes private-file and GitHub source tests.
Eight Windows test binaries cross-compiled on x64 and ARM64. These cover the shared package, runtime, GitHub source, and application.
The final descriptor change has separate shared-package builds for both Windows architectures.
No native Windows execution occurred. File and directory flush calls do not alone qualify power-loss durability.

The CLI guide explains old-mode upgrade refusals, passive inspection, consistent backup, explicit permission correction, and restart.
Newly rejected paths retain their bytes and permissions. Normal publication creates a new private record.
Configuration access checks, ancestor policies, orphan cleanup, service procedures, and native qualification remain open in CSP2.
No primary acceptance case receives credit. No commit or external publication occurred.

The [initial document checks](csp2/retained-private-files-documents.json) found five prose errors.
The corrected text and preserved historical diagnostics appear in [final document verification](csp2/retained-private-files-final-documents.json).

## Private configuration inputs

The [red regressions](csp2/config-private-access-red.json) reproduced three access failures before the implementation changed.
The YAML and dotenv readers accepted exposed credentials. A selected symlink also bypassed private-target checks.
The tests verify refusal without credential disclosure or permission repair. The dotenv test also checks environment preservation.

Configuration and dotenv reads now use the shared private-record reader under an open parent handle.
The caller retains directory-policy ownership. File checks require regular files, private ownership, supported ACLs, stable observed identity, and bounded bytes.
Each input permits at most 1 MiB. Private read-only files remain valid.

Explicit symlink selection remains supported. The reader resolves the selected target before its private-file checks.
An existing selection that loses its target returns a conflict. A genuinely absent default primary file remains optional.
All dotenv inputs pass access checks and parsing before environment mutation. Migration digest verification uses the same reader.

The [initial checks](csp2/config-private-access-initial-checks.json) found an unused test import after fixture permission changes.
The correction removed that import without changing assertions. Existing successful configuration fixtures now use private mode bits.
The malformed-file fixture also uses private permissions so it still proves parser refusal.

The [integration checks](csp2/config-private-access-integration-checks.json) passed 485 race results on the Go floor (1.25.12).
These comprise 27 shared-file, 178 application, 31 GitHub source, and 249 runtime results. No test skipped.
The record also contains passing ago, lint, package-layout, and dependency-direction checks.

The console summary script failed after saving the complete record. It treated a non-event JSON value as a test event.
A separate read verified every saved command exit code and test count.

The [native adapter checks](csp2/config-private-access-native-checks.json) passed five focused race results.
These include public macOS ACL refusal and explicit correction.

Four Windows test binaries cross-compiled for both architectures (x64 and ARM64).
They cover the shared package and application.
The Windows application fixture tests a public DACL grant and explicit correction. Native Windows execution remains unverified.

The README, CLI guide, settings reference, specification, and findings now state the private-file requirement.
The CLI guide explains that a refused primary file also prevents configuration-dependent diagnostics.
Operating-system file inspection and command help remain available. The loader does not modify existing permissions or file bytes.

The [initial document and reference checks](csp2/config-private-access-document-checks.json) found four prose errors and a missing glossary term.
The [final code checks](csp2/config-private-access-final-checks.json) passed and recorded one remaining prose error.
The corrected [document recheck](csp2/config-private-access-document-recheck.json) retains the final prose results.
The final diagnostic identifies the selected configuration path and retains its typed access error without file values.

CSP2 still needs ancestor policies, orphan cleanup, service procedures, and native qualification.
No primary acceptance case receives credit. No commit or external publication occurred.

## Explicit container roots

This step followed the configured-directory trace into deployment recipes. It found a startup defect before ancestor-policy implementation.
The [root contract regressions](csp2/container-roots-red.json) failed for both Compose and Kubernetes because neither selected its application roots explicitly.
The [real startup regression](csp2/container-roots-startup-red.json) failed inside an isolated Linux ARM64 container.
Baseline creation reached `/home/nonroot/.local/share/starmap/catalog/baseline`, outside the old writable `.starmap` mount.

Compose now selects `STARMAP_HOME=/home/nonroot/starmap` inside its named volume.
It publishes only on loopback and no longer reports CLI version execution as server health.
The guide uses an explicitly owned temporary mount for ephemeral evaluation.
It states that a new root selector does not migrate existing volume contents.
The environment template removes tilde-based legacy path examples.

The [rollout regression](csp2/container-rollout-red.json) also exposed the default rolling strategy for a single filesystem writer.
The Kubernetes example now uses `Recreate` for upgrades. Manual deletion can still overlap Pod termination and replacement, so runtime locking remains required.
The example requires provisioned PVC ownership and avoids automatic recursive group-permission changes to private retained records.
Its topology remains unqualified for a production cluster or replicated Starport deployment.

The [initial checks](csp2/container-roots-initial-checks.json) passed 36 race results on the Go floor (1.25.12).
These include three deployment-document tests and 33 product-path results, with no skips.
Ago and whole-repository Go lint passed. The initial prose check found one long paragraph, which the final edit splits.

The [first container smoke test](csp2/container-roots-smoke.json) passed all four functional checks and removed its test volume.
The [final smoke test](csp2/container-roots-final-smoke.json) adds assertions for each resolved root and the selected read-only configuration file.
Three server starts reach HTTP liveness and readiness with no external network and acquisition disabled.
They cover durable cold start, durable replacement with read-only configuration, and ephemeral cold start.
Baseline and runtime identity hashes remain equal across replacement. No credential values enter the proof.

The [engine record](csp2/container-roots-engine.json) identifies Linux ARM64 under Docker Desktop's Linux VM.
The test uses the source binary and cached images, including the release configuration's pinned base image.
The [replay script](csp2/container-roots-smoke.py) records commands, responses, binary hash, and cleanup outcomes.
Each run creates and removes only its own UUID-named volume and returned container IDs.
This is not evidence for a released image, native Windows, a Kubernetes storage driver, or power-loss durability.

The [initial document checks](csp2/container-roots-document-checks.json) found a missing glossary definition for `STARMAP_HOME`.
The [final document checks](csp2/container-roots-final-documents.json) retain the corrected verification results.
CSP2 remains in progress. Ancestor policies, orphan cleanup, native service procedures, and remaining platform qualification stay open.
No primary acceptance case receives credit. No commit, registry publication, or cluster deployment occurred.

## Native Linux ARM64 package execution

The [native run](csp2/native-linux-arm64/summary.json) passed all ten packages in the prepared native CI roster.
It recorded 697 test and subtest pass results, with no failures or skips.
The [independent audit](csp2/native-linux-arm64-audit.json) recounts raw test events and checks command exits, terminal container states, and cleanup.
All ten containers exited successfully. The runner removed its temporary containers and named volumes.

| Package | Pass results |
| --- | ---: |
| `runtime` | 239 |
| `internal/bootstrap` | 47 |
| `internal/filepublish` | 24 |
| `internal/privatefiles` | 9 |
| `internal/runtimeacl/windows` | 15 |
| `internal/sources/github` | 31 |
| `internal/cli/app` | 177 |
| `pkg/productpaths` | 33 |
| `pkg/catalogs/storage` | 25 |
| `internal/catalog/workspace` | 97 |

The [replay script](csp2/native-linux-suites.py) builds each package with the Go floor (1.25.12).
It executes the Linux ARM64 binary in the cached release base image under Docker Desktop's Linux kernel.
Each container uses an unprivileged account, a read-only root filesystem, and no external network.
Named volumes hold temporary test files. The source bind mount provides read-only fixtures.

`TestBaselineExportRecoversAfterProcessInterruption` passes four exit checkpoints.
`TestJournalReplacementRecoversAfterProcessExit` passes six exit checkpoints and verifies pending-read refusal before recovery.
The runtime tests also execute separate processes for locks, migration journals, staged copies, publication, and completion.
These tests exercise process interruption. They do not simulate kernel failure or hardware power loss.

The Windows ACL package tests portable policy logic in this run. It does not call Windows APIs on Linux.
The workspace tests force the shared journal implementation on Linux. Windows directory operations and editor handles still require native Windows execution.
This run disables CGO and has no race instrumentation. Earlier macOS race results remain separate evidence.

The records retain build commands, binary hashes, test events, and actual container start and finish times.
The audit hashes those records after execution. It does not establish a pre-build source manifest.
No primary acceptance case receives credit. Native service procedures, remaining platforms, and released-pair qualification stay open.

## Baseline staging cleanup

The [cleanup regression](csp2/baseline-stage-cleanup-red.json) recorded 14 failed test and subtest results and four passing results.
The old deferred recursive removal deleted operator additions and replacements at the staging path.
It also deleted a reused staging name after publication. Changed staging could reach publication before verification reported an error.

The exporter now retains its directory identity and open file handles through publication or cleanup.
It verifies identities, file metadata, exact bytes, and the complete two-file inventory before publication.
Directory enumeration reads at most one entry beyond the expected count. Each file read permits at most one byte beyond the created payload size.

Cleanup first verifies the whole stage, then checks each owned file before removal.
It uses nonrecursive removal and preserves unknown entries. Successful publication disables cleanup of the former staging name.
Typed cleanup conflicts retain the original operation error, including cancellation. The exporter closes its retained handles on return.

The [focused checks](csp2/baseline-stage-cleanup-focused.json) passed 18 race results on the Go floor (1.25.12).
The [first integration checks](csp2/baseline-stage-cleanup-integration.json) passed 89 race results across bootstrap and directory publication.
Ago and whole-repository Go lint passed. Three [application checks](csp2/baseline-stage-cleanup-application.json) passed for cold start, accepted-head preservation, and required-write failure.
The [first native Linux run](csp2/baseline-stage-native-linux-arm64/summary.json) passed 65 results without skips.
Two [Windows test binaries](csp2/baseline-stage-cleanup-windows-builds.json) cross-compiled (x64 and ARM64).

Review found a related cancellation gap after the last payload checkpoint.
The [cancellation regression](csp2/baseline-stage-cancellation-red.json) failed because publication still occurred after cancellation.
The exporter now checks cancellation immediately before the rename call.

The [final checks](csp2/baseline-stage-cleanup-final-checks.json) passed 269 race results across bootstrap, directory publication, and the application.
They also passed ago, whole-repository Go lint, and both Windows compilation checks.
The [final Linux run](csp2/baseline-stage-final-native-linux-arm64/summary.json) passed 66 bootstrap results, including the cancellation regression.
Both final test runs have no failures or skips. Native Windows execution remains unverified.

The change does not collect orphan directories from earlier processes. Those directories still lack ownership evidence that permits removal.
Arbitrary external edits do not participate in an atomic multi-file transaction with these checks.
Native Windows execution, hardware power-loss recovery, ancestor policies, and service procedures remain open.
No primary acceptance case receives credit. No commit or external publication occurred.

## Constructor passivity and repair ownership

The [constructor regression](csp2/constructor-passivity-red.json) failed both workspace cases and their parent.
`newClient` previously repaired the workspace after loading a durable current generation.

This violated D6 even though default construction remained passive.

`New` and `NewContext` now read the durable catalog without workspace writes.
`Client.RepairWorkspace(ctx)` owns explicit repair and serializes it with catalog publication.
The result identifies the workspace, durable generation, completed changes, and preserved operator edits.
Connected runtime startup invokes this operation. Repair does not commit another generation or change the catalog sequence.

The [first focused check](csp2/constructor-passivity-focused.json) passed four race results.
The [first integration run](csp2/constructor-passivity-integration.json) passed 513 results and failed two.
One migration fixture still expected passive application access to restore its workspace.
The corrected fixture now verifies passive preservation before runtime startup restores the workspace.
Its first [recheck](csp2/constructor-passivity-migration-recheck.json) found a missing test import.
The [corrected recheck](csp2/constructor-passivity-migration-corrected.json) passed the migration test.

The other integration failure exceeded the existing one-minute optional repair budget with the full embedded workspace under macOS race instrumentation.
The [initial Linux run](csp2/constructor-passivity-native-linux-arm64/summary.json) completed that full-workspace test, but failed the old migration fixture.
The [original full-workspace fixture](csp2/constructor-passivity-full-workspace-fixture.json) and both raw runs remain available.
The lifecycle test now uses a small durable generation produced through the real publication API.
Full-catalog repair performance remains unqualified. CSP22 must verify the complete release workload, including optional workspace recovery.

The registry now binds `A02.constructor_filesystem_silence` to fresh construction, durable-current workspace preservation, and passive embedded access tests.
The initial task check left network silence and worker silence UNVERIFIED. No primary acceptance case receives credit from this component registration alone.

### ER02: Staged-write collection belongs to CSP5

The CSP5 task already assigns staged-write recovery and generation retention to its third step.
Automatic abandoned-stage collection requires that lifecycle policy, ownership evidence, and active-writer exclusion.
CSP5 owns baseline, migration, workspace, retained-evidence, and discovery stages. CSP8 owns temporary Starport scratch cleanup under A43.
CSP2 retains path, access, migration, and native-startup requirements. This routing preserves every primary case and required subcase.

The [final constructor checks](csp2/constructor-passivity-final-checks.json) passed 516 macOS race results without failures or skips on Go (1.25.12).
The same run passed ago, golangci-lint, and all 30 verifier tests. Four root and runtime Windows test binaries cross-compiled for Windows (x64 and ARM64).
The [final native Linux run](csp2/constructor-passivity-final-native-linux-arm64/summary.json) passed 504 results without failures or skips across three packages.
Its root, runtime, and application packages passed 87, 240, and 177 results respectively.

The [constructor task gate](csp2/constructor-passivity-task-checks.json) passed two of 22 selected component checks. The other 20 remained UNVERIFIED.
The gate failed, and all 50 primary cases remained UNVERIFIED. These counts precede the worker check registration below.

## Constructor worker observation and network boundary

`TestReadOnlyConstructorsStartNoWorkers` now checks both constructors and an explicitly supplied durable memory store.
The test observes goroutine identities after construction and after 24 hours of virtual time, before caller cancellation.
The catalog state must remain unchanged. The observer control must detect both a blocked worker and a scheduled worker.
This checks persistent workers and delayed activity. It does not claim that stack snapshots detect every transient goroutine.

The [focused worker check](csp2/constructor-workers-focused.json) passed five race results without failures or skips on Go (1.25.12).
The initial registry bound `A02.no_workers` to this behavior, its observer control, and the existing root cadence boundary check.
The prepared native workflow now includes the root package. Its publication and native Windows execution remain UNVERIFIED.

A separate review found a network boundary conflict. D6 prohibits constructor network requests, but construction calls `Current` on caller-supplied storage.
The supplied store can use the existing S3 adapter. Its `Current` operation issues a network read.
The owner question asks whether construction must defer remote reads to an explicit operation or may read explicitly supplied storage.
The answer remains pending. `A02.constructor_network_silence` stays UNVERIFIED until the implementation and accepted requirement agree.

The [network boundary probe](csp2/constructor-network-boundary-probe.json) used the real S3 adapter with anonymous credentials and a local HTTP server.
Construction succeeded and returned the embedded catalog after one request received a missing-object response.
The probe made no external service request. Its successful exit confirms the contract conflict, not network-silence acceptance.

The [native Linux worker run](csp2/constructor-workers-native-linux-arm64/summary.json) passed all 92 root package results without failures or skips.
The [initial worker task gate](csp2/constructor-workers-task-checks.json) passed three of 22 selected component checks. The other 19 remained UNVERIFIED.
That gate failed, and all 50 primary cases remained UNVERIFIED. No commit or external publication occurred.

The [remote worker probe](csp2/constructor-remote-workers-probe.json) also found two HTTP transport goroutines after the constructor returned.
A third new goroutine belonged to the local test server. The probe preserved each stack and used no external service.

The default and memory-store checks do not prove the complete worker contract for remote stores.
Review removed the worker registration pending the storage boundary decision. Both network silence and worker silence remain UNVERIFIED.
The tests and initial task report retain that narrower result.

The [integration checks](csp2/constructor-workers-integration.json) passed 118 macOS race results without failures or skips on Go (1.25.12).
The root package passed 92 results, and the workflow package passed 26. Both Windows architecture test binaries cross-compiled.
Ago, golangci-lint, all 30 verifier tests, package layout, and all eight dependency-direction conditions passed.

The [final task gate](csp2/constructor-boundary-final-task.json) passes two of 22 selected component checks. The other 20 remain UNVERIFIED.
The overall gate fails. All 50 primary cases remain UNVERIFIED, and CSP2 remains in progress.

## POSIX ancestor access

The [initial regression](csp2/ancestor-access-red.json) failed five results: four private-file operations and their parent test.
Create, bind, read, and write all accepted a writable ancestor. Creation made new directories, and the write replaced retained bytes.

Linux and macOS now check ancestor ownership, directory-entry protection, and selected symlink routes.
Root and the effective user remain trusted. Shared read and search access remain valid.
Trusted sticky directories permit shared creation while child ownership and identity checks protect the selected path.

macOS also checks native ACL grants. Mutation grants require a trusted principal, including inherited and inheritance-only grants.
Read-only grants and deny entries remain valid. Other platforms retain their existing checks and require separate ancestor qualification.

Private directory creation now uses parent handles. Each child must pass identity, ownership, and access checks before descent.
The replacement test preserves a renamed child and rejects its replacement symlink without writing into the external target.
Private record publication repeats ancestor checks before switching its destination. Configuration reads validate their selected route before parsing.
Runtime checks precede baseline export, whose directory creation now uses the same guarded primitive.

The [first focused run](csp2/ancestor-access-focused.json) passed 32 private-file race results on Go (1.25.12).
The [first edge run](csp2/ancestor-access-edge-checks.json) passed 30 results across private files and application startup.
Those results cover trusted sticky ancestors, symlinked directories, replacement refusal, publication checks, native macOS ACLs, and recovery after explicit permission correction.

A subsequent [configuration symlink regression](csp2/ancestor-config-symlink-red.json) failed once.
The selected file linked through an unsafe intermediate directory to a valid private target. Final-target validation alone missed that route.
The guard now checks selected symlink ownership and intermediate target ancestors.
The [corrected edge run](csp2/ancestor-access-corrected-edges.json) passed 32 results without failures or skips on Go (1.25.12).

The first linter check found an unchecked effective-UID conversion in the macOS ACL adapter.
An explicit native-range check now precedes that conversion. The ACL rights mapping follows the installed macOS SDK `sys/kauth.h` definitions.
Linux uses the documented [ACL mask](https://man7.org/linux/man-pages/man5/acl.5.html) and [sticky-directory semantics](https://man7.org/linux/man-pages/man7/inode.7.html).

No primary acceptance case receives credit. Windows ancestors, other file roles, service-owned file exceptions, hostile mount replacement, and complete filesystem qualification remain open.
The constructor storage decision remains pending. No commit or external publication occurred.

The first [integration run](csp2/ancestor-access-integration.json) passed 764 race results across seven packages without failures or skips.
The first [native Linux run](csp2/ancestor-access-native-linux-arm64/summary.json) passed 506 results across four packages without failures or skips.
Those runs preceded the final traversal correction below.

A final [traversal regression](csp2/ancestor-traversal-red.json) showed that path normalization could erase an unsafe directory before `..`.
The guard now walks each path component and expands symlink targets before processing parent traversal.
It uses a bounded symlink count and validates directory identities along the selected route.
Trusted relative traversal remains supported. Cyclic routes cause refusal.

The next file-role check targets the filesystem catalog store. At that checkpoint, its commit path still used recursive creation and public catalog-tree modes.
That checkpoint did not qualify the store's complete access policy. CSP2 retains that work while the constructor boundary question remains pending.


The final [ancestor integration run](csp2/ancestor-access-final-integration.json) passed 766 race results without failures or skips.
The corresponding [native Linux run](csp2/ancestor-access-final-native-linux-arm64/summary.json) passed 508 results.
Both preceded the final relative-path change in the POSIX helper.
The helper now preserves Windows drive-relative and rooted configuration path behavior.

The [current relative-path check](csp2/ancestor-relative-config-preservation.json) passed 230 race results and both Windows application cross-builds.
Ago and golangci-lint passed. The [task gate](csp2/ancestor-access-task.json) still passes two of 22 selected components.
All 50 primary cases remain UNVERIFIED.

## Private filesystem catalog store

The [store regression](csp2/filesystem-private-access-red.json) failed all 11 results.
New state used public catalog-tree modes. Reads and commits accepted unsafe modes on roots, generations, records, locks, and ancestors.
A publication hook also made the root unsafe before the current pointer changed. The old implementation still published the replacement.

The filesystem adapter now uses private creation and native access checks from `internal/privatefiles`.
Public YAML export modes remain unchanged. Existing unsafe stores cause refusal without automatic ownership or permission changes.
Operators must correct their selected store's access before reads, commits, or legacy-store migration can proceed.
The constructor remains passive, including when the selected root does not exist.

Private creation covers the store root, generations, candidate directories, JSON records, current pointer, and commit lock.
The lock adapter opens an explicitly created private file and verifies its identity after locking.
The instance mutex serializes access to that lock object. Separate instances retain the filesystem lock and compare-and-swap contract.
Reads validate private records through directory handles. Their byte limit follows the inspected file size, without claiming a new absolute resource limit.

Generation publication verifies the candidate directory, record bytes, and absence of extra entries before a directory rename that refuses an existing destination.
Current publication uses the original directory binding and checks cancellation and access again.
A changed root cannot receive the new current pointer. These checks occur during store operations, outside the inference catalog lookup path.

Stage cleanup removes only unchanged records that this operation created. It preserves modified records, unknown files, and replacement directories.
CSP5 still owns the procedure to collect abandoned stages. This change does not establish power-loss durability or hostile-filesystem qualification.

The [first implementation run](csp2/filesystem-private-access-initial.json) passed 26 results and failed ten.
Existing successful-store tests supplied public temporary directories. Their fixtures now create private store roots while unsafe-access tests retain explicit public modes.

The [corrected fixture run](csp2/filesystem-private-access-fixtures.json) passed 36 race results.
An intermediate [integration attempt](csp2/filesystem-private-integration-initial.json) stopped at a compile error in the new lock helper.
A mechanical edit inserted stage bookkeeping there. The corrected helper removes that unrelated bookkeeping.

The [publication checks](csp2/filesystem-private-publication-corrected.json) passed 44 race results, ago, and golangci-lint.
The [final focused checks](csp2/filesystem-private-final-checks.json) passed 47 race results, including three native macOS ACL results.
They also passed ago, golangci-lint, package layout, eight dependency-direction conditions, and both Windows storage cross-builds.
The [native Linux checks](csp2/filesystem-private-native-linux-arm64/summary.json) passed 246 results across storage, private files, and application startup without skips.
The verification helper removed all temporary containers and volumes. Native Windows execution remains UNVERIFIED.


The [broader integration run](csp2/filesystem-private-integration.json) passed 741 race results and failed five across eight packages.
The failures identified public store directories in runtime and server fixtures.
The fixtures now select missing private store subdirectories. Their behavior and expected generation identities remain unchanged.
A [focused runtime reproduction](csp2/filesystem-private-runtime-fixtures-red.json) retains both runtime failures before that correction.

A [publication cancellation regression](csp2/filesystem-private-cancellation-red.json) failed three results: absent destination, existing destination, and their parent test.
The private-file publisher now accepts an explicit context and checks cancellation immediately before the destination operation.
Existing callers retain their previous behavior through the context-free wrapper. The filesystem current pointer uses the context-aware method.
The [corrected checks](csp2/filesystem-private-cancellation-checks.json) passed 97 race results, ago, and golangci-lint.
The [native Linux checks](csp2/filesystem-private-cancellation-native-linux-arm64/summary.json) passed 68 results without failures or skips.

One existing durability contract still needs correction under CSP5.
A directory flush can fail after a successful rename, when the new pointer is already visible.
That outcome is ambiguous, so the store contract cannot promise that every returned error preserves the old pointer.
CSP5 must define the error and retry contract and verify recovery at that boundary.
This turn verifies refusal before publication and preserves that separate durability obligation.


The [corrected caller run](csp2/filesystem-private-caller-corrections.json) passed 291 race results across runtime and both server packages.
The [native Linux caller run](csp2/filesystem-private-callers-native-linux-arm64/summary.json) passed 281 results across the same packages.
Neither run reported failures or skips. Temporary container and volume cleanup passed for every native package.
These runs follow the cancellation correction and preserve all earlier failed evidence.


The [task gate and Windows builds](csp2/filesystem-private-task-and-builds.json) retain six successful cross-builds.
The task gate passes two selected components and leaves 20 UNVERIFIED. All 50 primary cases remain UNVERIFIED.
No registry credit changed. The owner decisions for constructor network access and service-owned configuration remain pending.

A final [record cleanup regression](csp2/filesystem-private-record-cleanup-red.json) failed once.
The shared private-file writer removed a modified stage because its filesystem identity still matched.
The writer now compares metadata and the content digest before cleanup. It also verifies staged bytes before destination publication.
This protects the current pointer and existing retained-evidence and discovery callers.

The [initial cleanup checks](csp2/filesystem-private-record-cleanup-checks.json) passed 129 race results and four Windows cross-builds.
The linter found excessive complexity in the expanded write method. Cleanup now resides in a separate record-owned function without a policy suppression.
The [native Linux cleanup checks](csp2/filesystem-private-record-cleanup-native-linux-arm64/summary.json) passed 100 results without failures or skips before that extraction.

The manual source review verified that `Client.Catalog` returns the active in-memory snapshot without filesystem access.
The private-store changes do not move store access into that method. No request-latency qualification follows from these storage correctness tests.


The [settled checks](csp2/filesystem-private-settled-checks.json) passed 129 package race results and seven retained-layer and restart race results on Go (1.25.12).
Ago and golangci-lint passed after the cleanup extraction. Both Windows private-file test binaries cross-compiled.
The [settled native check](csp2/filesystem-private-settled-native-linux-arm64/summary.json) passed all 25 private-file results without skips and removed its temporary resources.
CSP2 remains in progress. The next implementation check concerns Windows ancestors and native service procedures.


## Windows ancestor access

The Windows adapter previously returned success from `ValidatePOSIXAncestors` without checking any parent.
The shared boundary now uses the name `ValidateAncestors` and selects a platform implementation.
POSIX callers retain their previous policy. Windows callers now check native ownership, grants, identities, and selected link routes.

The [portable policy regression](csp2/windows-ancestor-policy-red.json) recorded 12 passed results and 18 failed results.
Its initial policy returned success, matching the prior Windows ancestor behavior. This is portable policy evidence, not a native filesystem failure capture.
The corrected policy validates owners, rejects null or absent DACLs, and restricts effective mutation grants.
The leaf policy still rejects non-owner grants, including inherited and inheritance-only entries.

The ancestor policy trusts the process account, SYSTEM, Administrators, and TrustedInstaller.
It permits shared read and traversal grants. It also permits subdirectory creation while requiring trusted child ownership and protected private creation.
Deletion, file creation, attribute writes, security writes, generic write, and unknown rights require a trusted principal.
Inheritance-only grants do not apply to the current ancestor. Deny entries do not override the conservative grant check.

Microsoft documents separate [file and directory rights](https://learn.microsoft.com/en-us/windows/win32/wmisdk/file-and-directory-access-rights-constants).
Its [reparse operation contract](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-fsa/4aeefef8-92c3-4abc-af7a-a610caf8a165) accepts write-data or write-attribute access for a reparse change.
The [ACE inheritance rules](https://learn.microsoft.com/en-us/windows/win32/secauthz/ace-inheritance-rules) distinguish effective entries from inheritance-only entries.
These contracts support separate ancestor and private-leaf policies. Native tests must still verify their implementation.

The Windows walker preserves raw path components until it checks their ancestors. Parent traversal cannot erase an unsafe component first.
Native inspection opens the selected entry without following a reparse point and compares its identity against pathname metadata.
Symlinks receive their own ACL check before the walker checks their targets. Unsupported reparse points cause refusal.
Private creation repeats ancestor and bound-parent checks around child creation.

Path handling covers ordinary and extended drive and UNC forms, plus volume GUID filesystem paths.
The namespace conversion preserves traversal components. Physical device and GLOBALROOT paths remain outside this private-filesystem contract.
The Go (1.25.12) `os.normaliseLinkPath` implementation confirms that volume mount targets can use the extended volume GUID form.
The guard therefore accepts that form for validation instead of rejecting all extended prefixes.
No live UNC share, mounted Windows volume, or Windows path parser executed on this host.

The [initial checks](csp2/windows-ancestor-initial-checks.json) passed the portable policy suite and one Windows private-file cross-build.
The [policy and build checks](csp2/windows-ancestor-policy-and-builds.json) passed 96 macOS race results, ago, golangci-lint, and six Windows cross-builds.
The [integration run](csp2/windows-ancestor-integration.json) passed 326 macOS race results and Windows-targeted static analysis.
Two native-test builds failed because x/sys uses `FILE_APPEND_DATA` for the directory-create bit instead of exporting `FILE_ADD_SUBDIRECTORY`.
The native comparison now uses that documented alias. Production rights did not change because of that test correction.

The [native test build attempt](csp2/windows-ancestor-native-test-builds.json) preserves both failed constant references.
The [corrected namespace builds](csp2/windows-ancestor-namespace-builds.json) compiled both native test packages for both Windows architectures.
The [native Linux run](csp2/windows-ancestor-native-linux-arm64/summary.json) passed 70 results without failures or skips and removed all temporary resources.
Linux execution covers the portable policy and the unchanged POSIX behavior. It does not qualify Windows kernel behavior.

The prepared Windows tests cover canonical Starmap and Starport user roots without filesystem writes.
They also cover unsafe ACL refusal, retained bytes, missing paths, explicit recovery, shared-read parents, subdirectory grants, and inheritance-only grants.
Additional cases cover symlink targets, parent traversal, namespace forms, decoded ACE flags, and native constants.
Cross-compilation supplies no native Windows execution credit. The workflow already includes both affected packages.

CSP2 remains in progress. Native Windows execution, service procedures, and the pending owner decisions remain open.
The next independent work concerns native service procedures and the remaining file-role inventory.
No commit, publication, CI dispatch, or release occurred.


The [final checks](csp2/windows-ancestor-final-checks.json) passed 96 macOS race results, Windows-targeted static analysis, ago, package layout, and all eight dependency-direction conditions.
All eight Windows test binaries cross-compiled after the native-constant correction and namespace changes.
The native Windows results remain UNVERIFIED. The source and test changes add no acceptance-registry credit.

The [CSP2 task check](csp2/windows-ancestor-task.json) completed with two passing component checks and 20 unverified selected subcases.
All 50 primary cases remain UNVERIFIED. The task gate returned exit code 1.

## Catalog store inspection policy

The [regression](csp2/store-policy-red.json) failed all three path cases and their parent test before the descriptor correction.
The file inventory declared `deployment-controlled` access even though the filesystem adapter now requires private entries.
Application inspection therefore omitted mode conflicts for the root, current pointer, and generation payload.

The descriptor now declares `owner-only` access. The existing assessment reports conflicting POSIX mode bits for matching store entries.
The regression verifies retained bytes and permissions and refuses accidental catalog, runtime, or credential initialization.
The cross-platform manifest assertion also requires the private store policy. Windows native observations still remain unverified.

The [focused checks](csp2/store-policy-checks.json) passed 12 macOS race results on Go 1.25.12.
Ago and scoped golangci-lint passed. No acceptance-registry credit changed.

The [native Linux run](csp2/store-policy-native-linux-arm64/summary.json) passed 185 application results without failures or skips.
The runner removed its temporary container and volume. This run used Linux ARM64 with CGO disabled and no race instrumentation.
The [Windows builds](csp2/store-policy-windows-builds.json) compile the cross-platform policy assertion for both supported architectures.
Compilation supplies no native Windows execution credit.

The remaining inspection gap is native Windows ownership and DACL reporting.
Current observations retain `native-permissions-unverified` instead of claiming that the implemented access guard also supplies diagnostic evidence.
Native service procedures and the two pending owner decisions remain open. CSP2 remains in progress.

The first targeted document check found one seven-sentence paragraph in the engineering specification.
The correction separates inspection behavior from constructor behavior without changing either contract.

## Windows file inspection

The [initial regression](csp2/windows-inspection-red.json) failed the private-conflict case and its parent test.
The other three uncertainty cases passed. No native Windows execution occurred.

Inspection now reports owner and process SIDs, DACL state, entry count, and private-policy status.
The native reader requests metadata and security-read rights, skips selected symlink targets, and checks file identity around the security query.
The shared native descriptor decoder and private policy belong to `internal/runtimeacl/windows`.
The enforcement wrapper preserves its previous absent-DACL error identity. Windows privileged host grants remain unchanged.

Microsoft documents security reads through [GetSecurityInfo](https://learn.microsoft.com/en-us/windows/win32/api/aclapi/nf-aclapi-getsecurityinfo).
The [CreateFileW documentation](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew) defines metadata rights, sharing, and reparse-point handling.
These APIs do not provide a stable effective-access snapshot. Native behavior still requires execution evidence.

The [initial checks](csp2/windows-inspection-initial-checks.json) passed 102 race results and six Windows builds.
The [broader checks](csp2/windows-inspection-final-checks.json) passed 153 package race results and 11 application results.
Whole-module lint found a portable projection used only by the Windows adapter.
The correction moved descriptor assessment to its DACL policy owner and retained portable tests.
The first prose check also found one seven-sentence paragraph, which the correction separates without changing the contract.

The [settled checks](csp2/windows-inspection-settled-checks.json) passed 154 macOS race results, ago, whole-module lint, and Windows-targeted lint.
All eight Windows binaries cross-compiled. No native Windows test receives execution credit.
The prepared tests cover metadata reads when file contents are unreadable, unchanged ACLs, retained bytes, changed identities, and symlink refusal.
They also check absent, null, empty, and populated native DACLs and retained inheritance flags.

The [first native Linux run](csp2/windows-inspection-native-linux-arm64/summary.json) passed 91 results before the assessment moved to its policy owner.
That run supplies no evidence for Windows kernel behavior.

## Referenced access-policy review

The [conversation review](csp2/access-policy-conversation-review.md) compares each recommendation with both worktrees.
Editable YAML remains separate from the private catalog store. The current YAML-only provider test covers validated acquisition through a local HTTP adapter.

File-role declarations still require one semantic owner across enforcement and diagnostics. POSIX assessment also repeats policy checks.
Starport still pins Starmap v0.16.5 and does not consume the new file inventory or inspection API.
These gaps belong to CSP2, CSP8, and CSP12. No new primary case or reduced acceptance scope results from this review.

The [settled native Linux checks](csp2/windows-inspection-settled-native-linux-arm64/summary.json) passed 92 results without failures or skips.
The runner removed both temporary containers and volumes. Native Windows execution remains UNVERIFIED.
The [conversation checks](csp2/access-policy-conversation-checks.json) also passed one YAML-only provider result and four workspace results with the macOS race detector.

The PRD, engineering specification, and plan now require one semantic file-role policy across runtime enforcement, diagnostics, and documentation.
CSP2 owns the Starmap consolidation. CSP8 owns Starport adoption. Existing primary cases and subcase counts remain unchanged.

The [workspace permission probe](csp2/workspace-permission-review-red.json) failed with two mode changes during replacement.
The root changed from `0770` to `0755`, and an unrelated note changed from `0600` to `0644`.
The temporary probe source is no longer in the package. Its exact source and raw result remain in the evidence record.
CSP2 must preserve workspace access policy or refuse before publication. This verified defect takes priority over role-policy consolidation.


## Workspace access snapshots

Workspace replacement now binds native ownership and ACL metadata to content and mode snapshots.
The atomic path rejects observed late changes. Version 2 journals require access digests during recovery and backup cleanup.
The settled macOS workspace race suite passed 115 test events. Native Linux ARM64 passed 107 events without race instrumentation.

Windows AMD64 and ARM64 compiled. Native Windows execution and preservation during staging remained UNVERIFIED at that checkpoint.
The [snapshot record](csp2/workspace-access-snapshots.md) preserves failed attempts, verification commands, bounds, and legacy recovery obligations.
No primary acceptance status changes. CSP2 remains in progress.

## Workspace access preservation

The [preservation record](csp2/workspace-access-preservation.md) covers private preparation, source-bound copying, native access restoration, and non-YAML operator notes.
The final workspace checks passed 129 macOS race events and 120 native Linux events without skips.
The full Linux run passed 330 events across three packages. Six Windows binaries compiled, and static checks passed.
That verification matched all 134 recorded source hashes.

The final six-package macOS run passed 695 events and failed one runtime process-lock test.
Ten focused reruns passed. The original failure remains unresolved and prevents a passing verdict for that broader run.
CSP2 must investigate the process-lock lifetime before repeating its required runtime checks.

Native Windows inheritance, foreign-owner restoration, service procedures, and shared semantic policy ownership remain open.
CSP5 retains abandoned-stage cleanup and errors after publication. CSP8 and CSP12 retain Starport adoption and engine qualification.
All 50 primary cases remain UNVERIFIED. CSP2 remains in progress.

## Shared role policy and corrected lock lifetime

The [shared-policy record](csp2/shared-file-policy.md) records the public role registry, adapter checks, and shared POSIX classification.
The required role-policy test now compares runtime behavior with diagnostic output across five roles on macOS and Linux.
The workspace remains deployment-controlled, and private input checks preserve bytes and permissions.

Forced garbage collection reproduced the earlier child lock failure in five runs. Explicit lock reachability corrected the harness.
Ten focused reruns and the full 925-event macOS race run passed. The broader run had no failures, skips, or source-hash mismatches.
Native Linux passed 803 core events across ten packages. Three additional tooling tests failed because Bash, Git, and curl were absent.

Ago, host and Windows lint, layout, dependency direction, and 18 Windows builds passed.
The subsequent Windows inheritance correction also passed ago, Windows lint, and both workspace cross-builds.
Windows execution, native tooling, service procedures, foreign-owner restoration, and the constructor decision remain open.
CSP2 remains in progress, with no new primary acceptance credit.

The [current task gate](csp2/shared-file-policy-task.json) retains two passing subcases and twenty UNVERIFIED subcases.
All fifty primary cases remain UNVERIFIED. The document gate preserves 38 tasks, 50 cases, and 324 required subcases.

## Native tooling and foreign ownership

The [native ownership record](csp2/native-ownership-and-tooling.md) closes the Linux tooling environment gap with 72 passing events.
The isolated ownership fixture passes 15 events across 12 atomic and journaled scenarios.
It verifies unprivileged refusal, group-authorized updates, ownership preservation, and unchanged operator notes.
Foreign-owner qualification on other platforms, service procedures, legacy journal recovery, and the constructor decision remain open.
CSP2 remains in progress. No primary acceptance case changes status.

The [application verification](csp2/foreign-ownership-checks.json) passes 639 macOS race events without failures or skips.
Ago and Linux vet with the fixture tag also pass. Recorded code inputs remain unchanged during verification.

## Qualification boundary and successor work

CSP2 remains incomplete and is now blocked on native qualification and the pending constructor and service-configuration decisions.
The [compatibility inspection](csp2/journal-version-compatibility.json) records the checked v0.16.5 workspace implementation.
That version has no replacement journal and refuses directory exchange on Windows.
The local version-1 replacement journal therefore has no compatibility obligation established by this checked release.

Current recovery continues to refuse unbound or unknown journals without changing their files.
CSP18 owns explicit operator recovery guidance for those records. This routing does not weaken CSP2 native interruption requirements.
Independent CSP3 work now owns the active ledger row. No CSP2 acceptance status changes.

## Owner decisions and publication preflight

The [owner decisions](csp2/owner-decisions-2026-09-06.md) resolve the earlier constructor and service-configuration questions.
The owner also authorized reviewed commits, task-branch pushes, draft PRs, and native CI. Merges and releases remain outside that authority.
CSP2 still needs the service-managed primary configuration exception and native qualification.

The [first publication gate](csp2/publication-preflight.json) failed its initial `go test ./...` step with five test failures.
Two tests received noncanonical temporary paths because the runner appended a slash to an existing trailing slash.
The runner now resolves its operation-owned temporary directory before exporting it.

The synthetic application fixture supplied an existing catalog directory with group or other access.
It now selects an absent child directory so the store creates that directory with private access.
The deployment checks rejected the new node-owned `STARMAP_HOME` setting as a catalog-source override and an unknown setting.
They now verify its exact path inside the durable volume. The unknown-name check also tests a misspelled home setting.

The [focused correction](csp2/publication-preflight-focused.json) passed six race events without failures or skips. The tests used Go (1.25.12).
Ago and whole-module lint passed after these changes. The complete publication gate must pass before required review and publication.

The [corrected full run](csp2/publication-preflight-corrected.json) passed all package tests, then failed the external-consumer checksum check.
The six consumer modules now include the pinned native ACL dependency and its checksums.
Module tidying also removed obsolete indirect requirements from the server and remote fixtures. It introduced no dependency version upgrades.

The server-storage fixture also supplied an existing directory with excessive access. It now selects an absent private-store child.
All six consumer fixtures passed before the dependency-budget check. The original 32-package limit then refused the measured 37-package macOS closure.
The owner approved [measured platform budgets](csp2/owner-decisions-2026-09-06.md#native-dependency-budgets), with the existing forbidden-dependency rules intact.

The [pure-Go gate](csp2/publication-consumer-platform-budgets.json) now passes all consumer compositions, S3 checks, and the CLI build without cgo.
The read-only closure has 37 packages. The pinned-artifact closure has 38, including its existing artifact reader.
Server, remote, and server-storage closures remain within their existing limits of 260, 240, and 350 packages.

The [file-size gate](csp2/publication-test-file-sizes.json) passed.
The [generated documentation check](csp2/publication-generated-documents.json) passed after the repository generator refreshed API documents.
The complete publication gate still needs a successful run before review and draft PR publication.

The [full verification run](csp2/publication-preflight-final.json) finished with exit 2 after 661.6 seconds.
Package tests, race checks, dependency guards, file-size checks, coverage, lint, container smoke, and generated-document checks passed.

Prose lint reported four diagnostics. The new dependency-script comment now passes its focused lint check.
Three diagnostics remain in two historical output files. Their proposed lint exclusions await the owner decision.
The final CLI checks did not execute because prose lint stopped the script. No commit or publication occurred.


## Publication CLI isolation

The final verification commands previously inherited operator configuration except for the provider listing.
A [synthetic inherited CONFIG value](csp2/publication-cli-isolation-red.json) made catalog validation fail before it reached the catalog.
This test used a missing temporary file and did not read operator configuration.

The verifier now runs all four CLI checks with an empty environment and operation-owned directories.
It selects the embedded catalog, disables acquisition, and disables the optional YAML workspace.
The [actual CLI tail](csp2/publication-cli-isolation-green.json) passed version, catalog validation, provider listing, and model listing with hostile inherited selectors.
The inherited selectors were synthetic paths. The test did not use provider credentials.

This result covers only the CLI tail. The full publication gate still needs successful verification and review.

The [CSP3 classification record](csp3/historical-output-classification.json) resolves the earlier frozen-output lint blocker without changing historical bytes.

## Service-managed primary configuration

Commit `707d63db` implements the approved primary configuration exception.
The [service configuration proof](csp2/service-configuration.md) records selection, native access rules, diagnostics, migration rereads, and verification.
The local suites pass 394, 42, and 686 race results. Native Linux ownership checks pass, and Windows code compiles.
Native Windows execution and complete CSP2 acceptance remain UNVERIFIED. Starport adoption remains under CSP8.

## Native Windows service qualification fixture

Commit `fb54f39e` adds administrator-owner, denied-read, recovery, and diagnostic assertions to the existing Windows suite.
The [fixture record](csp2/windows-service-qualification.md) preserves AMD64 and ARM64 compilation and static checks.
Native execution remains UNVERIFIED. The fixture must not silently skip its ownership check in GitHub Actions.
