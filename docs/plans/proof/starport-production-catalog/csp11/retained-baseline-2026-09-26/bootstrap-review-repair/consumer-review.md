autoreview panel findings: 3
[P1] All existing Valkey deployments lose startup: mandatory PostgreSQL witness and approval with no adoption or restart-recovery path
internal/catalog/runtime_fleet.go:72
Reviewer: claude model=claude-opus-5-5 thinking=high

`openRecoveryFleet` now turns the fleet/witness path on for every store that implements `storage.IncarnationProvider`. Every Valkey KVStore does. `OpenRuntimeWithRecovery` is the only catalog factory the app uses.

For existing Valkey deployments this means:

1. Valkey with SQLite or MySQL fails startup with "requires the qualified PostgreSQL witness". The previous `TestAppWithValkey` ran without Postgres and had to be changed.
2. Valkey with Postgres fails with `recovery.ErrClosed` because no approval row exists.
3. `starport fleet init` refuses any populated store (`ErrNotFresh` / `ErrDatabaseNotEmpty`), and the docs say this build has no adoption command.
4. Even a freshly initialized fleet is bound to Valkey `run_id:master_replid`. Any routine Valkey restart or failover changes `run_id`, so `BindIncarnation` returns `ErrIncarnationChanged` and every gateway fails catalog startup and operations. The docs admit there is no recovery command.

The upgrade therefore hard-breaks every existing Valkey-backed gateway, and routine Valkey maintenance permanently bricks new ones. There is no migration, opt-in, or fallback.

Smallest fix: gate the fleet/witness path behind an explicit opt-in, or enable it only when a recovery row already exists. Otherwise keep the previous Valkey catalog path. A complete fix needs the adoption/recovery commands the docs defer to CSP13.

[P1] Existing Valkey deployments cannot start after upgrading
internal/catalog/fleet_store.go:44
Reviewer: codex model=gpt-6-sol thinking=high

Shared startup now requires an approved `catalog_recovery` row, but previous deployments have no such row. `Approved` returns `ErrClosed`, while `fleet init` refuses the populated stores an existing deployment has. The patch provides no adoption or migration path, so upgrading an existing Valkey deployment prevents its gateways from starting. Add that path before enforcing this requirement on existing installations.

[P1] A Valkey restart leaves an initialized fleet unable to restart
internal/catalog/fleet_store.go:51
Reviewer: codex model=gpt-6-sol thinking=high

Approval binds to Valkey's process run ID. A routine Valkey restart changes that ID, so `BindIncarnation` rejects the stored approval on every subsequent gateway startup. Fresh initialization refuses the prior approval, and this patch has no recovery operation for a changed backend. A controlled recovery procedure is needed before this becomes the required production startup path.

overall: patch is incorrect (0.97)
Panel review complete. claude model=claude-opus-5-5 thinking=high: 1 finding(s), codex model=gpt-6-sol thinking=high: 2 finding(s).
