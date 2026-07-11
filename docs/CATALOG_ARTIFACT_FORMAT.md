# Catalog Artifact Format

Starmap distributes a validated catalog generation as two byte objects:

1. `starmap-catalog.tar.gz`, a deterministic archive with media type
   `application/vnd.agentstation.starmap.catalog-artifact.v1+tar+gzip`;
2. `starmap-catalog.intoto.json`, a detached in-toto Statement v1 that binds
   the archive and its logical members.

The detached statement is reproducible signing input, not proof of publisher
identity by itself. Release publication adds a repository/workflow-bound signed
attestation; consumers verify that trust proof in addition to the byte-level
checks defined here.

## Archive layout

| Member | Role |
| --- | --- |
| `artifact.json` | Format version, generation identity, schema/consumer compatibility, and exact member descriptors |
| `manifest.json` | Complete validated `catalogs.GenerationManifest` |
| `catalog.json` | Exact canonical catalog payload bound by the manifest |

No other member, duplicate name, directory, link, or special file is accepted.
The descriptor binds each member's filename, media type, size, and SHA-256.
`manifest.json` independently binds `catalog.json`; normal generation validation
must pass before build and after open.

The generation ID is an immutable logical identifier. The payload and archive
are content-addressed independently by SHA-256; consumers must not infer a
digest from the generation ID. Catalog compatibility is determined only by the
manifest schema version and consumer-compatibility range, never a Starmap or
Starport binary/release version.

## Reproducibility

For identical validated generation inputs, `catalogartifact.Build` emits
byte-identical archive and detached-statement output. The archive fixes:

- member order: descriptor, manifest, payload;
- regular-file mode `0644`, zero owner/group, and Unix epoch timestamps;
- USTAR headers, gzip best compression, a fixed gzip OS marker, and no names or
  comments in the gzip header;
- compact deterministic JSON encodings.

The executable fixture pins both the archive SHA-256 and detached-statement
SHA-256. Any format-affecting change therefore requires an explicit format
version and fixture review.

## Verification order

Consumers:

1. bound compressed and expanded input sizes;
2. parse only the three allowed regular-file members;
3. strictly decode `artifact.json` and `manifest.json`;
4. verify manifest identity, validation result, payload checksum/size/media type,
   schema version, and consumer compatibility;
5. verify descriptor member hashes and sizes;
6. verify all detached in-toto subjects and the compatibility predicate;
7. at the publication boundary, verify the signed repository/workflow
   attestation before atomic activation.

Steps 1-6 are implemented by `catalogartifact.Open`; signed publisher identity
is the P8.9 publication trust boundary.

## Immutable release publication

`catalogartifact.StageReleaseAssets` verifies the archive and statement, then
fsyncs archive, statement, and GNU-compatible SHA-256 file into a temporary
directory and atomically publishes one generation-keyed immutable directory.
An exact retry is idempotent. Existing partial, tampered, or different bytes for
the same generation ID return a typed conflict and are never overwritten.

`go run ./cmd/starmap-catalog-release --output-dir <dir>` performs that staging
for the verified embedded generation and emits a JSON report of exact paths.
The scheduled catalog-generation workflow publishes those three paths in a
catalog-only prerelease keyed by payload digest; a rerun cannot silently replace
a published asset. Application releases never append catalog-generation assets.
Hosted workflow execution evidence remains separate from deterministic local
verification.

The workflow uses GitHub's `actions/attest-build-provenance` v2 action pinned
to an immutable commit with
`attestations: write` and `id-token: write`, then runs `gh attestation verify`
with the exact repository, signer workflow, and hosted-runner policy before and
after public download. See GitHub's [artifact attestation guidance](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations)
and the [`gh attestation verify` contract](https://cli.github.com/manual/gh_attestation_verify).

## Optional OCI mirror

The scheduled catalog-generation workflow can additionally mirror the same
three immutable assets to an OCI registry when
`STARMAP_CATALOG_OCI_MIRROR=true`. The target defaults to
`ghcr.io/<owner>/starmap-catalog` and can be replaced with
`STARMAP_CATALOG_OCI_REPOSITORY` for an enterprise registry. ORAS publishes the
manifest under `sha256-<archive digest>`, records the logical generation ID as
`ai.agentstation.starmap.generation`, and preserves the catalog archive as a
layer with the artifact media type defined above.

An OCI manifest digest identifies the manifest and is therefore not expected to
equal its catalog layer digest. Publication reads ORAS's immutable manifest
reference, pulls by that digest, and requires both the returned layer descriptor
and the downloaded archive bytes to equal the release archive SHA-256. It also
compares the detached statement byte-for-byte. Tags are discovery aids only;
enterprise consumers pin the manifest reference and validate the catalog layer
against a trusted release or hosted archive checksum. This follows ORAS's
[single-file layer media-type convention](https://oras.land/docs/1.2/how_to_guides/pushing_and_pulling/)
and [digest-addressed pull behavior](https://oras.land/docs/commands/oras_pull/).
