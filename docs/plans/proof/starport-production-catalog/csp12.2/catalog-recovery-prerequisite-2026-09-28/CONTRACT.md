# Catalog recovery prerequisite for budget admission

CSP12.2 requires controlled catalog adoption before its backend replacement test can pass.
This prerequisite belongs to `internal/catalog` and `internal/recovery` in Starport.
Starmap owns the publication, retained-input, replay, and permission contracts.
CSP13 retains complete migration, backup tooling, and operator procedures.

## Verified failure

The corrected fixture explicitly disables catalog acquisition and preserves the loaded canonical settings.
It publishes an embedded generation and accepts that publication before dispatch and snapshot transfer.
The fixture stops its writers, copies the deployment namespace, and approves the replacement budget authority through PostgreSQL.

The replacement gateway refuses the copied acceptance record.
The copied head belongs to the old recovery identity.

`FleetStore.CurrentHead` and `readAcceptance` require the independently approved identity.

`ApproveAuthority` changes budget approval but cannot adopt catalog publication history.

The original test lacked an accepted fleet publication. It failed at consumed bootstrap permission.
Both failures remain evidence. Neither failure proves successful replacement recovery.

## Scope correction

The previous task order placed catalog adoption in CSP13 after CSP12.2.
CSP12.2 already required successful production dispatch after backend replacement.
That dependency prevented its acceptance test from passing through supported behavior.

Move the catalog adoption prerequisite into CSP12.2. Keep one active ledger row and the existing paired PR policy.
Do not mark CSP13 complete or reduce its broader migration and recovery criteria.
Existing implementation authorization covers this prerequisite.

## Recovery contract

1. Close the independent recovery epoch before adopting copied catalog state.
2. Require explicit recovery inputs that identify the deployment, replacement backend, original publication, reconciliation evidence, and operation.
3. Verify external writer fencing through the operator procedure before executing recovery.
4. Validate the selected publication, accepted publication, retained generations, private inputs, pin, and inventory before activation.
5. Preserve original immutable publication evidence and generation checksums.
6. Bind the recovered selection to the new recovery identity through an explicit recovery record.
7. Preserve catalog authority restrictions, receipt expiry, source timestamps, and retained acquisition inputs.
8. Preserve budget reservations, original windows, pinned prices, correction evidence, and uncertain capacity.
9. Stage recovery with bounded memory, storage, and execution time.
10. Record durable progress before writes that can survive a crash.
11. Make exact retries idempotent after lost acknowledgements or process loss.
12. Reject a changed original head, recovery epoch, replacement incarnation, or conflicting operation.
13. Activate a complete recovered selection atomically before opening shared admission.
14. Keep partial recovery unavailable to gateway startup.
15. Keep obsolete owners unable to publish or admit work after recovery.

Recovery must retain rollback content and account for abandoned reader protections and pending payload operations.
Ordinary startup must never create recovery approval, reset bootstrap permission, or infer permission from copied records.
A new refresh must get a new native grant. Recovery must not revive a copied lease.
A budget approval alone must remain insufficient when the catalog still requires adoption.

The recovery record must distinguish original publication evidence from permission to use the replacement backend.
An implementation must not rewrite old acquisition grants as if they originally belonged to the replacement.
The exact storage format remains an implementation decision in the catalog and recovery packages.

## Required evidence

The existing `TestProductionBudgetAcrossProcesses/backend-replacement` remains the production acceptance test.
After controlled adoption, the replacement must start and return HTTP 402 while the original dispatch remains uncertain.
All five original meters must retain their reservations without another provider call.
Explicit no-charge evidence must reconcile once, permit the next request, and produce exactly one additional provider dispatch.

Add catalog recovery tests for corruption, missing chunks, retained pins, accepted history, and pending inventory work.
Add failure tests before and after staging, selection, and approval, including lost acknowledgements and exact retries.
Add concurrency tests for conflicting recovery operations and changed SQL approval or backend identity.
Verify expired receipts and known withdrawals remain restrictive after adoption.
Verify old owners cannot mutate the recovered catalog or budget authority.

Run these tests with real Valkey and PostgreSQL, race detection, and Go 1.27.1.
Preserve Linux, Windows, and Apple silicon native qualification before merge.
The complete CSP12.2 gate, latency, capacity, review, published dependency, and paired merges remain required.
