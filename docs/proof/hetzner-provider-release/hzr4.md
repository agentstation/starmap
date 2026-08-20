# HZR4 Starport release proof

Pull request: [agentstation/starport#116](https://github.com/agentstation/starport/pull/116).

Release: [Starport v1.0.5](https://github.com/agentstation/starport/releases/tag/v1.0.5).

## Source identity

- PR 116 passed all 10 protected checks and merged as
  `60f26a6d748b7667a2ed586e38d8a70cf9c161a0`.
- The annotated `v1.0.5` tag resolves to that exact commit.
- GitHub reports the release as public, non-prerelease, and immutable.

## Release verification

- Release workflow run
  [32321031784](https://github.com/agentstation/starport/actions/runs/32321031784)
  passed.
- The exact-tag release gate passed lint, the repository release gates, the
  published Starmap source check, and the GoReleaser configuration check.
- The release publishes 13 checksummed archives, SBOMs, and checksum records
  for macOS, Linux, and Windows on Arm64 and x86-64.
- Portable archive verification, container build, and publication passed.
- Archive and container attestations passed. Immutable release publication,
  container tag promotion, and publisher identity readback also passed.
- The generated Homebrew cask published successfully.
- Independent Homebrew installation and developer-artifact checks passed on
  macOS and Ubuntu.

The macOS Homebrew job emitted an unrelated local trust warning for `aws/tap`.
The AgentStation install, version, developer-artifact, and macOS metadata
checks passed.
