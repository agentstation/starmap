# Aggregate recovery review

Status: local implementation review. CSP12.2 remains in progress.
This review does not replace the required pre-PR autoreview.

## Contracts checked

- Recovery streams retained results in input order. It has no provider runner.
- Aggregate identities bind the account, batch, and result category.
- Exact retries retain the first digest, size, expiry, and storage bound.
- Complete batch metadata precedes public file exposure and checkpoint retirement.
- Cleanup validates all line references and result counts before deleting any checkpoint.
- Partial cleanup can retry after a lost acknowledgment without releasing unrelated capacity.
- Concurrent recovery preserves one aggregate per category and one byte charge.
- Incomplete failed runs retain their claims. Cancellation permits retained results from previously admitted lines.
- Status reads do not reconstruct aggregate bytes. Workers and background sweeps own recovery.
- The storage meter now belongs to `internal/limits/storedbytes`. The parent remains a vocabulary package.

## Evidence and repairs

The original aggregate retry probe allocated a second file identity.
The cleanup probe accepted a terminal record without aggregate references.
Failure tests exposed a non-idempotent deletion retry after a lost acknowledgment.
The broad run showed that recovery finished incomplete failed batches too early.

The import graph check exposed a storage dependency in the limits vocabulary.
The router test exposed temporary-directory cleanup before checkpoint retirement completed.
Its final assertions now wait for the durable cleanup marker before removing the byte store.
All original permission and result assertions remain unchanged.

Real child-process tests interrupt aggregate publication and batch metadata publication.
Restart preserves both output categories, their identities, and the exact storage charge.
Other checks cover quota exhaustion, original bounds, corruption, expiry, concurrent recovery, and file visibility.

## Remaining qualification

Valkey and shared-object aggregate recovery remain unverified in this revision.
Docker remains stopped. This revision uses memory, Badger, local filesystem, and local SQLite application tests.
Native CI, capacity, restore, failover, and complete A47 remain open.
File schema 5 and batch schema 4 require coordinated migration under CSP13.
The owner decision about whether untouched lines can continue automatically remains pending.
Audited correction and the full paid-operation matrix remain required.

Application shutdown does not yet join batch workers before closing their storage.
CSP12.2 must add a bounded drain contract and test shutdown during aggregate retirement.
A completed batch promises readable aggregate bytes. Its cleanup can still be active.
Do not treat terminal status alone as permission to remove a live byte store.
