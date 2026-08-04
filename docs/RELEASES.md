# Release Policy

## Go versions

Starmap deliberately separates its library compatibility floor from its build
toolchain:

- `go 1.25.0` is the module language and library compatibility floor.
- Go 1.25.12 is the patched 1.25 release exercised by required PR checks.
- `toolchain go1.26.5` is the preferred development toolchain.
- Go 1.26.5 is the exact toolchain used by verification, catalog generation,
  and application releases.

The floor cannot currently be lower without downgrading security-sensitive
runtime dependencies that require Go 1.25. When Go stops supporting the 1.25
family, Starmap will raise the floor to the oldest upstream-supported family.

`go fix ./...` is run with Go 1.26.5 after a toolchain upgrade. Because the
module language version remains 1.25, fixes may use APIs available in Go 1.25
but must not introduce Go 1.26-only syntax. Both version lanes must pass before
the migration is accepted.

## Application releases

Application releases use GoReleaser v2.17.0 and a tag of the form `vX.Y.Z` or
`vX.Y.Z-rc.N`. The tag commit must already be reachable from `main`. The release
workflow:

1. runs repository and release verification with Go 1.26.5;
2. builds Linux, macOS, and Windows archives for amd64 and arm64 with
   `CGO_ENABLED=0`;
3. verifies cgo-disabled build metadata for all six binaries, static ELF
   linkage on Linux, no Windows C/C++ runtime imports, and system-only dynamic
   linkage on Darwin;
4. publishes SBOMs, SHA-256 checksums, and a detached checksum signature;
5. emits GitHub build-provenance attestations for every checksummed artifact;
6. downloads and verifies the public assets, signature, checksums, repository,
   and publisher workflow;
7. publishes the cgo-disabled container from a digest-pinned static base image;
   and
8. updates the public AgentStation Homebrew tap for stable tags, then verifies
   the installed CLI version, cgo-disabled metadata, and system-only Darwin
   linkage.

Starmap does not require a C compiler. Repository verification runs the
read-only, store-only, server-embed, remote-subscriber, and CLI compositions
with `CGO_ENABLED=0`. The separate race suite explicitly uses
`CGO_ENABLED=1`, because Go's race detector normally requires cgo. On macOS,
pure-Go binaries still use Apple system ABI libraries from `/usr/lib` and
`/System/Library`; the gate rejects any separately distributed C runtime.

Release candidates never replace the stable Homebrew cask. Darwin binaries are
signed and notarized when the five `MACOS_SIGN_*`/`MACOS_NOTARY_*` repository
secrets are provisioned; stable launch must not rely on the quarantine-removal
fallback.

Stable Homebrew publication uses an SSH deploy key. Install its public key on
only `agentstation/homebrew-tap` with write access. Store the private key in the
Starmap repository secret named `HOMEBREW_TAP_DEPLOY_KEY`. Do not use a personal
access token for this cross-repository write.

Catalog generations are a separate product data channel. They are published by
the scheduled catalog workflow under payload-digest prerelease tags and are
never appended to an application release.

Use a minor version for a direct pre-v1 compatibility break and a patch version
for a compatible correction. Reserve `v1.0.0` for an explicit public
compatibility commitment across the Go API, CLI, configuration, and durable
catalog formats.

## Operator commands

Prepare a local, non-publishing release snapshot:

```bash
GOTOOLCHAIN=go1.26.5 make release-snapshot
./scripts/verify-release-binaries.sh dist
```

The local snapshot intentionally skips checksum signing and the container image;
those require release secrets and a registry-capable Docker environment and are
verified by the hosted tag workflow.

After maintainers merge the exact commit and authorize publication:

```bash
make release-tag VERSION=0.4.0
```

Pushing the tag is the publication action. Do not reuse or move release tags.

## Failed publication recovery

A failed GoReleaser run can leave a draft release and a `release-dist` workflow
artifact. A later recovery failure can leave the exact release published but its
Homebrew update incomplete. Do not move or reuse the tag. Correct the failure
cause first.

Use the Release workflow's manual dispatch only when all these conditions are
true:

- The source run is a failed tag-triggered Release run.
- The source run commit equals the immutable tag commit.
- The GitHub release is a mutable draft or the exact published immutable release.
- The tag is the most recently created application release or draft.
- The source artifact contains matching GoReleaser tag and commit metadata.

Start recovery with the exact tag and source run ID:

```bash
gh workflow run release.yaml \
  --repo agentstation/starmap \
  -f tag=v0.3.0 \
  -f source_run_id=30875507565
```

The recovery job rechecks the source identity, binaries, checksums, signature,
asset set, and provenance. For a draft, it emits provenance attestations through
the Release workflow and publishes the immutable release. It then verifies the
public downloads, publishes the generated stable Homebrew cask with the deploy
key, and tests a fresh Homebrew installation. A retry against the exact published
release can repair a tap-only failure. Recovery never rebuilds or replaces an
artifact.
