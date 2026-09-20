# Cache modes and HTTP admission

Starport commit: `2eb4169`. Starmap registration: `78b351c87`.
Go 1.27.1, darwin/arm64, workspace `/tmp/csp121-host-integration.work`.

The production cache now has a mode matrix covering disabled, exact,
semantic, request opt-out, and expired semantic entries.
Each mode runs through ordinary and streamed proxy responses.
The tests wait for real asynchronous fills and verify cache hits, upstream
calls, expiry, and absence of embedding work in ineligible modes.
The embedding implementation is a deterministic fixture. No paid call runs.

The HTTP test composes the server, routing registry, response cache, key
repository, and budget middleware. Both compatibility prefixes first serve
a request from the warmed production cache. Exhausted budgets return 402.
Unknown required budgets return 503. Expired keys return 401, and disabled
keys return 403. Every refusal occurs before optional cache lookup.
Healthy admission still serves a response when cache lookup fails.

The test controls budget totals and cache transport errors at their interfaces.
It does not qualify atomic shared-budget reservations or storage failover.
The registered outage case also runs the separate real-Valkey pause test.

## Test corrections

The initial HTTP fixture lacked a context-window limit, so routing refused
its model. The corrected catalog carries the limit required by the production
router. An expiry assertion also expected 403. The established API contract
returns 401 for expired keys. Only that expectation changed.
Historical logs retain both fixture failures.

## Final evidence

The [registered results](mode-admission-registered.json.gz) run the three
subcases through the catalog verifier using the final file contents.
All twelve distinct commands pass, with 31 passing race events and no skips.
The report retains exact commands and outputs.

A42.cache_outage_admission, A48.cache_semantic_mode_matrix, and
A48.admission_and_expiry_preserved pass locally. The disposable Valkey
service used `TEST_SHARED_CACHE_FAULT_URL=valkey://127.0.0.1:50194`.
It stopped after qualification. Final lint and diff whitespace checks pass.

The [mode log](semantic-matrix.jsonl.gz) records the first matrix run.
The [initial HTTP log](http-cache-admission.jsonl.gz),
[corrected catalog log](http-cache-admission-final.jsonl.gz), and
[expiry expectation log](http-cache-final.jsonl.gz) preserve test development.
The final registered report supersedes their failed assertions.

## Remaining work

Default memory-budget measurement remains open, along with production-cache
discovery isolation and known withdrawals. Full request latency, repository
gates, native qualification, review, the published dependency, and merges
remain required. No implementation task is complete.
