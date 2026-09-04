# Catalog Distribution Trust Model

No distribution channel is authoritative merely because bytes were reachable.
Every consumer verifies catalog schema, generation manifest, canonical payload,
SHA-256 descriptors, detached statement, and channel-specific publisher trust
before atomic activation. Failure retains the last-known-good generation.

| Channel | Trust root | Freshness and availability | Principal risks | Intended policy |
| --- | --- | --- | --- | --- |
| Embedded bootstrap | Installed Starmap binary signature/provenance plus embedded manifest checksum | Works offline and air-gapped; freshness stops at binary build time | Stale catalog, binary growth, compromised build supply chain | Startup fallback; enforce runtime/CI age and size budgets; replace with a verified committed generation when policy permits |
| GitHub Release assets | Expected `agentstation/starmap` repository and approved workflow identity, signed artifact attestation, archive/payload digests | Durable immutable public history, but depends on GitHub availability and outbound enterprise policy | Repository/workflow compromise, unavailable/blocked GitHub, mistaken asset replacement | Public immutable source of record; pin exact generation/digest; never use expiring Actions artifacts as runtime distribution |
| Starmap server, including `starmap.agentstation.ai` | TLS origin plus exact manifest/payload digests | Reactive current-generation discovery with immutable generation fetch; operated availability and telemetry required | Domain/operator compromise, stale current manifest, outage, or rollback error | Primary online adapter; verify compatibility and digest before activation, follow SSE hints, retain prior generations |
| OCI mirror | Enterprise registry authentication/policy plus identical artifact digest and publisher attestation | Fits replication, admission, and air-gap workflows; freshness follows mirror policy | Registry retention/tag mutation, delayed replication, mirror trust misconfiguration | Optional enterprise transport; consume by digest, require equality with the trusted release archive digest, never grant tags authority over digest |

## Sigstore trusted root

Every attested catalog release carries a Sigstore bundle. `internal/attestation`
verifies that bundle against a trusted root and against a build-provenance
policy. The policy binds the repository, the signer workflow, the GitHub OIDC
issuer `https://token.actions.githubusercontent.com`, the predicate type
`https://slsa.dev/provenance/v1`, and the artifact digest. It also rejects a
self-hosted runner environment.

### How the verifier reads its root

The binary compiles in the Sigstore public-good trusted root at
`internal/attestation/roots/sigstore-public-good-trusted-root.json`.
`attestation.DefaultTrustedRootJSON` returns a copy of those bytes, and
`internal/sources/github` uses that copy unless a caller overrides it. The
compiled document equals line 0 of `gh attestation trusted-root`, which is the
same root the GitHub command-line application verifies with. Verification
therefore makes no network call and works on an air-gapped host.

`attestation.Policy.TrustedRootJSON` is the only override. A Go caller sets it
through `github.WithTrustedRoot`, and Starmap limits the bundle to 4 MiB. There is
no environment name, no command flag, and no file path that replaces the root
at run time. Starmap reads no trusted root from disk.

### How an operator refreshes the root

Starmap ships no runtime refresh path, so an operator has exactly three
supported options.

1. Install a newer Starmap release. Each release compiles in the trusted root
   of its build, so a binary upgrade is the normal refresh.
2. Embed Starmap as a Go library. Refresh the root through TUF on a connected
   host, carry the bytes to the air-gapped host, and pass them to
   `github.WithTrustedRoot`.
3. Verify outside Starmap. Run `gh attestation trusted-root` on a connected
   host, copy the document to the air-gapped host, and check the release with
   `gh attestation verify --bundle <bundle> --custom-trusted-root <file>`.
   Then load the checked release through the `file` catalog source.

Option 2 and option 3 both need a connected host, because TUF refresh is the
only way to get a newer root. Record the refresh date with the artifact, so
an audit can show which root verified which release.

### What a stale root does

A trusted root expires when its signing material rotates past the compiled
copy. Verification then fails closed at the signature stage. The runtime
returns a `TrustError` whose stage is `signature`, keeps its last verified
generation, and reports `source_health = unavailable`. A malformed or oversized
root document fails earlier, as a parse error, and the same retention applies.
A stale root never activates an unverified catalog and never silently downgrades
to an unverified transport.

## Policy choices

- Air-gapped deployments pin the embedded or imported OCI/release generation and
  accept explicit freshness responsibility.
- The maintained external pinned-artifact composition proves this path. It uses
  a compile-time archive digest, blank provider credentials, and no HTTP request.
  It has no acquisition, server, or remote dependency. Verification precedes
  exact activation in a caller-selected catalog store.
- Portable release import uses `artifact.VerifyRelease` followed by
  `acquisition.Syncer.ImportRelease`: checksum, detached statement,
  compatibility, and caller-owned publisher verification all complete before
  mutation. The verified release is a low-authority observation above the
  embedded fallback and below human catalog workspace evidence. It is never a
  wholesale replacement for manual data.
- Restricted-egress deployments mirror an attested GitHub/OCI digest internally.
  They do not operate a transparent unverified proxy.
- OCI consumers distinguish the OCI manifest digest from the archive-layer
  digest. They pull by the immutable manifest reference. The archive layer
  bytes must equal the trusted GitHub Release archive checksum.
- Connected deployments may follow a Starmap server, but activation is still
  checksum/compatibility gated and preserves last-known-good.
- A configured Starmap server URL is an explicit publisher trust decision.
  Non-loopback servers require HTTPS. The Go transport must produce a verified
  certificate chain for that exact origin. Redirects cannot change origin.
  Loopback HTTP exists only for local embedding and tests.
- Starport policy decides whether to pin a generation or follow a configured
  server. Starmap supplies facts and verified generations. It does not silently
  choose risk.

Starmap reports availability, freshness, integrity, and publisher identity as
separate evidence. A local deterministic pass does not imply the server endpoint
is healthy. Server reachability does not imply a live provider refresh occurred.
Neither result substitutes for signed release provenance.
