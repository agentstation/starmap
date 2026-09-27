# Deployment KV isolation contract

CSP12 owns this repair. The earlier real-Valkey probe already shows cross-deployment access through ordinary gateway repositories.
Catalog-only namespace tests do not qualify the gateway contract.

## Ownership

`internal/deployment.KeyPrefix` already owns the encoded deployment identity and schema version.
Use that owner for durable KV keys and notification channels. Do not create another environment variable for a conflicting identity.
Keep logical repository keys unchanged. The storage adapter translates logical keys to physical keys and reverses scan results.

Project the canonical identity from `Config.EffectivePaths().DeploymentID` when opening runtime storage.
Prefer a root configuration projection over copying identity into unrelated mutable settings.
Apply the same projection in application startup, inspection, migration, and fleet commands.
Low-level test and administrative connections must remain explicit about any unscoped access.

The Valkey adapter must apply the namespace to basic operations, TTL operations, counters, conditional mutations, batches, and scans.
It must also apply it to bounded lifetime reads and the process-bound recovery adapter.
Pubsub publication and subscription must use the same deployment prefix. Handlers receive logical channel names.
Preserve one atomic command for each conditional mutation set and native ownership check.
Qualify the required multi-key slot layout without claiming cluster support.

## Initialization and existing records

Keep the fresh-fleet checks for an empty dedicated KV database and SQL schema.
A prefix alone does not authorize a second deployment to claim existing state.
Independent SQL approval and backend identity checks remain mandatory.
CSP13 must migrate existing unprefixed records with counts and reference checks, then switch authority explicitly.
Do not silently copy, dual-write, or fall back to old records during ordinary startup.

A key prefix does not isolate server memory, eviction, persistence, or failure.
Keep the dedicated durable-service recipe and separate optional cache service.
Local Badger remains private process storage. Do not change its stored layout merely to match a shared-service implementation detail.

## Required evidence

Use real Valkey to exercise two identities with equal logical keys and wildcard scans.
Prove isolation for reads, writes, deletes, batches, counters, TTLs, bounded reads, and conditional conflicts.
Prove namespace preservation through the process-bound adapter, including rejection after backend replacement.
Use two databases on the same server to prove notification isolation, because pubsub does not isolate channels by database.

At the application boundary, store distinct sentinel records for gateway keys, credentials, budgets, jobs, and notifications.
Prove that another deployment cannot observe or mutate them.
Retain recipe refusal tests and a configuration projection test for the canonical deployment identity.
The prepared connection tests and atomic mutation tests remain mandatory after this change.
