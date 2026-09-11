# Current pull request queue

Updated 2026-09-11 UTC. Starmap has one open PR, [#151](https://github.com/agentstation/starmap/pull/151). Starport has none.
The [queue evidence](queue-status-2026-09-11.json) records current heads and remaining gates.

PR #151 contains `a6545493`, the permission cancellation fix and Windows privilege diagnostic.
Forty-five retention race events pass per supported toolchain. Sol reports zero findings.
Opus flags the known native failure. The merge block remains, and every native test stays mandatory.

Workflow `34597460388` fails Windows status access before and after enabling the held time privilege on both architectures.
Other native jobs continue. Investigate the verified RPC authentication and impersonation contract before merge.

Origin follower startup `4bb4b870` passes seven final race events per toolchain and writes no shared catalog.
Periodic adoption, safe takeover, and transitions remain incomplete.
Clock configuration `e97d7fc3` passes all 41 verifier stages but still requires the qualified parent and final review.

Starport `190a8124` pins a published merged dependency and passes 185 focused race events with zero skips.
Its architecture gate passes eleven checks, including the full Go suite, and fails the tag-only V01 expression.
Four mapped consumer cases remain incomplete. Twenty-eight campaign PRs merged, nine tasks are complete, and CSP4 remains active.

## Historical queue snapshots

The sections below preserve earlier captures and superseded next actions. Use the current queue evidence above for execution.

Six PRs remain open after eight approved merges. All retain distinct work or required ancestry.
Starmap #127 remains closed because #128 preserves its dependency changes.
No additional PR qualifies for closure in this review.

The [completed parent update](../stack-parent-completion/verification.json) records current heads, bases, and attached checks.
That earlier capture records 54 successful checks and four neutral results.
At that capture, six stacked Starmap PRs had no attached checks.
Their existing manual evidence applies only to its recorded source.

| Order | PRs | Disposition and next action | Owner |
| --- | --- | --- | --- |
| 1 | Starport #367 | Merged at `565a4fcd` after pin review and ten passing checks. | Catalog plan executor |
| 2 | Starmap #130–#131 | PRs #128 and #129 merged. Retain the two remaining dependency updates. Combined adapter, consumer, and listed SDK credential-path checks pass. Preserve the consumer checksum repair during integration. | Catalog plan executor |
| 3 | Starport #368 | Merged at `857bd854` after local dependency checks and ten passing CI checks. | Catalog plan executor |
| 4 | Starmap #134, #133, #135, #136 | PR #125 merged at `96c098f9`, #126 at `b5c3f48a`, and #132 at `12fca002`. Merge the remaining stack in this order after required checks pass. Continue CSP3 under its publication gate. | Catalog plan executor |
| 5 | Starport #366 | Merged at `cffa9300` after 16 passing checks. CSP0.1 is complete. Six later local commits retain their separate release dependency. | Catalog plan executor |

The current branch ancestry confirms all six parent relationships.
The three published updates change 23 proof paths each and preserve every non-proof diff.
PRs #133, #135, and #136 now publish heads `07dcb63d`, `aff5fd53`, and `420c0d77`.
This parent-update capture predates the three default-branch merges. No release occurred.

## Dependency scope

The review inspected all six dependency diffs.
The diff files beside this document preserve their exact contents.
The [combined dependency check](../combined-dependencies/README.md) passes adapter, acquisition, and consumer integration tests.
The [SDK credential-chain checks](../sdk-credential-chains/README.md) pass for the listed environment, file, and web identity paths.
Live cloud policy and other identity services remain outside that evidence.

| PR | Verified scope | Required evidence before merge |
| --- | --- | --- |
| Starmap #128 | Azure core and identity, MSAL, testify, and removal of an indirect YAML requirement. | Current-head checks and owner-authorized merge. Local environment and workload identity SDK checks pass. |
| Starmap #129 | AWS config, credentials, signin, SSO, SSO OIDC, and STS. | Current-head checks and owner-authorized merge. Local environment, shared-file, and web identity SDK checks pass. |
| Starmap #130 | S3 and the server-storage consumer fixture module. | Current-head checks and owner-authorized merge. Combined consumer, module tidiness, and S3 wire checks pass locally. |
| Starmap #131 | Secrets Manager. | Current-head checks and owner-authorized merge. Real SDK secret requests and typed denial checks pass locally. |
| Starport #367 | Two workflow references to the verified Homebrew action commit. | Current-head confirmation and merge authorization. |
| Starport #368 | Nine direct modules plus AWS authentication, SQLite libc, and memory dependencies. | SQLite persistence and migrations, MySQL DSN behavior, AWS credentials, documentation rendering, and catalog integration. |

The tagged [Azure identity changelog](https://github.com/Azure/azure-sdk-for-go/blob/sdk/azidentity/v1.14.1/sdk/azidentity/CHANGELOG.md) identifies dependency updates.
The tagged [AWS config changelog](https://github.com/aws/aws-sdk-go-v2/blob/config/v1.33.2/config/CHANGELOG.md) identifies SDK dependency updates.
The [S3 changelog](https://github.com/aws/aws-sdk-go-v2/blob/service/s3/v1.110.0/service/s3/CHANGELOG.md) moves credential-source user-agent setup into client stack construction.
The [Secrets Manager changelog](https://github.com/aws/aws-sdk-go-v2/blob/service/secretsmanager/v1.47.0/service/secretsmanager/CHANGELOG.md) records the same change.
The generated Secrets Manager comparison link names unrelated module tags.
Use the tagged module changelog as the release source.

## Maintenance triggers

Before each publication, recheck remote heads, bases, checks, local commits, and parent ancestry.
After a parent changes, validate each dependent tree before publishing it.
Keep dependency updates independent of unfinished catalog acceptance.
Use the existing PR for work it already owns.
Close a replaced PR only after verifying that its successor preserves all required changes.
Preserve branches when closing superseded PRs.

The [earlier inventory](inventory.json) preserves local commit counts before these parent updates.
Published checks do not qualify those local changes.
The active plan owns their next publication and its required review.

The [earlier recheck](final-recheck.json) records heads, bases, and check totals before the parent updates.
The merge record below supersedes the earlier dispositions.
Starport #367 remains mergeable at `3f0817ad`, with the two reviewed workflow pin changes.
The owner later authorized its merge, which completed at `565a4fcd`.

The parent updates are complete. Seven PR descriptions now record the published ancestry or combined dependency evidence.
The [publication record](../stack-parent-completion/verification.json) verifies every updated body against its remote value.

## Daily queue review

The [latest queue check](queue-refresh.json) confirms fourteen open PRs and zero unresolved review threads.
Attached checks report 54 successes and four neutral results.
At that capture, six stacked Starmap PRs had no attached checks.
Every remote head and base matches the previous queue review.
Starmap #127 remains closed, and no further PR qualifies for closure.

The description for #134 now records the completed parent update across all dependent PRs.
Remote readback confirms the corrected body and unchanged head and base.
The merge order and required evidence above remain current.

The active daily task checks both repositories at 09:00 local time.
It reports actionable changes and reviews unchanged PRs after seven days without recorded progress.
It now merges reviewed PRs after required checks pass under the standing owner approval.
The task identifier is `maintain-starmap-and-starport-pr-queue`.

## Approved merges

The owner authorized reviewed merges without repeated permission requests on 2026-09-08.
The [merge record](../approved-merges/verification.json) confirms eight completed merges.
Starmap requires each PR branch to include current main before merge.
PR #129 merged at `df9585f5`. PR #132 now runs checks against that main revision.
Starmap #128 merged at `e12a865f` after its required checks passed.

Record the merge commit before marking an implementation task done.
Passing component evidence remains separate from a completed implementation merge.
Releases still require separate authority.

## Starport published scope

PR #366 now includes current main at head `2973fd08122bc8264c499b99e60b44c890f3c6c7`.
Its published source retains stable Starmap v0.16.5 and can merge independently of the six later local commits.
All 16 checks passed before merge at `cffa9300`. The local product worktree still preserves all six later commits.
The later pseudo-version and release contract do not block this published scope.

## Native retry for Starmap #132

The executor disabled auto-merge at head `70288ead`.
The [native capture](../pr132-native-retry/verification.json) records a Go module proxy failure on Linux x86-64.
The job passes 497 test results and nine packages. Three other packages fail setup because the module download fails.
No assertion fails. The other five native jobs pass.

GitHub rejects a job retry while its containing workflow runs.
After that workflow ends, recheck the PR head and main.
Retry the failed job if the head is current, or update main and qualify the new head.
Restore auto-merge only after the native checks pass.

The workflow later finished, and Starmap #129 merged at `df9585f5`.
The required main update gives #132 replacement head `e7bc93f1`.
All six native jobs pass on the replacement head. The executor enabled automatic merge after those passes.
The required Verification Gate remains in progress.
The [replacement check capture](../pr132-native-retry/replacement-checks.json) records the exact head and check results.

## Current main propagation

PRs #134, #133, #135, and #136 now include the current foundation parent.
All four PRs are ready for review.
The [propagation record](main-parent-propagation.json) verifies that every reviewed diff outside proof files retains identical bytes.
Merge these PRs into main in the recorded order after their required checks pass.
Do not count a merge into an intermediate branch as task completion.

## Foundation merge

Starmap #132 merged at `12fca002` after twelve successful checks.
The [task closeout](../approved-merges/starmap-132-task-closeout.json) verifies 30 verifier tests and all six CSP1 subcases against that merge.
CSP0 and CSP1 are complete. CSP2 still needs the later repair stack.

PR #134 now targets main at head `b75985eb`.
Automatic merge waits for its required checks.
Then advance #133, #135, and #136 into main in order.

## Evidence merge and next implementation

Starmap #134 merged at `74b66127` after twelve successful checks.
Nine PRs merged. Five Starmap PRs remain open. Starport has no open PRs.
The [merge capture](../approved-merges/starmap-134-merged.json) records the exact head and checks.

PR #133 now targets current main at head `392617c8`.
Its [diff check](pr133-main-update.json) confirms identical reviewed implementation bytes.
Native and required checks run before merge.
Then advance #135 and #136 into main.
Task completion requires the implementation merge and passing acceptance evidence.

## Allocation hint review

PR #133 now uses reviewed repair head `1ff884b9`.
The [review record](pr133-repair-review.json) binds the one-line allocation repair and both reviewers.
All three review portions pass with zero findings.
Fresh CI must clear CodeQL and all required checks before merge.

## Current qualification

Both CSP3.1 lookup subcases pass against PR #133 head `1ff884b9`.
All five historical lookup source files match this candidate.
The [candidate qualification](pr133-candidate-qualification.json) retains the checks.
CSP3.1 remains incomplete until main contains the implementation.

PR #131 now uses head `85d8d7b2` with the consumer module repair.
The [dependency evidence](../pr131-consumers/verification.json) records all six consumer checks and both SDK test suites.
Each toolchain passes 87 test results and two packages.
The current-base CI run must pass before merge.
