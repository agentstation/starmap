# Starmap and Starport storage review

Reviewed: 2026-09-05. Current code has stable storage owners but an incomplete platform and configuration contract.
The proposed specification improves that contract. Several details still need implementation or a more precise decision.

This review answers where data lives, which service owns it, and which deployment uses it.
It does not activate the [implementation plan](../../plans/starport-production-catalog-plan.html) or change product code.
The [engineering specification](ENGINEERING_SPEC.md) remains the proposed contract.

The inspected revisions are Starmap `4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225`
and Starport `042eb97851496ecdd1ef20bfa44fe3d84b86b2d8`.
References below use those immutable revisions.

**Storage roles.** A catalog source, catalog store, runtime snapshot, and response cache have different owners and lifetimes.
Selecting PostgreSQL does not move Starport's catalog or credentials out of its KV store.
Selecting Valkey does not move relational data or uploaded bytes out of local files.

| Concern | Current owner and storage | Persistence requirement |
| --- | --- | --- |
| Embedded catalog | Starmap module and compiled binary | Changes only with a new build or module version |
| Published catalog | GitHub release artifacts and channel | Distribution input, separate from installed state |
| Standalone accepted catalog | Starmap filesystem catalog adapter | Preserve accepted generations and current pointer |
| Starport accepted and candidate catalogs | Starmap contract over Starport's selected KV adapter | Preserve generations, payload chunks, heads, and history |
| Active catalog and routing view | Immutable process memory | Rebuild from verified, permitted persisted state |
| Source evidence and instance identity | Local catalog runtime directory | Required for restart, replay protection, and refresh ownership |
| Gateway accounts, API keys, provider credentials | Concept repositories over Badger or Valkey | Durable records, separate from SQL identity relationships |
| Usage, limits, presets, jobs, file metadata | Concept repositories over Badger or Valkey | Preserve records and references during migration |
| Users, teams, memberships, account grants | SQLite, PostgreSQL, or MySQL | Durable relational state |
| Account templates, incident transitions, audit | SQLite, PostgreSQL, or MySQL | Durable relational state |
| Uploaded bytes and job assets | Filesystem or S3-compatible object store | Preserve bytes referenced by KV records |
| Exact inference responses | Response repository over the cache manager | Disposable results with expiry and account isolation |
| Model and endpoint response projections | Ristretto process cache | Disposable derived views |
| Document extraction results | Selected KV store under `extraction:` | Disposable parsed text with scoped expiry |
| Secret-manager material | Role-specific resolver and external secret service | Secret references and recovery access remain necessary |
| Deployment configuration | Current files and environment, plus specific stored settings | Complete shared SQL authority is proposed |

The [application composition][sp-app] opens KV, SQL, and blob storage separately.
It opens and migrates SQL even without an identity provider.
The [SQL migrations][sp-sql-migrations] show which concepts currently use relational tables.

**Current user directories.** Starport uses Go's `os.UserConfigDir()` for its configuration root.
Its default databases and file bytes sit under that root's `data` directory.
Starmap uses a home-directory tree on all three platforms.

| Platform | Starmap root today | Starport configuration root today | Starport catalog runtime state today |
| --- | --- | --- | --- |
| Linux | `~/.starmap` | `$XDG_CONFIG_HOME/starport`, otherwise `~/.config/starport` | `$XDG_STATE_HOME/starport/catalog`, otherwise `~/.local/state/starport/catalog` |
| macOS | `~/.starmap` | `~/Library/Application Support/starport` | `$XDG_STATE_HOME/starport/catalog`, otherwise `~/.local/state/starport/catalog` |
| Windows | `%USERPROFILE%\.starmap` | `%APPDATA%\starport` | `$XDG_STATE_HOME/starport/catalog`, otherwise `%USERPROFILE%\.local\state\starport\catalog` |

