# CAT2.1 native Go verifier selection

Date: 2026-09-02. Owner task: CAT2.1. This record supplies the evidence that
CAT-D6 needs. It names the engine, the pinned version, the module cost, the
license review, the advisory review, and the GitHub CLI parity result.

Selected engine: `github.com/sigstore/sigstore-go`

Pinned version: `v1.2.1`, released 2026-06-09. See
<https://github.com/sigstore/sigstore-go/releases/tag/v1.2.1>.

## Why this engine

Sigstore publishes `sigstore-go` as the reference Go client for bundle
verification. It reads the same bundle media type that
`actions/attest-build-provenance` writes. It also reads the same trusted-root
document that `gh attestation trusted-root` prints. GitHub CLI itself verifies
attestations through this library. Starmap therefore shares one verification
engine with the tool that the catalog-generation workflow already runs.

The alternatives fail on evidence, not on taste.

- `sigstore/cosign` carries a large command surface and a much larger graph.
- `slsa-framework/slsa-verifier` binds the policy shape to its own command.
- A handwritten verifier would own certificate, transparency, and timestamp
  rules that a specialist library already owns.

## Version choice and the Go floor

The current latest release is `v1.3.0`, published 2026-07-30. Starmap pins
`v1.2.1` instead. The reason is the declared Go floor.

| Release | Declared `go` directive |
| --- | --- |
| v1.1.0 | 1.24.0 |
| v1.2.0 | 1.25.0 |
| v1.2.1 | 1.25.0 |
| v1.2.2 | 1.25.8 |
| v1.3.0 | 1.25.8 |

Go raises the main module directive to the highest directive in the build
list. The v1.3.0 release therefore moves the Starmap floor from 1.25.0 to
1.25.8. That move breaks two current repository contracts. The file CLAUDE.md
states a 1.25.0 language floor. A test in `internal/ciworkflow` also asserts
the exact string 1.25.0. That test is
`TestPullRequestWorkflowPinsToolchainActionsToolsAndRequiredJobs`.

`v1.2.1` keeps the floor at 1.25.0 and carries every published advisory fix.
The orchestrator owns the later decision to raise the floor and adopt
`v1.3.0`.

One indirect requirement needs an explicit note.
`github.com/sigstore/rekor-tiles/v2 v2.3.0` declares `go 1.25.8`. Starmap
records the version that `sigstore-go v1.2.1` itself requires,
`v2.2.2-0.20260601073857-5d098a2b6443`. That version declares no directive
above 1.25.0. No module in the resolved build list now declares more than
1.25.0.

## Published advisories

The GitHub advisory database lists three advisories for the engine. The
pinned release clears all three.

