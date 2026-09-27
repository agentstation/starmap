# Prepared storage review

CSP11 remains the active task. This review records CSP12 preparation and remaining work.
The prepared checkout is `/tmp/starport-fleet-review-20260926` on `codex/storage-recipe-contract`.
Its committed base is `16aacc6c2d4a5bad73028f2dd7a569b9ecb7e0c1`.
Connection, configuration, test, and operator-guide changes remain uncommitted.

The real-service race suite passes 625 named results with three explicit skips.
The code linter passes after the TLS fixture and file-manifest repairs.
Conflicting URI and explicit credentials initially passed validation, contrary to engineering specification section 8.8.
Two new conflict subcases fail before repair. Six named results pass after repair, including configuration projection.
The changed credential prose passes its writing check. The full operator guide still reports 48 findings outside that edited section.

## Findings for the CSP12 contract

The generic `storage.Transaction` contract needs review before final storage qualification.
`ValkeyTransaction.CompareAndSwap` ignores its expected value and queues an unconditional set.

`ValkeyTransaction.Commit` checks the outer EXEC error but does not inspect each returned command result.
The adapter therefore cannot support its declared atomic conditional contract as written.

A repository search finds no production caller of `BeginTransaction` outside storage implementations.
The additional caller is the `internal/repotest` wrapper. Tests and the internal storage README also use it.
CSP12 must determine whether to retire this unused contract or implement and qualify its complete semantics.
Do not count existing set/rollback tests as conditional transaction proof.

The prepared transaction deadline wrapper bounds a semaphore before calling `valkey.Client.Dedicated`.
Pinned valkey-go v1.0.78 calls its pool with `context.Background` inside that method.
Cold connection acquisition therefore does not receive the caller deadline directly.
Actual deadline failure remains UNVERIFIED. Test cold acquisition and cancellation before accepting the configured bounds.
Ordinary commands use the context-aware client operation.

Deployment-wide KV and notification namespacing remains incomplete.
The current catalog namespace does not qualify isolation for credentials, accounts, budgets, jobs, or notification channels.
CSP12 owns the canonical deployment prefix and its adapter behavior.
CSP13 must migrate existing unprefixed records and recover populated deployments through controlled procedures.
Preserve the distinction when implementing or verifying A41.
