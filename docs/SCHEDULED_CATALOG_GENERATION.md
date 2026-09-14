# Scheduled catalog generation

Starmap's public publisher collects catalog evidence, publishes verified assets, and promotes the same catalog into the default branch.
It requests a run every four hours at minute 17. Manual dispatch and completed promotion-PR checks can also resume publication.
One concurrency group serializes runs without cancelling an active publisher.

Provider API keys stay in Actions secrets. GitHub stores public catalog data and source observations without checkpoint encryption or a private object store.
The [public profile](../.github/catalog-publication.yaml) selects models.dev and twelve public provider scopes.
The [preparation command](../cmd/starmap-catalog-publish/README.md) documents admission, credential scope, and retained evidence.

## Publication sequence

The workflow executes trusted code from the default branch. A promotion PR can change only the embedded catalog.
Provider credentials enter only the acquisition step. The publisher verifies the archive, run receipt, checkpoint, and discovery documents against repository and workflow provenance.

1. Read both accepted channels and the pending publication record through Git.
2. Recover unfinished publication before collecting new evidence.
3. Prepare an artifact, run receipt, and checkpoint when no unfinished publication exists.
4. Validate the proposed embedded catalog with the existing generation and budget checks.
5. Attest the exact preparation and retain its files as an Actions artifact.
6. Persist the attested pending record before publishing release assets.
7. Publish and download the immutable receipt, checkpoint, and catalog assets. Verify their exact bytes.
8. Create or resume the promotion PR. Wait for checks and any required review.
9. Merge the verified PR and compare its committed embedded catalog with the published artifact.
10. Stage, attest, publish, and read back both discovery channels.

Publication completes only after both channels select the intended catalog and the modern channel binds its receipt and checkpoint.
A public release alone does not change the accepted catalog.
A source checkout contains the latest completed default-branch promotion. Existing binaries and pinned modules retain their embedded bytes until rebuilt.

## Publisher setup

Install a dedicated GitHub App on this repository. Configure `CATALOG_APP_CLIENT_ID` as a repository variable and `CATALOG_APP_PRIVATE_KEY` as an Actions secret.
The App key authenticates catalog PR operations to GitHub. It is separate from provider credentials and is not a checkpoint encryption key.

The installation token requests these repository permissions:

| Permission | Access | Purpose |
| --- | --- | --- |
| Contents | Write | Push the catalog branch and merge its checked PR. |
| Pull requests | Write | Create and inspect promotion PRs. |
| Administration | Read | Verify existing branch protection. |
| Actions | Read | Inspect workflow evidence. |
| Attestations | Read | Verify publication provenance. |
| Checks | Read | Verify results for the exact proposed commit. |

The workflow creates the installation token after acquisition because these tokens expire after one hour.
It derives the bot's author identity from the installed App.
The repository token handles public releases, pending records, and discovery channels.
[GitHub App token action](https://github.com/actions/create-github-app-token)

Main must require strict `Security & Reliability` and `Verification Gate` checks from the GitHub Actions app.
Promotion also requires successful native jobs for Ubuntu x64 and ARM, macOS ARM and Intel, and Windows x64 and ARM.
A newer incomplete check prevents an older successful result from qualifying its name.
The publisher never bypasses protection, supplies its own approval, or force-pushes a publication branch.
Required review appears as `awaiting_review` in the job summary with the PR link.

The App installation and controlled bot-PR qualification remain deployment prerequisites.
The workflow refuses missing App configuration before acquisition or public publication.
The refresh step has a 75-minute limit. The complete job has a 90-minute limit.

## Stored files and identities

| Location | Contents and lifetime |
| --- | --- |
| Runner temporary directory, `catalog-publication/` | Private preparation, verification, and checkout files for one job. |
| Actions artifact, `catalog-publication-<run ID>` | Exact prepared files retained for 90 days before public publication. |
| Branch `catalog/publication`, file `pending.json` | Attested identity of the latest preparation and its original workflow run. |
| Release `catalog-run-<receipt digest>` | Immutable `starmap-catalog-run.json` and `starmap-catalog-state.json`. |
| Release `catalog-<semantic digest>` | Archive, detached checksum, and statement. |
| Branch `catalog/promotion/<receipt digest>` | The proposed embedded catalog and its checked PR. |
| Branches `catalog/v2` and `catalog/v1`, file `channel.json` | Attested accepted publication pointers. |

The modern channel binds the artifact, receipt, checkpoint, and promoted source commit.
The legacy channel retains catalog discovery compatibility. Both advance only after verified promotion.
Consumers use public releases and channels. Expiring Actions artifacts are publisher recovery inputs, not runtime catalog sources.

An unchanged semantic catalog reuses its immutable artifact. A new run receipt reports current source outcomes and original observation ages.
Channel confirmation does not claim that every provider supplied fresh evidence.
Historical release namespaces remain readable by the release tooling for rollback.
New publications use the canonical names above.

## Recovery

Run the workflow again after a failed job. A pending publication selects its original run, artifact, receipt, and checkpoint before any new acquisition.
The publisher first restores public receipt and checkpoint assets. If those assets are incomplete, it restores the original Actions artifact.
A later job can reconstruct the exact archive without provider credentials.

An interrupted release upload can leave an unfinished draft asset. The publisher can remove that unfinished asset and upload its expected bytes again.
It never overwrites an uploaded asset. A byte mismatch stops recovery.
A lost response after successful publication reuses the verified release.

An interruption between the promotion branch push and PR creation reuses the existing branch after checking its catalog and changed paths.
Required checks and reviews keep the previous channels active.
An operator-closed promotion stops the pending publication. Resolve that disposition before resuming it.
An authored catalog change that conflicts with a pending promotion also requires resolution.

Channel commits use the previously read parent and a normal Git push.
A concurrent branch update rejects the stale push instead of overwriting it.
The workflow publishes v2 first. If v1 publication fails, the next run completes the same publication before acquiring again.
The pending branch remains as the last publication record. Completion requires both channel bindings to match it.

An optional OCI mirror retains the existing opt-in behavior after channel publication.
Set `STARMAP_CATALOG_OCI_MIRROR=true` and optionally `STARMAP_CATALOG_OCI_REPOSITORY`.
The mirror uses the archive digest, downloads the result, and checks its exact bytes.
It does not replace GitHub catalog authority.

## Verification boundary

`scripts/test_catalog_publication.py` uses real catalog tools and isolated Git repositories with simulated GitHub responses.
It checks recovery, immutable bytes, conditional branch updates, and checked promotion ordering.
These tests do not qualify GitHub App permissions, actual workflow triggering, or hosted provenance.
Controlled repository qualification must verify those contracts before enabling scheduled promotion.
