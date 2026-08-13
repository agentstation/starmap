# CPO8.1 Starport v1.0.4 release proof

Date: 2026-08-13 UTC  
Release commit: [`1a6b006f`](https://github.com/agentstation/starport/commit/1a6b006f20c15c9affc93f3b6fe0f9951fccc0d0)  
Release: [Starport v1.0.4](https://github.com/agentstation/starport/releases/tag/v1.0.4)  
Release workflow: [run 31670989948](https://github.com/agentstation/starport/actions/runs/31670989948)

## Fail-before

Before this task, Starport v1.0.3 was the latest public release. No public Starport release consumed Starmap v0.5.0.

The annotated `v1.0.4` tag did not exist. Starport `main` was at the exact CPO8 merge commit, `1a6b006f20c15c9affc93f3b6fe0f9951fccc0d0`.

## Tag and hosted release

The annotated tag object is `af134867f25e2d2ce9a94c8f9702a97a137f8e1a`. It points to the exact current `main` commit, `1a6b006f20c15c9affc93f3b6fe0f9951fccc0d0`.

Release workflow run 31670989948 used that exact source commit. These required jobs passed:

- Release Gate.
- Assemble, Verify, and Publish.
- Verify Homebrew on macOS.
- Verify Homebrew on Ubuntu.

The workflow published release ID 369691346 at 2026-08-13T05:51:19Z. GitHub reports that the release is stable, published, and immutable. The publisher is `github-actions[bot]`.

## Public assets and provenance

The release has 13 public assets:

- one SHA-256 checksum manifest.
- six archives for Darwin, Linux, and Windows on AMD64 and ARM64.
- six SPDX 2.3 SBOMs, one for each archive.

Independent downloads produced the same SHA-256 digest as each GitHub asset record. All 12 entries in `checksums.txt` passed. GitHub artifact attestation verification passed for all 13 assets with these constraints:

- repository `agentstation/starport`.
- signer workflow `agentstation/starport/.github/workflows/release.yaml`.
- source commit `1a6b006f20c15c9affc93f3b6fe0f9951fccc0d0`.
- source reference `refs/tags/v1.0.4`.
- no self-hosted runner.

The SBOM generator was Syft 1.51.0. The Darwin SBOMs contain 105 packages and 312 relationships. The Linux SBOMs contain 104 packages and 309 relationships. The Windows SBOMs contain 105 packages and 311 relationships.

## Portable binary verification

The repository release verifier passed for all six public binaries. Each binary has these properties:

- reports exact version 1.0.4.
- uses the expected operating system and architecture.
- has CGO disabled.
- has clean VCS metadata.
- records exact source commit `1a6b006f20c15c9affc93f3b6fe0f9951fccc0d0`.
- records `github.com/agentstation/starmap` v0.5.0 in `go version -m` output.
- satisfies the platform linkage policy.

| Target | Binary size in bytes |
|---|---:|
| Darwin ARM64 | 54,808,402 |
| Darwin AMD64 | 57,410,880 |
| Linux ARM64 | 53,215,394 |
| Linux AMD64 | 56,131,746 |
| Windows ARM64 | 53,783,040 |
| Windows AMD64 | 57,404,416 |

## Container verification

The stable container tags `1.0.4`, `v1.0.4`, and `latest` resolve to one digest:

`sha256:ae0108deefef0f8bf0da925b3606634f77ed6b9d66a63edbfd8cf36a85a2a6f4`

The manifest contains Linux AMD64 and ARM64 images and their attestation manifests. A digest-pinned container reported Starport 1.0.4 and user `65532:65532`. It also reported exact revision `1a6b006f20c15c9affc93f3b6fe0f9951fccc0d0` and version 1.0.4. GitHub provenance verification passed with the exact release workflow, tag reference, and source commit.

## Homebrew verification

The AgentStation Homebrew tap uses `main`. Tap commit `78787cca705f0f4efc15b746b14b55d7972f6b9c` updates the Starport cask to v1.0.4. Its four platform hashes match the exact public archives.

The repository cask verifier and strict cask audit passed. The macOS quarantine hook applies only to `#{staged_path}/starport`, does not use `sudo`, and runs only when macOS and `xattr` are present.

Hosted installations passed on macOS and Ubuntu. Both installations reported exact version 1.0.4 and included the binary, manual page, and Bash, Zsh, and Fish completions. The macOS job also confirmed that the installed executable did not have the quarantine attribute.

The Homebrew runner reported unrelated warnings about an untrusted AWS tap. Homebrew trusted and installed the AgentStation cask. This warning does not affect the Starport result.

## Result

CPO8.1 passes. Starport v1.0.4 is immutable and independently verified. Its public binaries consume Starmap v0.5.0. CPO-V14 passes against the exact published Starmap and Starport releases.
