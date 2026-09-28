# Batch restart and correction qualification

Consumer `501cb0d` implements the accepted batch recovery policy.
The [verification record](verification.json) binds the checks to their source and preserves earlier failures.
CSP12.2 remains in progress.

## Recovery contract

The recovery sweep reconstructs current caller permission before claiming untouched lines.
Each line then passes normal authorization and budget admission.
A durable exclusive claim prevents replay across competing workers.
Completed results remain available. Uncertain claims never authorize another provider call.

Cancellation prevents new claims. Admitted work drains through shutdown.

Private batch records retain the caller hash or a receipt scoped to the account and batch.
They never retain a bearer secret or a reusable console cookie.
Session receipts preserve the original expiry and token-rotation boundary.
A replica that cannot validate the receipt leaves untouched lines pending.
Batch payload version 5 requires migration ownership under CSP13.

## Evidence

The native jobs, session, and request-context suites pass 557 results.
Their two object-store skips pass in a separate seven-result run.
The production restart selection passes eleven results and skips one Valkey case.
The full native suite covers that Valkey case.
Fourteen pure-Go results pass, with one separately covered Valkey skip.
The SQLite/PostgreSQL budget-history handoff selection passes 23 results without skips.

A real Badger process-loss test preserves one completed result and one uncertain claim.
Recovery executes only the third, untouched line.
Production restart tests inspect provider calls and settled reservations.
A withdrawn caller leaves the fifth line unclaimed after four completed calls.
Session tests reject expired, rotated, foreign-account, and foreign-batch receipts.

Vet and Goago pass. Added comments and new files pass the writing check.
Historical file comments retain existing writing diagnostics outside this change.
The initial fail-before capture is a missing-API compilation failure.
It does not prove an old behavioral regression.

## Corrections and cost

Consumer `1fecf04` enforces the 90-day settled-charge correction horizon.
Unresolved reservations and immutable audit receipts remain persistent.
Three native measurements use ten backend calls before provider dispatch and six during settlement.
The added settlement call reads storage-authority time for the original settlement timestamp.
This diagnostic does not qualify production latency, allocation limits, or capacity.
The [profile](correction-budget-profile.json) preserves all three observations and its measurement limits.

## Remaining work

Register and qualify retention and online-batch retry acceptance.
Complete whole-task checks, performance, capacity, native hosted CI, and review.
Qualify the published dependency, then merge the producer and consumer PRs in that order.
No paid provider request or remote publication occurred during these checks.
All task containers stopped after verification.
