# CSP19 fail-before statements

Captured 2026-10-03 at Starmap `344eb64cf` and Starport `e7a58541` before any CSP19 edit. Each block quotes the named line range of the file at that head. The hash is the SHA-256 of the whole file.

## Per-replica polling (G20)

`docs/DEPLOYMENT-TOPOLOGIES.md` lines 93-130, sha256 `b89615d79c1601c141bb7effe276f70940fee5c60ec4f494143c978da6105029`

```text
**Settings.** Set `SOURCE_TOKEN` to one GitHub token that every replica shares.
The token raises the hourly ceiling from 60 requests for each address to 5,000
requests for each token. Keep `STARTUP_SPREAD` at `15m`, because the spread
holds a cold fleet away from one moment.

**Request budget.** A direct consumer budgets from the GitHub rate-limit
headers. The four headers are `x-ratelimit-limit`, `x-ratelimit-used`,
`x-ratelimit-remaining`, and `x-ratelimit-reset`. The runtime records the
measured requests for each refresh cycle. The fleet capacity is the remaining
budget minus a reserved headroom, divided by the measured requests for each
cycle. The status warns when `used` passes 80 percent of `limit`.

The rate that a fleet puts on GitHub follows the fleet size and the window:

| Fleet | 15-minute startup spread | 1-hour poll interval | 4-hour acquisition |
| --- | --- | --- | --- |
| 100 | 0.111 requests a second | 0.028 requests a second | 0.007 requests a second |
| 10,000 | 11.11 requests a second | 2.78 requests a second | 0.69 requests a second |
| 100,000 | 111.1 requests a second | 27.78 requests a second | 6.94 requests a second |

A published ceiling is not a safe threshold. Move to a central Starmap server
at any of these three points:

- Above 60 replicas behind one egress address with no token. Each replica
  needs one poll an hour, and the hourly ceiling is 60.
- Above about 5,000 replicas that share one token. The token ceiling is 5,000
  requests an hour.
- Above 10,000 replicas. The 15-minute spread then puts 11 requests a second
  against a secondary limit of 15 requests a second. No headroom remains.

**Freshness age.** The objective stays six hours, because every replica reads
the channel directly.

**Egress.** Every replica reaches GitHub and every replica reaches the provider
APIs.

**Failure behavior.** A rate-limit refusal leaves the accepted head in place
and the next phase retries. Each replica keeps its own accepted head, so one
```

## Per-replica polling (G20), site

`docs/site/architecture/topologies.md` lines 35-45, sha256 `92287a257dc6e207108d618de712ba7f3a670a7ad40ab826525c957f2ecbdc3c`

```text
One gateway follows the public channel `catalog/v1`. The shipped defaults apply, so this topology needs no catalog setting. The gateway polls the source each hour. One gateway uses about two percent of the unauthenticated GitHub budget.

## Starport fleet with direct GitHub

Several gateways share one egress address and follow the public channel. Set `STARPORT_CATALOG_SOURCE_TOKEN` to one GitHub token for all replicas. The token raises the hourly limit from 60 requests for each address to 5,000 requests for each token.

Move to a central Starmap server at any of these points:

- More than 60 replicas behind one address without a token.
- More than about 5,000 replicas that share one token.
- More than 10,000 replicas.
```

## Egress scope (G11), Starport

`docs/DEPLOYMENT-TOPOLOGIES.md` lines 197-206, sha256 `b89615d79c1601c141bb7effe276f70940fee5c60ec4f494143c978da6105029`

```text
`ACQUISITION_ENABLED=false` on every replica. A false value stops every
automatic observation, so the replica opens no provider connection for the
catalog.

**Request budget.** The replicas send no GitHub request and no provider
observation. The central server owns the complete external budget.

**Freshness age.** The objective stays six hours, because the server-sent
events reach the replicas without a poll.

```

## Egress scope (G11), Starmap

`docs/ENTERPRISE_CATALOG_SERVER.md` lines 116-123, sha256 `276f1e40b88e07552d92c93620dc0886134d56f176b1f20d33ef48d43f59043f`

```text

The `STARPORT_CATALOG_SOURCE_URL` value names the versioned API base URL of the
central server. A non-loopback endpoint must use HTTPS.

Set `STARPORT_CATALOG_ACQUISITION_ENABLED` to `false` when a replica must reach
only the central server. The replica then makes no provider request. The
central server keeps its own egress to GitHub and to the providers, so this
design is not air-gapped.
```

