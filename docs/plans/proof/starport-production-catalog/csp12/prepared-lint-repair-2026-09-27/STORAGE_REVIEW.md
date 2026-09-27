# Prepared storage review

CSP11 remains the active task. This review records CSP12 preparation and remaining work.
The prepared checkout is `/tmp/starport-fleet-review-20260926` on `codex/storage-recipe-contract`.
Its prepared commit is `63c763b`.
Connection, configuration, test, and operator-guide changes remain unpublished.

The real-service race suite passes 625 named results with three explicit skips.
The code linter passes after the TLS fixture and file-manifest repairs.
Conflicting URI and explicit credentials initially passed validation, contrary to engineering specification section 8.8.
Two new conflict subcases fail before repair. Six named results pass after repair, including configuration projection.
The changed credential prose passes its writing check. The full operator guide still reports 48 findings outside that edited section.

## Findings for the CSP12 contract

The generic `storage.Transaction` contract declared behavior that its adapters did not enforce.
`ValkeyTransaction.CompareAndSwap` ignores its expected value and queues an unconditional set.

`ValkeyTransaction.Commit` checks the outer EXEC error but does not inspect each returned command result.
The adapter therefore cannot support its declared atomic conditional contract as written.

A real Valkey probe confirms both defects with the race detector.
The conditional write changes the value despite a mismatched expectation.
The commit also discards a server command error. Both leaf cases fail.

The first probe matched no tests because its overlay used a noncanonical temporary path.
Treat that run as UNVERIFIED. The retained probe uses the resolved path and records three named failures, including its parent.

A repository search finds no production caller of `BeginTransaction` outside storage implementations.
The additional caller is the `internal/repotest` wrapper. Tests and the internal storage README also use it.
The prepared CSP12 change removes this unused internal contract and its adapter implementations.
Production callers retain `CompareAndSwapBatch` and its all-or-none contract tests.
Do not count existing set/rollback tests as conditional transaction proof.

The prepared transaction deadline wrapper bounds a semaphore before calling `valkey.Client.Dedicated`.
Pinned valkey-go v1.0.78 calls its pool with `context.Background` inside that method.
Cold connection acquisition therefore does not receive the caller deadline directly.

Actual deadline failure remains UNVERIFIED. The prepared change removes this unused dedicated-client path.
Ordinary commands and batches keep their context-aware client operations.
After the change, 604 named real-service race results pass and three explicit cases skip.
The code linter reports zero issues. Full CSP12 qualification remains open.

Deployment-wide KV and notification namespacing remains incomplete.
The current catalog namespace does not qualify isolation for credentials, accounts, budgets, jobs, or notification channels.
CSP12 owns the canonical deployment prefix and its adapter behavior.
CSP13 must migrate existing unprefixed records and recover populated deployments through controlled procedures.
Preserve the distinction when implementing or verifying A41.
