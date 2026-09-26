# Ownership contract repair

The previous review checkpoint now has two explicit operation contracts.
Runtime separates optional discovery candidates from required retained scopes.
Acquisition classifies only typed absence as optional. The real resolver tests
cover missing, configured, invalid, and required credentials without provider calls.
Runtime completes publication after successful ownership changes, including
unchanged source data. It uses the existing guarded activation and native fence.

Both runtime regressions failed before repair. The producer suite now passes
108 named race results with no failures or skips. The verifier passes 99 tests.
All eleven task subcases pass with real Valkey and PostgreSQL.
The full pre-PR review remains open.

The native test first exposed missing author targets in the metadata cache.
The repair retains author records without model payloads. A direct mapping test
now covers this condition. The next native run passed takeover and acceptance.

Its final assertion wrongly rejected an exact historical retry. The corrected
test requires the original result for that retry, an unchanged current head,
and refusal of a different write under the old grant. No store behavior changed.

A cross-workspace goago invocation also scanned Starport with Starmap's tool.
It reported six pre-existing findings outside this repair. Starport declares no
goago module tool. Its required lint command remains `make lint`. This borrowed
tool run does not replace that gate. Consumer vet and scoped writing checks pass.
