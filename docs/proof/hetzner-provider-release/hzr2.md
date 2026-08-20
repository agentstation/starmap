# HZR2 Starmap release proof

Pull request: [agentstation/starmap#92](https://github.com/agentstation/starmap/pull/92).

Release: [Starmap v0.6.0](https://github.com/agentstation/starmap/releases/tag/v0.6.0).

## Source identity

- The pull request merged as `45bcd37052a44087f24d24a7cced02a65ca48085`.
- The annotated `v0.6.0` tag resolves to that exact commit.
- `go list -m -json github.com/agentstation/starmap@v0.6.0` resolves to the
  same commit through the Go module service.

## Release verification

- Release workflow run
  [32317684483](https://github.com/agentstation/starmap/actions/runs/32317684483)
  passed.
- The full test and release-check job passed in 20 minutes and 51 seconds.
- GoReleaser published 14 cross-platform archives, SBOMs, checksums, and the
  detached checksum signature.
- Portable binary checks, provenance attestations, draft-asset checks, and
  downloaded-asset identity checks passed.
- GitHub reports the release as public, non-prerelease, and immutable.
- The independent Homebrew job installed the AgentStation tap release and
  verified the installed version.

The Homebrew job emitted an unrelated local trust warning for `aws/tap`. The
AgentStation installation and version checks passed.
