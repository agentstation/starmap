# Current pull request queue

Fourteen PRs remain open. All retain distinct work or required ancestry.
Starmap #127 remains closed because #128 preserves its dependency changes.
No additional PR qualifies for closure in this review.

The [completed parent update](../stack-parent-completion/verification.json) records current heads, bases, and attached checks.
There are 54 successful checks and four neutral results.
Six stacked Starmap PRs have no attached checks.
Their existing manual evidence applies only to its recorded source.

| Order | PRs | Disposition and next action | Owner |
| --- | --- | --- | --- |
| 1 | Starport #367 | Retain for an authorized merge. The pin review and ten current checks pass. Recheck the head before merge. | Release owner |
| 2 | Starmap #128–#131 | Retain the four independent dependency updates. Combined adapter and consumer checks pass. Default credential-chain checks remain open. | Catalog plan executor |
| 3 | Starport #368 | Retain the dependency group. Validate storage, credentials, documentation, and catalog integration before merge. | Catalog plan executor |
| 4 | Starmap #125, #126, #132, #134, #133, #135, #136 | Retain this dependency order. All current parents now propagate through #133, #135, and #136. Continue the unpublished CSP3 work under its publication gate. | Catalog plan executor |
| 5 | Starport #366 | Retain the product changes. Adopt the compatible approved Starmap tag, then complete integration checks and publication review. | Catalog plan executor |

The current branch ancestry confirms all six parent relationships.
The three published updates change 23 proof paths each and preserve every non-proof diff.
PRs #133, #135, and #136 now publish heads `07dcb63d`, `aff5fd53`, and `420c0d77`.
No default-branch merge or release occurred.

## Dependency scope

The review inspected all six dependency diffs.
The diff files beside this document preserve their exact contents.
The [combined dependency check](../combined-dependencies/README.md) passes adapter, acquisition, and consumer integration tests.
Default SDK credential-chain checks remain open.

| PR | Verified scope | Required evidence before merge |
| --- | --- | --- |
| Starmap #128 | Azure core and identity, MSAL, testify, and removal of an indirect YAML requirement. | Default Azure credential-chain behavior. Combined module and adapter checks pass locally. |
| Starmap #129 | AWS config, credentials, signin, SSO, SSO OIDC, and STS. | Default AWS credential-chain behavior. The combined module graph and adapter checks pass locally. |
| Starmap #130 | S3 and the server-storage consumer fixture module. | Current-head checks and owner-authorized merge. Combined consumer, module tidiness, and S3 wire checks pass locally. |
| Starmap #131 | Secrets Manager. | Default SDK credential-chain behavior. Combined secret adapter and module checks pass locally. |
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
The dependency order and dispositions remain current.
Starport #367 remains mergeable at `3f0817ad`, with the two reviewed workflow pin changes.
Its default-branch merge still requires owner authorization.

The parent updates are complete. Seven PR descriptions now record the published ancestry or combined dependency evidence.
The [publication record](../stack-parent-completion/verification.json) verifies every updated body against its remote value.
