# Usage adjustment component review

Consumer `27983f87100452b39c56f4bf6f8062a0a4c4862f` adds explicit usage adjustments.
The original request bytes remain immutable. Adjustments change token and cost totals in the original windows without another request count.
An atomic write stores the receipt, current adjustment, and affected totals together.

Exact retries preserve prior receipts. Stale predecessors and conflicting identifiers refuse changes.
Missing counters, corrupt records, expired history, and numeric overflow refuse the complete write.
Receipts expire with the original usage record. A correction cannot extend that deadline.

Activity JSON, CSV, and NDJSON show corrected billing and retain the original values.
Provider measurements remain separate from administrator evidence. Public adjustment identifiers hash the private job decision identifiers.
Account filters run before adjustment validation, so another account's corrupt adjustment cannot block a filtered result.

The final shared check passes 134 race-test results without skips. The final pure-Go check passes 60 results without skips.
Four existing production reconciliation results also pass. These results count parent tests and subtests.

The earlier full usage run passes 181 results before the final changes. It does not qualify the final source alone.
The [verification record](verification.json) preserves commands, raw outputs, failures, and source boundaries.

## Required integration

The job service does not yet call `CorrectionAccountant`. No administrator correction route exists.
The budget ledger and usage reporter pass component checks, but the complete correction flow remains unavailable.

The job owner must retain correction intent and immutable history before budget correction.
Late provider evidence and its dispute restriction must commit with the relevant job state.
A correction must compare that same state when it clears a dispute. Separate job and budget writes do not prove this contract.
Deterministic race tests must cover both commit orders and lost acknowledgements.

Correction reporting needs its own durable acknowledgement. The original `AccountedAt` value cannot acknowledge a later adjustment.
Reporting expiry must not reverse required settlement or recreate expired usage history.
The current job reporter does not deliver correction events to optional external sinks. This review makes no such delivery claim.

CSP12.2 remains in progress. Full operation coverage, shared recovery, capacity, native qualification, review, and paired merges remain required.
