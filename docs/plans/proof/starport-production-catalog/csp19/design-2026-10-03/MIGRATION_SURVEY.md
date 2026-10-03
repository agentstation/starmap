# CSP19 local-to-shared migration survey

Recorded 2026-10-03 for decision 19-2. An explorer agent (Opus 5.5) read Starport at `e7a58541` and the plan proof. The lead verified the marked claims against Starport main `ce1107d7`. A line marked "inference" states a claim that the code does not state directly.

## Result

The cross-recipe move path already exists. The command `backup create` produces a portable bundle from any source kind. The command `backup prepare` opens the target kinds that the configuration names. The command `backup activate` compiles the captured state into the shared owners. The catalog layer names the direction `local-to-fleet`. Three tests already move a local source into Valkey, PostgreSQL, and MinIO, and each one skips without the fixture variables.

The release documentation contradicts this. Three lines state that no test and no procedure exist. The missing work is qualification, not transport: a populated workload, parity checks, source retirement, a rollback procedure, a recipe-image test, and the documentation.

## Verified claims

The lead read each line below on 2026-10-03.

- `internal/app/restore.go:82-84`: the only identity refusal compares the target deployment ID with the verified backup. No storage-mode check exists in the file.
- `internal/app/restore.go:140-149`: `restoreBlobTarget` opens the filesystem or the object store from the configuration.
- `internal/app/recovery_activation_native.go:53-75`: activation requires PostgreSQL or MySQL, Valkey, and a non-filesystem blob backend.
- `internal/app/recovery_activation_test.go:239-267`: `TestRecoveryActivationNativeTopologyRecipes` runs `local-to-fleet`, `fleet-to-local`, `local-restore`, and `fleet-restore`, and starts one activation child.
- `internal/app/restore_test.go:153-181`: `TestRestoreApplicationSharedRecipe` prepares twice into the shared trio and proves the import barriers. It skips without `TEST_VALKEY_URL`, `TEST_POSTGRES_URL`, and `TEST_BLOB_S3_ENDPOINT`.
- `internal/app/recovery_activation_test.go:285-300`: `TestRecoveryActivationFullEmbeddedApplication` activates a completed migration into the shared trio and skips without the same variables.
- `internal/catalog/topology_replay.go:230-238`: the fleet compile sets the lease to nil and deletes the local current, candidate, and index keys.
- `docs/site/storage/backup-and-restore.md:111`: "No test covers a restore from one recipe to the other." False.
- `docs/site/storage/migration.md:31`: "Local recipe data to the shared recipe | No tested procedure". False for the transport, true for the populated qualification.
- `docs/site/storage/migration.md:90`: "This release has no tested procedure that moves keys, credentials, usage, or identity records…". False for the transport.

## Explorer findings

### What a populated local deployment holds

- KV (Badger): keys, provider credentials, rate limits, presets, usage, file records, and accepted generations (`docs/site/storage/backends.md:14`). Badger keys carry no deployment prefix.
- The recovery counters name the domain families (`internal/recovery/references.go:29-49`).
- SQL (SQLite): 13 migrations with the tables `sqlstore_meta`, `account_templates`, `users`, `teams`, `team_memberships`, `account_grants`, `incident_transitions`, `audit_log`, `authorization_revision`, `catalog_recovery`, `team_budget_origins`, `schema_migration_reconciliations`, `deployment_configuration_head`, and `deployment_configuration_revisions`.
- Files: the backup inventory maps Badger to `kv-snapshot` and SQLite with its journals to `sql-snapshot`. It maps the files directory to `blob-snapshot` and the source caches to a rebuild (`internal/config/backup_inventory.go:99-123`).
- Deployment-scoped: all KV, SQL, and blob records, the deployment ID, the master key, and the configuration revisions.
- Process-scoped: the instance ID, the catalog runtime directory, the source caches, journals, CA files, and the per-replica admin token (`docs/FLEET_INITIALIZATION.md:147-151`).
- The backup set holds a master key reference and never the key (`docs/site/storage/backup-and-restore.md:19`).

### Shared equivalents and asymmetries

