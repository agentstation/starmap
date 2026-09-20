# Production cache discovery and repository qualification

Starport commits: `8f73619` and `c1c272b`.
Starmap registry commit: `706654ead`.
Go 1.27.1, darwin/arm64. The local workspace links these two repositories.
The declared Starmap module remains unchanged and lacks the required host API.

## Discovery contracts

Discovery permission tests now use the production serialized cache.
They cover caller membership, endpoint denial before cache lookup, and
permission withdrawal after a cache hit. Adapter revision coverage remains.
The [focused report](discovery-qualified.jsonl.gz) records the final race run.
The [registered additions](discovery-acceptance.json.gz) pass three commands
across the two existing acceptance subcases.
This report tests the additions, not every check in those subcases.

All 89 registry unit tests pass.

The first two fixture runs assumed immediate visibility after asynchronous
fills. The corrected fixture waits for each fill before asserting a hit.
The [first run](discovery-fixture-before.jsonl.gz) and
[second run](discovery-fixture-second.jsonl.gz) retain both failures.
Neither failure establishes a production defect.

## Architecture repairs

The repository gate found two cache dependencies that needed separate treatment.
The proxy guard included test imports and refused real cache contract tests.
Its cache rule now checks production imports. Other rules retain their scope.

The verifier fixture accepts internal and external cache test imports.
It still rejects a production cache import through the original mutation test.
All six dependency conditions pass.

The admin controller imported the cache implementation to name its status type.
The controller now owns the API status contract. Application composition
converts the cache status into that contract. A JSON comparison verifies
that every operator field retains its value and wire name.

All eight [focused race events](status-contract-final.jsonl.gz) pass without skips.
The [initial run](status-contract-before.jsonl.gz) records an old test fixture
that still returned the implementation type. The fixture now uses the API type.

## Repository evidence

At `8f73619`, the [component race run](repository-components.jsonl.gz)
passes 578 test events and skips two deployment tests.
The [full default suite](repository-tests.jsonl.gz) passes 4,134 test events
and skips 61 optional test events. Skips do not qualify optional behavior.
The component run enables default cache capacity and uses separate disposable
Valkey services for normal cache traffic and deliberate service pauses.
Both services stopped after the run.

The [script record](repository-gates.json) records 25 passing commands and
one failed architecture command. The initial architecture run found the
two dependency issues above and the published-module failure.
The [final architecture run](repository-v1-recheck.log.gz) passes 11 of 12
conditions, including the full Go suite. Only the published-module condition
remains failed. This run includes the status contract repair.

Final vet, lint, and build pass at `c1c272b`.
Installation and SDK smoke checks pass at `8f73619`.
The controller overhead check passes at `c1c272b`: p50 0 ms and p99 4 ms
across 200 requests to a mock upstream with a 20 ms delay.
This component measurement does not establish full gateway overhead.

## Remaining work

Autoreview completed one pass against reviewed parent `31feeca`.
The reviewer was Claude Opus 5 at high effort, with a P0 threshold.
It reported zero findings at that threshold. The
[structured result](autoreview.json.gz) and [report](autoreview.md.gz) retain
the review scope.

The reviewer also noted a possible nil document-cache call.
`WithDocumentCache` takes a pointer, and both `Get` and `Put` handle nil.
That observation does not establish a defect.
The 2 ms read limit remains the accepted optional-cache latency bound.
The unused lifetime-reader contract remains outside this repair.

Native qualification, published-module qualification,
and merge evidence remain required. CSP12.1 is not complete.
