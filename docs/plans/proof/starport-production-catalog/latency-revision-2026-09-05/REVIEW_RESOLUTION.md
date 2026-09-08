# Latency target revision: 2026-09-05

The PRD, engineering specification, findings, and proposed plan now include the accepted latency recommendations.
The user requested these document changes after the [latency review](../../../../design/catalog-lifecycle/LATENCY_REVIEW.md).
This revision changes requirements and planned checks. It does not implement or qualify the target behavior.

## Accepted target

D19 requires low allocation cost and bounded, valid process memory for stable request data.
D20 preserves atomic budget admission while moving optional work outside response completion.
D21 requires complete gateway measurements and qualified product claims.

P34 through P38 state the product requirements.
Engineering specification [section 8.9](../../../../design/catalog-lifecycle/ENGINEERING_SPEC.md#89-request-latency-memory-and-admission) defines the contracts.
The [acceptance map](../acceptance-map.json) names their owners and behavior checks.

| ID | Review finding | Accepted work | Tasks | Primary evidence |
| --- | --- | --- | --- | --- |
| LR01 | F1: Repeated catalog construction and complete provider copies | Index Starmap lookup and precompute Starport candidates. Preserve aliases, caller ownership, and generation leases. | CSP0.4, CSP3.1, CSP10.1 | A44, A50 |
| LR02 | F2: Repeated stored-credential derivation | Manage bounded usable material by identity and revision. Preserve encryption, grants, rotation, expiry, and revocation. | CSP9.1 | A45 |
| LR03 | F3: Repeated stable authentication and policy reads | Use validated authorization bundles and applied configuration in memory. Bound validity and recover missed invalidations. | CSP10.2, CSP16 | A46, A28 |
| LR04 | F4: Repeated meter reads and incomplete strict admission | Reserve capacity atomically across applicable limits. Recover uncertain attempts without duplicate spending or releases. | CSP12, CSP12.2, CSP15 | A15, A47 |
| LR05 | F5: Blocking cache fills and unbounded stream accumulation | Bound optional reads, asynchronous fills, workers, and retained stream data. Preserve delivery, original expiry, and admission. | CSP12.1, CSP15 | A42, A48 |
| LR06 | F6: Synchronous advisory fleet exchange | Exchange health and latency hints through bounded workers. Keep local breaker decisions immediate. | CSP10.3 | A49 |
| LR07 | F7: Incomplete overhead measurement | Measure complete request stages and resources. Qualify UI, documentation, README, and demo claims against the tested artifact. | CSP0.4, CSP17, CSP19, CSP20, CSP22, CSP24 | A28, A31, A34, A38, A50 |

## Architecture and correctness

Starmap owns catalog facts, canonical lookup, and accepted generation identity.
Starport owns static gateway candidates, authorization, inference credentials, admission, and request measurements.
Persistent storage ownership remains as defined in the [storage review](../../../../design/catalog-lifecycle/STORAGE_REVIEW.md).

The same memory contracts apply to local developers, single-server teams, and replicated enterprises.
Badger and SQLite remain local durable stores. Valkey and PostgreSQL remain the primary shared recipe.
Selecting those stores does not require stable catalog, config, or credential reads during every request.
Explicit in-memory Badger remains an ephemeral database mode, separate from the bounded application working set.

Warm requests use valid resident records. Cold loads have explicit deadlines, concurrency limits, and separate measurements.
No response-cache hit is necessary for the warm request target.
Normal inference must not read GitHub, an internal Starmap server, or the catalog filesystem.

Memory does not extend permission validity or suppress known withdrawals.
Cached material does not establish a shared-credential grant.
An unknown required policy still refuses admission under D14.
An ordinary cached balance cannot authorize spending.
Strict admission uses an atomic reservation before dispatch. Local leases remain optional and require equivalent correctness evidence.

Uncertain provider charges retain their reserved capacity until reconciliation supplies adequate evidence.
Verified remaining capacity can still serve other requests.
Required accounting cannot depend on a lossy analytics queue.
Optional cache fills and advisory exchange use bounded workers.
Streams stop accumulating cache data at the configured bound and continue delivery.

## Performance evidence and limits

The [historical manifest](../../../../design/catalog-lifecycle/evidence/latency-review-2026-09-05/manifest.json) binds the inspected artifacts, commands, measurements, and profiles.
Its measurements expose repeated work. They do not qualify complete gateway overhead.
Allocation bytes per operation do not establish resident memory.
An isolated benchmark mean does not establish a production percentile.

The historical review reports Go `1.26.5`. Its manifest reports Go `1.27.0`.
The exact toolchain for each historical run remains UNVERIFIED because these records disagree.
This revision preserves both records and does not select one version without evidence.
CSP0.4 must capture the actual binary toolchain and build identity for every new measurement.

CSP0.4 establishes a complete HTTP baseline and a reviewed numeric performance profile before dependent optimization tasks.

The profile names hardware, workload, backend latency, durability, concurrency, timing boundaries, allocation limits, memory limits, and correctness limits.
Missing numeric limits or required environments block qualification.
The plan does not claim a sub-millisecond target without that evidence.

A50 includes authentication, decoding, planning, credential use, applicable admission, protocol conversion, and encoding.
It separates provider wait, connection setup, gateway work, and client backpressure.
Streaming checks include first-byte and first-token delay, forwarding delay, cancellation, and retained memory.
Local and fleet profiles exercise real supported storage compositions.

README claims must name their qualified profile and artifact.
The [demo brief](../readme-demo-brief.md) requires real-speed inference footage and disclosure of cuts elsewhere.
Changed artifacts or relevant runtime settings invalidate affected performance evidence.

## Plan and acceptance changes

Seven new tasks preserve all existing task IDs and order:

- CSP0.4 establishes complete measurements and the numeric profile.
- CSP3.1 repairs Starmap offering lookup.
- CSP9.1 manages stored credential material in memory.
- CSP10.1 precomputes static candidates and model indexes.
- CSP10.2 serves validated authorization records from memory.
- CSP10.3 moves advisory fleet exchange outside inference.
- CSP12.2 implements atomic capacity reservations and recovery.

Existing cache, storage, configuration, UI, documentation, README, demonstration, and qualification tasks now include the relevant latency checks.
The plan contains 38 proposed tasks, 50 primary acceptance cases, and 309 required subcases.
This adds 68 subcases while preserving the prior 241.
The candidate gate requires 43 primary cases. Seven publication-dependent cases remain separate until shipped assets exist.
Final qualification requires all 50 cases and all 309 subcases.

## Verification and remaining work

The [document verifier](verify_documents.py) checks task dependencies, acceptance mappings, counts, links, source hashes, and historical evidence preservation.
Its [result](verification.json) records document checks separately from product acceptance.
The [input manifest](input-manifest.json) records pre-edit hashes and backup locations.

Targeted writing checks pass for the ten changed prose and verification files.
The repository writing gate still reports three diagnostics in two unchanged historical output files.
Its glossary checks pass. The [writing results](writing-checks.json) preserve the exact commands, output, and exit status.
These diagnostics do not represent product acceptance failures. They prevent a passing repository-wide writing verdict.

All implementation tasks remain `todo`, and the plan remains `proposed`.
No product implementation test receives credit from this document revision.
Production performance, backend behavior, and native platform support still require the planned implementation and qualification work.
