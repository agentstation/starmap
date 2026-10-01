# Recovery measurement record (D42)

Date: 2026-10-01 UTC. Source head: Starport `53df341c` (`codex/recovery-measurement-20260930`, the Phase C tree plus the `inspect-import` fleet fix `da9425bd` and the harness). Operator binary: race build, SHA-256 `9798dbf72406…`, bound to the source head, `vcs.modified=false`. Record: `measurement.json` (version 1). Runner: `TestRecoveryMeasurement` in `internal/app`, 854.95 seconds, 9 of 9 runs complete, exit 0.

This record presents laboratory measurements. It approves no production limit. Owner decision D42 requires owner approval of numeric RPO and RTO targets before production readiness.

## Deployment

- Host: Mac14,5, 12 CPUs, 32.0 GiB. Docker engine: 12 CPUs, 15.6 GiB. macOS, arm64, Go 1.27.1.
- Topology: gateway and operator on the host. Valkey, PostgreSQL, and S3 object storage in the local Docker engine with isolated schemas and buckets. The test owns a private Valkey primary and replica.
- Images: `valkey/valkey:7-alpine` (`9acdf6f0ae17`), PostgreSQL (`f1c3376c26f2`), `quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z` (`14cea493d9a3`).
- Valkey persistence: `appendonly yes`, `appendfsync always`, `save` disabled.
- Load: idle. Two explicit sequential Valkey probe writes before the fence. No external traffic, provider load, or worker dispatch.
- Fence: the test closes every writer that it owns before the capture. No gateway or worker runs before restored readiness. The old primary stays isolated.

## Method

Each scenario runs three times. Outage start is the instant after the last acknowledged probe write, when no writer remains open. Restoration runs through the shipping binary: `backup close`, `backup create`, then the import or adoption commands, then a fresh `starport serve` process. Readiness restored is the first HTTP 200 from `/health/ready` on that process. State observed is the read of the probe keys after readiness through the restored configuration.

| Scenario | Commands after capture |
| --- | --- |
| Empty-target import | `backup prepare`, `backup inspect-import`, `backup activate` |
| Valkey restart with persistent data | `backup adopt prepare`, `adopt activate`, `adopt inspect` |
| Replica promotion with acknowledged loss | `backup adopt prepare`, `adopt activate`, `adopt inspect` |

In the promotion scenario, the replica diverges before the second probe write. The old primary acknowledges that write alone. The test then promotes the replica and restores through it.

## Measurements

Outage start to readiness restored, in seconds, three repetitions:

| Scenario | Min | Median | Max |
| --- | --- | --- | --- |
| Empty-target import | 47.64 | 50.73 | 53.18 |
| Valkey restart with persistent data | 52.33 | 52.49 | 54.82 |
| Replica promotion with acknowledged loss | 53.11 | 53.27 | 53.54 |

Outage start to state observed adds 0.03 to 0.04 seconds in every run.

Acknowledged state after readiness, per run (acknowledged, present, lost):

| Scenario | Run 1 | Run 2 | Run 3 |
| --- | --- | --- | --- |
| Empty-target import | 2, 2, 0 | 2, 2, 0 | 2, 2, 0 |
| Valkey restart with persistent data | 2, 2, 0 | 2, 2, 0 | 2, 2, 0 |
| Replica promotion with acknowledged loss | 2, 1, 1 | 2, 1, 1 | 2, 1, 1 |

In every promotion run the reachable old primary retains both writes and refuses recovery authority. The promoted primary also refuses authority before adoption.

Command durations across all nine runs, in seconds, minimum to maximum:

| Command | Min | Max |
| --- | --- | --- |
| `close` | 1.20 | 2.61 |
| `capture` | 2.27 | 2.69 |
| `prepare` | 2.05 | 2.08 |
| `inspect-import` | 2.37 | 2.41 |
| `activate` | 16.82 | 22.67 |
| `adopt prepare` | 2.27 | 2.54 |
| `adopt activate` | 18.91 | 20.17 |
| `adopt inspect` | 4.61 | 6.16 |
| gateway start to readiness | 20.48 | 21.26 |

Every command exits 0.

Backup age at outage start: -2.0 to -3.4 seconds. History age at outage start: -4.2 to -10.7 seconds. Negative values mean that the capture and the synthetic final-only history occur after the fence, inside the outage interval.

Native identities per run: the prepared boundary holds the closed source epoch (8 for import, 7 for adoption) and an empty backend. The restored approval is open at epoch 12 with the restored Valkey backend identity. The Valkey incarnation changes in every run. SQL and object storage retain snapshot digests, not independent backend identities.

## Limits

- Component test duration is not deployment RTO. The race build, child process exit, and the 20-second gateway start dominate the interval.
- A laboratory zero-loss result is not a blanket production RPO guarantee. The counts cover explicit Valkey probe writes only.
- The source fleet setup is outside the outage interval. The operator commands and the fresh HTTP readiness are inside.
- The harness runs on one host with local Docker. It is not in the hosted recovery roster.
- Adoption supports Valkey, PostgreSQL, and object storage only. MySQL application-level adoption stays UNVERIFIED.

## Proposed targets for owner approval

These proposals derive from the table above. They are not approved.

- Reference RTO for the fenced operator procedure on the idle reference deployment: 120 seconds. This is about two times the observed maximum of 54.82 seconds. The margin covers a pure build, a cold cache, and operator latency between commands.
- Reference RPO after a Valkey restart with `appendfsync always` persistent data: zero acknowledged writes. Measured zero in three runs.
- Reference RPO after an empty-target import of a fenced capture: zero acknowledged writes after the fence. Measured zero in three runs.
- Reference RPO after a replica promotion: the writes that only the old primary acknowledged after the divergence. The product does not bound this count. Measured one of two in three runs. A production target needs a replication lag bound or synchronous acknowledgement, which the product does not provide.

Pending owner decision: approve, change, or reject these numeric targets. Production readiness waits on that decision. The pull requests for the fix and the harness do not wait on it.

## Evidence

- `measurement.json`: the full record without endpoints or secrets.
- `summary.txt`: the per-scenario summary of the record.
- Private command outputs and gateway logs stay in `/private/tmp/starport-measurement-proof-20260930/final2/private/`.
- Harness log: `/private/tmp/starport-measurement-proof-20260930/race-final2.log`.
