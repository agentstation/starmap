# Catalog Distribution Trust Model

No distribution channel is authoritative merely because bytes were reachable.
Every consumer verifies catalog schema, generation manifest, canonical payload,
SHA-256 descriptors, detached statement, and channel-specific publisher trust
before atomic activation. Failure retains the last-known-good generation.

| Channel | Trust root | Freshness and availability | Principal risks | Intended policy |
| --- | --- | --- | --- | --- |
| Embedded bootstrap | Installed Starmap binary signature/provenance plus embedded manifest checksum | Works offline and air-gapped; freshness stops at binary build time | Stale catalog, binary growth, compromised build supply chain | Startup fallback; enforce runtime/CI age and size budgets; replace with a verified committed generation when policy permits |
| GitHub Release assets | Expected `agentstation/starmap` repository and approved workflow identity, signed artifact attestation, archive/payload digests | Durable immutable public history, but depends on GitHub availability and outbound enterprise policy | Repository/workflow compromise, unavailable/blocked GitHub, mistaken asset replacement | Public immutable source of record; pin exact generation/digest; never use expiring Actions artifacts as runtime distribution |
| Hosted `starmap.agentstation.ai` | TLS origin plus the same signed repository/workflow attestation and immutable digests | Lowest-latency exact-current-schema discovery; operated availability, CDN, telemetry, and promotion SLO required | Domain/operator/object-store compromise, stale latest pointer, outage or rollback error | Primary online adapter; same-origin URLs, ETag revalidation, staged channel promotion, instant pointer rollback, retain prior generations |
| OCI mirror | Enterprise registry authentication/policy plus identical artifact digest and publisher attestation | Fits replication, admission, and air-gap workflows; freshness follows mirror policy | Registry retention/tag mutation, delayed replication, mirror trust misconfiguration | Optional enterprise transport; consume by digest, require equality with release/hosted archive digest, never grant tags authority over digest |

## Policy choices

- Air-gapped deployments pin the embedded or imported OCI/release generation and
  accept explicit freshness responsibility.
- Restricted-egress deployments mirror an attested GitHub/OCI digest internally;
  they do not operate a transparent unverified proxy.
- OCI consumers distinguish the OCI manifest digest from the archive-layer
  digest, pull by the immutable manifest reference, and require the archive
  layer bytes to equal the trusted GitHub/hosted archive checksum.
- Connected deployments may track the hosted exact-current generation, but
  activation is still checksum/schema/attestation gated and preserves
  last-known-good.
- Starport policy decides whether to pin, track canary, or track stable. Starmap
  supplies facts and verified generations; it does not silently choose risk.
- Stable hosted promotion requires the identical generation to pass dev and
  canary plus a recent availability, freshness, latency, and checksum-bound
  canary probe. Rollback is reasoned, telemetry-producing, and limited to a
  generation previously served by the target channel.

Availability, freshness, integrity, and publisher identity are reported as
separate evidence. A local deterministic pass does not imply the hosted endpoint
is healthy; hosted reachability does not imply a live provider refresh occurred;
and either does not substitute for signed release provenance.
