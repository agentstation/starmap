# Shared cache deployment identity

Starport commit `4e277db` binds shared response entries to the canonical deployment ID.
The prior cache namespace could cross deployment boundaries even when operators selected different deployment IDs.

## Evidence

The [original fixture](identity-before-test.go.gz) failed against `51767ba` with real Valkey.
The [before log](identity-before.jsonl.gz) records one failed isolation test.
The same deployment could share its response, but a different deployment could also read it.

The repair removes the separate cache namespace setting.
Configuration and shared cache construction use one deployment identity validator.
Keys use `starport:v1:<base64url-deployment-id>:cache:` with unpadded UTF-8 encoding.
The admin cache status reports the effective prefix for ACL provisioning.
Unicode, delimiters, and wildcard characters remain opaque identity data.

Independent deployments must select different IDs. Replicas share an ID. The default remains `local`.

The [intermediate log](identity-intermediate.jsonl.gz) preserves a fixture failure.
That fixture bypassed configuration loading and supplied no deployment identity.
It now uses the production loader. The runtime still requires a valid identity.

The [final race log](identity-final.jsonl.gz) records 117 passing test events, including subtests, with no failures or skips.
It covers real cache sharing, deployment isolation, TLS, scoped ACLs, expiry, configuration, and identity encoding.
The command used Go 1.27.1 and the local integration workspace:

```sh
GOWORK=/tmp/csp121-host-integration.work TEST_SHARED_CACHE_URL=redis://127.0.0.1:16389 go test -json -race ./internal/config ./internal/cache/... ./internal/deployment ./internal/app -run 'Test(SharedCache|Cache|Development|Endpoint|KeyPrefix|.*Paths.*|.*Deployment.*)'
```

`make lint`, affected-package `go vet`, README prose checks, and `git diff --check` pass.
Verification stopped and removed the disposable Valkey service.

## Remaining work

This evidence covers the shared response cache prefix. Other shared stores still need canonical prefix qualification.
Remove unused internal KV cache adapters, then bound model and extraction fills.
Capacity, latency, discovery behavior, pre-PR review, native CI, and merge remain required.
CSP12.1 remains incomplete.