## Rotation (G12)

`docs/ENTERPRISE_CATALOG_SERVER.md` lines 125-133, sha256 `276f1e40b88e07552d92c93620dc0886134d56f176b1f20d33ef48d43f59043f`

```text
## Rotate the key and read the health routes

Rotate the server API key without a service interruption:

1. Add the new key to the client secret of each replica.
2. Restart each replica and confirm its readiness.
3. Change `API_KEY` on the server and restart the server.
4. Confirm that each replica reconnected.
5. Remove the old key from every secret store.
```

## Generic SQL adapter (G21), contract

`docs/CATALOG_STORE_CONTRACT.md` lines 16-18, sha256 `0e9d12b8ed841b6a4c99e89fe7096b7b98398544ad8bcb7cc50ccfd56bd9d2c6`

```text
Starport may provide SQLite, MySQL, PostgreSQL, or other adapters. Starport owns
each concrete driver, connection pool, schema, migration, credential, backup,
close, and dialect-specific transaction concern.
```

## Generic SQL adapter (G21), Docker

`docs/DOCKER.md` lines 322-324, sha256 `a320774b19ce3c89aa25c5f1ea2691dfc4a6a826acd7bf7a58b1f16db1dbff05`

```text
Starport may instead implement `storage.Store` with its own relational
database. Starport, not Starmap, owns the driver, connection pool, schema,
migrations, backups, transactions, CAS semantics, and lifecycle.
```

## Shared volume (G22), Starmap

`docs/ENTERPRISE_CATALOG_SERVER.md` lines 159-162, sha256 `276f1e40b88e07552d92c93620dc0886134d56f176b1f20d33ef48d43f59043f`

```text
The example is a single-server design. It runs one Starmap replica on a
persistent volume. Raise the Starmap replica count only after you move the
store to a lease-capable backend that supplies the CAT-D18 lease and
conditional writes.
```

## Shared volume (G22), Starport

`docs/DEPLOYMENT-TOPOLOGIES.md` lines 300-304, sha256 `b89615d79c1601c141bb7effe276f70940fee5c60ec4f494143c978da6105029`

```text

**A plain shared volume supports one writer.** A plain shared filesystem volume
gives neither the lease nor the conditional write. Such a volume therefore
supports the single-server design alone, in the active and passive form. In
that form one standby server starts only after the active server stops. Two
```

## Replica SQL claim (MySQL)

`docs/PRODUCTION-STATUS.md` lines 9-9, sha256 `ca314c6cff9fb4be65a72352f43a749a0ba8f0df780acf7f9bb8dfd37e5e7b54`

```text
| One persistent process | Badger for KV records, SQLite for relational records, and local catalog and upload files. | Back up each required store. A KV backup alone does not restore the deployment. |
```

## Stale fleet compose comment

`docker-compose.fleet.yml` lines 1-2, sha256 `acc5a7f881657aa2df242aabb75e36447c1b3394e7a802e6c4d30ed460ba27e2`

```text
# Candidate fresh fleet. External stores must satisfy docs/FLEET_INITIALIZATION.md.
# CSP13 and CSP15 still block production fleet use and existing fleet upgrades.
```

## Target status table

`docs/site/architecture/targets.md` lines 12-20, sha256 `a84c8e2e48ee2c23f5c85c0a6bf73f8217d05e3157cf3adceab263160f3d8c30`

```text
| ID | Architecture | Storage recipe | Status in this release |
| --- | --- | --- | --- |
| T1 | Standalone Starmap CLI or server | Starmap generation files on one host | Starmap product. Outside this release. |
| T2 | Persistent local Starport | Badger, SQLite, and local file bytes | Supported |
| T3 | Starport on one production server | Badger and SQLite on durable volumes | Available. Production qualification open. |
| T4 | Replicated Starport with direct catalog updates | Valkey, PostgreSQL, and object storage | Adapters available. Qualification pending. |
| T5 | Internal Starmap with Starport deployments | Starmap uses T1. Each gateway uses T2, T3, or T4. | Source kind available. Server is Starmap. |
| T6 | Restricted or air-gapped installation | T1 to T5 storage, by process count | File source available |
| T7 | Ephemeral development composition | In-memory Badger and SQLite, scratch files | Supported |
```

