# CSP5 committed candidate qualification

Commit `8683287dfcd5fc17268d501e70f9b6753aea1c67` consolidates automatic local retention and its configuration, diagnostics, and regression corrections.
The [verification record](qualification-2026-09-12/verification.json) binds 63 changed files through follow-up `624d993a8eb091473b956affca721b4cb4e8cf1f` and all recorded commands.
The qualification worktree is clean and uses the committed Go module graph.
The earlier worktree still contains five unrelated tooling edits. Those edits remain outside this commit.

## Runtime test duration

The earlier required suite exceeded 30 minutes after 1,069 passing test events.
Its interrupted startup test passed separately on both supported toolchains.
The [earlier result](automatic-retention-required-runtime-20260912.json) retains that failure and its source bindings.

A race-enabled profile of the startup test recorded about 3.36 GiB of total allocations over 24.26 seconds.
Opening runtimes and rebuilding retained catalogs account for substantial work.
These values describe one test process. They do not measure resident memory or inference request overhead.
Immutable catalogs already cache encoded payloads. The profile does not justify another payload cache.

Eight slow recovery tests now call `t.Parallel`.
Each test owns its stores, runtime directories, sources, and mutable fixture state.
The change preserves every case, assertion, and deadline. It does not change production behavior.

The shuffled race run passes 114 test events across those eight tests in about 190 seconds.
Their individual elapsed times totaled about 557 seconds in the earlier sequential suite.
These observations use separate runs and do not establish a production performance claim.
Runtime lint and repository policy also pass before commit.

## Committed-source checks

The Go 1.25.12 check passes 115 race events for the parallel tests and the runtime snapshot regression on `8683287df`.
Initial full qualification exposed an expired context in `TestAuthorityRuntimePermissionRefreshBypassesBlockedCatalogAndLease`.
That test reused a one-second context after runtime shutdown and reconstruction.
All three isolated repetitions failed when the follower received the expired context.

Follow-up `082a760a2` gives each independent permission refresh its own one-second context.
The test explicitly cancels the first context before opening the follower.
Both supported toolchains pass three repetitions on the corrected source. Runtime lint and repository policy pass.
Neither the test deadline nor production permission behavior changes.

The initial runtime and repository runs stopped after this defect reproduced.
Their records retain terminal interruption status and provide no complete qualification credit.
The task verifier passes all twelve mapped subcases on the corrected commit, with 42 passing race events across 21 Go invocations.

The replacement repository check fails on that commit. The required runtime/storage race suite passes 1,357 test events in about 27 minutes.
The verification record identifies each live session. Running checks receive no acceptance credit.

The public catalog fixture passes cache verification at 411,974 bytes.
The qualification worktree uses the pinned `ago` tool from its committed module graph.
The unrelated rename to `goago` remains in the earlier worktree.

## Full repository failure

`make verify` exits with status 2 during `go test ./...`.
Three assertion failures and four package timeouts prevent qualification.
Later repository gates did not run.

| Package | Evidence | Required action |
|---|---|---|
| `acquisition` | `TestBoundAcquirerRuntimeRetainsIndependentScopesAfterFailureAndRestart` sees three binding-directory entries and expects two. | Identify each entry before changing the assertion or record behavior. |
| `internal/catalog/settings` | Deployment coverage omits retention settings in `docker-compose.yml`. The sample map contains 39 entries for 45 canonical settings. | Add the six settings to the deployment example and sample coverage. |
| `internal/catalog/workspace` | Legacy migration waits inside `Filesystem.Current` while it holds the exclusive publication lock. | Keep migration reads under the same checked lock ownership. |
| `internal/cli/app` | Workspace migration waits on the same nested publication lock. | Verify the caller after the storage and migration correction. |
| `runtime` | The aggregate package reaches its ten-minute default timeout during public-catalog tests. | Inspect terminal race-suite evidence before choosing a duration correction. |

The migration stack and source identify nested lock acquisition.
`legacyLayoutMigrator.migrate` takes the exclusive `.commit.lock` before `inspectLegacyStore` calls `Filesystem.Current`.
`Filesystem.Current` now requests a separate shared handle for that same lock.
The focused race check also reaches its 30-second deadline at the same nested lock.
This defect requires a code correction. A longer timeout does not resolve it.

