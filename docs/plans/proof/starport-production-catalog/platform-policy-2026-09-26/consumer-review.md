autoreview panel findings: 5
[P1] Shared Valkey deployments cannot start, and nothing in the patch can approve them
internal/catalog/fleet_store.go:37
Reviewer: claude model=claude-opus-5-5 thinking=high

`OpenRuntimeWithRecovery` is now the production catalog factory. For any `IncarnationProvider` store, it calls `NewFleetStore`, which requires an open `catalog_recovery` row for the deployment (`witness.Approved`). Otherwise it returns `ErrClosed`.

The patch only ever calls `Initialize` and `Approve` from tests. It adds no CLI, admin API or migration step that creates the approval. As a result, every existing Valkey-backed deployment fails at startup after upgrade. The only workaround is for an operator to hand-write a SQL row containing the exact `run_id:master_replid` string, which they would have to extract from `INFO` themselves.

The approved backend ID is bound to the Valkey `run_id` and `master_replid`, and both change on every Valkey restart, even with persistence enabled. After any Valkey restart or failover, every fleet read and CAS returns `ErrIncarnationChanged`. New gateway instances then refuse to start until someone edits SQL by hand again.

The same code path also newly refuses shared Valkey deployments whose runtime SQL is MySQL or SQLite (`runtime_fleet.go` `openRecoveryFleet`), even though a MySQL migration is shipped.

The smallest landable fix is a supported operator command (initialize/observe/approve) plus an upgrade note. Otherwise the fleet path needs to be gated behind an explicit opt-in so that current shared deployments keep their previous behaviour.

[P2] Fleet publications and chunks are never pruned, so Valkey memory grows without bound
internal/catalog/fleet_store.go:173
Reviewer: claude model=claude-opus-5-5 thinking=high

Every `CommitPublication` stages a new blob: `stageBlob` writes content-addressed chunks with no TTL. It also creates a new `publication:<revision>:<digest>` descriptor. Chunks from a failed commit are also left behind.

Each snapshot embeds the head, grant, expected predecessor and up to `MaxFleetRecoveryBytes` of retained inputs. Its JSON therefore differs on every revision (including input-only revisions, as the tests show), and fixed-offset chunking gets almost no dedupe.

Nothing in the patch deletes old publication descriptors or unreferenced chunks. The accepted history is capped at `catalogGenerationIndexCap`, but the underlying blobs are not. On an in-memory Valkey backend, each refresh that produces a new revision leaks up to `2*(recovery+payload)+1MiB`, which eventually exhausts Valkey memory for the shared gateway store.

Fix: after a successful commit or acceptance, prune publications that are neither the current head, the accepted head, nor retained history, along with their unreferenced chunks. Alternatively, give staged chunks a TTL that only a commit clears.

[P2] Standalone mode now drops a published candidate after a transient lease-epoch read error
internal/catalog/runtime.go:463
Reviewer: claude model=claude-opus-5-5 thinking=high

Before this change, `offer` explicitly tolerated a `leases.CurrentEpoch` error. It set the epoch to 0 and still delivered the candidate, because the acceptance transaction re-fences the epoch.

`offer` now calls `candidateFromState`, which in non-fleet mode returns that same error. `offer` then records `validation.reject(...)` and returns without updating `lastSeen` or sending the candidate. The retry ticker only exists when `r.fleet != nil`, and `runtime.Updates()` does not re-emit an unchanged state.

So in standalone deployments, a single transient storage error while reading the lease epoch permanently discards that published generation until the source changes again. It is also reported as a route-validation rejection even though route validation never ran.

Fix: keep the previous behaviour on the non-fleet path (fall back to epoch 0 and deliver the candidate). Only fail closed when the fleet path is active.

[P1] Provision recovery approval before enabling shared startup
internal/catalog/fleet_store.go:37
Reviewer: codex model=gpt-6-sol thinking=high

Every existing and fresh Valkey deployment reaches `witness.Approved` during startup, but the new SQL migration creates an empty `catalog_recovery` table. The only approval setup in this bundle is in tests, so production startup returns `ErrClosed` until an operator manually creates a valid approval record. This needs a concrete provisioning and upgrade procedure before the new startup requirement is enabled.

[P1] Refresh the durable fleet head on the follower retry tick
internal/catalog/runtime.go:437
Reviewer: codex model=gpt-6-sol thinking=high

The tick reoffers only `r.runtime.State()`, which is the process's local state. A follower receives no cross-process notification when another process publishes, so this tick cannot discover the new durable head. The new process test demonstrates that catch-up requires an explicit `RefreshFleet` call, but production does not make that call here. Followers can continue serving a stale catalog indefinitely; poll the durable head before offering candidates.

overall: patch is incorrect (0.92)
Panel review complete. claude model=claude-opus-5-5 thinking=high: 3 finding(s), codex model=gpt-6-sol thinking=high: 2 finding(s).
