# Recovery activation contract

CSP13 owns this implementation contract. It preserves the acceptance criteria in engineering specification section 8.6 and A16/A33.
The contract does not claim that the activation command exists.

## Behavior before this change

`PrepareBundle` verifies the backup and imports SQL, KV, blobs, and selected files with persistent startup barriers.
SQL preparation closes recovery gates and disables bootstrap permission.
Its new epoch equals the captured epoch plus one.
This epoch identifies restricted preparation. It cannot establish continuity after loss of a newer SQL witness.

`AdoptFleet` validates catalog publications and retained inputs, then opens native and SQL authority.
Full restore needs catalog preparation while SQL authority remains closed.
The storage owners have no activation operations that release their import barriers.

The existing SQL audit trail cannot reconstruct later history.
Controller audit writes occur after mutations and permit failure.
Audit records lack complete mutation payloads and interval coverage.
Authorization revisions also cannot prove continuity after their own stores lose writes.

`reservation.EstablishWindow` creates absent history. Its contract prohibits repair of existing or lost state.
Recovery must not use it to replace a restored balance or infer zero consumption.

## Owning concepts

| Concept | Owner | Required behavior |
| --- | --- | --- |
| Recovery decision and ordering | `internal/recovery` | Bind evidence, scope, operation, target identities, epochs, and admission disposition. |
| Complete operator procedure | `internal/app` and CLI | Compose owner operations without bypassing their contracts. |
| Permission reconstruction | Account, gateway key, identity, and credential-policy repositories | Apply current policy and known withdrawals through typed operations. |
| Accounting reconstruction | `internal/limits/reservation` | Preserve original windows, confirmed spending, held capacity, and exact retry receipts. |
| Execution reconstruction | Jobs, batches, and execution repositories | Preserve uncertainty. Missing records never prove that execution did not start. |
| Catalog preparation | `internal/catalog` | Validate and select recovered publications without opening SQL authority. |
| Import activation | KV, SQL, and blob owners | Verify exact import claims and retain activation receipts before releasing their barriers. |

## Evidence contract

An immutable recovery decision must bind these facts:

- Backup digest, deployment ID, operation ID, and prepared component identities.
- Source approval, independently retained highest epoch, and selected replacement backend.
- Interval under review, affected identities, and evidence source digests.
- External fencing evidence and the operator who accepts those external facts.
- Applied reconciliation receipts, remaining restrictions, and final admission disposition.

A digest proves integrity. It does not prove that an external record is complete or that an old writer stopped.
Operator attestations must remain distinct from facts that Starport verifies.
The backup cannot supply its own sole proof that no later changes occurred.

Planned migration starts after admission closes and all writers stop.
The source remains fenced throughout capture and transfer.
The procedure must capture or account for previously admitted streams and pending work.
These conditions can establish an empty later interval through retained operator evidence.
They do not qualify disaster recovery after acknowledged writes disappear.

Disaster recovery requires separate records for the interval after the backup.
The initial implementation can import explicit evidence and operator reconciliation records.
It must not claim automatic continuous journaling.
Each replay operation needs complete owner data, an expected-state binding, ordering, and an immutable receipt.
Duplicate exact records must be safe. Conflicting or out-of-order records must not restore older permission.

Missing evidence keeps affected access restricted.
Unknown scope keeps the entire deployment restricted.
Uncertain execution retains its reservation until the existing settlement or administrator procedure establishes an outcome.

## Activation order

1. Verify the recovery decision and confirm that external fencing remains in force.
2. Reconcile later history through its owners, or retain explicit restrictions for the affected scope.
3. Validate canonical files, active configuration, transport trust, administrator credentials, and catalog identity.
4. Prepare catalog adoption while SQL authority remains closed.
5. Retain the bound recovery decision and component activation receipts.
6. Release blob and KV import barriers through owner operations bound to those receipts.
7. Install the native authority and finish SQL approval with SQL barrier release in one final SQL transaction where feasible.
8. Start fresh gateways. Require the exact approved epoch, backend identity, and rebuilt authorization state.

Every intermediate state must refuse ordinary startup or inference.
An exact retry can finish the same operation.
After the first component release, the coordinator must resume activation from retained receipts.
It must not repeat preparation against active components.
A changed backup, target identity, decision, or operation must conflict.
The coordinator must not delete marker keys or files directly.

Final activation must choose an epoch above the independently retained highest epoch.
Unknown epoch continuity keeps the deployment restricted.
Local `352d468af` implements this closed-state step through `Witness.PrepareImportedEpoch`.
Its SQL repair and evidence receipt commit together under the retained import barrier.
The operator still owns the completeness of the external epoch evidence.
Reachable gateway acknowledgments cannot prove that external controls fenced unreachable gateways or a former primary.

## Required evidence

Qualify the actual operator commands with native stores and retain exact source commits:

