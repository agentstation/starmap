# Current pull request queue

Fourteen PRs remain open. All retain distinct work or required ancestry.
Starmap #127 remains closed because #128 preserves its dependency changes.
No additional PR qualifies for closure in this review.

The [inventory](inventory.json) records exact heads, bases, and attached checks.
There are 54 successful checks and four neutral results.
Six stacked Starmap PRs have no attached checks.
Their existing manual evidence applies only to its recorded source.

| Order | PRs | Disposition and next action | Owner |
| --- | --- | --- | --- |
| 1 | Starport #367 | Retain for an authorized merge. The pin review and ten current checks pass. Recheck the head before merge. | Release owner |
| 2 | Starmap #128–#131 | Retain the four independent dependency updates. Complete the specific compatibility checks below before merge. | Catalog plan executor |
| 3 | Starport #368 | Retain the dependency group. Validate storage, credentials, documentation, and catalog integration before merge. | Catalog plan executor |
| 4 | Starmap #125, #126, #132, #134, #133, #135, #136 | Retain this dependency order. Integrate parent 99843ff8 into #133, then propagate it through #135 and #136. | Catalog plan executor |
| 5 | Starport #366 | Retain the product changes. Adopt the compatible approved Starmap tag, then complete integration checks and publication review. | Catalog plan executor |

The current branch ancestry confirms one immediate gap: #133 lacks its current #134 parent.
That parent changes 23 proof paths.
PRs #135 and #136 contain their direct published parents, but need the same update after #133 changes.
No default-branch merge or release occurred.

## Dependency scope

The review inspected all six dependency diffs.
The diff files beside this document preserve their exact contents.
This scope review does not complete dependency compatibility testing.

| PR | Verified scope | Required evidence before merge |
| --- | --- | --- |
| Starmap #128 | Azure core and identity, MSAL, testify, and removal of an indirect YAML requirement. | Azure credential resolution, module checksums, and combined catalog integration. |
| Starmap #129 | AWS config, credentials, signin, SSO, SSO OIDC, and STS. | Credential resolution and the combined AWS module graph. |
| Starmap #130 | S3 and the server-storage consumer fixture module. | Consumer build, module tidiness, and catalog store operations with combined dependencies. |
| Starmap #131 | Secrets Manager. | Secret resolution and the combined AWS module graph. |
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

At capture, Starmap has 35 unpublished commits through 33728b3f and eleven changed or new CSP3 files.
Starport has six unpublished commits through 69b5aff.
These counts precede this audit commit.
Published checks do not qualify those local changes.
The active plan owns their next publication and its required review.
