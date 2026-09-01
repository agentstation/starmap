# CAT2 catalog distribution review

Date: 2026-09-01

Baseline: `codex/catalog-publisher-six-hour` at `7247a002`

## Publication naming

The live repository contains 28 `catalog-semantic-*` prereleases and 17 older
`catalog-payload-*` prereleases. It contains no `catalog-latest` release.
The current workflow creates `catalog-semantic-<digest>` tags and titles each
release `Catalog <generation_id>`.

The normalized facts digest is the catalog's public content identity. The exact
payload checksum remains the evidence-bearing byte identity. The public names
should use `catalog`, while manifests can retain the precise checksum fields.

The target names are:

- immutable release tag: `catalog-<catalog-digest>`
- stable channel: `catalog-latest`
- immutable release title: `Catalog <generation-id>`
- channel asset: `catalog-latest.json`
- public configuration concept: `catalog source`
- generation time: `generated_at`
- channel publication time: `published_at`

The workflow and GoReleaser must recognize all three historical namespaces.
Existing releases remain immutable and readable. New publication uses only the
canonical namespace.

## Current Starmap flow

The root `starmap.Client` loads an explicit durable generation, an optional
human catalog workspace, or the embedded bootstrap. It owns no network source,
scheduler, or shutdown lifecycle. A mutation requires an explicit catalog
store.

`acquisition.Syncer.ImportRelease` verifies caller-supplied release bytes and a
caller-supplied publisher verifier. It reconciles the release as a low-authority
observation below the human catalog workspace. No production GitHub downloader
or GitHub attestation verifier exists.

The public `remote.Subscriber` follows one Starmap HTTP API. It fetches a
manifest and immutable payload, follows SSE publication hints, stores verified
generations, and retains last-known-good state. It is opt-in and exact-activates
the received generation.

The CLI configuration has `RemoteServerURL`, `RemoteServerAPIKey`, and
`RemoteServerOnly` fields. The application rejects a configured remote URL
because it does not compose the subscriber. The standalone server and Docker
image start from durable or embedded state and do not follow an upstream source.

## Current Starport flow

Starport 0.15.0 already has two catalog runtimes. The local runtime owns Starmap
acquisition. The remote runtime follows a configured Starmap server through
`remote.Subscriber`.

The remote runtime keeps separate remote and accepted generation stores.
Starport builds and validates a complete routable candidate before it moves the
accepted pointer. A failure retains the current runtime and cache identity.

Remote mode is opt-in through `STARPORT_CATALOG_REMOTE_URL`. It is mutually
exclusive with the local workspace and local acquisition schedule. The tested
acceptance transaction should remain the Starport integration boundary.

## Target flow

The public `catalog-latest` channel points to one immutable
`catalog-<catalog-digest>` release. A consumer verifies the channel document,
the immutable release attestation, the archive checksum, the detached statement,
schema compatibility, and monotonic generation order before activation.

A connected Starmap runtime uses the public GitHub channel by default. A
configured catalog source replaces that source. A Starmap server can therefore
follow GitHub or another Starmap server and publish its accepted generation to
downstream consumers.

Starport instances use the same catalog-source policy. A default installation
can follow the public channel. An enterprise deployment points every Starport
instance at its Starmap server. The Starmap server becomes the only process that
needs public GitHub egress.

## Trust and availability

GitHub exposes public repository attestation bundles by subject digest without
authentication. `sigstore-go` supplies a production Go verifier for Sigstore
bundles. A runtime verifier must bind the exact archive bytes to the
`agentstation/starmap` repository and catalog-generation workflow identity.

The mutable channel has discovery authority only. It cannot replace immutable
digest and publisher verification. A missing or invalid channel retains the
last-known-good generation. A first run can use the embedded bootstrap while
readiness reports source degradation.

Direct GitHub consumers should use conditional requests, a bounded timeout, and
jittered polling. A configured enterprise source should not fall back to public
GitHub unless the operator explicitly configures that fallback.

## Native Go verifier review

The verifier has two separate responsibilities:

