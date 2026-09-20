# CSP12.1 optional response fills

Starport `eee04f9dde56e930c0e80f0431ada5606ceb16f7` moves optional response byte writes into a bounded worker queue.
The [qualification record](response-fills.json) preserves the failed blocking-write test and subsequent passing checks.

Two workers share a limit of 1,024 retained jobs and 4 MiB of charged bytes.
Charges include copied keys, copied payloads, and 64 bytes per job.
Pressure drops optional work. The caller does not wait for cache persistence.
Write contexts expire after at most 500 ms. Shutdown cancels work and joins the workers before closing stores.

Blocking cache adapters must honor context cancellation.
Injected cache reads have a 2 ms context deadline. Local reads avoid an additional timer.

The admin info response reports limits, retained work, completions, drops, and failures without keys or payloads.
Semantic lookup requires the exact response. Pending or dropped exact fills cause a miss even when an index entry exists.
Cache writes do not guarantee a subsequent hit.

## Evidence

The blocking-write regression fails before the repair.
The component race run passes 886 test events with eight skips.
It preceded admin status wiring and the final failure-counter test.
Later targeted runs pass six queue tests and three status, architecture, and application tests.

Seven semantic tests pass. Real Valkey integration passes three events without skips.
Lint reports zero issues. Vet passes. The agent stopped the disposable Valkey service.

A single local warm-read benchmark reports 86.33 ns/op, 7 B/op, and 0 allocs/op.
This measurement does not qualify end-to-end latency or aggregate memory.
Charged queue bytes do not include shared-client buffers or total heap use.

## Remaining scope

Dedicated shared-service configuration, model and extraction fills, stream bounds, capacity, and discovery checks remain open.
Response encoding still runs on the caller. Full task qualification, review, native CI, and merge remain required.
CSP12.1 remains incomplete. The reviewed authorization worktree remains unchanged.
