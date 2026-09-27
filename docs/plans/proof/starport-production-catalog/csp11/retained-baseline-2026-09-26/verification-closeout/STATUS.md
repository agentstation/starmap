# Published-pair verification closeout

Starmap PR185 now contains `dff93d12e`. Starport's uncommitted module pin selects
`v0.16.6-0.20260926235631-dff93d12ec93`. All eleven CSP11 task subcases pass with
`GOWORK=off`, real Valkey, and PostgreSQL. That gate finished before the codec edit.
All 34 Starport repository commands pass on that pin. The report remains in
`consumer-dff93d12e-gates/report.json`.

Producer CI run 36281444461 failed two stale Intel macOS expectations.
The local repair requires all five approved targets and rejects Intel macOS.
All 56 workflow tests pass after repair.

The full local gate then found an
unchecked gzip-reader close error. The repair returns joined read and close
errors. Its codec race tests and lint pass. No assertion or linter rule changed
to hide that error.

Commit `e7710f16f` contains the two workflow test repairs and codec-close fix. The final local verification command is
`GOTOOLCHAIN=go1.27.1 GOWORK=off make HAS_DEVBOX= GOCMD=go verify-checks`.
It passes. Its log is `producer-verify-checks.log`. Full producer review remains active. Publish after review, then update and qualify the consumer pin.

Both PRs remain unmerged. CSP11 remains in progress.
