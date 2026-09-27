# Interrupted batch recovery contract

Status: architecture repair within CSP12.2. The owner decision about untouched lines remains pending.
This contract supplements the paid-operation matrix. It does not create another plan or authorize a component PR.

## Verified gap

At consumer `7551e8d4`, the batch record persists aggregate state, but the runner retains line progress in memory.
A second worker can acknowledge cancellation without stopping subsequent dispatch on the first worker.
The [failing probe](verification.json) reproduces three dispatches where only the first admitted line should run.
All three storage backends fail: memory, Badger, and Valkey.

A restart cannot reconstruct which input lines produced results from the current batch record.
The recovery sweep only releases finished claims. A timeout cannot prove that an interrupted provider attempt incurred no charge.

## Owner decision

Should recovery continue lines that durable records prove never started, or stop the batch and require explicit new submission?
The recommendation continues only proven unstarted lines and retains completed results.
Uncertain dispatches must never repeat automatically under either choice.
Do not implement restart behavior until the owner answers.

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
