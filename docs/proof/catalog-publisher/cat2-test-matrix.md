# CAT2 test matrix

Date: 2026-09-02. Owner task: CAT2. The plan keeps a pointer to this record.
Each row names the cases that the owning tasks must cover. The campaign
verifier names the tests that prove them.

| Dimension | Required cases |
| --- | --- |
| Discovery | new channel, unchanged ETag, missing channel, malformed document, legacy immutable rollback |
| Trust | valid workflow identity, wrong repository, wrong workflow, wrong digest, missing bundle, expired or invalid chain |
| Activation | new facts, new identity with equal payload, older generation, incompatible schema, concurrent readers |
| Failure | startup outage, idle transfer, slow-drip transfer bound, oversized body, poll outage, SSE outage, unauthorized source, partial channel update, caller cancellation, shutdown timeout |
| Configuration | default public, embedded-only, custom GitHub, custom Starmap, file, enabled and interval combinations, direct migration, forbidden fallback |
| Composition | public-only, operator-only, public plus operator, source server plus operator, partial provider failure with per-provider last-known-good retention, retained provenance |
| Operations | channel heartbeat, no-change check, slow progress, async refresh join and cancel, full-interval phase, 15-minute startup and outage spread, retry not-before, rate limit, rate-limit warning, source admission, lease, route idle limits, independent usability, freshness, fallback, and health, hop age, bounded metrics |
| Determinism | injected clock, injected random source, `httptest` transports, `-race -count=3`, no sleep above 100 ms |
| Deployment | offline Go, CLI filesystem store, Docker restart, source server, standalone Starport, three Starport consumers |
| Console | one element per status concept, unavailable and authorization glyphs, allowlisted safe response with no sentinel operational value, sanitized 503 without a catalog, admin route split, embedded baseline always present, direct and upstream-reported labels, chip on every route, 44 px small-screen control, viewport-bound panel, keyboard focus, no second request after 403 |
| Documentation | five topologies, replicated variant with the lease-capable store, six diagrams, link check, canonical names, no removed name in the example, eight runbook sections, structurally parsed Kubernetes pair |
