# Atomic late-evidence publication review

Consumer `0db2fbcdbefcfe5b345ddbf256f41ce684174326` commits provider evidence with its required budget restriction.
The job repository prepares a validated replacement. The budget transaction compares that replacement with the attempt and all original windows.
A stale job prevents every write. A concurrent budget change causes another bounded comparison.

The original provider receipt remains available when publication fails. Recovery reads the durable job after a lost acknowledgement.
An existing dispute does not increment its count again. Resolved evidence cannot attach to a different job state without a new decision.
The transaction does not change charges, repeat provider generation, or grant capacity.

The baseline test reproduces the old two-write sequence. A stale job leaves a budget dispute behind and fails the expected invariant.
The corrected test covers the combined operation. Further tests cover concurrent job changes, write failure, lost acknowledgement, existing disputes, and resolved evidence.
The job repository test confirms that preparation writes nothing and that stale state prevents a companion write.
Four production tests preserve initial administrator reconciliation and late-provider refusal behavior.

The full shared suite passes 546 results with two skips. The skips require PostgreSQL and object storage, which remained stopped.
Final targeted race and pure-Go runs each pass 21 results without skips. Counts include parent tests and subtests.
The [verification record](verification.json) retains commands, failures, output, and source boundaries.

## Remaining correction contract

The administrator correction operation remains unfinished. It must persist job-owned intent and immutable history before required settlement.
It must compare the inspected job state when it changes the budget correction receipt and clears a dispute.
The new late-evidence transaction supplies one side of that contract. A separate ledger correction followed by job publication remains insufficient.

Reporting requires a separate durable acknowledgement for each correction. Recovery must preserve the first decision and provider evidence without another paid request.
The full CSP12.2 acceptance gate remains open. No component result proves complete fleet recovery or release readiness.
