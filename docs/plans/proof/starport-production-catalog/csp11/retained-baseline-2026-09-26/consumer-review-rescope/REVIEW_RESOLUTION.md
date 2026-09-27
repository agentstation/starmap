# Consumer review disposition

The complete Sol/Opus review at `76a5687c` returned four findings.
Two identify defects in CSP11. Two identify planned work required before release.
The reviewed commit remains local. PR385 still contains `7068380bd`.
Starmap PR185 contains reviewed commit `b46c077a7`, with native CI still active.

## Accepted CSP11 defects

**Distinct rollback history.** `AcceptPublication` appends an entry for every accepted publication.
Repeated publications of one generation consume the 32 history slots.
They remove distinct accepted generations from the rollback index.
The collector protects that index and the most recent 32 receipts separately.
A receipt limit must not replace the distinct-generation history contract.

`TestFleetAcceptanceRetainsDistinctRollbackGenerations` reproduces the history loss against real Valkey and PostgreSQL.
The test expects two distinct accepted generations after more than 32 input-only publications.
It finds 32 copies of the current generation.
Preserve the separate receipt window for exact publication retries.

**Complete fleet-key loss.** Existing markers detect isolated head or inventory loss.
They cannot detect when every key under the deployment's fleet prefix disappears.
The SQL approval still matches the unchanged Valkey process, so the adapter reports an unused fleet.
That result can authorize another embedded bootstrap and violates D41.

`TestFleetStoreCompleteKeyLossRequiresRecovery` reproduces this failure with the same live Valkey process and retained PostgreSQL approval.
The constructor accepts the store, and `CurrentHead` returns ordinary not-found status.
The regression requires a recovery refusal instead.
Both regressions fail with race detection. Neither test skips.

## Existing downstream owners

**Restart and adoption.** The current production composition requires an approved PostgreSQL witness for shared catalog storage.
Fresh initialization cannot approve populated stores. Valkey restart or failover changes the backend identity and closes use of the prior approval.
CSP13 owns adoption, external fencing, reconciliation, and controlled approval of the replacement backend.
The complete product must qualify that procedure before release. A bypass would violate the approved recovery contract.

**SQL combinations.** The approved primary fleet recipe uses Valkey and PostgreSQL.
CSP12 owns complete configuration validation and actionable startup errors.
CSP15 owns qualified alternatives. MySQL schema files do not establish supported fleet qualification.
Do not restore a weaker non-fleet path for MySQL or SQLite to avoid a validation error.

## Repair scope before publication

The independent first-publication evidence needs a durable contract in the SQL witness.
Define its ownership and state transitions before changing the schema or publication path.
The contract must distinguish authorized fresh use, interrupted first publication, and loss after established publication.
A timeout cannot prove whether a SQL or Valkey write committed.

The repair must retain original-grant fencing and exact-predecessor checks in the final native Valkey transaction.
It must reject all-key loss without relying on another marker in the same loss domain.
Define crash and retry behavior at each SQL/KV boundary and bind evidence to the approved deployment and recovery epoch.
First-publication recovery must not silently initialize another baseline.

Keep SQL and KV reads outside inference requests. Do not claim an atomic transaction across the two stores.
Add real-store tests for complete loss, partial loss, uncertain writes, first-publication interruption, and concurrent initialization.
Register the new tests in CSP11 acceptance, then rerun the task gate, repository checks, and complete review.

The autoreview skill requires a pause for a new protocol or schema.
Consumer publication pauses at that boundary. D41 and the existing implementation authorization remain valid.
No new product decision remains. CSP11 remains in progress, and both merges remain open.