| Advisory | CVE | Severity | Affected | Patched |
| --- | --- | --- | --- | --- |
| [GHSA-wqqc-jjcq-vfxm](https://github.com/advisories/GHSA-wqqc-jjcq-vfxm) | CVE-2026-54787 | low | <= v1.2.0 | v1.2.1 |
| [GHSA-9vcr-p3rj-q5q6](https://github.com/advisories/GHSA-9vcr-p3rj-q5q6) | CVE-2026-49834 | medium | <= v1.1.4 | v1.2.0 |
| [GHSA-cq38-jh5f-37mq](https://github.com/advisories/GHSA-cq38-jh5f-37mq) | CVE-2024-45395 | low | <= 0.6.0 | 0.6.1 |

Source: <https://api.github.com/repos/sigstore/sigstore-go/security-advisories>,
read 2026-09-02.

## Transport choice

Starmap keeps the standard library HTTP client and adds no GitHub SDK.

`google/go-github` v90 would add only two modules. The cost is small. The
benefit is also small, because CAT4 calls two public endpoints. One endpoint
lists attestations for a digest. One endpoint downloads a release asset.

CAT-D12, CAT-D15, and CAT-D16 need a Starmap-owned transport. That transport
must bound seven values:

- the connect time, the TLS handshake, and the header wait
- the transfer inactivity time
- the byte count, the page count, and the record count

It must also read rate-limit headers and send conditional requests. An SDK
client would sit between Starmap and those bounds without removing any of
them.

The public attestation endpoint needs no credential. A plain read of
`https://api.github.com/repos/agentstation/starmap/attestations/sha256:<digest>`
returned HTTP 200 and 15564 bytes with no token.

## Package shape

The spike lives at `internal/attestation`, where CAT4 will own it. The public
surface stays small.

```go
type Policy struct {
    Repository            string
    Workflow              string
    Issuer                string
    PredicateType         string
    TrustedRootJSON       []byte
    DenySelfHostedRunners bool
}

type Result struct {
    PredicateType          string
    SignerIdentity         string
    SourceRepositoryURI    string
    SourceRepositoryDigest string
    RunnerEnvironment      string
    ObservedAt             time.Time
}

func Verify(ctx context.Context, bundleJSON []byte, artifactDigest string, policy Policy) (Result, error)
```

The split is deliberate. The engine checks the cryptography, the certificate
chain, the transparency evidence, the observer timestamps, and the artifact
digest. The package checks the Starmap trust policy on the verified
certificate and statement.

`Verify` returns typed errors. It returns `*errors.ValidationError` for a bad
argument. It returns `*errors.ParseError` for a document it cannot decode. It
returns `*attestation.TrustError` for evidence that fails the policy.

## Parser and size bounds

`Verify` bounds both documents before it decodes them.

- One bundle: 1 MiB. A real GitHub provenance bundle is about 11 KiB.
- One trusted root: 4 MiB. The public-good root is about 6 KiB, and the
  GitHub root is about 29 KiB.
- One artifact digest: exactly 64 hexadecimal characters, SHA-256 only.
- Evidence threshold: at least one signed certificate timestamp, one
  transparency log entry, and one observer timestamp.

`Verify` also checks the context before it starts work.

## TUF trust root

The engine reads a trusted root from JSON bytes that the caller supplies.
`Verify` never fetches a root and never reaches the network. A connected
caller refreshes the root through TUF and passes the bytes here. That keeps
the TUF refresh, its schedule, and its failure policy with CAT4.

The tests stay offline because the trusted root is a checked-in fixture. The
fixture is line 0 of `gh attestation trusted-root`. It holds two transparency
logs, two certificate authorities, two certificate transparency logs, and one
timestamp authority. A run under a blocked proxy still passes:

```
HTTPS_PROXY=127.0.0.1:1 HTTP_PROXY=127.0.0.1:1 ALL_PROXY=127.0.0.1:1 \
  go test ./internal/attestation/ -count=1
ok  github.com/agentstation/starmap/internal/attestation  0.279s
```

## Fixtures

Every fixture comes from one public catalog prerelease,
`catalog-semantic-f03df976d3164471b47fe874e23b4b45a13e2dc4d7dc2e83edfe55b43a353dc4`.

| File | Source | Bytes |
| --- | --- | --- |
| `gh-attestation-download.jsonl` | `gh attestation download starmap-catalog.tar.gz --repo agentstation/starmap` | 14501 |
| `catalog-provenance-bundle.json` | the SLSA provenance line of that capture | 10988 |
| `sigstore-public-good-trusted-root.json` | line 0 of `gh attestation trusted-root` | 5748 |

GitHub returns two attestations for this artifact. One is a release
attestation with the predicate type
`https://in-toto.io/attestation/release/v0.2`. It carries a timestamp
authority signature and no transparency log entry. The other is the build
provenance with the predicate type `https://slsa.dev/provenance/v1`. The spike
verifies one bundle, and the fixture is the build provenance.
`TestBundleFixtureMatchesGitHubCLICapture` proves that the fixture is an
unchanged line of the raw capture. CAT4 owns the choice among several returned
attestations.

## GitHub CLI parity

Both tools verified the same public artifact under the same repository and
workflow identity.

Command, run with `gh` version 2.98.0:

```
gh attestation verify starmap-catalog.tar.gz \
  --repo agentstation/starmap \
  --signer-workflow agentstation/starmap/.github/workflows/catalog-generation.yaml \
  --deny-self-hosted-runners
```

Exit status 0. The JSON result reports one matching attestation.

| Fact | GitHub CLI result | Starmap `Result` |
| --- | --- | --- |
| Subject alternative name | the catalog-generation workflow at `refs/heads/main` | same |
| Issuer | `https://token.actions.githubusercontent.com` | same |
| Source repository URI | `https://github.com/agentstation/starmap` | same |
| Source repository digest | `ab84e57d5fceb907c57994db9a8a6a860d58a6d3` | same |
| Runner environment | `github-hosted` | same |
| Predicate type | `https://slsa.dev/provenance/v1` | same |
| Artifact digest | `92f1fb8b...d69b8adc` | same |

The Starmap policy is stricter in two places. GitHub CLI matched the issuer
with the regular expression `.*`, and Starmap requires the exact GitHub OIDC
issuer. GitHub CLI matched the signer prefix, and Starmap anchors the pattern
through `@refs/` to the end of the name.

## Dependency cost

| Measure | Before | After | Delta |
| --- | --- | --- | --- |
| Modules in `go list -m all` | 298 | 518 | +220 |
| Lines in `go.sum` | 280 | 471 | +191 |
| `cmd/starmap` bytes, package unlinked | 56425026 | 56425026 | 0 |
| `cmd/starmap` bytes, package linked | 56425026 | 60884930 | +4459904 |

The unlinked row is the state of this branch. Nothing imports
`internal/attestation` yet, so the shipped binary does not change. The linked
row measures the real future cost. It adds 4.25 MiB, or 7.90 percent.

The verifier build graph holds 72 modules. 51 of them are new to Starmap.

## License review

Every one of the 51 newly linked modules carries a permissive license. No
copyleft license entered the graph.

| License | Modules |
| --- | --- |
| Apache-2.0 | 41 |
| MIT | 5 |
| BSD-3-Clause | 3 |
| BSD-2-Clause | 2 |

The two BSD-2-Clause modules are `github.com/digitorus/timestamp` and
`github.com/pkg/errors`. The reviewer read both license files directly.

## govulncheck

| Scope | Before | After |
| --- | --- | --- |
| Called by Starmap code | 0 | 0 |
| In imported packages | 0 | 2 |
| In required modules | 3 | 1 |

The advisory set does not grow. The same three advisories appear before and
after, and all three sit in `golang.org/x/crypto@v0.55.0`. The engine imports
`x/crypto`, so two advisories move from the required class to the imported
class. Starmap code calls none of them.

The release `golang.org/x/crypto@v0.56.0` fixes GO-2026-6355 and GO-2026-6354.
Starmap cannot take that release yet, because `x/crypto@v0.56.0` declares
`go 1.26.0` and would break the Go 1.25 floor. GO-2026-5932 has no fix.

## Verification commands

Every command ran from the worktree root with `GOTOOLCHAIN=go1.26.6`, except
the Go 1.25 checks.

| Command | Result |
| --- | --- |
| `go build ./...` | exit 0 |
| `go test ./internal/attestation/ -race -count=1` | ok, 16 test results, 0 failures |
| `make lint` | golangci-lint 0 issues, ago 0 findings, prose 0 diagnostics |
| `make test` | 2286 pass lines, 1 pre-existing failure |
| `govulncheck ./...` | 0 called |
| `GOTOOLCHAIN=go1.25.12 go build ./...` | exit 0 |
| `GOTOOLCHAIN=go1.25.12 go vet ./internal/attestation/...` | exit 0 |
| `GOTOOLCHAIN=go1.25.12 go test ./internal/attestation/ -race` | ok |
| `GOTOOLCHAIN=go1.25.0 go build ./internal/attestation/` | exit 0 |

The one `make test` failure is
`pkg/catalogs/artifact.TestScheduledGenerationWorkflowPublishesOnlyValidatedChangedPayload`.
It wants the cron string `17 3 * * *`, and
`.github/workflows/catalog-generation.yaml` holds `17 */4 * * *`. CAT2.1
changed no workflow and no test, so this failure predates the branch. CAT-D10
chose the four-hour schedule, so the test text is the stale side.

## Decisions the orchestrator must review

1. Pin `v1.2.1` now, or raise the Go floor to 1.25.8 and pin `v1.3.0`.
2. Accept the standard library transport instead of `google/go-github`.
3. Accept 4.25 MiB of binary growth once CAT4 links the package.
4. Accept the pinned `rekor-tiles` pseudo-version that holds the Go floor.
5. Decide who fixes the stale cron assertion in `pkg/catalogs/artifact`.
