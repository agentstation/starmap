# CSP19 Starmap docs slice lead review

Reviewed 2026-10-03 by the lead (Fable 5.1). The implementer (Opus 5.5, high) committed `d1c470ae3` on `csp19-docs` from `344eb64cf` in `/private/tmp/starmap-csp19-docs-20261003`. The commit carries no attribution and no trailer.

## Change

- `docs/ENTERPRISE_CATALOG_SERVER.md` (+284 / -24) adds the T1 recipe and a latency section without values. It adds the G11 egress, G12 rotation, and G22 commit lock statements.
- `docs/DOCKER.md`: +22 / -5. A Compose start without GitHub and the G21 adapter list.
- `docs/CATALOG_STORE_CONTRACT.md`: +13 / -4. The shipped adapters and the embedder-owned database extension point.

## Lead checks

- `make technical-writing-check`: exit 0, strict lint 1975 files and 0 diagnostics, glossary PASS.
- `go test -race -count=1 ./internal/deploymentdocs ./internal/catalog/settings`: ok (1.2 s, 92.9 s).
- `go test -race -count=1 -run '^TestCatalogStoreConcurrentSameBaseCAS$' ./pkg/catalogs/storage`: ok.
- Citation spot-checks at `344eb64cf`: `pkg/catalogs/storage/store.go:20-23` (adapter ownership), `go.mod` without a SQL driver, `pkg/catalogs/storage/filesystem.go:132,177-178` (lock and compare-and-set), `internal/cli/app/app.go:347` (filesystem store only), `internal/server/router.go:224` and `internal/server/middleware/auth.go:28` (legacy auth), `runtime/status.go:17-18` (`awaiting_source`). Each one states what the document claims.
- `autoreview --gate pre-pr --mode auto`: exit 0, skipped as a documentation-only change (`csp19-docs-autoreview-d1c470ae3.log`). The log also shows a pyenv `hashlib` traceback for `blake2s` that the helper tolerated.

## Implementer evidence

- The 12 cited tests in the runbook pass with `-race -count=1` in `./runtime`, `./internal/cli/app`, `./server/administration`, `./server`, and `./pkg/catalogs/storage`.
- The T1 procedure ran end to end on loopback with a binary from the branch. Steps: init, serve, and the 401, 200, and 403 checks. HTTP rotation with a 10 minute overlap, then a second rotation with a 409 answer. SIGTERM gave a zero exit status. Archive and restore into a fresh root followed. The `current` pointer and generation stayed unchanged, both keys worked after the restore, and credentials stayed in files.

## Decisions applied

- 19-1: the Starmap T1 page states no latency value and points to the CSP22 measurement. The Starport engineering targets file does not cover the Starmap central server, so the page gives no number.
- 19-3 and 19-4: unchanged by this slice.
- The G11 text agrees with Starport `docs/site/architecture/targets.md:38`: one catalog acquisition owner, inference egress at each gateway.

## Limits

- With the `embedded` source, readiness reports `fallback=true` and `fallback_reason=awaiting_source`. The recipe documents this as normal in that mode. The alert guidance keeps `fallback=true` as an alert for the GitHub source. This is current runtime behavior, not a product decision for this task.
- The `file` source was not exercised end to end.
- No claim about flock behavior on network filesystems.
- `TestAdministratorHTTPRotationAndRevocation` uses a zero overlap. The nonzero HTTP overlap evidence is the manager test and the exercise.

## Result

Accepted at `d1c470ae3`. Starmap PR #216 opened on 2026-10-03 ahead of the CSP18 registry PR. That PR waits on Starport #411, and the docs slice has no dependency on it. The squash merge landed on Starmap main as `18766e474` at the exact reviewed head with 62 of 62 checks green. The merge record is `starmap216-merge-2026-10-03/`.
