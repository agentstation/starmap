# Retention and batch acceptance

Producer `bcf20ca1c` registers the final two CSP12.2 acceptance cases.
Consumer `5cfa731` qualifies retention with native Badger and Valkey.
All sixteen selected results pass. All 99 verifier tests pass.
The [verification record](verification.json) preserves the initial fixture compilation failure and the local Valkey skips.

Old accepted evidence remains idempotent after one year.
Missing attempt or correction evidence cannot refund recorded consumption.
Unresolved reservations retain their original capacity without an expiry deadline.
The registry also names production batch retry, restart, cancellation, and process-loss tests.

Consumer `a99fd31` adds native reservation and job checks to the existing fleet CI job.
It supplies Valkey, PostgreSQL, and the pinned recovery object store.
Actionlint passes. Hosted execution remains unverified.

## Boundary repairs

The architecture check found the shared correction-limit dependency missing from its explicit jobs import contract.
Commit `b8072e9` records that dependency. The limits package remains a leaf.
Execution and provider connectors remain outside the job-record boundary.

The reconciliation HTTP handler also imported storage only to classify conflicts.
Commit `8a0c586` moves that classification into jobs and preserves HTTP behavior.
Thirteen authorization-route and architecture results pass.
The first controller-only selection matched no tests and provides no behavior evidence.

A full-suite process-loss test observed an empty marker before its child finished writing it.
The test now publishes the marker with an atomic rename.
Ten repeated race runs pass forty results without changing recovery assertions.

## Open work

Whole-task verification and the remaining boundary checks remain active.
The task gate started at consumer `5cfa731`. Later boundary repairs need final-source qualification.
Production latency, capacity, hosted native CI, review, dependency qualification, and paired merges remain open.
CSP12.2 remains in progress.
