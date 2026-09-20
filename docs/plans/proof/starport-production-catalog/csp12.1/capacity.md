# Optional cache capacity

Starport commit: `9dda506`, from `b2bcd80`.
Go 1.27.1, darwin/arm64.
Workspace: `/tmp/csp121-host-integration.work`, with Starmap `152148130`.

Local cache cost used slice length. A one-byte slice with a two MiB backing
buffer could enter a one MiB cache. The
[regression before repair](capacity-before.jsonl.gz) fails.
Local storage now charges buffer capacity. Production fill workers own those
buffers. This change adds no payload copy.

## Evidence

`GOWORK=/tmp/csp121-host-integration.work go test -json -race ./internal/cache`
passes 28 test events and skips three real-service cases because
`TEST_SHARED_CACHE_URL` was unset.
The [component log](capacity-components.jsonl.gz) records that run.

After adding queue checks during concurrent load, the final command was:
`GOWORK=/tmp/csp121-host-integration.work go test -json -race -count=3 ./internal/cache -run '^(TestConcurrentOptionalCacheCapacity|TestLocalCacheChargesRetainedBuffer)$'`.
All six events pass without skips in the [final log](capacity-final.jsonl.gz).
Lint, prose lint, and diff whitespace checks pass.

Each load run uses 32 callers, 4,096 keys per cache, eight KiB payloads,
and three local caches with two MiB cost limits.
Both queues remain within their entry, byte, and worker bounds during load.
All stores remain within their cost limits after draining.
Shutdown releases every queued and active fill.

Retained heap deltas after collection are 8,580,096, 8,337,136, and
8,360,720 bytes. Total allocation volumes are 84,745,688, 85,345,424, and
86,157,968 bytes. Optional fill drops occur under this burst workload.
The raw logs retain their counts. These are process measurements for a
component fixture, not full gateway or default-size capacity.

## Remaining work

CSP12.1 still needs unavailable shared-cache latency, stream reconstruction,
remaining discovery cases, acceptance registration, repository qualification,
review, and merge. CSP22 owns full gateway resource and latency qualification.
No implementation task is complete.
