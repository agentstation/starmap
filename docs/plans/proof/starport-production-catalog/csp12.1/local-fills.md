# Bounded model and extraction fills

Starport `68a2fbe` moves model and extraction byte writes behind bounded queues.
Model fills share the response queue. Extraction uses an independent queue so disabling response caching does not disable extraction caching.
Each queue permits 1,024 queued or active jobs, 4 MiB of charged data, and two workers.
The combined limits are 2,048 jobs, 8 MiB, and four workers.

The charge covers copied keys, payloads, and 96 bytes per job. It does not bound total heap.

Model invalidation advances a local epoch before clearing records.
A fill takes its epoch before serialization. Old queued writes cannot become visible under the new epoch.
Shutdown joins each queue before closing its stores. Pressure drops optional fills and preserves the answer.

The admin response exposes extraction limits and counters under `extraction_cache`.
The existing `response_cache` counters now cover response and model fills together.

## Evidence

Both [regression tests](model-fill-test.go.gz) fail against parent `96860c6`.
The [before log](model-before.jsonl.gz) proves stale model restoration and bypass of the shared queue.
The [contract run](local-fills-contract.jsonl.gz) passes 70 race-test events without skips.
The [component run](local-fills-components.jsonl.gz) passes 500 events with four explicit skips.
The [real-service run](local-fills-real.jsonl.gz) passes ten events without skips.

Commands use Go 1.27.1 and `GOWORK=/tmp/csp121-host-integration.work`:

```sh
go test -json -race ./internal/cache ./internal/document ./internal/proxy ./internal/server/controllers ./internal/server ./internal/app -run 'Test(BufferedLocal|ModelFill|CacheManager|ResponseFill|ExpiredQueued|ExtractionCache|HTTPAliasRemoval|Admin.*Cache|CacheStatus|.*Document.*|.*Discovery.*)'
go test -json -race ./internal/cache/... ./internal/document ./internal/proxy ./internal/server/controllers
TEST_SHARED_CACHE_URL=redis://127.0.0.1:16389 go test -json -race ./internal/cache ./internal/app -run '^TestSharedCache'
go vet ./internal/cache/... ./internal/document ./internal/app ./internal/server/... ./internal/proxy
make lint
```

The component skips are three shared-service tests and `TestGatewayOverheadBenchmark`.
The separate service run covers the shared-service cases. It does not replace the overhead benchmark.
Lint, vet, README prose checks, and `git diff --check` pass.
Verification stopped and removed the disposable Valkey service.

## Remaining work

Serialization still runs on the caller before queue admission.
Encoded queue bounds do not prove a bound on concurrent encoding allocations or retained heap.
Measure serialization, stream reconstruction, aggregate capacity, and cache failure latency before closing CSP12.1.
Discovery acceptance, native CI, pre-PR review, publication, and merge remain required.
