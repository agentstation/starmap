# Stream reconstruction

Starport commit `bd2499b`, parent `9dda506`.
Go 1.27.1, darwin/arm64, Apple M2 Max.
Workspace: `/tmp/csp121-host-integration.work`, Starmap `152148130`.

Repeated string concatenation copied each earlier prefix during completion.
A 64-event fixture fits the production stream retention limit but allocated
1,668,064 bytes while reconstructing 49,152 payload bytes.
The [failing regression](stream-completion-before.jsonl.gz) records this result.

Per-choice builders now accumulate text, reasoning, and tool arguments.
The repair preserves choice ordering, interleaved tools, initial text media,
and input ownership. EOF reconstruction and record encoding still run on the
caller. This change does not claim asynchronous serialization.

## Tests

`GOWORK=/tmp/csp121-host-integration.work go test -json -race ./internal/response/cache ./internal/proxy`
passes 347 test events with no skips in the
[component log](stream-completion-after.jsonl.gz).

A final interleaving regression extends the response contract.
`GOWORK=/tmp/csp121-host-integration.work go test -json -race ./internal/response/cache`
passes 127 events with no skips in the
[final contract log](stream-final-contract.jsonl.gz).

Lint initially reported an unchecked return from a string builder.
Explicitly discarding its always-nil error resolves that diagnostic.
Final lint, prose lint, and diff whitespace checks pass.

## Measurements

Both benchmark variants use
`go test -run '^$' -bench '^BenchmarkBoundedStreamCompletion$' -benchmem -benchtime=200ms -count=3 ./internal/response/cache`
with the workspace above. The before command overlays the parent's
`internal/response/cache/stream.go`.

| Measurement | Before | After |
| --- | --- | --- |
| Time per completion | 188–201 µs | 23–24 µs |
| Bytes allocated | 1,668,070–1,668,084 | 189,032–189,033 |
| Allocations | 196 | 45 |

The [before](stream-benchmark-before.log.gz) and
[after](stream-benchmark-after.log.gz) logs retain exact results.
These are component averages. They exclude encoding, queue admission,
HTTP delivery, shared storage, and production concurrency.
Shared-cache outage latency, remaining discovery cases, acceptance
registration, review, native qualification, and merge remain open.
