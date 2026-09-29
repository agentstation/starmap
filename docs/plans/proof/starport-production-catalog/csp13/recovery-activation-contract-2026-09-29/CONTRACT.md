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
