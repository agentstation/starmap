# Cache encoding and endpoint reuse

Starport `d289fe3` repairs endpoint cache reuse and rejects oversized extraction input before serialization.
The benchmark source in this commit measures optional cache work at the component boundary.
It includes test assertions and can count concurrent worker allocations. It does not measure full HTTP latency or percentiles.

## Findings and repairs

The [initial measurements](cache-cost-before.txt.gz) ran against parent `68a2fbe` with the new benchmark file.
On this Apple M2 Max, local response hits took 95.94–97.65 ns/op. Misses took 47.56–51.66 ns/op.
Both reported zero allocations per operation, with amortized background bytes in the allocation totals.

A 4 MiB model fill took 2.62–2.66 ms/op and always dropped after serialization.
A 4 MiB extraction fill took 2.64–2.68 ms/op and allocated about 4 MiB before dropping.
The model case remains unresolved. It is not a production latency pass.

Extraction caching now limits all identity, text, and offering strings to 512 KiB before encoding.
The conservative limit leaves room for six-byte JSON escaping and record overhead inside the fill budget.
Larger records skip caching and increment its drop counter. The caller still receives the extraction result.
The [new measurement](extraction-size-benchmark.txt.gz) reports 204.1–205.6 ns/op and zero allocations for the 4 MiB skipped extraction.

Endpoint reads expected a typed pointer, but the production cache returned a decoded map.
The [before regression](endpoint-before.jsonl.gz) proves a warm endpoint request rebuilt and rewrote its result.
The repaired path decodes the stored representation and returns it without another fill.
The HTTP test still verifies known alias withdrawal before cache lookup.
Map decoding and re-encoding remain additional work that needs measurement and simplification.

## Verification

The [endpoint checks](endpoint-after.jsonl.gz) pass 69 race-test events without skips.
The [extraction control](extraction-size-before.jsonl.gz) fails before repair.
The [extraction checks](extraction-size-after.jsonl.gz) pass 23 events with five shared-service skips.
The [boundary checks](extraction-boundary.jsonl.gz) pass two events, covering escaped text, exact input capacity, and offering bytes.
Lint, affected-package vet, prose checks, and `git diff --check` pass.

Commands use Go 1.27.1 and `GOWORK=/tmp/csp121-host-integration.work`:

```sh
go test -run '^$' -bench '^BenchmarkOptionalCacheWork$' -benchmem -benchtime=100ms -count=3 ./internal/app
go test -run '^$' -bench '^BenchmarkOptionalCacheWork/extraction-fill/4194304$' -benchmem -benchtime=100ms -count=3 ./internal/app
go test -json -race ./internal/proxy ./internal/server -run 'Test(HTTPAliasRemovalWithSerializedDiscoveryCache|Discovery|.*Cache.*|.*Endpoint.*)'
go test -json -race ./internal/document ./internal/cache ./internal/app -run 'Test(.*Cache.*|ModelFill|BufferedLocal|ExtractionCache)'
go test -json -race ./internal/document -run '^TestCache(InputBudget|SkipsOversized)'
```

No background processes or disposable services remain active.
CSP12.1 still needs model encoding bounds, aggregate memory and latency qualification, and the full discovery acceptance matrix.
Native CI, pre-PR review, published dependencies, and merge remain required.
