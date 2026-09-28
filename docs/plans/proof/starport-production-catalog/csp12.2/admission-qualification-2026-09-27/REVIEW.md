# Admission qualification review

CSP12.2 remains in progress. This change adds behavior evidence and registers fourteen required checks.
Nine checks remain unregistered. The [coverage map](coverage-map.json) identifies each remaining requirement.
The registry does not weaken the acceptance roster or qualify the full paid-operation matrix.

## Production process contract

`TestProductionBudgetAcrossProcesses` starts separate gateway processes with real Valkey and PostgreSQL.
A local HTTP fixture replaces the external provider. Application composition, authentication, routing, connectors, and storage use production code.
The fixture creates an account, key, and team. All five applicable meters participate in admission.

While the first request holds capacity, the second process returns HTTP 402 without another provider dispatch.
Measured completion settles all five meters. The second process can then admit another request.
Killing the first process after dispatch preserves its reservation and unknown usage.
A replacement process refuses another request against that allowance. The provider dispatch count remains one.

This test uses an explicitly approved test deployment. It does not qualify operator bootstrap, storage replacement, or background recovery workers.
The test removes only its generated deployment namespace and PostgreSQL schema.
Child processes stop before those removals. Other test services and deployment data remain untouched.

## Preprocessing and retry contracts

The semantic-cache test now sends a paraphrase after the first response enters the cache.
The second request reports a semantic hit and reserves only its embedding charge.
Three provider calls produce three reservations. The fixture's total measured charge is 3,320 nano-USD, with no remaining reservation.

The recognition test waits for chat dispatch after measured recognition completes, then cancels the client request.
Recognition retains its exact settled record. Chat retains uncertain capacity. Both operations use the same budget meters.

The initial test compared a record value with a pointer and could block during HTTP fixture cleanup.
The repair compares the same types, consumes the fixture request body, and releases the handler during cleanup.
The initial failure and timeout remain evidence. They do not identify a production billing defect.

Fallback tests now inspect consumed and reserved capacity after two successful requests and one failed attempt.
The failed attempt retains its full reservation. The two measured attempts consume 22 tokens.
UTC tests cover exact day, ISO week, and month boundaries, a leap February, and an offset across midnight.

## Remaining qualification

The process test qualifies gateway process loss while storage remains available.
Storage failover, complete backend operation counts, and capacity measurements remain required.
The nine coverage gaps, native CI, final dependency pin, pre-PR review, and paired merges remain open.
No paid external provider call ran. No release, merge, or publication occurred in this change.
