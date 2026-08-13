# CPO7 Starmap v0.5.0 release proof

Date: 2026-08-13 UTC  
Tag: [`v0.5.0`](https://github.com/agentstation/starmap/releases/tag/v0.5.0)  
Tag commit: [`64f23c41`](https://github.com/agentstation/starmap/commit/64f23c41ae989bf2f6b61b6ae9a9f41bd26b6d77)  
Release run: [`31666668460`](https://github.com/agentstation/starmap/actions/runs/31666668460)

## Fail-before

Before publication, the local and remote tag namespaces did not contain `v0.5.0`, and the GitHub release endpoint did not contain a v0.5.0 release. The release command ran only after `main` equaled `origin/main` at `64f23c41` and the worktree was clean.

The task did not record a separate direct pre-tag Go proxy request. The absent local tag, remote tag, and release establish that the new immutable version was not available. Post-release verification used a fresh empty module cache and the public Go proxy.

## Publication result

`make release-tag VERSION=0.5.0` created and pushed an annotated tag. The tag object `979c0026` points to exact `main` commit `64f23c41`. The Release workflow started from that SHA and completed successfully.

The public release is not a draft or prerelease. GitHub reports it as immutable. It contains exactly 14 assets:

- `checksums.txt` and its detached GPG signature.
- six Darwin, Linux, and Windows archives for amd64 and arm64.
- one SPDX SBOM for each archive.

The hosted test job proved tag ancestry, embedded catalog policy, full repository verification, and release readiness. The hosted release job built and verified the portable binaries. It emitted and verified attestations, checked the draft asset set, published the immutable release, and checked every public redownload. The hosted macOS Homebrew job installed the stable cask. It verified version 0.5.0, cgo-disabled build metadata, and system-only dynamic linkage.

## Independent verification

| Check | Result |
|---|---|
| Release metadata | PASS. Release ID `369672297` is immutable, public, stable, and published at `2026-08-13T04:46:49Z`. |
| Tag identity | PASS. Annotated tag object `979c0026` points to `64f23c41`, which equals current `origin/main`. |
| Asset set and digests | PASS. All 14 public asset SHA-256 digests match GitHub release metadata. The signed checksum manifest covers all 12 archives and SBOMs. |
| GPG signature | PASS. Repository key `D605A0BB6842D5737E68B7D787E93209AF2CD255` produced a good signature for `checksums.txt`. |
| Checksums | PASS. All 12 checksummed assets match. |
| SBOMs | PASS. All six files are SPDX 2.3 documents from Syft 1.46.0. Each names its exact archive and contains 109 through 111 packages plus 324 through 329 relationships. |
| GitHub provenance | PASS. All 12 checksummed assets verify against `agentstation/starmap/.github/workflows/release.yaml` with self-hosted runners denied. |
| Portable binaries | PASS after extraction from the public archives. The repository verifier found all six targets, recorded `CGO_ENABLED=0`, proved static Linux binaries, rejected Windows C or C++ runtime imports, and accepted only system Darwin dependencies. |
| Go proxy | PASS from a fresh empty module cache through `proxy.golang.org`. Module sum: `h1:CqGc7VqTjKwldypbhmrAC5i2vmyL8qEor24sTJDzE4k=`. Go module sum: `h1:PpHoL/VXQr1k6cfaaixJwou4DnAOYw9a+PCLwtaz86I=`. |
| Published package surface | PASS. The downloaded module contains `pkg/catalogs/evidence`, `projection`, `storage`, `artifact`, and `remote`. It does not contain `pkg/catalogmeta`, `catalogstore`, `catalogartifact`, or `catalogremote`. |
| Container | PASS. Tags `0.5.0`, `v0.5.0`, and `latest` resolve to OCI index `sha256:b979568de5a24b89cb7ff88e4644ac7ca3ec3030717e56735e6d09c51ba03cbe` with Linux amd64 and arm64 manifests. A digest-pinned pull ran `starmap version` and returned `0.5.0`. The local image runs as user `65532` and labels exact revision `64f23c41`. |
| Homebrew | PASS. The tap checkout uses branch `main`. Its cask is version 0.5.0, matches the public `main` cask byte for byte, and uses the release archive hashes. Online cask audit passed. A local install at `/opt/homebrew/bin/starmap` returned version 0.5.0, recorded cgo disabled, used system-only linkage, and had no quarantine attribute. |

The machine had three unmanaged completion files from an older non-cask install. Homebrew correctly rolled back its first partial install instead of overwriting them. CPO7 moved the files to `/tmp/starmap-pre-cask-completions.DIINjS`. The successful cask installed managed replacements. The older `/Users/jack/go/bin/starmap` binary remains unchanged and precedes Homebrew in this shell's `PATH`. Verification therefore used the exact Homebrew path.

CPO-V14 passes for the immutable Starmap v0.5.0 release. CPO8 now owns Starport's exact dependency migration and pull request.
