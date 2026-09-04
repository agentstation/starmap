# Release Policy

## Go versions

Starmap deliberately separates its library compatibility floor from its build
toolchain:

- `go 1.25.0` is the module language and library compatibility floor.
- Go 1.25.12 is the patched 1.25 release exercised by required PR checks.
- `toolchain go1.26.6` is the preferred development toolchain.
- Go 1.26.6 is the exact toolchain used by verification, catalog generation,
  and application releases.

Devbox pins `go@1.26.5` because its package index does not yet publish 1.26.6.
That pin only bootstraps the `go` command. The `toolchain go1.26.6` directive
selects Go 1.26.6, so a Devbox shell compiles with the release toolchain. Raise
the Devbox pin when the index publishes it.

Security-sensitive runtime dependencies require Go 1.25, so the floor cannot
currently be lower. When Go stops supporting the 1.25 family, Starmap will
raise the floor to the oldest upstream-supported family.

After a toolchain upgrade, use Go 1.26.6 to run this command:

```bash
go fix ./...
```

The module language version remains 1.25. Fixes may use APIs available in Go 1.25 but must
not introduce Go 1.26-only syntax. Accept the migration only after
both version lanes pass.

## Application releases

Application releases use GoReleaser v2.17.0 and a tag of the form `vX.Y.Z` or
`vX.Y.Z-rc.N`. The tag commit must already be reachable from `main`. The release
workflow:

1. runs repository and release verification with Go 1.26.6.
2. builds Linux, macOS, and Windows archives for amd64 and arm64 with
   `CGO_ENABLED=0`.
3. verifies cgo-disabled build metadata for all six binaries. It also verifies
   static ELF linkage on Linux and no Windows C/C++ runtime imports. Darwin
   binaries may use only system dynamic linkage.
4. publishes SBOMs, SHA-256 checksums, and a detached checksum signature.
5. emits GitHub build-provenance attestations for every checksummed artifact.
6. downloads and verifies the public assets, signature, checksums, repository,
   and publisher workflow.
7. publishes the cgo-disabled container from a digest-pinned static base image.
   and
8. updates the public AgentStation Homebrew tap for stable tags. It then checks
   the installed CLI version, cgo-disabled metadata, and system-only Darwin
   linkage.

Starmap does not require a C compiler. Repository verification runs the
read-only, store-only, server-embed, remote-subscriber, and CLI compositions
with `CGO_ENABLED=0`. The separate race suite explicitly uses
`CGO_ENABLED=1`, because Go's race detector normally requires cgo. On macOS,
pure-Go binaries still use Apple system ABI libraries from `/usr/lib` and
`/System/Library`. The gate rejects any separately distributed C runtime.

Release candidates never replace the stable Homebrew cask. GoReleaser signs
and notarizes Darwin binaries when the repository has all five
`MACOS_SIGN_*` and `MACOS_NOTARY_*` secrets. Without those optional
credentials, the cask removes quarantine from only the staged `starmap`
binary through its payload-scoped post-install hook.

Stable Homebrew publication uses an SSH deploy key. Install its public key on
only `agentstation/homebrew-tap` with write access. Store the private key in the
Starmap repository secret named `HOMEBREW_TAP_DEPLOY_KEY`. Do not use a personal
access token for this cross-repository write.

Catalog generations are a separate product data channel. The scheduled catalog
workflow publishes them under their own tags and never appends them to an
application release.

### Catalog release tags

| Tag | Namespace | Meaning |
| --- | --- | --- |
| `catalog-<catalog-digest>` | `canonical` | The current immutable release namespace |
| `catalog-semantic-<digest>` | `legacy-semantic` | A retired facts-digest namespace |
| `catalog-payload-<digest>` | `legacy-payload` | The first retired payload namespace |
| `catalog/v1` | none | A branch, not a tag. The mutable discovery channel |

The `catalog/v1` branch carries the attested channel document `channel.json`
with the media type
`application/vnd.agentstation.starmap.catalog-channel.v1+json`. The document
advances its sequence and its `channel_updated_at` value after every successful
verification. A consumer therefore grades origin freshness even when the
catalog facts did not change.

The pointer lives on a branch because this repository enables immutable
releases. GitHub freezes an immutable release at creation. It accepts no asset
replacement and no asset deletion, and it never releases the tag again. A
mutable document therefore cannot live on a release. The content stays
immutable and the pointer stays mutable, the way a moving tag names one fixed
digest.

A repository ruleset protects the branch against deletion and force push, so
the pointer history only grows. GitHub rejects its own Actions app as a ruleset
bypass actor, so the ruleset restricts no writer. The catalog generation
workflow is the only intended writer. A human never commits to it, because a
hand-written commit breaks the sequence that every client uses to reject a
replayed pointer. The workflow fails the run with an explicit message when
GitHub rejects the push.

`artifact.ReleaseTagNamespace` reports the namespace of one tag. It recognizes
the canonical namespace and both retired namespaces, so historical readback and
rollback keep every published immutable release. Publication uses the canonical
namespace only. Reading and rollback accept all three. Do not delete a retired
tag, because an operator rolls back to it by exact tag.

Use a minor version for a direct pre-v1 compatibility break and a patch version
for a compatible correction. Reserve `v1.0.0` for an explicit public
compatibility commitment across the Go API, CLI, configuration, and durable
catalog formats.

## Operator commands

Prepare a local, non-publishing release snapshot:

```bash
GOTOOLCHAIN=go1.26.6 make release-snapshot
./scripts/verify-release-binaries.sh dist
```

The local snapshot intentionally skips checksum signing and the container
image. Those steps require release secrets and a registry-capable Docker
environment. The hosted tag workflow verifies them.

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
