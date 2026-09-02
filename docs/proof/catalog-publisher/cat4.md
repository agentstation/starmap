# CAT4 proof: verified GitHub catalog source

CAT4 owns the public GitHub source adapter, the artifact verification glue, and
the catalog transfer policy. It also owns the pure pacing rules that CAT5 and
CAT7 wire into the runtime loop. The runtime loop, `starmap.Open`, the retained
layers, the lease, and the scheduler identity stay outside this record.

## Fail before

`docs/proof/catalog-publisher/cat2-fail-before.txt` holds the plan-wide capture
on the base commit. The seven CAT4 conditions read:

```text
FAIL CAT-V07 the GitHub source reads legacy catalog-semantic and catalog-payload releases.
FAIL CAT-V20 a stalled body stops at the two-minute inactivity timeout.
FAIL CAT-V21 the per-transfer maximum duration defaults to 60 minutes and rejects zero.
FAIL CAT-V23 catalog transfer clients set no client-wide total timeout.
FAIL CAT-V24 the catalog transport applies the response-header bound to every request.
FAIL CAT-V26 a slow-drip transfer fails at the per-transfer bound.
FAIL CAT-V37 the GitHub source reports the rate-limit budget from limit, used, remaining, and reset headers and warns at 80 percent.
```

## Mutation evidence

This record disables each rule below in turn. The named test then fails, and
the rule returns.

| Disabled rule | Failing test |
| --- | --- |
| The replay floor comparison in `ReadChannel` | `TestGitHubSourceRejectsReplayedChannelDocuments` |
| The channel document verification in `readChannelDocument` | `TestGitHubSourceRejectsUnattestedChannelDocuments` |
| The channel-recorded checksum and size checks | `TestGitHubSourceRejectsAssetsThatMissTheChannelRecord` |
| The 80 percent rate-limit warning threshold | `TestGitHubSourceReportsRateLimitBudget` |

## Packages

| Path | Role |
| --- | --- |
| `internal/sources/github` | The public GitHub source adapter |
| `internal/attestation/trustedroot.go` | The compiled Sigstore public-good trusted root |
| `internal/fleet` | The pure pacing rules that CAT5 and CAT7 wire |
| `pkg/catalogs/remote/transport.go` | The shared catalog transfer policy |
| `internal/transport/client.go` | The provider client that adopts that policy |

## Discovery and verification

`Source.ReadChannel` runs one refresh cycle. The cycle reads the `catalog-latest`
release, downloads `catalog-latest.json`, and verifies the document provenance.
It rejects a document whose sequence falls below the stored floor. It then reads
the immutable release that the document names.

Verification precedes activation at every step. The source checks the recorded
size and the recorded checksum of each named asset. Only then does a byte reach
the artifact reader. The `artifact.VerifyRelease` call owns the checksum file,
the statement, and the compatibility checks. The `provenanceVerifier` type owns
the publisher trust decision alone. It calls `internal/attestation.Verify`.

A failure at any step returns before the durable write. The stored state
therefore keeps the release that verification last accepted, and that release
stays the rollback target.

`Source.Changed` sends one conditional request with the stored `ETag`. It never
advances the durable validator, because only a complete verification may move
the replay floor.

## Trust policy

`Config.policy` builds the policy of every verification:

| Field | Value |
| --- | --- |
| Repository | `agentstation/starmap` by default |
| Workflow | `.github/workflows/catalog-generation.yaml` by default |
| Issuer | The GitHub OIDC issuer |
| Predicate type | The SLSA v1 build provenance type |
| Trusted root | The compiled Sigstore public-good root |
| Self-hosted runners | Denied |

`WithTrustedRoot` overrides the compiled root for a private deployment.

## Request budget

| Cycle | Requests |
| --- | --- |
| A cycle that promotes a release | 8 |
| A cycle that finds an unchanged channel | 1 |
| A cycle that stops at the channel document | 3 |

The eight requests are the channel release, the channel document, the channel
provenance, the immutable release, its three assets, and the archive provenance.
The `RateLimitBudget.Requests` field reports the count of the finished cycle.

## Rate-limit budget

`parseRateLimit` reads `x-ratelimit-limit`, `x-ratelimit-used`,
`x-ratelimit-remaining`, and `x-ratelimit-reset`. A missing `used` header falls
back to the difference between the limit and the remainder. The `Warn` method
reports true at or above 80 percent of the limit. The `retryBoundary` helper
prefers `Retry-After` over the reset time. It accepts both the seconds form and
the HTTP date form.

## Transfer policy

`pkg/catalogs/remote` owns the shared bounds:

| Bound | Default |
| --- | --- |
| Connect | 30 s |
| TLS handshake | 30 s |
| Response header | 60 s |
| Body inactivity | 2 min |
| Per-transfer maximum | 60 min, and zero is invalid |
| Compressed body | 64 MiB |
| Expanded body | 256 MiB |

`Transfer.Body` reads one bounded body and reports progress. It reports the
header stage, each body step, and the complete stage. Every report carries a
safe resource label, never a URL, a token, or a host.

## Fleet pacing rules

`internal/fleet` holds four pure rules. CAT5 and CAT7 wire them into the runtime
loop, and CAT-V30 through CAT-V36 belong to those tasks.

| Rule | Behavior |
| --- | --- |
| `StablePhase` | `hash(instance + controller + source) mod interval` with FNV-64a |
| `StartupOffset` | The stable phase inside the 15-minute cold spread |
| `RetryPolicy` | Decorrelated jitter from 1 s to 15 min, at most three retries per cycle |
| `NotBefore` | A hard boundary plus up to five minutes of jitter |

