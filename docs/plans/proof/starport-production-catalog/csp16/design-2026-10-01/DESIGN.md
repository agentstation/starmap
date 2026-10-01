# CSP16 design: shared configuration authority

Date: 2026-10-01. Owner: the plan lead. Status: draft before the fail-before capture.

## Outcome

Starport selects one deployment configuration authority at startup. Local management reads the product file. Shared management reads one persisted PostgreSQL revision. A replica serves requests from the applied revision in process memory. A28 and A40 pass with two replicas, conflicting files, an initialization race, an unavailable store, and a local-to-shared migration.

## Inputs

- PRD D15: bootstrap stays node-local. Shared PostgreSQL stores configuration revisions and audit records. Valkey retains catalog state and coordination.
- Specification 7.4 covers scopes and the effective report. Section 7.5 covers the selector, startup sequence 1–7, and the outage rule. Section 7.7 covers the API, which CSP16.1 owns. Section 8.9.3 covers the applied revision in process memory.
- D41 contract: conditions 1–10, 15, and 17 are mandatory. SQL owns the approved revision and audit. Valkey owns the applied catalog policy and publication fence. One apply coordinates both owners and recovers a partial result.
- Acceptance map `task_checks.CSP16`: 14 subcases. The registry holds `A24.bootstrap_file_location` and `A03.effective_path_report`. The registry holds none of the 12 A28 and A40 subcases. The acceptance map has no contract text for them.

## Current state (fail-before)

- `internal/config/loader.go` `Loader.load` is one pass: bootstrap paths, source lookuper, managed paths, decode, overrides, validate. No management selector exists in either repository.
- `internal/sqlstore/migrations` ends at `0012`. No table stores a deployment configuration revision.
- Starmap `pkg/catalogs/config.Resolve` orders layers by precedence only. `Descriptor.Scope` already separates `deployment` and `node`. No layer can declare itself the shared authority.
- Starmap `runtime.fleetLayerCompatibility` is private. Replay compares the recovery record compatibility with the local layers and refuses a mismatch. Starport lease acquisition does not compare an applied configuration revision.

## Concept boundaries

| Concept | Owner | Contract |
| --- | --- | --- |
| Management selector | Starport `internal/config` bootstrap | `STARPORT_CONFIG_MANAGEMENT` = `local`, `shared`, or `external`. Default `local` for embedded stores. Default `shared` when `Storage.Distributed()` is true. Bootstrap and node values never come from the shared record. |
| Scope classification | Starmap descriptors for catalog settings; Starport for its own settings | The shared revision carries the Starmap `DeploymentScope` catalog descriptors. Every Starport setting, including inference destination approvals, stays bootstrap or node in this delivery. |
| Shared authority resolution | Starmap `pkg/catalogs/config` | `ResolveAuthority(shared Layer, local ...Layer)`: the shared layer must contain only deployment-scope names; every local deployment-scope value is `Ignored` with reason `shared-authority`; node-scope values come from the local layers. `Resolution` reports the authority name. |
| Configuration revision store | Starport `internal/configrevision` (new) | Migration `0013_deployment_configuration` for postgres, sqlite, and mysql: `deployment_configuration_head(deployment_id PK, namespace, sequence, revision_id)` and `deployment_configuration_revisions(revision_id PK, deployment_id, sequence, predecessor, operation_id UNIQUE, actor, checksum, record, created_at)` with `UNIQUE(deployment_id, sequence)`. `Head(ctx)` returns `ErrNotInitialized` for an absent head and a distinct unavailable error for a store failure. `Initialize(ctx, seed, actor, operationID)` inserts the head and revision 1 and the audit record in one transaction against an absent head. A second initializer reads the existing head and refuses with the stored namespace. `Commit(ctx, expectedSequence, values, actor, operationID)` inserts sequence N+1 with `UPDATE head WHERE sequence = N`; zero rows is a stale-revision refusal; a repeated `operation_id` returns the original result. |
| Transactional audit | Starport `internal/audit` | `Repository.RecordTx(ctx, tx, record)` writes `audit_log` inside the caller transaction. The configuration store uses it for initialize and commit. |
| Applied revision | Starport `internal/config` | `Config.AppliedRevision()` reports `{Authority, Namespace, Desired, Applied}`. Request paths read the loaded `Config` and never the store. Startup reports desired and applied before readiness (sequence step 7). |
| Startup sequence | Starport `internal/app` | `openSQLStore` migrates. A new `openConfigurationAuthority` step runs after it and before `openConcepts`: read the head and namespace; absent head → refuse with `starport config init --shared` instruction; unavailable store → refuse (not an empty store, no file fallback); present head → resolve the shared layer and replace the deployment-scope values in the loaded `Config` before any adapter reads them. Catalog settings reach `catalogSettings(b.config)` only after this step. |
| Operator operations | Starport `internal/cli` | `starport config init --shared` seeds revision 1 from the validated local deployment values (preview, then write). `starport config migrate --to shared|local` records the authority switch as a revision with an audit record. `starport config effective --json` reports scope, winning authority, origin, ignored local values, desired revision, and applied revision. HTTP routes stay in CSP16.1. |
| Policy fence | Starport `internal/catalog` | The policy identity of a revision is its checksum: SHA-256 over the canonical deployment-scope values, with no credential. The apply operation (`starport config apply`) commits revision N+1 in SQL, then acquires the fleet lease (which advances the durable epoch and fences the previous publisher), writes the applied policy record `catalog:config:applied = {sequence, checksum}` under that lease, and releases. A replica compares its applied checksum with that record before lease acquisition and before activation. A mismatch reports `policy_mismatch`, blocks leadership, and keeps serving. A partial apply (SQL committed, Valkey record absent or older) is recovered by `starport config apply --resume <operation-id>`, which repeats only the Valkey phase with the same operation ID. Rollback is another apply that commits sequence N+2 with the previous values. The Starmap runtime is unchanged in this delivery. |

