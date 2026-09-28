# Interrupted batch recovery contract

Status: architecture repair within CSP12.2. The owner approved automatic recovery of proven-unstarted lines on September 28, 2026.
This contract supplements the paid-operation matrix. It does not create another plan or authorize a component PR.

## Verified gap

At consumer `7551e8d4`, the batch record persists aggregate state, but the runner retains line progress in memory.
A second worker can acknowledge cancellation without stopping subsequent dispatch on the first worker.
The [failing probe](verification.json) reproduces three dispatches where only the first admitted line should run.
All three storage backends fail: memory, Badger, and Valkey.

A restart cannot reconstruct which input lines produced results from the current batch record.
The recovery sweep only releases finished claims. A timeout cannot prove that an interrupted provider attempt incurred no charge.

## Owner decision

On September 28, 2026, the owner selected automatic recovery of proven-unstarted lines.
Retain completed results. Never repeat uncertain attempts automatically.
Every resumed line requires current authorization and normal budget admission.
A missing result does not prove that a line never started.

## Ownership

`internal/jobs` owns durable batch and line transitions, cancellation, result references, and recovery enumeration.
The production runner owns protocol decoding and current authorization. Existing router admission owns each paid provider attempt and its budget reservation.
File and blob services own retained input and output bytes. Storage provides atomic comparisons and writes.

Current ownership:

```text
Batch service -> Batch repository: aggregate state
Batch service -> local worker: line progress and result pipe
Other worker -> Batch repository: cancellation
Local worker -> Provider: subsequent lines continue
```

Required ownership:

```text
Batch service -> Batch repository: batch state and immutable input identity
Batch service -> Durable line records: claim, attempt association, result reference
Other worker -> Batch repository: cancellation
Line claim -> atomic batch-state check -> admitted worker
Admitted worker -> Router admission -> Provider
Recovery worker -> Durable line records -> retained results and uncertain attempts
```

## Required behavior

1. Bind each line to its account, batch, input identity, and ordinal before execution.
2. Atomically compare current batch state when admitting each line. Acknowledged cancellation prevents later claims across workers.
3. Preserve the agreed policy for lines admitted before cancellation. Their results and reservations remain recoverable.
4. Persist dispatch identity before paid work. An ambiguous claim or dispatch acknowledgment cannot authorize another provider call.
5. Preserve each actual retry and paid child operation under its existing reservation owner.
6. Persist result references before counting a line complete. Recovery must not invent a result from a settled charge.
7. Recover completed results without another provider call. Keep uncertain attempts visible and retain their reservations.
8. Bound scans, workers, and retained bytes. Do not load an entire batch into memory.
9. Use current authorization for every newly admitted line. A durable queue cannot preserve withdrawn permission indefinitely.
10. Refuse unavailable or inconsistent state without granting capacity. Wall-clock timeout alone cannot refund an uncertain attempt.
11. Preserve account isolation, original input identity, and immutable execution evidence through restart and competing recovery.
12. Keep caller-visible recovery state separate from private provider and audit evidence.

## Verification

Restore `batch_remote_probe_test.go.txt` to its recorded package path and run the command in `verification.json`.
The cancellation probe must pass without changing its expected dispatch count.

Add contract and production tests for these cases:

- Competing workers claim one line, including a lost successful write acknowledgment.
- Cancellation on another worker wins before the next claim. Previously admitted work drains.
- Process loss occurs before claim, after claim, after dispatch, after receipt, and before final batch publication.
- Completed results survive restart. Uncertain work never dispatches again automatically.
- Recovery follows the owner's selected policy for untouched lines.
- Current authorization denies a newly claimed line after permission withdrawal.
- Storage failure preserves result and charge evidence and retries publication without repeating inference.
- Badger restart and shared Valkey/PostgreSQL execution preserve the same ownership contracts.
- Bounded recovery reaches records beyond the first scan page without duplicate release or unbounded memory.

Count provider calls and inspect durable reservations in production-path tests.
Two services in one process do not replace the required process-loss and fleet qualification.
CSP12.2 remains in progress until its complete matrix, A47, checks, review, dependency pin, and paired merges pass.


## Claim implementation evidence

Consumer `0683b733` implements the atomic claim and cancellation boundary.
The original cancellation probe passes on memory, Badger, and Valkey. Claims retain input digests and request identities.
Lost acknowledgment and concurrent claim checks pass without repeated dispatch.
The [claim proof](../batch-line-claims-2026-09-27/verification.json) records exact scope and counts.

The remaining contract still requires durable results, interrupted-run recovery, and process-loss qualification.
The owner approved automatic recovery of proven-unstarted lines on September 28, 2026. Do not count the claim repair as complete batch recovery.
