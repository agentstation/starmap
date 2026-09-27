# Independent first-publication contract

CSP11 owns this repair under D41. The protocol pause ends with this contract.
The existing authorization covers implementation and qualification. No product default changes.

## Owners and state

PostgreSQL owns one durable bootstrap permission in the catalog recovery row.
The permission belongs to the exact deployment, open recovery epoch, backend identity, and approval evidence.
Only explicit fresh initialization grants it. Schema migration defaults existing rows to denied.
Ordinary approval, restart, and recovery cannot infer fresh permission from empty KV state.
Closing or replacing recovery approval clears the permission.

The permission has two states: available and consumed. Consumption never reverses within an approval.
This field does not alter the immutable recovery identity or invalidate a valid existing publication.
Valkey retains catalog bytes, publication receipts, native grants, and the atomic publication transaction.

## Publication order

1. Validate the original grant, exact predecessor, and retained payload under the native maintenance lease.
2. Stage the complete pending publication in Valkey.
3. For an empty predecessor only, consume bootstrap permission through one conditional SQL update.
4. Publish the first head through the existing native Valkey transaction.
5. Return the accepted head only after that transaction succeeds.

The SQL update compares the complete approval and available permission. One caller can consume it.
It does not replace the final native lease, grant, incarnation, and predecessor checks.
A SQL timeout stops publication. It cannot establish whether consumption committed.
There is no atomic transaction across SQL and Valkey.

## Failure and retry

| Observed state | Required behavior |
| --- | --- |
| Fresh approval, permission available, no head or prior publication | Allow initial publication under a valid native grant. |
| Crash before SQL consumption | Permit cleanup of an abandoned pending upload and a fresh attempt. |
| SQL consumption committed, first head absent | Refuse startup bootstrap and new publication. Require controlled recovery. |
| SQL response uncertain | Stop the attempt. Read the independent state before another decision. |
| Native commit succeeded, response lost | Return the retained exact publication receipt on retry. |
| All fleet keys disappear while Valkey process remains unchanged | Consumed SQL permission requires recovery. |
| Head exists but inventory or chunks are missing | Refuse operation through existing integrity checks. |
| Recovery approval changes, closes, or moves to another backend | Refuse the old approval and original grant. |

CSP13 owns recovery after interrupted first publication, populated deployment adoption, and backend replacement.
Its procedure must establish a new authorized state after external fencing and reconciliation.
Do not clear the consumed permission during an ordinary restart or retry.

## Rollback history

Maintain up to 32 distinct accepted generation IDs in most-recent-acceptance order.
Accepting an existing ID moves that ID to the end without consuming another slot.
Preserve its validated immutable generation metadata. Publication receipt retention remains separate.
The collector protects every retained accepted generation even when its requested limit is smaller.

## Required evidence

Use real PostgreSQL and Valkey for both existing failing regressions and publication boundary failures.
Cover atomic single-winner SQL consumption, wrong approval, closed approval, and conservative migrated rows.
Cover a failed first native commit after consumption, a lost successful native response, complete key loss, and first-use concurrency.

Fresh initialization must prove that it grants permission. Completed initialization must not grant another permission on retry.
Verify distinct rollback history and collection protection after more than 32 input-only publications.
Keep all storage operations outside inference requests. Preserve existing exact-grant and predecessor tests.
