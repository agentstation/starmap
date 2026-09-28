# Administrator correction review

Consumer `0e3e3aba950b01422012c2c591f6d8c0706f2d94` connects durable job intent to the required budget transaction.
Administrator routes retain authenticated actor, reason, evidence reference, inspected state, and stable decision identity.
The first decision and independent provider evidence remain immutable. A pending intent changes no charge.

The correction transaction publishes the job, applied audit, report ordering link, budget receipt, attempt, and original windows together.
The same KV authority owns these records. Changed evidence refuses the pending decision.
A new decision can supersede unapplied intent while preserving its history. An exact old retry cannot undo a newer correction.

Optional usage adjustments follow applied-decision order through durable forward links. Each recovery call handles at most sixteen reports.
Report outcomes distinguish delivery, expiry, and disabled reporting. Failed optional reporting cannot reverse required settlement or block another operator decision.
Original reports retain their initial billing input. Adjustments change totals without adding a request count.

Original-report expiry now retains `reporting_expired_at`. It leaves `accounted` false and stops delivery retries.
Required audit history remains available after optional retention expires. The real reporter refuses expired writes without recreating usage or counters.

## Review findings and repairs

A pending correction can coexist with new provider evidence. Status text cannot determine whether a budget dispute exists.
The required owner now reads `BillingConflict` directly. Its production test retains the dispute until a new operator decision resolves the evidence.
An explicit reviewed decision can also resolve late evidence without measurements. It cannot invent provider measurements or repeat inference.

The original-report expiry probe failed before the repair. It now passes on memory, Badger, and Valkey.
Ordinary replacement cannot erase the expiry marker or correction history. Legacy job payloads fail closed until migration.

## Evidence and limits

The [verification record](verification.json) preserves commands, exact counts, failed intermediate runs, and source boundaries.
Final job race checks pass 458 results with one optional MinIO skip. Final production checks pass four results.
Final adapter and route race checks pass sixteen results. Pure-Go checks pass 54 results and skip sixteen Valkey subtests.

Counts include parent tests and subtests. Vet, goago, package layout, and changed-section writing checks pass.
The full operator-guide writing check still reports 51 earlier diagnostics.

Job payload schema 6 and job correction history schema 1 require CSP13 migration.
This local component does not establish fleet failover, complete A47, native CI, release qualification, or a merged consumer dependency.
No further paid generation ran. CSP12.2 remains in progress.
