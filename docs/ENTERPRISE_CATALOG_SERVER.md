# Enterprise catalog server runbook

This runbook builds one central Starmap catalog server for a Starport fleet.
The central server follows the public catalog channel and serves every replica.
Each replica then reads one internal endpoint instead of GitHub.

Read [ARCHITECTURE.md](ARCHITECTURE.md#connected-catalog-runtime) for every
setting and default. Read [REST_API.md](REST_API.md) for every route.

## Select the store

The store holds the catalog generations that the server serves.

1. Use a persistent volume for one server. The standalone command-line
   composition is a single-writer design.
2. Use a lease-capable store for two or more active servers. That store must
   supply the CAT-D18 refresh lease and a conditional compare-and-swap on the
   generation record.
3. Do not run two active servers on one shared filesystem volume. A plain
   volume supplies neither the lease nor the conditional write.

A shared volume still supports an active and passive pair. Start the standby
only after the active server stops.

## Start the server

Start the server with authentication and a bound address:

```bash
starmap serve --auth --host 0.0.0.0 --port 8080
```

The `--auth` flag makes the server require the `API_KEY` value on every
protected route. The health and readiness routes stay public, so a probe needs
no credential.

Give the process its own state directory:

```bash
export STARMAP_STATE_DIR=/var/lib/starmap/state
```

That directory holds the runtime state, the retained generations, and the
instance seed. Put it on the persistent volume.

`STARMAP_CATALOG_WORKSPACE_PATH` is a separate setting. It names the reviewed
operator catalog input, not the runtime state. Set it only when this server
serves a catalog that an operator maintains on disk.

Confirm the server serves a catalog:

```bash
curl -fsS http://localhost:8080/api/v1/ready
```

## Provide credentials

The server uses three separate credential groups. Keep each group in its own
secret.

| Credential | Name | Purpose |
| --- | --- | --- |
| Server API key | `API_KEY` | The value that a fleet client sends to this server |
| Catalog source token | `STARMAP_CATALOG_SOURCE_TOKEN` | The GitHub token that raises the channel rate-limit budget |
| Provider credentials | `OPENAI_API_KEY` and the other provider names | The acquisition inputs that read provider APIs |

A provider credential is never a server API key. A server API key never reaches
a provider. Store each secret outside the image and outside the repository.

## Set the interval and the policy

The server reads its catalog source on a poll interval and runs its own
acquisition on a separate interval:

```bash
export STARMAP_CATALOG_SOURCE_POLL_INTERVAL=1h
export STARMAP_CATALOG_ACQUISITION_INTERVAL=4h
export STARMAP_CATALOG_SOURCE_STARTUP_POLICY=prefer_source
```

The `prefer_source` policy serves the verified embedded catalog until the first
upstream reply arrives. Choose `require_source` when the server must never
serve the embedded baseline. Choose `prefer_local` when the server must keep
its retained generation until an operator refreshes it.

Set `STARMAP_CATALOG_ACQUISITION_ENABLED` to `false` when the central server
must not reach a provider API. The server then serves catalog data from its
source only.

## Size the server for the fleet

Each replica holds one long-lived stream connection to the server. Size the
connection budget from the replica count.

1. Count one stream connection for each replica process.
2. Add the readiness and manifest requests of each replica.
3. Keep the default 20-second heartbeat, because the subscriber expects it.
4. Raise `--sse-heartbeat-interval` only after you raise the subscriber
   liveness deadline. The deadline must hold at least two heartbeats.
5. Set `--rate-limit` above the total request rate of the fleet, or set it to
   zero behind a trusted internal network.

One central server carries a large fleet, because a stream connection stays
idle between publication events. Add a second server only for availability, and
only on a lease-capable store.

## Point each Starport at the server

Set the source of each Starport replica to this server:

```bash
export STARPORT_CATALOG_SOURCE=starmap
export STARPORT_CATALOG_SOURCE_URL=http://starmap:8080/api/v1
export STARPORT_CATALOG_SOURCE_API_KEY=<the server API key>
```

The `STARPORT_CATALOG_SOURCE_URL` value names the versioned API base URL of the
central server. A non-loopback endpoint must use HTTPS.

Set `STARPORT_CATALOG_ACQUISITION_ENABLED` to `false` when a replica must reach
only the central server. The replica then makes no provider request. The
central server keeps its own egress to GitHub and to the providers, so this
design is not air-gapped.

## Rotate the key and read the health routes

Rotate the server API key without a service interruption:

1. Add the new key to the client secret of each replica.
2. Restart each replica and confirm its readiness.
3. Change `API_KEY` on the server and restart the server.
4. Confirm that each replica reconnected.
5. Remove the old key from every secret store.

Read the state of the fleet through three routes:

| Route | Reports |
| --- | --- |
| `GET /health` | Process liveness only |
| `GET /api/v1/ready` | Catalog readiness and every connected runtime field |
| `GET /api/v1/catalog/source-chain` | The hop list from this server to the origin |

Alert on the readiness fields, not on the process probe. A `warn` or `critical`
value in `channel_freshness` reports a stall at the origin or at a hop above
this server. A `warn` or `critical` value in `source_check_freshness` reports a
failure of the last check of this server. A `true` value in `fallback` reports
that the server serves the embedded catalog.

## Run the pair on Kubernetes

[DOCKER.md](DOCKER.md#kubernetes) holds the example manifests. The example runs
two Deployments and one Service:

- The Starmap Deployment mounts a persistent volume and runs
  `starmap serve --auth`.
- The Service exposes the Starmap pods on port 8080.
- The Starport Deployment sets `STARPORT_CATALOG_SOURCE_URL` to that Service.

The example is a single-server design. It runs one Starmap replica on a
persistent volume. Raise the Starmap replica count only after you move the
store to a lease-capable backend that supplies the CAT-D18 lease and
conditional writes.