## Delivery order

1. Starmap contract PR: `ResolveAuthority`, the `shared-authority` ignored reason, and settings documentation. The same PR registers the 12 A28 and A40 subcases with `subcase_contracts` text and the agreed Starport test names. Merge, then pin Starport to the merge commit pseudo-version.
2. Starport configuration PR: selector, scope classification, migration 0013 for three dialects, and `internal/configrevision`. The same PR adds transactional audit, the startup step, CLI operations, and the applied-revision report. It also adds the fence, documentation, and the 14 subcase tests.
3. Acceptance: `--task CSP16` with the real PostgreSQL and Valkey fixtures, then the ledger update.

## Subcase to test map (Starport unless stated)

| Subcase | Test |
| --- | --- |
| A28.desired_applied_drift | Replica A applies revision 2 while replica B still runs revision 1; both report desired 2 and applied 1 or 2 without a file read. |
| A28.shared_authority_mismatch | A bootstrap namespace that differs from the stored head refuses startup and names the stored namespace. Starmap: a shared layer with a node-scope name fails validation. |
| A28.leadership_config_revision | A replica whose applied checksum differs from the applied policy record cannot acquire the lease and reports `policy_mismatch` while it serves. |
| A28.applied_config_memory | Request handling under a paused PostgreSQL and a removed file keeps the applied revision. |
| A40.local_file_authority | Local management ignores the shared tables and reads the file. |
| A40.shared_revision_beats_local_values | Starmap: `ResolveAuthority` ignores a local deployment-scope value with reason `shared-authority`. Starport: two replicas with conflicting files resolve the same values. |
| A40.atomic_initialization_race | Two concurrent initializers against one PostgreSQL schema produce one head and one revision 1; the loser reports the winner. |
| A40.unavailable_not_empty | A paused PostgreSQL refuses startup with the unavailable error, writes nothing, and never reads the file for deployment values. |
| A40.bootstrap_connection_cycle | The shared record cannot change `STORAGE_*` or `CONFIG_*` bootstrap values; a revision that names one fails validation. |
| A40.local_shared_migration | `config migrate --to shared` previews the seed, writes revision 1 with an audit record, and the next startup reads it. |
| A40.retained_shared_revision | A store outage after a successful load retains the applied revision under its validity and reports it as a cache of shared authority. |
| A40.no_outage_file_fallback | The same outage with a changed file never applies the file values. |

## Limits

- The shared revision covers Starmap deployment-scope catalog settings. Starport gateway policy, including inference destination approvals, keeps node or bootstrap scope until a named subcase requires a shared copy. CSP17 owns that question under FBL-08.
- D41 conditions 6, 7, and 8 depend on Starmap retained-input validation that CSP11 proved. This task keeps those checks on the apply path and records their evidence. Conditions 13 and 16 belong to CSP16.1.
- The HTTP configuration API, drafts, `If-Match`, and operation receipts belong to CSP16.1.
