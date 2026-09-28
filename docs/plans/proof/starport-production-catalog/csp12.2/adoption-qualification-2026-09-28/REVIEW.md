# Catalog adoption qualification

CSP12.2 remains in progress. The new tests cover repeated adoption, competing operations, backend replacement during selection, and receipt expiry.

The native test uses PostgreSQL and two Valkey processes. A TCP relay changes the server behind the same client address.
Read-only probes wait for reconnection. The selection mutation executes once and must reject the changed backend identity.
The independent SQL gate stays closed, and the original catalog head stays unchanged.

A second recovery epoch preserves the original publication. Competing recovery operations cannot both succeed.
The first recovered owner cannot use the second recovery epoch.
An adopted catalog cannot renew an expired authority receipt, including after catalog replay and permission refresh.

## Initial failures

The first transport test received EOF before the native identity check.
The client intentionally disables automatic command retries. The fixture now waits for reconnection through bounded read-only probes.
It retains the original identity-error assertion and never retries the mutation.

The first expiry assertion required refresh to return an error for an expired receipt.
The API retains the latest authenticated authority requirement after expiry.
The corrected test verifies unusable permission, the original deadline, and refused admission.
No production behavior changed.

The broader recovery invocation supplied a database in the URL to fixtures that already select database 14.
Those fixtures rejected the conflicting configuration. All 21 recovery results pass with the fixture-owned database selection.
The failed invocation remains in the raw evidence.

The broader catalog suite exceeded its seven-minute deadline after both adoption tests passed.
The concurrent production run also missed its 45-second replacement readiness deadline.
CI separates adoption from the existing fleet step. Each step keeps its seven-minute bound.
The completed adoption tests took 385.99 seconds in the combined run.

Both separated CI commands still need complete qualification.
The prepared local command is `python3 /tmp/csp122-run-separated-fleet.py`.
It needs the three existing task containers and reads their local connection settings without printing credentials.
It uses URL-only connection addresses so each fixture can select its own database.

The separate production run passes all five results without the competing catalog workload.
The original readiness assertion remains unchanged.

## Acceptance coverage

The crash-recovery subcase now requires both native adoption tests and both runtime authority tests, plus the production process test.
CI now gives adoption a separate bounded step. The existing fleet step excludes those same tests. Native platform suites include the runtime tests.
Hosted qualification remains unverified.

The cost-bound subcase now requires fourteen focused tests and the nineteen existing paid-operation checks.
The verifier shares named test results within one invocation, so overlapping checks do not rerun the same test.
The focused tests cover token limits, unsupported billing surfaces, enforced recognition limits, media quantities, exact valuation, and integer overflow.
The complete cost-bound subcase passes all 33 checks through the repository verifier.
The [result summary](bounded-check-summary.json) names every check and binds the compressed raw evidence.

## Remaining work

Complete the retained-charge horizon and interrupted-batch behavior after the owner decisions.
Complete whole-task qualification, latency, capacity, review, native CI, published dependency, and paired merges.
The plan remains at 26 of 40 completed tasks. No release or remote publication occurred.
The three task containers stopped after the storage tests. Docker reports no running containers.
