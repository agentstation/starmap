# CSP12.2 shared recovery evidence

The backend replacement check fails at replacement startup. It remains a required test.
Concurrent settlement and gateway process-loss checks pass against real Valkey and PostgreSQL.
The first failed run compared a record value with a pointer. The corrected assertion compares matching types.

## Observed replacement behavior

The fixture stops its gateway writers and copies its scoped KV records to an independent Valkey process.
Copied recovery records cannot approve that process. Production construction refuses it.
The fixture then closes the SQL recovery epoch and explicitly approves the replacement budget authority.
The original reservation still has the same identity, valuation, windows, and amounts.

A replacement gateway still refuses startup with `catalog bootstrap permission is consumed; recovery is required`.
The test expects that gateway to retain uncertain capacity, refuse unfunded dispatch, and resume only after explicit reconciliation.
Those remaining assertions did not run. This result does not qualify successful backend recovery.

Inspect the original catalog publication and copied inventory before changing the implementation.
The current evidence cannot distinguish an incomplete fixture from missing catalog recovery behavior.
Never restore service by resetting consumed bootstrap permission or replacing the catalog with an unrelated baseline.
The full catalog and budget restore contract remains within the production plan.

## Media settlement evidence

Fourteen named tests supply 88 passing results across supported media operations and required job settlement.
The mapping reuses the preceding matrix and local-restart evidence. It adds the real Valkey approval test.
The shared test proves that a settled job cannot bypass closed approval or silently adopt another recovery epoch.
The evidence preserves its original commands and source locations. It does not represent a new complete task-gate invocation.

## Required follow-up

CSP12.2 owns the failing production dispatch probe and the accounting invariants.
CSP13 owns complete migration, backup manifests, and controlled catalog adoption after backend replacement.
Resolve the catalog recovery prerequisite before accepting this probe or publishing these branches.
Retain both the successful gateway process-loss evidence and the failed replacement evidence.

The workflow now schedules the production fleet test with PostgreSQL and two Valkey services.
Its native CI run remains unverified. The verifier suite passes 99 tests, and fourteen action references match their release tags.
Vet, Goago, and the changed test's prose check pass. No paid inference ran.

The three task database containers and Docker Desktop stopped after verification.
The correction-window question and the earlier interrupted-batch decision await the owner.
Bounds, retention, batch recovery, latency, capacity, full qualification, review, and paired merges remain required.
