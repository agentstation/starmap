# Fable Starmap Plan Review Disposition

Date: 2026-07-27

Independent verdict: **GO WITH REQUIRED REVISIONS**

This record maps Fable's independent review back to
[`../STARMAP_ARCHITECTURE_CONTROL_PLANE.md`](../STARMAP_ARCHITECTURE_CONTROL_PLANE.md).
It does not replace the review or rewrite historical evidence.

## Blocking findings

| Review item | Disposition | Control-plane change |
| --- | --- | --- |
| B-01: P2 required red tests while every phase required green checks | `ACCEPTED` | P2 now requires green characterization tests that pin current defective behavior and carry a finding ID; the correcting PR rewrites those expectations. Failing tests are never merged. |
| B-02: `~/.starmap/catalog` is already a generation store on existing installations | `ACCEPTED` | Added historical supersession mapping, typed legacy-layout detection, an explicit pre-mutation migration decision, and restart/downgrade coverage in P3.1/P3.10. |
| B-03: YAML and generation storage lacked one declared commit point | `ACCEPTED` | The generation-store CAS is the sole commit point. P3.6a lands that invariant as a narrow hotfix; P3.6b makes YAML a post-commit projection repaired by digest after interruption. |
| B-04: workspace edit detection depended on authority/provenance work scheduled later | `ACCEPTED` | Execution order now lands the store-only/atomicity hotfix, then P4 authority/provenance, then the remaining P3 workspace lifecycle. |
| B-05: the root dependency-closure gate lacked a concrete mechanism and budget | `ACCEPTED` | P6.2 explicitly inverts the `pkg/sources` provider-client edge behind the existing factory seam and rejects named heavy dependency families from the read-only root closure. Numeric baselines and budgets must be recorded in P2.6 before implementation. |
| B-06: historical release findings were orphaned | `ACCEPTED` | Added a supersession map and P11.9. Historical F-099/F-105/F-106 must be completed, superseded, or user-accepted as rejected; publication still requires separate authority. |

## Reliability and execution findings

| Review item | Disposition | Control-plane change |
| --- | --- | --- |
| B-07: publication callbacks can reorder or drop generations | `ACCEPTED` | P7.2/P7.4 require cross-generation ordering, one publication event, monotonic per-stream sequence, and correct cache activation placement. |
| B-08: server/background shutdown is not joinable | `ACCEPTED` | P7.5/P7.10 require owned goroutines, bounded joins, subscribe-after-stop behavior, and blocked-subscriber shutdown tests. |
| B-09: deletion could precede distribution/scheduler product decisions | `ACCEPTED` | Added P2.8 as a prerequisite to P6.5. |
| B-10: same-commit ledger updates are impossible for third-party merges and final cleanup | `ACCEPTED` | Operating Rule 3 now contains narrow, auditable exceptions for third-party PRs and the post-merge machine gate. |
| B-11: local HTML rendering depends on CDN assets | `ACCEPTED WITH SCOPE` | P0.4 claims local parse validity, not offline rendering. The reviewed report intentionally retains its CDN dependencies; offline report delivery is not a product requirement. |
| B-12: deterministic authority policy is not a valuable fuzz target | `ACCEPTED` | Authority selection and presence use table/property tests. Fuzzing remains for untrusted provider envelopes, YAML/JSON artifacts, provenance decoding, and SSE framing. |
| B-13: protected PR execution requires explicit human approval pauses | `ACCEPTED` | Operating rules and the goal prompt now identify protected review/merge decisions as user-owned pauses. |

## Canonical Go findings

| Finding | Disposition | Control-plane change |
| --- | --- | --- |
| `Catalog()` lacks the nil-receiver behavior of neighboring methods | `ACCEPTED` | Added P6.8 construction and accessor ergonomics. |
| `New` performs synchronous storage I/O through an internally created, uncancellable context | `ACCEPTED` | P6.8 requires a context-aware construction path and bounded cancellation tests; the exact API is decided through consumer fixtures. |
| `internal/application.Application` is broader than its consumers need | `ACCEPTED` | P6.4 explicitly requires per-consumer role interfaces. |
| `internal/utils/ptr`, `provenance.ProvenanceFile`, and seven `catalog*` packages indicate naming or depth problems | `ACCEPTED` | Added concrete review targets to P8.4–P8.6; deletion/consolidation is preferred over mechanical renaming. |
| WebSocket has no named bidirectional consumer | `ACCEPTED` | P7.3 deletes the WebSocket transport, hub, adapter, dependency, and tests. Reintroduction requires a named bidirectional use case. |

## Deliberate constraints

- The review does not authorize publishing an application or catalog release.
- Existing installations are not silently migrated. Detection must happen
  before mutation, and the selected migration must be explicit, typed,
  transactional, and tested.
- Numeric dependency and performance budgets will be frozen from the P2.6
  baseline rather than invented in this review disposition.
- Historical evidence remains unchanged even where this plan supersedes its
  prescriptive architecture.
