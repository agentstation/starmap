# Recovery catalog preparation lane

The catalog compiler selects the exact private topology mutations. The recovery owner retains and applies each bounded transaction under the original closed native import claims. This lane grants no inference or activation permission.

## Composition order

1. Compile the actual topology against the selected native target using a guarded passive reader. The compiler needs target preimages before it can produce its digest. The current typed non-final history does not mutate catalog keys. This remains a required composition invariant.
2. Declare `HistoryReplayRequest.CatalogPreparation = &CatalogPreparationPlan{TopologySHA256: compiled.TopologyDigest(), StageCount: compiled.StageCount()}`. The stage count excludes the distinct final selection stage. The declaration is part of immutable `runner.json` and must not change on retry.
3. Call `Witness.ReplayImportedHistoryPrefix`. It requires exactly two final history entries, KV rotation then SQL rotation, and stops before both. The prefix refuses earlier rotation entries.
4. Open `HistoryPrefix.OpenCatalogPreparation`. Supply the private lane to the catalog compiler as its structural `TopologyTarget`.
5. Apply all predetermined stage indices, starting at zero. Retain and materialize original producer capsules through the actual producer APIs.
6. Call the compiler's `ApplySelection` with its actual checked materialization capability. Its distinct index is `StageCount`. The selection mutation binds the producer proof. This lane does not manufacture that proof.
7. Call ordinary `Witness.ReplayImportedHistory` to rotate the final KV and SQL authorities. This method refuses a declared lane that is incomplete. Both final prepared records consume the actual lane KV position and completion digest chain.

These operations must not mutate the catalog target outside its journal. External writers remain fenced throughout the procedure. Final catalog-owner permission and materialization verification, the full retained graph census, and native barrier release remain separate activation requirements.

## Structural target API

`CatalogPreparationLane` implements:

- `ReadCatalogTopology(ctx, key, limit)` reads one bounded native record under the original exact claim, incarnation and cursor.
- `ApplyCatalogTopology(ctx, topologySHA, index, mutations)` retains the exact immutable assets and prepared pointer. It runs native reconciliation under `Witness.GuardClosedImport` on the actual SQL connection.
- `CompletedCatalogTopology(ctx, topologySHA, index)` validates the retained exact historical stage and the still-current original native claim/cursor. An old completed retry does not reapply mutation. After native barrier release this method refuses. It is not a current permission check.

Each native guard uses a ten-second deadline. An earlier caller deadline applies. The lane checks accepted history and the exact retained typed prefix before it takes the SQL guard. The guarded callback opens no nested SQL transaction or second SQL connection. It calls no ordinary witness method.

## Retained records and bounds

- `catalog-assets/%06d.json` retains the exact original run, topology, stage index, previous digest, original native positions, keys, preimages and new values. Presence bits preserve nil versus non-nil empty bytes.
- `catalog-%06d.prepared.json` binds that asset's exact SHA and size, run, topology, index, original positions and prior chain digest.
- `catalog-%06d.applied.json` binds the prepared SHA, exact storage-owned native receipt SHA and resulting native positions.
- `catalog.complete.json` binds the declared stage count, original prefix positions and digest, complete stage chain and resulting native positions.

Native batches retain the existing limits of 128 mutations and 4 MiB total raw keys, preimages and new values. Asset JSON has a separate limit of `ceil(4 MiB * 4/3) + 128 KiB`. Base64 carries all raw key/value bytes. Each record's metadata fits within 1 KiB. Journal records remain 64 KiB. The lane keeps the existing 64 MiB typed history and 16 GiB streamed-asset aggregate bounds.

The private publication owner reserves its metadata directory. The asset census first calls `CheckNoPendingPublications`. It then excludes only that exact owner directory. It refuses other missing, extra, corrupt or reordered assets and records.

An asset must be durable before its prepared pointer. A cut between these writes retains an incomplete lane. Active exact retry may publish the pointer only from the same original asset. A changed request cannot replace it. A lost native reply can publish the same original applied record only if the actual native next cursor proves the exact prepared transaction. A changed prepared pointer, unexpected native successor, changed prefix or different topology declaration refuses recovery.

## Passive retained-history integration

`historyRunner.scanJournal` is passive. It binds the original decoded run declaration to `request.CatalogPreparation`, checks the strict final-two boundary, and scans every retained catalog asset/pointer/applied record/completion. It derives each expected native receipt through `storage.ImportReconciliationSHA256`, not a recovery-owned opaque codec. It bridges the checked lane's exact position and completion SHA into final typed history.

Root's retained-history constructor must copy the original canonical `run.CatalogPreparation` into the passive replay request before scanning. It must not supply a new operator plan. Root's `retainedCatalogJournal` hook can require:

- No declaration: `state.catalog == nil`. The shared scanner already rejects catalog records.
- Declaration: `state.catalog != nil`, `state.catalog.pending == nil`, `state.catalog.count == StageCount + 1`, and a nonempty checked `state.catalog.complete`.

Passive validation has no target, repair, materialization, barrier release or new validation time. Original `ValidatedAt` remains unchanged. Historical completion does not grant current permission. The passive check preserves later ordinary withdrawals.

## Qualification limits

Tests use actual Badger, Valkey, SQLite, PostgreSQL and MySQL. They cover exact native receipts, lost replies, pointer cuts, changed declarations, missing prefixes and corrupt assets. They also cover concurrent identical stages, nonzero final positions, and passive checks after native release with a later withdrawal. The lane fixtures use small catalog-namespace records. They do not prove actual producer semantic replay, full catalog-owner activation, or a 2 GiB end-to-end memory/RPO envelope. Root owns those composition and capacity checks.
