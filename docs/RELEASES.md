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
2. builds Linux, macOS, and Windows archives for amd64 and arm64;
3. publishes SBOMs, SHA-256 checksums, and a detached checksum signature;
4. emits GitHub build-provenance attestations for every checksummed artifact;
5. downloads and verifies the public assets, signature, checksums, repository,
   and publisher workflow;
6. publishes the container from a digest-pinned base image; and
7. updates and smoke-tests the public AgentStation Homebrew tap for stable tags.

Release candidates never replace the stable Homebrew cask. Darwin binaries are
signed and notarized when the five `MACOS_SIGN_*`/`MACOS_NOTARY_*` repository
secrets are provisioned; stable launch must not rely on the quarantine-removal
fallback.

Catalog generations are a separate product data channel. They are published by
the scheduled catalog workflow under payload-digest prerelease tags and are
never appended to an application release.

The next release should be `v0.1.0-rc.1`. Promote to `v0.1.0` only after the RC
installation, provenance, container, Homebrew, rollback, and upgrade drills are
green. Reserve `v1.0.0` for an explicit public compatibility commitment across
the Go API, CLI, configuration, and durable catalog formats.

## Operator commands

Prepare a local, non-publishing release snapshot:

```bash
GOTOOLCHAIN=go1.26.5 make release-snapshot
```

The local snapshot intentionally skips checksum signing and the container image;
those require release secrets and a registry-capable Docker environment and are
verified by the hosted tag workflow.

After the exact commit is merged and the RC publication is authorized:

```bash
make release-tag VERSION=0.1.0-rc.1
```

Pushing the tag is the publication action. Do not reuse or move release tags.
