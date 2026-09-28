# Settled-charge correction horizon

The owner selected a 90-day horizon on September 28, 2026.
Consumer `1fecf04894ef6a27ffa964083fda78a9472744dc` implements the deadline.
CSP12.2 remains in progress.

## Behavior

Budget attempts retain their first settlement timestamp from storage-authority time.
New corrections stop at that timestamp plus 90 days. Subsequent corrections do not extend the horizon.
The native conditional write checks the deadline again before publication.
Whole-second storage boundaries round the deadline down by less than one second.

Exact accepted retries retain their receipts after expiry.
Unresolved reservations have no retention deadline. Their original window capacity remains reserved until reconciliation.
Audit records remain persistent. This change does not authorize audit deletion.

Jobs without required reservations use the original administrator decision time.
An accepted intent retains its original decision time through recovery.
Administrator inspection reports `correction_horizon_expired` and omits a new correction binding after expiry.
New expired requests return HTTP 409. Required reservations enforce their own deadline during the atomic correction write.

## Evidence

The [verification record](verification.json) binds all runs to source and lists skipped cases.
Both new deadline tests fail against the preceding implementation.
The final reservation race suite passes 190 results, with one PostgreSQL handoff skip.
The native job, application, and controller selection passes 72 results without skips.

The pure-Go selection passes nine results, with four Valkey skips.
Vet and Goago pass. Seven changed prose files pass the writing check.

A native recovery test stopped after 100 scans before reaching every record.
It now requires a complete scan within 30 seconds and preserves all original count assertions.
The regression passes three results. The original failure remains recorded.

## Remaining work

Attempt payload version 4 requires migration ownership under CSP13.
Settlement adds one storage-authority time read. Remeasure the real-storage operation profile before task completion.
Batch restart now has a separate [implementation proof](../batch-resume-2026-09-28/verification.json).
Complete task qualification, native hosted CI, review, dependency qualification, and paired merges remain required.
