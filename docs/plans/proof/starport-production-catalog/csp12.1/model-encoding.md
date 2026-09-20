# Bounded model encoding and direct discovery decoding

Starport `7aae8da` bounds model encoding and removes the map round trip from discovery cache hits.
Model, provider, and endpoint reads decode directly into the caller's response type.
Typed decoding also preserves integer values above 2^53. The old generic map used floating-point numbers.

The encoder keeps the previous JSON options. It rejects strings and byte slices above 512 KiB before encoding their contents.
Other containers permit at most 65,536 entries. Pointer unwrapping stops after 64 steps, including cycles.
The output writer enforces the remaining fill budget and checks cancellation.

Fresh encoded buffers transfer to the queue without another payload copy. Admission charges their capacity, keys, and job overhead.

## Verification

The [allocation control](model-encoding-before.jsonl.gz) fails at parent `d289fe3` with the new test file.
The first implementation missed named scalar types. The [compatibility log](model-encoding-contract.jsonl.gz) preserves that failure.
An intermediate generic callback failed to unwrap pointers. Its [failed component log](model-encoding-components-final.jsonl.gz) remains historical evidence.

The final guard handles named types and pointer wrappers. A compile correction uses Go 1.27's `errors.ErrUnsupported` sentinel.

The [component checks](model-encoding-qualified.jsonl.gz) pass 507 race-test events with four explicit skips.
The [integration checks](model-encoding-integration.jsonl.gz) pass ten events with two shared-service skips.
Those runs preceded the final container-count guard. The [final boundary checks](model-final-boundary.jsonl.gz) pass seven events without skips.
They cover oversized encoding, JSON compatibility, typed integers, invalidation, queue sharing, pointer cycles, and capacity charging.
Lint, affected-package vet, prose checks, and `git diff --check` pass on the final source.

Commands use Go 1.27.1 and `GOWORK=/tmp/csp121-host-integration.work`:

```sh
go test -json -race ./internal/cache/... ./internal/proxy ./internal/document ./internal/server/controllers
go test -json -race ./internal/server ./internal/app -run 'Test(HTTPAliasRemovalWithSerializedDiscoveryCache|Discovery|SharedCache|ExtractionCache)'
go test -json -race ./internal/cache -run '^Test(Model|Owned)'
go test -run '^$' -bench '^BenchmarkModelCacheDecode$' -benchmem -benchtime=100ms -count=3 ./internal/app
go test -run '^$' -bench '^BenchmarkOptionalCacheWork$/^model-fill$' -benchmem -benchtime=100ms -count=3 ./internal/app
```

## Component measurements

The [decode comparison](model-final-comparison.txt.gz) uses the same production local cache for both paths.
The control reproduces the old map decode, encode, and typed decode. The direct path decodes once.
The [fill benchmark](model-final-fill-benchmark.txt.gz) records final-source measurements on an Apple M2 Max.

| Case | Previous or control | Final component result |
| --- | --- | --- |
| 1 KiB warm decode | 4.04–4.11 microseconds, 16 allocations | 1.46–1.49 microseconds, 3 allocations |
| 256 KiB warm decode | 675–683 microseconds, 16 allocations | 254–257 microseconds, 3 allocations |
| Rejected 4 MiB model fill | About 2.6 milliseconds and 4 MiB allocated | 627–639 nanoseconds, 184 bytes, 8 allocations |
| 1 KiB model fill | About 1.5 microseconds in the earlier sample | 1.68–1.87 microseconds |
| 256 KiB model fill | About 181–187 microseconds in the earlier sample | 177–181 microseconds |

The earlier fill samples come from the preceding encoding proof. Concurrent worker allocations and drop rates differ across samples.
These are averages from component benchmarks, not full-request percentiles or fleet qualification.

## Remaining work

Full aggregate heap and concurrency qualification remain open, including encoder buffers and custom serialization methods on supported response types.
Qualify stream reconstruction, unavailable shared-cache latency, and the full discovery acceptance matrix.
Native CI, pre-PR review, published dependencies, and merge remain required. CSP12.1 remains incomplete.
No background processes or disposable services remain active.
