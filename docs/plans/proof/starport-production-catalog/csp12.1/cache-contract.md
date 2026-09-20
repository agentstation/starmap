# CSP12.1 cache contract failures

Seven behavior checks fail against Starport `31feeca89b9716b9d1bb162c38db72787818ed82` on Go 1.27.1, macOS arm64.
The run has eight failed test events, including the parent expiry test. It has no passes or skips.
The [result](before.json) records the command and source. The [output](before.jsonl) records each assertion.

The overlay adds tests without changing the reviewed implementation worktree.
Each test uses real Badger in a temporary directory. Cleanup closes all caches and stores.

| Check | Observed failure | Repair owner |
| --- | --- | --- |
| Layered Get, GetMulti, and Warm | Memory returns the entry after Badger reports expiry. Refill creates an entry without expiry. | Cache read and refill contract |
| Hybrid Get, GetMulti, and Warm | Memory returns the entry after Badger reports expiry. Refill grants the configured local TTL. | Cache read and refill contract |
| Default response cache | SetResponse writes `response:answer` through the authoritative KV handle. | Cache composition and configuration |

The expiry tests wait for actual backing-store expiry before checking the cache.
They do not infer expiry from a sleep duration. The assertion requires a cache miss after that observation.

## Required repair

Give cache storage its own interface and lifecycle. Use bounded memory by default in local and replicated deployments.
Select an optional cache-only service through explicit configuration. Do not infer that choice from durable storage or its pub/sub support.

A backing read must bind the value and its remaining lifetime to the same record version.
Separate Get and GetTTL calls cannot prove that contract during replacement.
Anchor a local deadline before the backing read, so transfer delay consumes the returned lifetime.
Refill must cover single reads, batch reads, and warming. Unknown lifetime must skip local refill.

Add replacement-race and delayed-read tests when implementing the cache-owned interface.
Preserve generation, account scope, and admission checks through the change.

## Reproduction

Use the paired workspace named in the result. From the canonical plan worktree, run:

```sh
GOWORK=/tmp/csp102-host-integration.work python3 docs/plans/proof/starport-production-catalog/csp12.1/run-before.py /Users/jack/src/github.com/agentstation/starport-authorization-memory
```

Exit status 1 is the recorded failure. The runner writes the complete result beside this document.
The [test source](cache_contract_test.go.txt) defines the assertions. The [runner](run-before.py) builds its temporary overlay.

## Limits

This is failure evidence, not a repair or task completion.
Valkey expiry, cache outage latency, asynchronous fills, stream bounds, and the discovery audit remain unverified here.
The reviewed authorization branch remains unchanged. CSP6 remains the sole task in progress.
