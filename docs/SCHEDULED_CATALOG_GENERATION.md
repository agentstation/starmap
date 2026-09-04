# Scheduled Catalog Generation

The repository-owned Catalog Generation workflow is a distribution producer,
not a scheduler embedded in ordinary Starmap consumers. It requests a run every
four hours at minute 17 and supports manual dispatch. One non-cancelling
concurrency group serializes publisher runs.

The workflow bounds each run with nested limits. The catalog transfer bound is
60 minutes. The refresh step limit is 75 minutes. The job limit is 90 minutes.

The workflow runs these gates in order:

1. refresh source and catalog candidates through the checked generation script.
2. canonicalize the catalog and calculate separate facts-only semantic and
   exact evidence-payload checksums.
3. derive a new logical generation only when catalog facts change.
4. run catalog-generation and embedded age/size/coverage gates.
5. stage the validated deterministic archive and checksum assets.
6. create and verify repository/workflow-bound provenance.
7. publish an immutable GitHub prerelease keyed by the catalog digest while the
   artifact remains bound to its exact evidence payload.
8. download the three public assets and reopen the archive and detached
   statement. Verify the checksum and exact repository and workflow provenance.
   Compare the downloaded bytes with the staged publication set.
9. when a prior catalog prerelease exists, download and reopen it with the same
   identity checks. Verify its checksum, detached statement, repository, and
   workflow so the rollback target remains readable.
10. advance the stable channel over the verified immutable release. Attest,
    publish, and read back the channel document.

Manual execution cannot force an unchanged publication. If the immutable
release already exists, the workflow downloads all three assets and verifies the
archive, checksum, detached statement, and facts-only digest. It then advances
the channel without creating a second release. Exact payload checksums remain
the integrity and evidence identity inside each generation. Observation times
can therefore change audit evidence without manufacturing a second release for
identical catalog facts.

## Canonical names

The immutable release tag joins `catalog-` to the full facts-only SHA-256 hex
digest. The release title joins `Catalog ` to the generation identifier. Both
are distribution identities, not Starmap binary versions.
GitHub holds two retired namespaces, `catalog-semantic-*` and
`catalog-payload-*`. The release command still reads them, so every historical
release remains a rollback target. New publication uses the canonical namespace
alone.

## Stable channel

The stable channel is the `catalog/v1` branch. It carries one attested file,
`channel.json`. The document names the selected immutable release, its assets,
and their checksums. Consumers read the channel to discover the current
catalog.

The channel is a branch because this repository enables immutable releases.
GitHub freezes an immutable release at creation, so it never replaces a
published asset. The pointer stays mutable on a branch, and the content stays
immutable in a `catalog-<digest>` release.

The workflow reads the branch through `git fetch` and `git show`, never through
the raw content host, because that host caches a changed file for minutes. It
commits the document with `git commit-tree` against a private index, so the
publish leaves the checked-out source tree untouched. The first run writes a
root commit that carries no source history, plus a short `README.md` that names
the branch as a machine-written pointer.

The workflow advances the channel sequence and `channel_updated_at` after every
successful run. An unchanged catalog therefore proves freshness without a new
generation and without a new immutable release. The `published_at` and
`generation_id` fields change only when the channel selects a different release.

The channel never selects a release before that release verifies. The release
assets must be present, the checksums must match, and the attestation must
verify. `Channel.Advance` in `pkg/catalogs/artifact` rejects any other order
with a typed validation error.

## Operational notes

The workflow injects provider credentials only into the refresh step. It uses
noninteractive dependency policy, and any refresh, typed validation, budget,
attestation, identity verification, or release command failure stops the run.
Expiring Actions artifacts are never used as the runtime catalog source.

Deployments that invoke acquisition on their own cadence own any cross-process
lease, retry policy, and durable run records above the explicit operation.
This repository publication workflow does not define those concerns.
