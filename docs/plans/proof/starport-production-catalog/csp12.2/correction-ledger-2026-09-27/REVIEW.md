# Correction ledger review

Consumer `3e1729c73ba30a9b9c2e53cf52326da3f7bbf214` implements reservation corrections.
The administrator API still accepts only the initial decision. CSP12.2 remains in progress.

## Contract

One atomic storage operation updates the attempt, every original budget window, and one immutable correction receipt.
The receipt preserves the complete prior attempt, operator identity, evidence reference, reason, corrected units, and authority time.
Its prior correction identifier links the history without an unbounded attempt record.

The operator must bind a correction to the inspected attempt state. A stale binding cannot replace a newer decision.
An exact retry returns its original receipt, including after another correction. Changed reuse of a correction identifier fails.

Each disputed attempt contributes one count to its original windows. A correction removes only that contribution.
Other disputes continue to block admission. Repeated observations of the resolved evidence do not restore that dispute.

Correction uses the original valuation and windows. It changes consumed units by the accepted charge difference or settles retained capacity.
It preserves unrelated reservations, initial consumption, and unknown monetary cost for token-only evidence.
An overflowed window remains restricted. This operation cannot reconstruct a saturated aggregate or replace window history.

Attempt payload 3, window payload 2, and correction payload 1 are current. History payload 1 remains unchanged.
Readers reject older attempt and window payloads. CSP13 owns migration before release qualification.

## Evidence

The baseline test fails because two disputed attempts have no separate retained count.
The final limits race run passes 236 results with one PostgreSQL-dependent skip.
The final pure-Go run passes 41 results. Four production reconciliation results pass.
Counts include parent tests. The verification manifest records exact commands, output hashes, and source boundaries.

Actual process termination before and after the atomic correction verifies Badger recovery and exact retry.
Real Valkey tests cover competing corrections, concurrent exact retries, lost acknowledgements, and overlapping disputes.
Late correction tests preserve the newer window's reservation.

Vet, goago, package layout, and final source prose checks pass.
The first broad run exposed a prior test compile error from the stored-byte relocation.
The repair restores the local limits pointer in the clone assertion. It preserves the assertion's meaning.

## Manual review

Reviewed storage mutations, record validation, concurrent readers, retry identity, original-window arithmetic, and caller boundaries.
The storage transaction retains the audit before it grants capacity. No provider operation appears in the correction path.
A missing previous audit prevents another correction. Invalid counters and overflow prevent partial release.
No new dependency or shared SQL authority enters the reservation package.

The required pre-PR second-model review has not run. This component is not a production correction procedure.

The whole-repository prose check reports 158 diagnostics across 1,780 files during editing, including historical proof.
Final targeted checks pass for all six edited documentation files. The whole-repository prose gate remains open.

## Remaining integration

The job owner must retain correction intent and audit before requesting ledger correction.
It must expose authenticated inspection and correction, preserve the first decision and late provider evidence, and recover after partial completion.
Jobs without a budget reservation still require durable operator audit.

Usage reporting needs explicit adjustments. Replacing the original usage record would violate its existing immutable identity contract.
Reporting failures must not undo required ledger settlement or cause duplicate adjustments.
The current job accounted flag alone cannot establish that a correction reached reporting.

Full A47, shared failover, capacity, native checks, required review, and paired merges remain open.