Windows joins use native path separators. An XDG override is unusual on Windows but the current state resolver still reads it.
These paths describe code behavior, not native platform qualification.
See [Starport paths][sp-paths], [catalog state resolution][sp-state], and [Starmap defaults][sm-defaults].
Go documents the configuration parents in [UserConfigDir](https://pkg.go.dev/os#UserConfigDir).

**Current Starport files.** In the next table, `C` means the resolved Starport configuration root.
`R` means its separately resolved catalog runtime directory.
Explicit leaf settings can change several locations.

| File or directory | Current default | When Starport uses it |
| --- | --- | --- |
| Operator configuration | `C/config.env` | Normal configuration loading, including `serve` |
| General data root | `C/data/` | Default parent for managed local data |
| Embedded KV database | `C/data/badger/` | Normal operation with `STORAGE_MODE=badger` |
| Embedded SQL database | `C/data/sqlite/starport.db` | Normal operation with `STORAGE_SQL_MODE=sqlite` |
| SQLite sidecars | `starport.db-wal`, `starport.db-shm` beside the database | SQLite WAL operation, when the engine needs them |
| Uploaded objects | `C/data/files/objects/<hash-prefix>/<hash-prefix>/<hash>` | Filesystem blob backend |
| Incomplete blob writes | `C/data/files/staging/put-*` | A filesystem upload before atomic promotion |
| Local console administrator token | `C/data/local-admin-token.json` | Local console bootstrap and token recovery |
| Welcome marker | `C/data/welcomed` | Tracks whether the first-use greeting has appeared |
| Instance seed | `R/instance-seed` | Stable runtime identity and schedule spread |
| Retained source layer | `R/catalog-runtime/source.json` | Restart and catalog reconciliation |
| Retained provider layers | `R/catalog-runtime/providers/<provider-id>.json` | Restart and provider evidence |
| GitHub discovery state | `R/github-catalog-source/<channel-hash>.json` | ETag, replay sequence, and verified release reference |
| Application logs | Standard output by default | File output requires an explicit logging selection and path |
| Usage export | No destination by default | Optional NDJSON file or HTTP destination |

The runtime layer layout comes from [Starmap layers][sm-layers], [scheduler state][sm-scheduler], and [GitHub discovery state][sm-github-state].
The blob adapter hashes object keys for filesystem names. Its hash is not a promise of content-addressed deduplication.
See [filesystem blobs][sp-blob] for the exact layout.

Badger owns multiple internal files, rather than one `badger.db` file.
Operators must treat the entire database directory as engine-managed state.
Individual table, value-log, lock, and manifest files are not separate application configuration.

`starport init` creates a private local configuration and an initial gateway key in Badger.
Its generated `config.env` currently contains `STARPORT_SECURITY_MASTER_KEY`.
That file therefore contains a secret, even though the proposed shared configuration stores references.
The setup file mode is `0600`, with private setup directories.

POSIX mode bits do not establish Windows ACL behavior. Native ACL tests remain required.
See [local setup][sp-setup] and [local token storage][sp-token].
The local token has no independent environment override. Changing `STARPORT_CONFIG_DIR` moves its managed default location.

`starport init --configured-storage` initializes the first gateway key in selected storage.
It does not migrate an existing local installation or initialize the proposed shared configuration authority.
See [initialization commands][sp-cli].

**Current Starmap files.** Starmap does not need Badger, Redis, Valkey, or SQL for its normal standalone catalog server.
It supplies a filesystem catalog adapter and keeps the serving snapshot in memory.
Its library also exposes memory and conditional object-storage catalog adapters.
An adapter interface alone does not qualify an active-active server deployment.

| File or directory | Current default | Role |
| --- | --- | --- |
| Operator configuration | `~/.starmap/config.yaml` | Viper configuration for supported application fields |
| Human catalog workspace | `~/.starmap/catalog/` | Authored catalog YAML, distinct from machine state |
| Catalog store root | `~/.starmap/state/catalog/` | Standalone immutable catalog storage |
| Current catalog pointer | `<store>/current` | Selected generation ID |
| Catalog commit lock | `<store>/.commit.lock` | Coordinates filesystem head changes |
| Generation manifest | `<store>/generations/<sha256-of-generation-id>/manifest.json` | Generation identity and evidence |
| Generation payload | `<store>/generations/<sha256-of-generation-id>/catalog.json` | Canonical catalog bytes |
| Runtime root | `~/.starmap/state/runtime/` | Same layer, seed, and GitHub discovery layout described above |
| models.dev HTTP cache | `~/.starmap/cache/models.dev/api.json` | Cached source evidence |
| models.dev cache metadata | `~/.starmap/cache/models.dev/api.json.metadata.json` | Origin, checksum, validators, and validation time |
| models.dev Git checkout | `~/.starmap/sources/models.dev-git/` | Optional source checkout and build inputs |
| Logs directory constant | `~/.starmap/logs/` | Declared default, not proof that every command writes logs there |

The [filesystem catalog adapter][sm-filesystem] defines manifests, payloads, locks, and pointers.
It hashes the generation ID for its directory name. The manifest retains the original ID.
[HTTP source storage][sm-http] and [Git source storage][sm-git] have their own paths.
The baseline file export required by the PRD remains implementation work.
Do not infer that every passive read currently materializes these files.

**Configuration files and precedence.** The current products do not yet share one complete file schema or loader.
Their existing loading rules must remain visible during migration.

| Product and mode | Current source order and behavior |
| --- | --- |
| Starmap application | Explicit flags, environment, loaded dotenv values, supported YAML fields, then defaults |
| Starmap canonical catalog settings | Registered flags and process environment, plus selected legacy remote mappings |
| Starport normal loader | Explicit overrides, process environment, `config.env`, then defaults |
| Starport development loader | Process environment and explicit overrides, followed by development constraints. It skips `config.env`. |
| Starport stored authentication mode | A specific KV-backed console setting. Explicit configuration can override it. |
| Proposed shared deployment configuration | Bootstrap connects the stores. An initialized shared revision then owns deployment settings. |

Starmap loads working-directory `.env` before `.env.local` with `godotenv.Load`.
That function preserves already-set variables. The earlier `.env` value therefore wins when both files define an absent environment variable.
The source comment claiming that `.env.local` overrides `.env` does not match that order.
This needs a documented and tested migration rule. See [Starmap configuration][sm-config].

The canonical Starmap catalog loader does not map every catalog setting from YAML.
See [catalog composition][sm-runtime]. A YAML configuration file's existence does not establish complete setting coverage.
Starport does not load an incidental working-directory `.env` by default.
Its [loader][sp-loader] reads the platform file without changing the process environment.

`STARPORT_CONFIG_DIR` must be available before Starport finds `config.env`.
Putting that selector inside the file cannot change which file the loader selected.
The current XDG state lookup also reads the process environment directly.
A similarly named value inside `config.env` is not a substitute for that process environment value.

Provider credential references can select environment values, explicit files, AWS Secrets Manager, Google Secret Manager, Azure Key Vault, Vault, or OpenBao.
Those external files and services have operator-selected locations, not an automatic product catalog directory.
See [credential reference types][sp-credential-references]. Keep acquisition, inference, subscriber, and storage-access credentials separate.
A shared database cannot be the sole source of the credentials needed to connect to itself.

The complete shared configuration repository remains an engineering proposal.
The existing [authentication-mode repository][sp-authmode] is a narrower stored-setting exception.
It does not establish the desired revision, audit, controller, and replica-application contract for all settings.

**Current storage selectors.** All Starport names in this table are complete environment variable names.
These select adapters and locations. They do not copy records between stores.

| Selector | Current role or default |
| --- | --- |
| `STARPORT_CONFIG_DIR` | Absolute configuration root override |
| `STARPORT_STORAGE_MODE` | `badger` by default, or `valkey` |
| `STARPORT_STORAGE_BADGER_PATH` | Badger directory override |
| `STARPORT_STORAGE_BADGER_SYNC_WRITES` | Defaults to `false` during normal serving |
| `STARPORT_STORAGE_SQL_MODE` | `sqlite` by default, or `postgres` or `mysql` |
| `STARPORT_STORAGE_SQL_SQLITE_PATH` | SQLite file override. An explicitly empty path selects memory storage. |
| `STARPORT_STORAGE_SQL_POSTGRES_URL` | PostgreSQL connection URL |
| `STARPORT_STORAGE_SQL_MYSQL_DSN` | MySQL connection string |
| `STARPORT_STORAGE_VALKEY_URL` | Defaults to `valkey://localhost:6379` |
| `STARPORT_STORAGE_VALKEY_PASSWORD` | Separate Valkey password |
| `STARPORT_FILES_BACKEND` | `filesystem` by default, or `objectstore` |
| `STARPORT_FILES_PATH` | Filesystem blob root override |
| `STARPORT_FILES_OBJECT_STORE_BUCKET` | Object-store bucket |
| `STARPORT_FILES_OBJECT_STORE_REGION` | Object-store region |
| `STARPORT_FILES_OBJECT_STORE_ENDPOINT` | Optional non-AWS endpoint |
| `STARPORT_FILES_OBJECT_STORE_PREFIX` | Deployment prefix inside the bucket |
| `STARPORT_FILES_OBJECT_STORE_ACCESS_KEY_ID` | Optional static access key |
| `STARPORT_FILES_OBJECT_STORE_SECRET_ACCESS_KEY` | Optional static secret key |
| `STARPORT_CATALOG_STATE_DIR` | Explicit local runtime state directory |
| `STARPORT_CATALOG_WORKSPACE_PATH` | Optional human catalog workspace |
| `STARPORT_CATALOG_SOURCE` | `public`, `github`, `starmap`, `file`, or `embedded` |
| `STARPORT_CATALOG_SOURCE_URL` | Internal server URL or file source identity |
| `STARPORT_CACHE_ENABLED` | Enables the main response and projection cache manager |
| `STARPORT_LOGGING_OUTPUT` | `stdout` by default, with configured alternatives |
| `STARPORT_LOGGING_FILE_PATH` | Optional application log file |
| `STARPORT_TELEMETRY_USAGE_EXPORT` | Optional usage export file or HTTP destination |

Object storage requires a bucket and a region or endpoint.
Both static credential fields absent select the AWS credential chain.
A partial static credential pair fails validation. See [blob configuration][sp-files-config].

Starmap currently exposes `--config`, `STARMAP_CATALOG_WORKSPACE_PATH`, and `STARMAP_STATE_DIR` for the relevant selections.
The older loader also reads unprefixed `CONFIG` and `CATALOG_PATH` values.
Canonical workspace selection wins over the older catalog-path field.
Its catalog store root still comes from the standalone default path.

`STARMAP_CATALOG_STORE_PATH` is a proposed addition, not a current replacement for that default.
`STARMAP_HOME`, `STARPORT_HOME`, and the full root-override family are also proposed.

**How Badger and memory caching work.** Badger is an embedded transactional KV database inside the Starport process.
Normal mode stores its data on disk and uses memory buffers, indexes, and a block cache to accelerate access.
Development mode enables Badger's explicit in-memory option and loses database contents when the process ends.
The [Badger API](https://pkg.go.dev/github.com/dgraph-io/badger/v4@v4.9.6#Options.WithInMemory) documents that distinction.

The configured KV adapter contains authoritative records as well as disposable cache entries.
Deleting its contents is not a cache-clear operation: it also removes catalog state, accounts, gateway keys, and other durable records.
The alternative Valkey adapter takes the same KV role. Badger and Valkey are not two mandatory layers running together.

| Layer | Local persistent deployment | Replicated deployment |
| --- | --- | --- |
| Catalog read model | Immutable process memory | Immutable memory snapshot in every replica |
| Catalog persistence and gateway KV | Disk-backed Badger | Shared Valkey |
| Exact response cache | Ristretto memory over the selected KV store | Direct shared KV cache with the current Valkey adapter |
| Model and endpoint projections | Local Ristretto cache | Local Ristretto cache per replica |
| Starmap HTTP response cache | Local `go-cache` memory | Local memory in each standalone server |
| Document extraction cache | Selected KV store | Shared KV store |

The [cache manager][sp-cache-manager] chooses direct shared response caching when the store supplies Pub/Sub.
It still creates a local projection cache. [Starmap HTTP caching][sm-server-cache] is a separate mechanism.
The optional semantic cache adds an embedding-based lookup and does not replace catalog storage.

Current Starport defaults include a 256 MiB response-cache budget and a 16 MiB projection-cache budget.
Responses default to one hour, with a 1 MiB item bound. The hybrid memory tier uses up to 30 minutes.
Badger separately configures a 256 MiB block cache, five memtables, and 64 MiB per memtable.
These values are capacities and tuning inputs, not a measured fixed RSS or a whole-process memory bound.
See the [Badger adapter][sp-badger] and [runtime storage projection][sp-storage-config].

Normal Badger serving defaults to unsynchronized writes. Local initialization separately enables synchronized writes.
Synchronized writes address hard-reboot durability. An unsynchronized disk database is still persistent storage, unlike in-memory mode.
Production must choose and test its accepted-write loss policy.
See [Badger SyncWrites](https://pkg.go.dev/github.com/dgraph-io/badger/v4@v4.9.6#Options.WithSyncWrites).

**SQLite and shared SQL.** SQLite runs inside Starport and opens one connection with WAL, foreign keys, and a five-second busy timeout.
An empty configured SQLite path opens a process-memory database.
The proposed production recipe must reject accidental ephemeral SQL outside an explicit temporary mode.
See [SQLite opening][sp-sql-open].

The WAL can contain committed data that is absent from the main database file.
Use a database-aware backup or a verified stopped/checkpointed procedure.
Do not copy only `starport.db` from a running service and call it a complete backup.
WAL also does not support a database shared across hosts over a network filesystem.
See [SQLite WAL](https://www.sqlite.org/wal.html).

PostgreSQL and MySQL replace SQLite for the relational concepts only.
The current network adapters open a pool and check connectivity at startup.
The primary proposed fleet recipe is PostgreSQL. MySQL remains an alternative requiring its own qualification.
The database service owns its physical files. Starport configures a connection, not a local PostgreSQL or MySQL data directory.

**Valkey and Redis.** Valkey replaces local Badger when replicas need shared KV records and coordination.
The current adapter also accepts a simple `redis://host:port` spelling under `STORAGE_MODE=valkey`.
That does not qualify every Redis version, URI form, authentication mode, or Cluster deployment.

Valkey keeps data in memory and needs an explicit persistence policy for authoritative records.
Its RDB and AOF policies have different recovery behavior.
For example, AOF synchronization once per second permits an acknowledged-write loss interval during a disaster.
See [Valkey persistence](https://valkey.io/topics/persistence/).

Current Starport uses one KV handle for authoritative records and response-cache entries.
A separate prefix does not isolate eviction, memory pressure, or a failed backend.
A production recipe must protect authoritative records from cache eviction and bound cache growth.
A separate cache endpoint would require an additional application composition contract.
It is not an existing configuration switch.

Independent deployments must not share an unscoped KV keyspace.
Current code uses fixed catalog head and lease keys.
Its configuration projection does not expose a deployment prefix or database selector.
Use a dedicated endpoint for each current deployment until namespace support is explicit and tested.
See [catalog keys][sp-generations], [catalog lease][sp-lease], and [Valkey construction][sp-valkey].

**Deployment recipes.** Choose storage from process count, durability, and recovery needs, rather than company size alone.
An enterprise evaluation on one laptop can use the same local recipe as an individual developer.

| Deployment | KV | SQL | File bytes | Configuration and catalog behavior |
| --- | --- | --- | --- | --- |
| Temporary `starport dev` | In-memory Badger | In-memory SQLite | Session scratch filesystem | Skips `config.env`. Catalog state defaults to scratch. |
| Persistent developer machine | Local Badger | Local SQLite | Local filesystem | `config.env` plus environment. Public catalog or explicit source. |
| Startup on one server | Durable local Badger | Durable local SQLite | Local disk or object store | Dedicated service account, explicit paths, backups, and recovery objectives. |
| Startup or enterprise fleet | Shared Valkey | Shared PostgreSQL | Shared object store | Node-local bootstrap and runtime identity. Shared configuration remains proposed. |
| Enterprise with internal Starmap | Same local or fleet choice | Same local or fleet choice | Same local or fleet choice | Starmap adds catalog authority. It does not replace gateway databases. |
| Standalone Starmap server | Filesystem catalog store | None required | Catalog files and source evidence | Own config, acquisition credentials, and subscriber administration. |
| Offline or air-gapped setup | Same persistent storage choices | Same relational choice | Local or internal storage | Verified baseline/import and controlled egress. Inference needs separate compute. |

`starport dev` creates a `starport-dev-*` directory under the operating system's temporary directory.
It holds `files/` and normally `catalog-state/` and removes the scratch root on normal close.
A crash can leave scratch files behind. An explicit catalog state directory remains an intentional persistence exception.
The development runtime can read an existing local admin token without writing one.

Development constraints reset KV and SQL, but currently preserve the selected blob backend.
An explicit object-store environment selection can therefore keep external blob storage active.
Scratch-directory creation alone does not prove that all development bytes remain local.
See [development composition][sp-dev] and [development configuration][sp-storage-config].

Multiple processes on one host are already a replicated gateway problem when they share one deployment.
Changing only the port does not create a safe second persistent instance.
Separate deployments need separate config, KV, SQL, blob, and runtime identity scopes.
Replicas of one deployment need shared records but distinct runtime identities.

A container's writable layer is not a persistence contract.
The local recipe needs persistent mounted data and state.
A fleet needs shared services plus an explicitly separate runtime directory for each replica.
CPU architecture does not change these path rules. Each advertised OS and architecture still needs native storage and recovery evidence.

**Current container examples.** The Starport Dockerfile does not use one common data root for every store.
Its configuration root is `/var/lib/starport/config/starport` because the loader appends `starport` to `XDG_CONFIG_HOME`.
It separately overrides Badger to `/var/lib/starport/data/badger`.
SQLite, file bytes, and the local token still default beneath the configuration root's `data` directory.
Catalog runtime state uses its separate state resolver.
See the [Starport Dockerfile][sp-docker].

The shipped [Starport Compose example][sp-compose] persists the Valkey `/data` directory only.
It does not select PostgreSQL or mount Starport's SQLite, blob, token, or runtime paths.
Container recreation can therefore retain KV records while losing their local relational and blob state.
That example does not establish the complete production fleet recipe.
Its explicit `env_file` imports `.env` into the container environment, outside Starport's own loader.

The [Starmap Compose example][sm-compose] mounts `/home/nonroot` as its writable data volume.
That preserves the default `.starmap` tree for its single server and keeps `/tmp` on tmpfs.
Replicas still need separate identity scopes. A mounted volume alone does not supply fleet coordination.

An internal Starmap subscriber should point directly to that server.
The server can follow GitHub while subscribers have no direct GitHub access.
Turning off public pulls does not remove the need to choose acquisition, manual refresh, import, and inference egress separately.
D1, D2, and D13 govern the proposed authoritative startup and outage behavior.

**Proposed canonical layout.** The specification separates configuration, durable data, process state, and disposable cache.
These paths are design targets, not locations that the current binaries already honor.

| Platform | Configuration | Durable data | State | Cache |
| --- | --- | --- | --- | --- |
| Linux user | `~/.config/<product>` | `~/.local/share/<product>` | `~/.local/state/<product>` | `~/.cache/<product>` |
| macOS user | `~/Library/Application Support/<product>/config` | `~/Library/Application Support/<product>/data` | `~/Library/Application Support/<product>/state` | `~/Library/Caches/<product>` |
| Windows user | `%APPDATA%\<product>` | `%LOCALAPPDATA%\<product>\data` | `%LOCALAPPDATA%\<product>\state` | `%LOCALAPPDATA%\<product>\cache` |

Linux honors the corresponding XDG root overrides before those fallbacks.
The [XDG specification](https://specifications.freedesktop.org/basedir/0.8/) defines the root roles and absolute-path requirement.
The exact product subdirectories are our proposed choices.

| Installation | Proposed service roots |
| --- | --- |
| Linux service | `/etc/<product>`, `/var/lib/<product>`, `/var/cache/<product>`, with explicit data and state children |
| macOS service | Explicit `/Library/Application Support/<product>` tree and service-owned permissions |
| Windows service | ACL-protected `%ProgramData%\<product>` tree |
| Container | Explicit read-only configuration and writable data/state mounts, or shared services |

The final file contract must name one primary file per product and define any alternate format's precedence.
The current specification still says `config.yaml` or existing `config.env`.
My recommendation is to retain Starmap's YAML and Starport's dotenv compatibility until engineering defines a deliberate schema migration.
Do not add automatic searches across both formats or both product directories.

For shared deployment settings, PostgreSQL remains the proposed authority.
Bootstrap files and environment still provide connection addresses, trust, node identity, and secret access.
Once initialized, the shared revision wins over stale local deployment values.
An outage does not switch authority back to a file.

**Findings that need work.** These findings supplement the earlier audit.
The task column names the existing owning area. It does not mark that task complete or alter its acceptance count.

| ID | Priority | Finding and required correction | Owning task area |
| --- | --- | --- | --- |
| SR01 | P1 | Runtime directories default per user, not per process. Give every persistent replica a distinct seed and directory. | CSP2, CSP8, CSP11 |
| SR02 | P1 | Valkey construction strips URI prefixes without complete URL or TLS setup. Define credentials, CA verification, timeouts, and supported endpoint forms. | CSP12, CSP15 |
| SR03 | P1 | Fixed KV keys can collide across deployments sharing the default keyspace. Define deployment scope or require dedicated services. | CSP11, CSP12, CSP15 |
| SR04 | P1 | Durable and cache records share one backend. Specify eviction isolation, capacity, retention, and acknowledged-write loss. | CSP12, CSP13, CSP15 |
| SR05 | P2 | Platform roots remain inconsistent. Windows default data lives under roaming configuration, and macOS runtime state uses an XDG-style path. | CSP2, CSP8 |
| SR06 | P2 | Path normalization differs by field. SQLite and runtime overrides do not use the same config-root resolution as Badger and blobs. | CSP2, CSP8 |
| SR07 | P2 | `config paths` reports default managed paths, not every effective override. Its text output omits SQL, blobs, tokens, and catalog state. | CSP8, CSP16, CSP17 |
| SR08 | P2 | Some storage knobs do not reach adapters. Wire them, reject them, or remove their advertised effect. | CSP12, CSP15 |
| SR09 | P2 | Cache capacities and TTLs are internal defaults in application composition. Expose bounded settings through the canonical descriptor contract. | CSP1, CSP8, CSP16 |
| SR10 | P2 | Hybrid cache refill grants a fresh local TTL without checking the backing entry's remaining lifetime. Add expiry-preserving refill tests. | CSP12, CSP15 |
| SR11 | P2 | Starmap dotenv order conflicts with its comment, and YAML does not cover the complete canonical catalog schema. | CSP1, CSP7 |
| SR12 | P2 | Target file names omit several operational artifacts and leave alternate configuration formats ambiguous. Complete the file manifest before migration. | CSP2, CSP8, CSP16 |
| SR13 | P1 | The Starport Compose example persists Valkey but leaves SQL, blob, token, and runtime files in the container. Qualify a complete persistence recipe. | CSP12, CSP19, CSP22 |
| SR14 | P1 | Development mode preserves an explicit object-store backend while creating local scratch. Guard or clearly declare that persistent-storage exception. | CSP0.2, CSP8, CSP12 |

SR02 and SR08 follow [Valkey construction][sp-valkey] and [storage projection][sp-storage-config].
That projection omits connection-count and dial/idle timeout fields.
Read/write timeout fields reach the internal configuration but not `valkey.ClientOption` in `OpenValkey`.
OpenValkey stores and logs the Cluster flag without configuring the client's selection behavior from it.

RuntimeStorage forwards Badger's compression choice. OpenBadger does not apply that choice to its opening options.
Its configured GC interval and discard ratio do not reach the adapter.
The adapter instead uses five minutes and `0.5` directly.
These are source-verified gaps, not claims about a measured production outage.

SR09 follows [cache composition][sp-cache-compose], which passes an empty `ManagerConfig`.
Struct tags in the separate manager type do not prove public environment support.
SR10 follows [hybrid cache refill][sp-hybrid]. Current selected tests do not establish expiry preservation after refill.

SR12 includes baseline export files, local admin tokens, welcome markers, source checkouts, logs,
audit receipts, migration journals, trust bundles, and backup manifests.
Each needs an owner, exact name, override rule, permissions, lifetime, and recovery treatment.
The deployment recipes must also state which files remain when shared services replace local databases.

**Backup and recovery contract.** A complete Starport backup spans KV, SQL, blob bytes, configuration, and credential decryption access.
It must record a consistent reference boundary across those systems.
SQL backup alone cannot recover KV accounts, gateway keys, catalogs, or usage.
KV backup alone cannot recover relational grants, audit history, or blob bytes.

Preserve accepted authority and replay floors. Treat restored permissions and budgets under the existing restricted-recovery contract.
Retain encryption-key access separately from encrypted credential records.
Do not clone one runtime seed into several active replicas.
A restore must also fence the old writers before the replacement admits requests.

The adapter backup methods do not establish a complete product backup procedure.
The recipes still need coordinated export, checksums, restore validation, and measured recovery objectives.
Caches can rebuild. An accepted authority record, gateway key, or missing usage interval cannot simply become a cache miss.

**Verification.** Selected macOS tests passed with the race detector: 19 top-level tests and 46 subtests across four packages.
There were no selected failures or skips.
This run removed external Valkey, PostgreSQL, and MySQL test variables.
Conditional backend cases therefore did not run and remain UNVERIFIED.
No provider calls, product edits, migrations against external systems, or publication occurred.

See the [test summary](evidence/storage-review-2026-09-05/local-tests-summary.json)
and [raw test output](evidence/storage-review-2026-09-05/local-tests.jsonl).
The new findings above require their own regression or platform evidence before implementation can close them.

[sp-paths]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/config/paths.go#L42

[sp-state]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/config/catalog.go#L231

[sp-app]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/app/app.go#L225

[sp-sql-migrations]: https://github.com/agentstation/starport/tree/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/sqlstore/migrations

[sp-loader]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/config/loader.go#L124

[sp-storage-config]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/config/storage.go#L36

[sp-generations]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/catalog/generation_store.go#L17

[sp-lease]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/catalog/lease.go#L18

[sp-blob]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/blob/filesystem.go#L18

[sp-files-config]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/config/files.go#L103

[sp-setup]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/setup/service.go#L406

[sp-token]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/localauth/store.go#L16

[sp-cli]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/cli/app.go#L217

[sp-dev]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/app/development.go#L41

[sp-authmode]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/app/app.go#L992

[sp-cache-manager]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/cache/manager.go#L43

[sp-cache-compose]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/app/app.go#L602

[sp-hybrid]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/cache/hybrid.go#L62

[sp-badger]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/storage/badger.go#L43

[sp-valkey]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/storage/valkey.go#L22

[sp-sql-open]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/sqlstore/open.go#L44

[sm-defaults]: https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/constants/constants.go#L91

[sm-config]: https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/cli/app/config.go#L56

[sm-runtime]: https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/cli/app/catalog_runtime.go#L144

[sm-filesystem]: https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/pkg/catalogs/storage/filesystem.go#L20

[sm-layers]: https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/runtime/layers.go#L18

[sm-scheduler]: https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/runtime/scheduler.go#L25

[sm-github-state]: https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/sources/github/state.go#L15

[sm-http]: https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/sources/modelsdev/http_client.go#L90

[sm-git]: https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/sources/modelsdev/git_client.go#L54

[sm-server-cache]: https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/server/cache/cache.go#L1

[sp-docker]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/Dockerfile#L35

[sp-compose]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/docker-compose.yml#L1

[sm-compose]: https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/docker-compose.yml#L66

[sp-credential-references]: https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/credentials/reference.go#L15
