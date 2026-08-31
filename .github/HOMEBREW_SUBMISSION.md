# Homebrew Distribution

The release workflow publishes Starmap as a versioned Homebrew cask in the public
[`agentstation/homebrew-tap`](https://github.com/agentstation/homebrew-tap)
repository. Users install it with:

```bash
brew install agentstation/tap/starmap
```

GoReleaser updates `Casks/starmap.rb` only after a stable tag's tests pass. It
uses `skip_upload: auto`, so a release candidate cannot replace the stable cask.
The release workflow then installs that cask on a fresh macOS runner. It verifies
that the CLI reports the tagged version and records `CGO_ENABLED=0`. It also checks
that the CLI links only Apple system ABI libraries under `/usr/lib` and `/System/Library`.

The tap therefore installs the same portable pure-Go archive verified before
release publication. The tap is the supported Homebrew channel. Publishing to
the central `homebrew-cask` repository is not part of the release gate.

The cask currently removes macOS quarantine because the binary is not yet
notarized. Before declaring a stable 1.0 release, replace that fallback with
Developer ID signing and Apple notarization.
