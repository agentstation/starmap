# CPO8 Starport migration proof

Date: 2026-08-13 UTC  
Starport candidate: [`f53e49ea`](https://github.com/agentstation/starport/commit/f53e49ea234272fde72ade8738aefdb884b8472a)  
Pull request: [agentstation/starport#112](https://github.com/agentstation/starport/pull/112)  
Merge commit: [`1a6b006f`](https://github.com/agentstation/starport/commit/1a6b006f20c15c9affc93f3b6fe0f9951fccc0d0)

## Fail-before

The Starport baseline was `main@4fbde1eaf3a624a57112a54a872ed2e15de139dd`. Its `go.mod` required Starmap v0.4.1.

CPO8 changed only the Starmap requirement and sums to v0.5.0. It then ran the focused catalog, application, and diagnosis packages. The focused tests failed to compile.

The compiler reported that `github.com/agentstation/starmap/pkg/catalogstore` does not exist in v0.5.0. This result proved the required direct migration before a production source edit.

The pre-change `go.mod` SHA-256 digest was `947fcff25b038488dddfd7c16ffc2bf9790e5d9582da4ba2a2ebf698b619591f`. The pre-change `go.sum` SHA-256 digest was `a65d8e5dc26fb4ce76ec474af15fd9f67d70718bba4dac869c21a5b0c9157054`.

## Migration result

Starport now requires exact Starmap v0.5.0 without a `replace` directive. The migration made these direct changes:

- `catalogstore.Generation` moved to `catalogs.Generation`.
- the storage interface moved to `catalogs/storage.Store`.
- payload codecs moved to `catalogs`.
- catalog evidence moved to `catalogs/evidence`.
- the remote wire protocol moved to `catalogs/remote` and keeps the local `protocol` alias at that naming boundary.
- the v1 architecture verifier now rejects imports from all four removed Starmap package paths.

The change did not add a compatibility wrapper, schema change, provider behavior, or dependency other than the approved Starmap version update.

## Published-module verification

`go mod verify` passed. Starport resolved Starmap v0.5.0 from `/Users/jack/go/pkg/mod/github.com/agentstation/starmap@v0.5.0`. The module sum is `h1:CqGc7VqTjKwldypbhmrAC5i2vmyL8qEor24sTJDzE4k=`.

Starport's ownership verifier ran against that exact directory and reported `Summary: 12 passed, 0 failed`. The Starmap structural verifier reported 12 passing conditions. Its only direct failure was CPO-V10 because Go module archives exclude nested modules, including the six verifier-owned consumer fixtures. CPO8 reproduced CPO-V10 from the exact v0.5.0 Git tag. It removed each fixture's local replacement, pinned v0.5.0, and passed all six consumer suites:

- pinned artifact.
- read only.
- remote subscriber.
- server embed.
- server storage.
- store only.

This split proves every structural condition against the published module without treating intentionally omitted nested fixture modules as shipped module content.

## Consumer disposition

Modelwiki was clean at `122d27007678eeea32e927678f853db48a3c30b5`. It requires Starmap v0.0.25 and imports only surviving `pkg/catalogs` and `pkg/sources` paths. It does not import a package removed by v0.5.0, so this task made no Modelwiki change.

Starport pull request #86 was already closed. Current `main` already used `Homebrew/actions/setup-homebrew` 2026.08.10.1 in both CI and release workflows. CPO8 therefore recorded #86 as superseded and made no duplicate action update.

## Local verification

| Check | Result |
|---|---|
| Focused catalog, application, and diagnosis tests | PASS after migration. |
| `bash scripts/verify-v1-architecture.sh` | PASS. Summary: 12 passed and 0 failed. |
| `go test ./...` | PASS. The architecture verifier also repeated this full suite. |
| `go test -race ./...` | PASS with normal Go scheduling and no scheduler cap. |
| `go vet ./...` | PASS. |
| `make lint` | PASS with zero issues. |
| `make build` | PASS. |
| `bash scripts/smoke-openrouter-sdks.sh` | PASS for raw HTTP and the Python, TypeScript, and Go OpenRouter SDKs. |
| `make release-check` | PASS with repository-pinned GoReleaser 2.17.1. |
| `make release-snapshot` | PASS at exact commit `f53e49ea` in a clean normal clone with GoReleaser 2.17.1 and checksum-verified Syft 1.51.0. The linked worktree was not used because Go does not stamp native VCS metadata when `.git` is a worktree pointer file. |
| Release snapshot artifacts | PASS for six clean-VCS cgo-disabled binaries, six archives, six SPDX SBOMs, checksums, the Homebrew cask contract, and strict cask audit. |
| Pre-PR autoreview | PASS. The credential scan was clean, and the isolated review reported no accepted or actionable findings. |

## Hosted verification and merge

Pull request #112 ran ten hosted checks. Action Pin Provenance, Build, Lint, OpenRouter SDK Compatibility, Release Contract, Release Snapshot, Security Scan, and the Ubuntu, macOS, and Windows test jobs all passed. GitHub merged the pull request to `main` at `1a6b006f20c15c9affc93f3b6fe0f9951fccc0d0`.

CPO8 passes. CPO8.1 now owns the immutable Starport v1.0.4 release and independent release verification.
