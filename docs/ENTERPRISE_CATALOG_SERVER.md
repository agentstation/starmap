# Enterprise catalog server runbook

This runbook builds one central Starmap catalog server for a Starport fleet.
The central server follows the public catalog channel and serves every replica.
Each replica then reads one internal endpoint instead of GitHub.

Read [ARCHITECTURE.md](ARCHITECTURE.md#connected-catalog-runtime) for every
setting and default. Read [REST_API.md](REST_API.md) for every route. The
[T1 recipe](#t1-recipe-standalone-starmap-server) gives the complete procedure
for one standalone server, with its backup and restore.

## Select the store

The store holds the catalog generations that the server serves.

1. Use a persistent volume for one server. The standalone command-line
   composition is a single-writer design.
2. Use a lease-capable store for two or more active servers. That store must
   supply the CAT-D18 refresh lease and a conditional compare-and-swap on the
   generation record.
3. Do not run two active servers on one shared filesystem volume. The
   filesystem store supplies a conditional head commit but no refresh lease.

The filesystem store guards its head with a lock and a comparison. Each commit
takes the `.commit.lock` file lock and then compares the `current` head with the
expected generation ID. A mismatch returns a conflict and leaves the head as it
was. The store writes the new generation before it replaces the head file.
[`pkg/catalogs/storage/filesystem.go`](../pkg/catalogs/storage/filesystem.go)
holds this commit path.

The store has no refresh lease, and the standalone composition injects no lease
store. The lease decides which replica refreshes, so a shared volume still
supports only one active writer. A shared volume supports an active and passive
pair. Start the standby only after the active server stops.

## Start the server

Start the server with authentication and a bound address:

```bash
starmap serve --auth --host 0.0.0.0 --port 8080
```

The `--auth` flag makes the server require the `API_KEY` value on every
protected route. This legacy check applies only while the state root holds no
administration state. After `starmap admin init`, the server authenticates each
protected route against its managed identities and ignores `API_KEY` completely.
The health and readiness routes stay public, so a probe needs no credential.

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
| Server credential | `API_KEY`, or a managed subscriber credential | The value that a fleet client sends to this server |
| Catalog source token | `STARMAP_CATALOG_SOURCE_TOKEN` | The GitHub token that raises the channel rate-limit budget |
| Provider credentials | `OPENAI_API_KEY` and the other provider names | The acquisition inputs that read provider APIs |

A provider credential is never a server credential. A server credential never
reaches a provider. Store each secret outside the image and outside the repository.

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
export STARPORT_CATALOG_SOURCE_API_KEY=<the server credential>
```

The `STARPORT_CATALOG_SOURCE_URL` value names the versioned API base URL of the
central server. A non-loopback endpoint must use HTTPS.

Set `STARPORT_CATALOG_ACQUISITION_ENABLED` to `false` when a replica must get
its catalog only from the central server. The replica then sends no catalog
request to GitHub or to a provider API.

This setting limits catalog egress only. Inference egress stays with each
gateway. Each Starport replica sends its inference requests straight to its
providers, because the central server serves catalog routes and no inference
route. Allow that provider egress from each gateway. The central server keeps
its own catalog egress to GitHub and to the providers, so this design is not air-gapped.

## Rotate the server credential and read the health routes

The rotation procedure depends on the authentication path of the server.

**Managed identities.** Use this path after you run `starmap admin init` on the
server. Rotate a subscriber credential while the server runs:

1. Send `POST /admin/identities/<id>/rotate` with an administrator credential.
2. Give the overlap in the request body, such as `{"overlap":"10m"}` for ten minutes.
3. Store the new credential from the response. The server returns it only once.
4. Put the new credential in the client secret of each replica.
5. Restart each replica and confirm its readiness.
6. Let the overlap end. The server then refuses the old credential.

During the overlap, the server accepts the old and the new credential. The
maximum overlap is 24 hours. A second rotation fails with a conflict until the
current overlap ends. A zero overlap withdraws the old credential at once.

A catalog event stream ends when its credential expires. It also ends when an
administrator revokes the credential or the server closes. The local
`starmap admin rotate --id <id> --overlap 10m` command applies the same overlap.
That command needs exclusive state access, so stop the server before you run it.
[SERVER_ADMINISTRATION.md](SERVER_ADMINISTRATION.md) describes every
administration route.

**Legacy `API_KEY`.** Use this path only when the server has no administration
state. The server then compares each request with one `API_KEY` value, so the
legacy path has no overlap. Catalog transport stops for each replica until the
replica and the server hold the same key:

1. Change `API_KEY` on the server and restart the server.
2. Put the new key in the client secret of each replica.
3. Restart each replica and confirm that it reconnected.
4. Remove the old key from every secret store.

Move to managed identities when the fleet cannot accept that interruption.

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
persistent volume. The filesystem store refuses a stale head commit, but it
supplies no refresh lease. Raise the Starmap replica count only after you move
the store to a lease-capable backend that supplies the CAT-D18 lease and
conditional writes.

## T1 recipe: standalone Starmap server

This recipe runs one Starmap server on one host. It covers setup, verification,
offline backup, restore, and operation without GitHub.

### Audience and outcome

This recipe serves an operator who runs one Starmap server on one host. The
server keeps its catalog generations in the filesystem catalog store. It keeps
identity and evidence records in a private runtime directory. The recipe needs
no SQL database. The outcome is one server with managed identities, an offline
backup, and a tested restore.

### Prerequisites

- One host with a persistent local disk.
- The `starmap` binary, or the container image from [DOCKER.md](DOCKER.md).
- One service user that owns the product root.
- A secret manager for the administrator and subscriber credentials.
- The `curl` and `tar` commands on the host.

### Configuration file and loading order

Set `STARMAP_HOME` to one absolute directory, such as `/var/lib/starmap` for a
service. Starmap then uses its `config`, `data`, `state`, and `cache` children.
The primary configuration file is `config.yaml` in the configuration root. Use
the `--config` flag or the `CONFIG` variable to select another file. Starmap
reads a dotenv file only when the `--env-file` flag names that file.

Starmap selects each value in this order:

1. An explicit command flag.
2. The process environment, including an explicit empty value.
3. Explicit dotenv files, with the last listed file first.
4. The selected configuration file.
5. The runtime default.

By default, each selected configuration file and dotenv file must be a private
regular file. [CATALOG_SETTINGS.md](CATALOG_SETTINGS.md#value-selection) lists
each catalog setting. [CLI.md](CLI.md#node-directories-and-identity) describes
the product roots and the runtime identity files.

### Mounts and store ownership

Mount one persistent directory at the `STARMAP_HOME` path. Give that directory
to the service user with the `0700` mode. Startup refuses a runtime directory
with another owner or a wider mode, and it does not repair them.

| Path under `STARMAP_HOME` | Owner | Contents | In the backup |
| --- | --- | --- | --- |
| `config/config.yaml` | Operator | Selected configuration | Yes |
| `data/catalog/workspace` | Operator | Optional reviewed YAML catalog input | Yes |
| `data/catalog/baseline` | Starmap | Export of the verified embedded catalog | Yes |
| `state/catalog` | Filesystem catalog store | The `current` head, `.commit.lock`, and immutable generations | Yes |
| `state/catalog/runtime/default` | Runtime directory | Owner record, instance seed, and retained evidence | Yes |
| `state/admin` | Administration | Identities, audit history, and operation receipts | Yes |
| `state/credentials` | Credential policy | Credential selection policy | Yes |
| `state/migrations` | Runtime migration | Migration journal | Yes |
| `cache` | Source cache | Source downloads that acquisition can rebuild | No |

The filesystem catalog store and the runtime directory share the state root. No
SQL database holds any part of this state. Run `starmap config paths --output json`
to list each selected path with its retention policy. Keep the `cache` root in the
backup only when it holds a models.dev checkout with local edits.

Do not share the state root between two processes. The runtime directory and
the administration state each take an exclusive owner lock.

### Start and verify the server

Initialize the identities, then start the server:

```bash
export STARMAP_HOME=/var/lib/starmap
starmap admin init --id operator
export STARMAP_ADMIN_TOKEN=<administrator credential>
starmap admin create --id gateway --role subscriber
starmap serve --host 0.0.0.0 --port 8080
```

Each `admin` command writes its credential to standard output as JSON. Move each
credential into the secret manager at once. The state file keeps only a digest
of each credential. The administration state turns on authentication, so the
server needs no `--auth` flag here.

Verify the server from the host:

```bash
curl -fsS http://localhost:8080/health
curl -fsS http://localhost:8080/api/v1/ready
curl -fsS -H "Authorization: Bearer $SUBSCRIBER_CREDENTIAL" http://localhost:8080/api/v1/providers
curl -fsS -H "Authorization: Bearer $STARMAP_ADMIN_TOKEN" http://localhost:8080/admin/status
starmap config paths --inspect --output wide
```

The readiness route reports the `ready` status. A protected route returns the
HTTP 401 status without a credential. An administrator route returns the HTTP
403 status for a subscriber credential.

### Take an offline backup

1. Stop the server.
2. Archive the product root without the cache.
3. Keep the archive as private as the credentials.
4. Start the server again.

```bash
tar -C "$STARMAP_HOME" --exclude ./cache -cpf starmap-backup.tar .
```

Starmap needs the identities, audit history, receipts, and accepted catalog
authority from one moment. One archive of a stopped server keeps them together.
Do not copy the state root while the server runs. The archive holds credential
digests and the instance seed, so give it the same protection as a secret.

### Restore from the backup

1. Stop every process that uses the product root.
2. Move the damaged root aside, and keep it until the restore passes.
3. Create an empty root with the `0700` mode for the service user.
4. Extract the archive with its file modes.
5. Run `starmap admin status` and compare its revision with your records.
6. Start the server and run the verification commands again.

```bash
tar -C "$STARMAP_HOME" -xpf starmap-backup.tar
```

The restored server serves the same generation ID as the server at backup time.
Each administrator and subscriber credential from backup time still works.

### Operate without GitHub

The default `public` source reads the public channel on GitHub. Select one of
these sources to stop every GitHub request:

- `embedded` serves the catalog that the binary compiles in.
- `file` reads a local catalog file from the absolute path that `STARMAP_CATALOG_SOURCE_URL` names.

```bash
export STARMAP_CATALOG_SOURCE=embedded
export STARMAP_CATALOG_ACQUISITION_ENABLED=false
export STARMAP_CATALOG_NETWORK_MODE=offline
```

Turn off acquisition as well. The CLI and server otherwise get metadata from
models.dev and from the configured provider APIs. The `offline` network mode
lets only the embedded and file readers read a source. With these settings, the
server sends no catalog request to any host.

With the `embedded` source, readiness reports `true` in the `fallback` field
and `awaiting_source` in the `fallback_reason` field. That report is normal in
this mode, so do not alert on it here.

### Failure handling

| Symptom | Cause | Action |
| --- | --- | --- |
| `serve` exits and names `starmap admin init` | An internal catalog without administration state | Run `starmap admin init`, then start the server |
| A local `admin` command fails | The running server holds the state | Stop the server, or use the HTTP route |
| Startup refuses the state root | Wrong owner or wrong mode | Correct the owner and modes. Startup does not repair them. |
| Lost administrator credential | The secret store lost the value | Stop the server and run `starmap admin recover --id operator` |
| Missing identity file after initialization | Damaged state root | Restore the last consistent backup |
| `fallback` stays `true` with a GitHub source | No upstream reply yet, or every read failed | Read `fallback_reason` and the source fields in readiness |

### Related settings

| Setting | Purpose |
| --- | --- |
| `STARMAP_HOME` | Parent of the four product roots |
| `STARMAP_CONFIG_DIR`, `STARMAP_DATA_DIR`, `STARMAP_STATE_ROOT`, `STARMAP_CACHE_DIR` | Replace one product root |
| `STARMAP_CATALOG_STORE_PATH` | Moves the filesystem catalog store |
| `STARMAP_STATE_DIR` | Names the runtime directory for this process |
| `STARMAP_INSTANCE_ID` | Names this instance and its default runtime directory |
| `STARMAP_ADMIN_TOKEN` | Administrator credential for local `admin` commands |
| `STARMAP_SERVER_HOST`, `STARMAP_SERVER_PORT` | Listener address |
| `STARMAP_CATALOG_SOURCE`, `STARMAP_CATALOG_SOURCE_URL` | Catalog source kind and location |
| `STARMAP_CATALOG_ACQUISITION_ENABLED` | Automatic acquisition switch |
| `STARMAP_CATALOG_NETWORK_MODE` | The `configured` or `offline` network mode |

### Runnable checks

These Go tests prove the recipe. Run them from the repository root:

```bash
go test -race -count=1 ./runtime -run '^(TestOfflineRestorePreservesAdministrationAndCatalogAuthority|TestOfflineStartupRetainsEmbeddedGeneration|TestOfflineBaselineStorageFailureRefusesStartup|TestFileSourceRestartRetainsReceiptAndAcceptsChangedPayload)$'
go test -race -count=1 ./internal/cli/app -run '^(TestApplicationUsesExplicitDeploymentRootsWithoutHome|TestServiceConfigurationDoesNotLoadIncidentalDotenv|TestExplicitDotenvPrecedenceAndConflictDiagnostics|TestCommandFlagsOverrideInvalidEnvironmentSettings)$'
go test -race -count=1 ./server/administration -run '^(TestDurableCredentialRotationAndRevocation|TestStreamingCredentialRevocationAndExpiryCancelRequests)$'
go test -race -count=1 ./server -run '^TestAdministratorHTTPRotationAndRevocation$'
go test -race -count=1 ./pkg/catalogs/storage -run '^TestCatalogStoreConcurrentSameBaseCAS$'
```

| Test | Proves |
| --- | --- |
| `TestOfflineRestorePreservesAdministrationAndCatalogAuthority` | An offline copy of the state root restores identities, history, and the accepted head |
| `TestOfflineStartupRetainsEmbeddedGeneration` | An offline start keeps the embedded generation |
| `TestOfflineBaselineStorageFailureRefusesStartup` | A baseline storage failure stops an offline start |
| `TestFileSourceRestartRetainsReceiptAndAcceptsChangedPayload` | The `file` source keeps its receipt through a restart |
| `TestApplicationUsesExplicitDeploymentRootsWithoutHome` | Explicit product roots work without a home directory |
| `TestServiceConfigurationDoesNotLoadIncidentalDotenv` | Starmap ignores a dotenv file that no flag names |
| `TestExplicitDotenvPrecedenceAndConflictDiagnostics` | A later dotenv file wins, and the process environment beats each file |
| `TestCommandFlagsOverrideInvalidEnvironmentSettings` | A flag replaces an environment value |
| `TestDurableCredentialRotationAndRevocation` | Rotation keeps the old key for the overlap only |
| `TestStreamingCredentialRevocationAndExpiryCancelRequests` | An event stream ends on credential expiry, on revocation, and on close |
| `TestAdministratorHTTPRotationAndRevocation` | Only an administrator can rotate, and a zero overlap refuses the old credential at once |
| `TestCatalogStoreConcurrentSameBaseCAS` | Two filesystem stores on one root give one commit and one conflict |

## Latency profile

No measured latency profile exists for the T1 recipe. This runbook therefore
gives no latency value for startup, readiness, publication, or replica catch-up.
Plan task CSP22 measures the latency of each deployment topology. Until that
measurement exists, base alerts on the readiness freshness fields.
