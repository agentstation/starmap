# Current pull request queue

Seven PRs remain open after seven approved merges. All retain distinct work or required ancestry.
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
| 4 | Starmap #132, #134, #133, #135, #136 | PR #125 merged at `96c098f9`, and #126 merged at `b5c3f48a`. Merge the remaining stack in this order after required checks pass. Continue CSP3 under its publication gate. | Catalog plan executor |
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
The [merge record](../approved-merges/verification.json) confirms seven completed merges.
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
Fresh checks on that head replace the old-head retry. Automatic merge remains off until the native checks pass.
