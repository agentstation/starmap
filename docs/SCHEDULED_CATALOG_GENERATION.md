# Scheduled Catalog Generation

The repository-owned Catalog Generation workflow is a distribution producer,
not a scheduler embedded in ordinary Starmap consumers. It runs every day at
03:17 UTC and supports an identical manual dispatch. One non-cancelling
concurrency group serializes publisher runs.

The workflow performs these gates in order:

1. refresh source and catalog candidates through the checked generation script;
2. canonicalize the catalog and preserve the current manifest when payload bytes
   are unchanged;
3. derive a new logical generation only for a changed canonical payload;
4. run catalog-generation and embedded age/size/coverage gates;
5. stage the validated deterministic archive and checksum assets;
6. create and verify repository/workflow-bound provenance;
7. publish an immutable GitHub prerelease keyed by canonical payload digest;
8. download the three public assets, reopen the archive and detached statement,
   verify the checksum and exact repository/workflow provenance, and compare the
   downloaded bytes with the staged publication set;
9. when a prior catalog prerelease exists, download and reopen it with the same
   checksum, detached-statement, repository, and workflow identity checks so a
   rollback target remains demonstrably readable.

Manual execution cannot force an unchanged publication. If a payload-digest
release already exists, the workflow downloads its archive, extracts the bound
manifest, and requires the embedded payload checksum to equal the candidate.
It then exits without publishing. This makes retries and an unmerged embedded
catalog refresh safe from duplicate releases while detecting a tag bound to the
wrong payload.

The scheduled prerelease tag is catalog-payload followed by the full SHA-256 hex
digest. It is a distribution identity, not a Starmap binary version and not a
mutable latest pointer. Runtime channel promotion remains a separate hosted
control-plane action. Expiring Actions artifacts are never used as the runtime
catalog source.

Provider credentials are injected only into the refresh step. The workflow uses
noninteractive dependency policy, and any refresh, typed validation, budget,
attestation, identity verification, or release command failure stops the run.
Deployment-owned `pkg/catalogscheduler` adds a named publisher lease beyond the
repository concurrency group. P9 later adds durable run records, retry
classification, and per-source freshness policy; those concerns must not be
inferred from this publication workflow alone.