At that checkpoint, the branch contained 42 local commits beyond the merged CSP4 baseline and had no CSP5 PR.
The twelve mapped component results do not qualify these integration failures or shared collection.
Finish the identified failures and required shared contract before review, native CI, and merge.
Avoid further isolated feature checkpoints that do not advance those delivery gates.

## Committed integration corrections

Commit `064eb02c0e6443abfa277d47bffac7bc76cc3992` corrects migration reads, configuration coverage, and the binding-record assertion across 21 files.
Legacy inspection requires the held publication lease and validates its identity before and after reads.
Bounded private reads preserve manifest, payload digest, schema, directory-name, and authority-record checks.
The initial read, relocated-store check, and recovery inspection use this path.

The first correction still encountered a nested reader after relocation. Its two-minute failure remains in the record.
The final correction passes 75 selected race events, including the original migration, CLI restart, settings, and acquisition failures.
The extra binding entry is the reserved `.record-publications` directory.
The corrected assertion requires two regular JSON records and rejects unexpected entries.
All six retention settings now have Compose examples, application aliases, and sample values.

Recovery validation now has separate functions for journal headers, record events, relocation events, publication guards, and backup transitions.
Four test-only operations moved into a test file.
The final baseline and replacement checks pass 97 race events on each supported toolchain.
These selections overlap earlier checks. Repository lint, policy, prose, and Windows compilation pass.
Native execution remains required before delivery.

Two broader package runs reached their ten-minute limit, after 616 and 529 passing events.
Neither run reports an assertion failure. Their incomplete results remain failures in the verification record.
The interrupted endpoint-drift test passes separately on Go 1.25.12.
The second interrupted completion test has a separate recorded check.

Ordinary, race, and native suites now use the same thirty-minute package bound and run packages sequentially.
The native job has sixty minutes for setup and its complete test sequence.
Individual operation deadlines remain unchanged.
The repository runner captures its starting worktree bytes, which match commit `064eb02c0`.
That run later failed on the file-inspection case described below.

## Current task gate

The CSP5 task command passes on `064eb02c0` with all twelve selected subcases.
A19 passes its four product checks. A22 and A23 pass eight producer checks and retain their Starport consumer requirements.
The command ran from 15:26:58 to 15:33:11 UTC on 2026-09-12 and exited zero.

Its text output preserves the task result but omits child test events. This invocation receives no exact test-event count.
The verification record retains the earlier detailed task evidence under its original source commit.

Repository verification session `61969` finished with exit status 2.
The workspace package passes in about 346 seconds, including the corrected migration path.
Review preflight passes its input, secret-scan, and engine checks. The actual code review remains open.
The prepared PR description identifies the local delivery and the remaining shared collection contract.

## File-inspection integration correction

Repository verification on `064eb02c0` reports one failing parent test in `internal/cli/app`.
Both runtime-path cases omit the generation `.read.lock` from the file manifest.
The runtime package passes in 594 seconds. Later repository gates did not run because ordinary package tests failed.

Commit `624d993a8` adds catalog read locks, authority records, retirement journals, and their temporary records to file inspection.
Inspection preserves private access classification and excludes unrelated files from the manifest.
The focused regression also checks unchanged bytes, omitted file contents, and passive application state.
Its initial run fails on the original manifest.

The corrected Go 1.26.6 run passes six race events, including both original pin-and-removal cases.
The focused Go 1.25.12 run passes three race events. These selections overlap.
Package lint, repository policy, and source prose pass.
Full repository verification now runs on committed source `624d993a8` in session `42639`.

## Delivery sequence

Deliver verified local update controls and retention through the required repository checks, pre-PR review, native CI, and merge.
Shared/object collection remains required for CSP5 completion.
The first delivery does not close CSP5 or claim that shared collection works.
The pending coordinator decision selects that remaining implementation.
This sequence preserves the task acceptance criteria and the full production scope.

## Remaining delivery work

Shared and object generation collection remain incomplete.
The owner decision about mandatory shared coordination versus S3-only automatic cleanup remains pending.
Safe cleanup must protect publishers, retained pins, and readers across instances.
A refresh lease alone does not make separate object deletion and publication atomic.

Required review, native CI, and merge remain open.
A19 qualifies its four product subcases. A22 and A23 qualify eight producer subcases and still require Starport consumer qualification.
The verifier does not qualify the other primary cases.
CSP5 remains in progress. Existing merge authority persists.