1. A Sigstore engine validates the bundle, signature, trust root, transparency
   evidence, timestamp, and artifact digest.
2. A Starmap GitHub policy binds the verified certificate and statement to the
   expected repository, workflow, OIDC issuer, predicate, and artifact.

The reviewed implementations do not replace both responsibilities:

| Candidate | Useful role | Reason not to import as the complete verifier |
| --- | --- | --- |
| `sigstore/sigstore-go` 1.3.0 | Native verification engine and TUF trust roots | It does not define Starmap's GitHub repository and workflow policy. |
| `cli/cli` | Canonical GitHub behavior and parity oracle | Its implementation is under command packages and includes CLI policy and dependencies. |
| `github/artifact-attestations-opa-provider` 0.4.0 | GitHub trust-root and multi-issuer reference | Its public verifier targets OCI, disables identity checks in the verifier, and requires Go 1.26. |
| `google/go-github` 90.0.0 | Typed GitHub attestation API transport and example | Its verifier is explicitly a non-production example and leaves production policy to the caller. |
| `DataDog/go-attestations-verifier` 0.1.2 | Maintained registry-consumer reference | Its public API targets NPM, PyPI, and RubyGems and also delegates cryptography to `sigstore-go`. |
| `sigstore/cosign` 3.1.3 | Canonical Sigstore CLI and interoperability oracle | Its API and dependency graph focus on container images and signing workflows. |
| `slsa-framework/slsa-verifier` 2.7.1 | SLSA provenance policy reference | It carries Cosign and an older Sigstore stack for a broader CLI contract. |
| Packer's internal verifier | Production policy and failure-handling reference | It is an internal package and cannot be a supported dependency. |

The provisional selection uses `sigstore-go` directly. A small Starmap adapter
owns the GitHub policy and transport. `cli/cli` is the behavioral oracle, not a
library dependency. `go-github` remains an API transport candidate. A small
standard-library client can cost less for this bounded endpoint contract.

The dependency spike must record module and binary-size deltas, Go 1.25
compatibility, licenses, advisories, `govulncheck`, parser bounds, and TUF update
behavior. It must verify the same public fixture with Starmap and GitHub CLI.

At review time, GitHub CLI and the Datadog verifier use `sigstore-go` 1.3.0.
This version includes fixes for all three published repository advisories. The
implementation must still pin and scan the selected dependency graph.

### Primary references

- [GitHub CLI attestation verification](https://github.com/cli/cli/blob/d528f20f2ee02f6703773e9f56c90e3c3f5d46b0/pkg/cmd/attestation/verification/sigstore.go)
- [GitHub CLI verification policy](https://cli.github.com/manual/gh_attestation_verify)
- [`sigstore-go` 1.3.0](https://github.com/sigstore/sigstore-go/tree/v1.3.0)
- [GitHub OPA verifier 0.4.0](https://github.com/github/artifact-attestations-opa-provider/blob/v0.4.0/pkg/verifier/verifier.go)
- [`go-github` verifier example](https://github.com/google/go-github/blob/v90.0.0/example/verifyartifact/main.go)
- [Datadog verifier 0.1.2](https://github.com/DataDog/go-attestations-verifier/tree/v0.1.2)
- [Cosign 3.1.3](https://github.com/sigstore/cosign/tree/v3.1.3)
- [SLSA verifier 2.7.1](https://github.com/slsa-framework/slsa-verifier/tree/v2.7.1)
- [Packer attestation verifier](https://github.com/hashicorp/packer/blob/v1.16.0/internal/attestation/verify.go)

## Open owner decisions

The [package and operator design](cat2-dx.md) replaces the earlier combined
source proposal. It asks the owner to approve these contracts:

1. Keep `starmap.New` offline and use `starmap.Open` for connected distribution.
2. Separate exact `follow` mode from explicit `author` acquisition mode.
3. Use the public source by default and use `none` as the network opt-out.
4. Replace the public source when an operator configures another source.
5. Prefer the source at startup and retain last-known-good state on failure.
6. Poll GitHub hourly with jitter and conditional requests.

The requested Starport configuration migration and dependency update are in
scope for this campaign.
