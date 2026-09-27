# Batch output recovery and storage accounting

Status: required repair within CSP12.2. This contract extends the durable line contract without creating a separate component PR.
The owner decision about continuing untouched lines remains pending. The storage repair does not depend on that decision.

## Evidence

Consumer `0683b733` retains line claims but does not retain each completed result before starting the next line.
The [process-loss probe](verification.json) kills a real child worker after line one returns and line two starts.
Badger preserves both claims. The file service cannot read line one's result after restart.

The same [proof](verification.json) reproduces duplicate storage release with the production meter on memory, Badger, and Valkey.
Two workers retire one already-deleting file. Both release its charge, so another retained eight-byte file leaves a reported total of zero.
Checkpoint cleanup cannot use this contract without repair.

## Owning boundaries

- `internal/jobs` owns batch and line identity, completed-result references, cancellation, reconstruction, and recovery state.
- `internal/files` owns prepared output identity, readability, retention, and the relationship between file metadata and bytes.
- `internal/limits` owns durable byte claims, capacity bounds, and idempotent release.
- `internal/blob` owns opaque byte storage. It does not infer account identity or permission from an object key.
- `internal/storage` supplies atomic conditional writes. It does not choose file retention or budget policy.

Current behavior:

```text
Line runner -> in-memory result -> aggregate pipe -> readable file at batch end
File cleanup -> delete record -> anonymous byte decrement
```

Required behavior:

```text
Line claim -> stable output identity
Line result -> durable bytes and completion evidence -> completed-result reference
Recovery -> retained result references -> aggregate publication
Cleanup -> durable file claim -> exactly-once byte release
```

## Required contract

1. Allocate a stable output identity before a line can dispatch. Bind it to the account, batch, ordinal, and request identity.
2. Keep byte storage behind the file owner. Do not store large result bodies in the KV ledger.
3. Persist actual result bytes and their digest before marking a line complete or treating its output as recoverable.
4. Distinguish absent output, incomplete output, confirmed output, and expired output. Missing bytes cannot become a successful empty result.
5. Preserve the original result after an ambiguous successful write. A retry must inspect the same identity and must not invoke the provider again.
6. Keep the first accepted result immutable. Reject changed bytes, account identity, digest, or result disposition.
7. Bind storage reservation and release to a durable file claim. Exact retries must not add or subtract capacity twice.
8. Couple file-state transitions and byte-accounting transitions through atomic ownership or a recoverable durable protocol.
9. Retain recovery evidence across crashes between byte writes, metadata publication, quota settlement, deletion, and acknowledgment.
10. Keep retention deadlines stable through retries. A normal abandoned-upload sweep must not delete a live prepared output before its declared recovery deadline.
11. Reconstruct aggregate output from retained results without paid work. Preserve canceled batches and uncertain lines.
12. Remove checkpoints only after durable aggregate publication or their original expiry. Cleanup retries cannot release the same bytes twice.
13. Account for both retained checkpoints and aggregate bytes while both exist. Enforce configured limits without silently widening them.
14. Bound buffers, scans, worker concurrency, and individual writes. Stop further claims when the service cannot confirm durable results.
15. Keep internal preparation records separate from completed files in caller-facing listings. Expose safe recovery and expiry status.
16. Do not treat missing or corrupt byte-accounting state as a confirmed empty account. CSP13 owns migration and recovery approval.

The new contract must serve ordinary file retirement and batch checkpoint cleanup through one byte-accounting owner.
A batch-only counter would leave the same account with inconsistent storage totals.

## Acceptance evidence

Restore the two probe files to their recorded paths and run the exact commands in `verification.json`.
Both probes must pass without weakening their assertions.

Add real-backend checks for these boundaries:

- Concurrent preparation, result publication, cleanup, and exact retries preserve one accepted identity and charge.
- A lost successful storage acknowledgment causes neither duplicate inference nor duplicate byte release.
- Process loss after each durable transition preserves readable output or an explicit incomplete state.
- Recovery never infers a provider result from a settled budget reservation.
- A canceled batch retains previously admitted results while refusing new claims.
- Quota exhaustion preserves existing data and charges. Recovery succeeds after legitimate capacity becomes available.
- Expiry and concurrent cleanup release one charge and preserve unrelated files.
- Badger/filesystem and Valkey/shared-object deployments enforce the same contract.
- Final aggregate publication survives an acknowledgment failure without rebuilding paid work.
- Scans reach later pages and stay within declared time and memory bounds.

Continue the existing producer-then-consumer PR sequence after full CSP12.2 qualification.
These probes do not complete A47, native CI, review, dependency publication, or paired merges.