- Planned Badger, SQLite, and filesystem migration to Valkey, PostgreSQL, and object storage, plus explicit reverse migration.
- Backup followed by revocation and spending, then restore and independent replay before attempted admission.
- Missing, incomplete, duplicate, conflicting, and out-of-order evidence.
- Unknown scope and known affected-scope restrictions.
- Uncertain execution without duplicate submission, lost reservations, or an unproved refund.
- Process interruption after each component release and before and after final SQL approval.
- Old primary, stale gateway, fresh replica, recovered connection, and restored SQL witness behavior.
- Measured recovery point and recovery time against the actual retained activity boundary.

Catalog preparation alone cannot pass A16 or A33.
The complete task remains open until these operator paths and every assigned acceptance subcase pass.


## Independent owner replay

Local `222a21535` adds ordered KV replay under the retained import barrier.
Each step atomically binds exact preimages, mutations, independent evidence, and its preceding receipt.
The first completed step prevents further snapshot import. Exact retries never restore an earlier value.

The native sequence remains local. Portable backups retain historical receipts without granting replay authority.
This component does not validate domain transitions or complete recovery.

Domain owners must prepare typed replay records. Operator input must not contain arbitrary storage mutations.
Accounting must retain original windows, history identities, pinned valuations, and correction times.
Compute changes from the old and new attempt contributions. Do not infer zero consumption from absent state.
A saturated total requires complete window reconstruction or continued restriction.

Execution replay must include batch claims, job-slot attachments, and account counters.
A lost batch claim must not make a paid line appear unstarted.
A lost attachment must not permit the pending-slot worker to release capacity.
Validate job, reservation, correction, asset, and claim references before activation.
Unknown execution coverage keeps background dispatch closed as well as new requests.

Until affected-scope restrictions cover both paths, keep the entire deployment closed.

The coordinator must combine related owner changes in one bounded native step.
SQL identity changes and blob publication retain their separate receipts.
Final activation must verify those receipts and the complete reviewed interval.
Existing audit records and nonempty evidence references do not establish interval completeness.


## Prepared accounting and retained work

Local `9d97ae2a9` prepares accounting writes from typed later records and new correction receipts.
It requires the same immutable pre-step snapshot for every exact retry.
It preserves original windows, prices, other consumption, pending reservations, and the first settlement time.
Missing original windows and saturated source totals still require reconstruction or continued restriction.
The coordinator must establish independent interval completeness before it uses these writes.

Local `f6430984` validates job-slot counts, history markers, and both directions of each work reference.
An existing claim key does not prove that its job, kind, account, or attachment matches retained work.
These checks do not reconstruct missing post-backup execution or grant dispatch permission.
Complete execution replay and controlled activation remain required.

## Typed execution replay

Local `c496990c` stages batch lines and publishes the parent after complete claimed-line checks.
Local `4dab785c` reconstructs account slot history through an independent census and bounded staging.
Local `7cc3a251` retains complete private video records, correction chains, receipts, and asset references.
Local `bf31ce37` refuses unfinished slot replay markers during final bundle inspection.

These components retain import barriers. The coordinator must validate the complete final view before activation.
Terminal video execution can release concurrency while a separate uncertain money reservation stays held.
This behavior follows the existing service contract. It does not refund the charge.

Long accounting correction histories use bounded intermediate owner states from the retained receipt chain.
Each step retains the same immutable pre-step view for exact retry.
Complete permission replay, missing or saturated window reconstruction, and operator activation remain required.

## Original windows and ordered SQL transitions

Local `b4c4d3be` reconstructs missing or saturated original windows from complete independent history.
Capture and final census verify every retained attempt and correction for that window.
Staging binds the original target preimage and preserves history identity, seed consumption, and first settlement time.
The final view must refuse `budget-replay:v1:window:` markers.

Local `7545cb42` adds ordered SQL replay under the exact relational import barrier.
Each receipt binds sequence, previous receipt, independent evidence, and canonical typed transition digest.
A callback failure rolls back domain changes, the receipt, and the cursor together.
An exact old retry returns its retained receipt without applying earlier state.
Later imports retain historical receipts and remove the source replay cursor.

Permission replay must preserve account and key budget holders, API-key indexes, collection metadata, and the initial-key marker.
SQL identity replay must preserve subjects, membership and grant tuples, and retained team budget origins.
Ordinary creation methods cannot reconstruct these records because they can grant fresh history or create new timestamps.
Delete and recreate require distinct ordered evidence.
The coordinator must also rotate authorization revision authority and validate the complete final graph.

The current ordinary KV enumerator refuses imported stores while their barrier exists.
The coordinator needs an explicit read-only inspection path bound to the exact import claim and replay position.
That inspection must retain the barrier, detect changed ownership or position, and expose no raw mutation authority.
Complete independent interval proof and final operator activation remain required.
