# CSP0.2: Current-release first use

Status: native archive evidence is complete. Draft Starport PR #366 contains the README changes. E02 remains UNVERIFIED pending registration and complete advertised-method review.

The README now shows catalog access before provider setup. It separates the temporary development server from persistent storage. It links production limits and corrects the controller-only latency claim.

## Evidence

- The [published release record](csp0.2/release.json) identifies Starport v1.2.0 and its assets.
- The [native macOS result](csp0.2/native-darwin-arm64.json) verifies the archive checksum, version, and two keyless catalog commands.
- The [release workflow record](csp0.2/release-run.json) reports successful Linux and macOS Homebrew jobs. Cross-platform archive inspection does not prove native Windows execution.
- The [writing check](csp0.2/writing-current.json) passed for all three Starport prose files.
- The [first checks](csp0.2/first-checks.json) record the quickstart and documentation link results.
- The [authorization record](csp0.2/authorization.json) permits a short real-provider request and credential discovery.
- The [credential discovery result](csp0.2/credential-discovery.json) records one distinct OpenAI key across the two repository environment files. The evidence contains no secret values.

## Real-provider verification

Command:

```sh
python3 docs/plans/proof/starport-production-catalog/csp0.2/verify_current_release.py \
  --authorized-provider-env-file /Users/jack/src/github.com/agentstation/starport/.env
```

The [first attempt](csp0.2/real-inference-billing-refusal.json) reached the provider but received a billing refusal. Both repository environment files held that same key.

The user supplied a different OpenAI credential. The [successful attempt](csp0.2/real-inference.json) used the verified v1.2.0 archive and returned HTTP 200. Its stream contained “Hello! How can I assist you today?” and the terminal `[DONE]` event. The request used `max_tokens: 32` and completed in about 1.925 seconds, including provider time.

The isolated process stopped with exit code 0. Its temporary home had no remaining files. The second attempt completed successfully. Its temporary credential file no longer exists. No provider or gateway key value appears in the retained evidence.

## Native CI preparation

The user proposed GitHub Actions because no local Windows machine is available. The local workflow covers six native runners:

| Archive | Runner |
|---|---|
| Windows x64 | `windows-2025` |
| Windows ARM64 | `windows-11-arm` |
| Linux x64 | `ubuntu-24.04` |
| Linux ARM64 | `ubuntu-24.04-arm` |
| macOS Intel | `macos-15-intel` |
| macOS ARM64 | `macos-15` |

The [GitHub runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners) lists these native platforms. Each job downloads the selected published release. It verifies both the checksum file and GitHub asset digest. It then checks archive members, version, two catalog commands, readiness, authenticated catalog access, admin refusal, clean shutdown, and temporary home cleanup.

The child process receives no provider key or GitHub token. Each job retains a JSON result with artifact hashes and workflow identity. It does not retain the generated gateway key or raw server logs. These jobs test published archives. They do not qualify service installation, persistent storage, migration, or full Windows product support.

The [preparation record](csp0.2/native-ci-preparation.json) records eight passing verifier tests, a passing workflow syntax check, and 16 verified action pins. The [local rehearsal](csp0.2/native-ci-local-darwin-arm64.json) passed with the actual v1.2.0 macOS ARM64 archive. It returned 511 models, refused unauthenticated administration, and stopped without persistent user files.

The three local Starport files are `.github/workflows/native-release.yaml`, `scripts/verify-native-release.py`, and `scripts/test-native-release.py`. A pull request triggers the six jobs. Manual dispatch becomes available after the workflow reaches the default branch. Commit `481e71f` records those files. [Draft PR #366](https://github.com/agentstation/starport/pull/366) publishes them with the current main integration. The owner authorized reviewed commits, task-branch pushes, draft PRs, and native CI on 2026-09-06. Merges and releases remain outside that authority.

## Remaining work

Real inference passes for the tested macOS ARM64 archive. [Hosted run 34084336162](https://github.com/agentstation/starport/actions/runs/34084336162) passes all six native archive jobs. The [record review](csp0.2/native-run-34084336162-review.json) verifies the platform roster, required results, and GitHub workflow identity. Register E02 and verify every advertised installation method before closing CSP0.2. CSP3 remains the active task.


## Additional installation evidence

The [source-build check](csp0.2/source-install.json) passed from a fresh public clone at commit `25c1196bcaa4f957b027be4e9f5baa59a6c4b05d`.
It ran the README build and version commands, followed by both keyless catalog commands.
The temporary user home contained no files after those reads.
This macOS check used Go 1.26.6 and the installed console build tools.

The [container check](csp0.2/container-install.json) used release `v1.2.0`.
The README pull, attestation, and version commands passed.
Both catalog commands passed with container networking disabled on native Linux ARM64.
The image digest is `sha256:543e4ac8df312339c4f24371be89e51bd5b5014d492b6831681963fc1e3a470a`.
These checks do not qualify the Compose initialization recipe or persistent recovery.
E02 registration and the complete installation-method review remain open.
