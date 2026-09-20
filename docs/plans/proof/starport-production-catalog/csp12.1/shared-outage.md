# Shared cache pause and recovery

Starport commit: `df91956`, parent `bd2499b`.
Go 1.27.1, darwin/arm64.
Starmap workspace dependency: `152148130`.

The test uses a disposable Valkey container with image
`sha256:9acdf6f0ae1771ea63c401e127054b2d1779227b9230dcfae37fa684610eaa4f`.
It warms a real entry, pauses all server commands for one second, then issues
32 concurrent reads and optional fills through the production cache manager.
Each read must return before its independent 200 ms caller deadline.
After the pause, the original entry and available status must recover.
Shutdown must release queued and active fills.

Command:
`TEST_SHARED_CACHE_URL=valkey://127.0.0.1:58668 TEST_SHARED_CACHE_FAULT_URL=valkey://127.0.0.1:58668 GOWORK=/tmp/csp121-host-integration.work go test -json -race ./internal/cache`.

All 36 test events pass without skips.
The [raw log](shared-outage.jsonl.gz) includes existing real-service ACL,
authentication, TLS relay, expiry, reconnect, and capacity checks.
Lint, prose lint, and diff whitespace checks pass.
The disposable container stopped after verification.

Observed read-plus-fill durations under race detection:
p50 2.463958 ms, p95 2.724875 ms, maximum 2.729333 ms.
These are 32 component samples, not production percentiles or HTTP evidence.
A paused server does not establish network-partition behavior.
The fault URL is separate from the ordinary service-test URL because
the test pauses all clients of the selected service.

## Remaining acceptance work

A42 has nine required subcases and A48 has eight.
The registration file currently contains neither group.
Map each subcase to current behavior tests, then add missing behavioral
evidence. Do not count historical removed adapters as current coverage.
Complete remaining discovery isolation and withdrawal checks with the
production cache. Full EOF cost, outage admission, repository qualification,
review, native evidence, and merge remain open.
