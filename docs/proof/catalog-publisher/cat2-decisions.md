# CAT2 decisions

Date: 2026-09-02. Owner task: CAT2. The plan keeps a one-line summary of
each decision and a pointer to this record. The fourth review corrected
CAT-D12, CAT-D18, CAT-D19, and CAT-D20.

## Canonical names

| Concept | Canonical name | Migration |
| --- | --- | --- |
| Immutable GitHub release | `catalog-<catalog-digest>` | Read legacy `catalog-semantic-*` and `catalog-payload-*` tags. |
| Stable public discovery | `catalog-latest` | No legacy equivalent exists. |
| Channel document | `catalog-latest.json` | Attested mutable document with sequence, `channel_updated_at`, and immutable release identity. |
| Normalized fact identity | catalog digest | Retain `semantic_checksum` inside compatible manifests. |
| Exact byte identity | payload checksum | No change. |
| Configured upstream | catalog source | Deprecate remote server and remote URL names. |
| Source endpoint setting | `catalog_source_url` | Replace old Starport names directly; document the migration without runtime aliases. |
| Binary baseline | embedded public catalog | Refine the existing embedded catalog term. |
| Released baseline | released public catalog | Selected by `catalog-latest`. |
| Consumer read model | effective catalog | New composed runtime term. |

## Decisions

## Runtime and naming

**CAT-D1, accepted 2026-09-01:** New releases use `catalog-<digest>`, discovery uses `catalog-latest`, and only technical checksum fields use semantic terminology.

**CAT-D2, accepted 2026-09-01:** Keep `starmap.New` offline, and make `starmap.Open` the connected API with narrow `Catalog`, `State`, and `Status` reads.

**CAT-D3, accepted 2026-09-01:** Use independent source and acquisition policies that retain each layer and treat operator model observations as additive, not entitlement evidence.

**CAT-D4, accepted 2026-09-01:** Support `public`, `github`, `starmap`, `file`, and `embedded` sources without public fallback from a custom source.

**CAT-D5, accepted 2026-09-01:** Use fast `prefer_source` startup, last-known-good fallback, one-hour GitHub polling, and Starmap SSE with conditional polling fallback.

## Acquisition decisions

**CAT-D6, provisional engineering selection:** Use `sigstore-go` behind Starmap-owned GitHub identity and transport policy, subject to CAT2.1 evidence and GitHub CLI parity.

**CAT-D7, accepted 2026-09-01:** Give Starport one Starmap runtime, default its source to `public`, enable credential-detected acquisition, and preserve its accepted head.

**CAT-D8, accepted 2026-09-01:** Remove the acquisition mode, enable automatic acquisition by default at a four-hour interval in both applications, and keep explicit `Sync`.

## Publication decisions

**CAT-D9, accepted 2026-09-01:** Advance an attested channel sequence and `channel_updated_at` after every successful four-hour verification without a no-change catalog generation.

**CAT-D10, accepted 2026-09-01:** Publish every four hours at minute 17 and poll consumers every hour. Use a six-hour end-to-end freshness objective.

## Automatic policy decision

**CAT-D11, accepted 2026-09-01:** Use `CATALOG_ACQUISITION_ENABLED` plus `INTERVAL`. A false enabled value disables all automatic work, and no periodic-only state exists.

**CAT-D12, audited 2026-09-01:** Bound connect, TLS, headers, transfer inactivity, bytes, pages, and records. Add no whole-refresh deadline by default. Nest the limits: a 60-minute transfer, a 75-minute publisher step, and a 90-minute workflow job.

**CAT-D13, audited 2026-09-01:** Use full-interval stable phases, a 15-minute cold-start spread, one-second to 15-minute decorrelated retry, retry not-before, source admission, and a distributed refresh lease.

## Contract and transport decisions

**CAT-D14, audited 2026-09-02:** The runtime has one `Sync(ctx, ...SyncOption) (AcquisitionReport, error)` method. `Close` is idempotent and joins runtime-owned work within five seconds.

