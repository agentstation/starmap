# Cache adapter cleanup

The application uses local memory or a dedicated shared-cache client.
The removed layered, hybrid, and durable-KV cache constructors had no production callers at parent `4e277db`.
Their remaining callers were tests. Removing them prevents their results from overstating production coverage.

The local adapter keeps the live get, set, clear, statistics, and close contract.
The response-store interface replaces the unused general cache interface in fill-worker fixtures.
The HTTP alias test now uses the production local manager.
The application deployment-isolation test proves sharing with distinct clients against real Valkey.
It replaces the old mock-KV sharing test, which gave two managers ownership of one wrapper.

Local expiry and clear behavior have a direct test.
Real shared-service expiry remains under `TestSharedCacheRealService`.
The integration Makefile selects the dedicated service tests.
The storage lifetime contract and its backend tests remain unchanged.
Historical layered-cache defects and repairs remain in the earlier proof files and commit history.

## Verification

Work commit: `96860c6`.
The [component race log](adapter-cleanup.jsonl.gz) records 776 passing events and 10 explicit skips, with no failures.
The [real-service log](adapter-real.jsonl.gz) records the separate shared-cache qualification.
The commands use Go 1.27.1 with `GOWORK=/tmp/csp121-host-integration.work`.

```sh
go test -json -race ./internal/cache/... ./internal/proxy ./internal/server ./internal/app
TEST_SHARED_CACHE_URL=redis://127.0.0.1:16389 go test -json -race ./internal/cache ./internal/app -run '^TestSharedCache'
go vet ./internal/cache/... ./internal/app ./internal/server ./internal/proxy
make lint
```

The real-service run passes 10 events without skips. Lint and vet pass.
README prose checks and `git diff --check` pass.
Verification stopped the disposable Valkey service.

## Remaining work

Model and extraction fills still call the local population barrier synchronously.
Their repair must preserve model invalidation during queued work and bound aggregate retained data before ownership transfers.
Response encoding, total capacity, latency, discovery behavior, native CI, review, and merge remain open.
This cleanup does not complete CSP12.1.

Component skips:

- `TestSharedCacheRealService`
- `TestSharedCacheURIAuthenticationAndDatabase`
- `TestSharedCacheTLSWithRealService`
- `TestAuthorizationWarmRequestsAvoidStableReads/valkey`
- `TestAuthorizationUnknownAccountRefusesAdmission/valkey`
- `TestSDKCanonicalRemovalAfterSuccessfulInference`
- `TestSharedCacheCompositionUsesSeparateService`
- `TestSharedCacheCanonicalDeploymentIsolation`
- `TestAppWithValkey`
- `TestSharedStartupDoesNotCreateLocalDatabases`
