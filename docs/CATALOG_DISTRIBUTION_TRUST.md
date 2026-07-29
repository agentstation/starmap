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

## Policy choices

- Air-gapped deployments pin the embedded or imported OCI/release generation and
  accept explicit freshness responsibility.
- Portable release import uses `catalogartifact.VerifyRelease` followed by
  `acquisition.Syncer.ImportRelease`: checksum, detached statement,
  compatibility, and caller-owned publisher verification all complete before
  mutation. The verified release is a low-authority observation above the
  embedded fallback and below human workspace evidence; it is never a
  wholesale replacement for manual data.
- Restricted-egress deployments mirror an attested GitHub/OCI digest internally;
  they do not operate a transparent unverified proxy.
- OCI consumers distinguish the OCI manifest digest from the archive-layer
  digest, pull by the immutable manifest reference, and require the archive
  layer bytes to equal the trusted GitHub Release archive checksum.
- Connected deployments may follow a Starmap server, but activation is still
  checksum/compatibility gated and preserves last-known-good.
- A configured Starmap server URL is an explicit publisher trust decision.
  Non-loopback servers require HTTPS, the Go transport must produce a verified
  certificate chain for that exact origin, and redirects cannot change origin.
  Loopback HTTP exists only for local embedding and tests.
- Starport policy decides whether to pin a generation or follow a configured
  server. Starmap supplies facts and verified generations; it does not silently
  choose risk.

Availability, freshness, integrity, and publisher identity are reported as
separate evidence. A local deterministic pass does not imply the server endpoint
is healthy; server reachability does not imply a live provider refresh occurred;
and either does not substitute for signed release provenance.