- Valkey stores `{starport:v1:<base64url id>:}kv:<logical key>` (`internal/storage/valkey.go:31`). Transfer strips the prefix, so the logical key identity is equal on both sides.
- PostgreSQL uses its own 13 migration files. The SQLite image is the portable form, and the import uses the portable table contract (`internal/sqlstore/relational_schema.go:41-43`).
- The object store stores `<prefix>/<key>` with the control objects `.starport/layout` and `.starport/import` (`internal/blob/objectstore_layout.go:13-17`).
- The command `fleet init` approves only empty Valkey and PostgreSQL stores (`docs/FLEET_INITIALIZATION.md:15-42`). It refuses application records. A populated source uses a restore instead.
- Local catalog keys carry no prefix. Fleet catalog keys live under `catalog:fleet:{<digest>}:v1:` (`internal/catalog/topology_replay.go:158,224-238`).
- Badger rounds expiry down to whole seconds (CSP13 kv-transfer contract). Inference: a move to Valkey loses no more precision.

### Catalog authority after the move

- The fleet compile rewrites the local accepted and candidate generations as fleet heads with revision 1 and a `RecoveryOrigin` (`internal/catalog/topology_replay.go:176-218`).
- Inference: the carried head becomes the retained fleet baseline. A newer packaged baseline then needs the CSP16.2 `promote-baseline` command.
- Not verified: whether the CSP16.2 leader executor accepts a head with a `RecoveryOrigin`, and how a local-to-fleet activation sets `catalog_recovery.bootstrap_allowed`.

### Encryption and identity

- Credentials use AES-GCM with an Argon2 key from the master key and a per-value salt, without associated data (`internal/credentials/encryption.go:36,82,142-144`).
- Inference: the ciphertext binds to neither the deployment nor the backend. The same master key decrypts after the move.
- The SQL import copies `audit_log` rows and restores the audit high water. Inference: no audit entry records the move itself.

### Gaps

| Gap | Statement | Estimate |
| --- | --- | --- |
| G1 | No fixture seeds credentials, gateway keys, usage, identity, audit, budgets, jobs, and files. The existing fixtures seed one KV key, an empty SQL store, and one blob. | 1 fixture file, about 3 tests |
| G2 | No source-to-target count or digest parity check. The source counts and the target recapture exist (`internal/recovery/references.go:29-49`, `internal/recovery/imported_references.go:47-60`). | 1 to 2 files, 1 to 2 tests |
| G3 | Nothing marks the local source retired after the move. The fence stays external (`docs/site/storage/backup-and-restore.md:37-43`). | 1 to 2 files, 2 tests |
| G4 | No documented rollback. The source stays unchanged, and no test reopens a closed local boundary. | docs, 1 test |
| G5 | No cross-mode intent guard beyond the deployment ID. A design choice. | 1 to 2 files, 1 test |
| G6 | No test joins a second replica after a local-to-fleet activation. | 1 two-process test |
| G7 | No recipe-image test for the move (decision 19-6). | script, 1 Go test |
| G8 | Three wrong documentation lines and no operator procedure. | docs |

### Shared deployment upgrade

The same path covers an existing shared deployment only in part. Capture supports `--unprefixed-valkey`. `BackupLegacyObjects` (`internal/blob/snapshot.go:53`) has no caller outside tests, so a legacy object prefix has no capture path. The shared upgrade stays outside CSP19.1.

### Exemplars

- Internal: the four recipe directions, the shared restore test, the full embedded activation, and `TestContainerRecipePersistence` (`internal/config/compose_recipe_test.go:104-128`).
- On disk, same-kind: Consul `snapshot/archive.go` (SHA256SUMS verification), Nomad `operator snapshot inspect` and `state`, PocketBase `CreateBackup` and `RestoreBackup`.
- Not read: Vault `operator migrate`, CockroachDB, FerretDB.

## Not checked

- The full list of KV key families and owners.
- Whether `InspectImportedReferences` compares counts with the source.
- How a move sets `bootstrap_allowed`.
- Key rotation.
- The explorer ran no tests and no Docker.