## Configuration names

Every setting arrives through an option. The package reads no environment
variable. The field names follow the canonical `CATALOG_SOURCE_*` suffixes:

| Option | Canonical name |
| --- | --- |
| `WithRepository` | `CATALOG_SOURCE_REPOSITORY` |
| `WithChannel` | `CATALOG_SOURCE_CHANNEL` |
| `WithSignerWorkflow` | `CATALOG_SOURCE_SIGNER_WORKFLOW` |
| `WithToken` | `CATALOG_SOURCE_TOKEN` |
| `WithAPIBaseURL` | `CATALOG_SOURCE_URL` |

## Errors

Every failure is a typed value from `pkg/errors`. A backwards sequence returns a
`*errors.ConflictError`. A refused reply returns a `*RefusalError` that carries
the hard not-before boundary and wraps an `*errors.APIError`. A trust failure
returns the wrapped `*attestation.TrustError`. Every reason code names the
operation, such as `release` or `asset`. No reason code carries a URL, a token,
or the host of a custom source.

## Fixtures

The tests run against an `httptest` server that holds every release in memory.
No test reaches a network. The package reuses the committed CAT2.1 evidence in
`internal/attestation/testdata`. The archive fixtures come from
`pkg/catalogs/testdata/generation/manifest.json` and an empty catalog payload.

## Tests

| Test | Purpose |
| --- | --- |
| `TestGitHubSourceVerifiesCatalogLatest` | The acceptance path, the policy, the budget, and the progress reports |
| `TestGitHubSourceReadsLegacyReleaseTags` | CAT-V07, both retired namespaces |
| `TestGitHubSourceReportsRateLimitBudget` | CAT-V37, the headers and the warning |
| `TestGitHubSourceRejectsReplayedChannelDocuments` | The replay floor and the kept rollback target |
| `TestGitHubSourceRejectsUnattestedChannelDocuments` | The channel document trust decision |
| `TestGitHubSourceRejectsTrustNegativeReleases` | The release trust decision |
| `TestGitHubSourceRejectsAssetsThatMissTheChannelRecord` | The recorded checksum and size |
| `TestGitHubSourceRejectsUnsignedEvidenceWithTheDefaultEngine` | The default engine rejects foreign evidence |
| `TestGitHubSourcePolicyAcceptsPublishedCatalogProvenance` | The real engine accepts the published bundle |
| `TestGitHubSourceRejectsUnusableConfiguration` | The configuration checks |
| `TestGitHubSourceDeclaresItsContract` | The `pkg/sources` contract |

## Decisions for the orchestrator

### The verification function type

`Config.Attester` names the verification function, and it defaults to
`internal/attestation.Verify`. A test may replace it with a recorder.

This exists because no test can prove a positive end-to-end result with the real
engine. The committed bundle attests the real published archive, whose digest is
`92f1fb8b…`. That archive is 393 KB and the repository does not carry it. A
synthetic archive cannot reach that digest.

Three tests cover the split. The acceptance test proves the glue with a
recorder. The test named
`TestGitHubSourcePolicyAcceptsPublishedCatalogProvenance` proves that the real
engine accepts the genuine bundle under this policy. The test named
`TestGitHubSourceRejectsUnsignedEvidenceWithTheDefaultEngine` proves that the
real engine rejects a bundle that binds another artifact.

### The client timeout change

`remote.NewClient` previously set a client-wide timeout of two minutes. CAT-V23
forbids a client-wide total timeout, because `http.Client.Timeout` also covers
the body read. The nil-client path now uses `remote.DefaultTransferClient`.
`TestClientDefaultHTTPTimeout` therefore became
`TestNewClientSetsNoClientWideTimeout`. The bounds still apply, and they now
live in the transport instead of the client.

### The new fleet package

The pacing rules live in `internal/fleet` rather than in the source adapter.
CAT5 and CAT7 need the same rules for the runtime loop and the subscriber.
Neither should import a source adapter to reach them.

### The transfer reply type

`Transfer.Body` returns a `remote.Reply` value rather than an
`*http.Response`. The transfer already read and closed the body, so no caller
owns a stream. The value type also removes a false `bodyclose` report.

## Follow-up

The compiled trusted root is a fixed capture of the Sigstore public-good root.
A later task should add a TUF refresh that updates the root on its own schedule.
That refresh will supply new bytes through `WithTrustedRoot`. A caller can
supply a refreshed engine through the `Attester` field.

## Gate results

| Gate | Result |
| --- | --- |
| `make lint` | PASS |
| `make test` | PASS, 68 packages |
| `go tool ago -stale-ignores -format json ./...` | PASS, no finding |
| `make technical-writing-check` | PASS, 701 files, 0 diagnostics |
| `bash scripts/verify-catalog-package-ownership.sh` | PASS, 13 of 13 |
| `make docs-check` | PASS |
| `shellcheck scripts/*.sh` | PASS |
| `bash scripts/verify-catalog-distribution.sh` | See below |

The distribution verifier reports:

```text
Summary: 16 passed, 33 failed, 19 unverified.
```

All seven CAT4 conditions pass. The remaining failures belong to CAT5, CAT6, and
CAT7. The unverified conditions need a Starport tree or a console toolchain.

The package tests also pass with the race detector and with `HTTPS_PROXY` set to
`127.0.0.1:1`, which proves that no test reaches a network.