## Transfer decisions

**CAT-D15, audited 2026-09-02:** Each finite HTTP body transfer has a maximum duration, default 60 minutes. A 64 MiB body at the 256 Kbps floor rate takes about 35 minutes, and the default provides headroom over that case. The limit does not bound an SSE subscription lifetime. A zero value is invalid and fails startup, and an operator on a slower link raises the value. A Starmap-owned transport wrapper applies the 60-second header bound to every catalog request, including the SSE open. Peak memory is one buffered body per in-flight transfer.

## Fleet decisions

**CAT-D16, audited 2026-09-02:** The direct-consumer request budget comes from the rate-limit headers. The headers are limit, used, remaining, and reset. The budget subtracts a reserved headroom and divides by the measured requests per cycle. Status warns at 80 percent of the reported limit. A fleet above its budget uses authenticated conditional polling or a central Starmap source. The six-hour objective applies to direct consumers and SSE-push chains, and each polling hop adds one poll interval.

## Lease and dependency decisions

**CAT-D17, audited 2026-09-02:** CAT8 pins a Starmap pseudo-version from the plan branch. CAT11 replaces the pin with the released tag.

**CAT-D18, audited 2026-09-02:** A shared-storage refresh lease has a 90-second TTL, a 30-second renewal interval, and an epoch. Active-active servers require a store with the lease and a conditional compare-and-swap on the generation record. A plain shared filesystem volume serves one writer at a time. CAT5 owns the runtime lease, and a replicated Starmap fences its durable generation commit with the epoch. CAT8 keeps Starport's separate candidate-to-accepted transaction, which carries the epoch and rejects a stale one. A holder that loses the lease cancels its run within one renewal interval, discards the results, reports `lease_lost`, and retries at the next phase.

## Console decision

**CAT-D19, designed 2026-09-02:** One shell-owned chip in a shell header slot on every route replaces the freshness bar and the overview card. The third and fourth reviews corrected it. It shows the freshness dot, the short generation ID, and the policy age, and a click opens a panel no wider than the viewport. The record is `docs/proof/catalog-publisher/cat2-console-catalog-design.md`.

- The safe route serializes an allowlisted `CatalogSummary` with identity, counts, ages, times, and the server freshness verdict. No operational field reaches it, and a missing catalog answers a sanitized `503`.
- Only the admin status route carries the source, layers, hop chain, fallback state, schedule, and provider outcomes.
- Usability, authorization, freshness, degradation, fallback, source health, and active work each have their own element. The dot means freshness only, and the unavailable and lock glyphs differ by shape.
- The panel draws the layers figure with the embedded baseline. A separate hop chain labels direct and upstream-reported hops.
- The wide screen adds 16 px above each page heading for the slot. The small screen uses the 44 px top-bar control and adds no height. A `403` stops polling until the session changes.
- CAT-V50 through CAT-V55 guard the surface.

## Documentation decision

**CAT-D20, designed 2026-09-02:** Starport gets a topology guide with five designs, a replicated variant, a diagram each, and a decision table. The third and fourth reviews corrected it. The operator guide gets the catalog configuration reference and the removed-name table. Starmap gets a server runbook and a Kubernetes pair example, and both READMEs name the central server topology. The record is `docs/proof/catalog-publisher/cat2-enterprise-docs-design.md`.

- Restricted replica egress and the air-gapped mirror are separate designs, because a central server with egress is not air-gapped.
- Active-active central servers require a lease-capable store with conditional writes. A shared filesystem volume serves one writer at a time.
- The air-gapped design uses an external pull, an offline verification, and the `file` source, because the runtime has no OCI source.
- CAT9.1 owns the Starport documents and CAT9.2 owns the Starmap documents. Each task maps to one pull request.
- CAT-V56 through CAT-V59 and CAT-V64 guard the documents.

