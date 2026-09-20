# Default cache memory and performance contract

Starport commit: `5e2d06e`. Starmap commit: `bf8af19fe`.
Go 1.27.1, darwin/arm64.

The default-capacity test fills and churns the 256 MiB response, 16 MiB
model, and 16 MiB extraction caches. Each store must occupy more than
75 percent of its budget without exceeding its cost limit.
The test measures heap after collection, repeats the load with new keys,
then closes the caches and checks payload release.

The allocation-heavy test requires `TEST_DEFAULT_CACHE_CAPACITY=1`.
Normal runs report a skip. The acceptance verifier treats that skip as
unverified. This keeps routine tests small without granting capacity credit.

The first native race run measured 317,963,408 bytes of filled heap above
baseline and 317,547,776 bytes after churn. Churn allocated 522,232,728 bytes.
After close, the delta was 15,519,184 bytes. Cache metadata remains reachable
in the test so the close measurement can detect retained payloads.
This is cache-component evidence, not full gateway heap or RSS qualification.

## Clock contract correction

The maintained performance profile still required 300-second gateway
authorization and a global refusal when clock qualification failed.
The revised profile requires 60 seconds and a two-second propagation target.
It distinguishes suspend-aware elapsed gateway time from catalog receipt
time. The catalog receipt contract retains qualified UTC unless that
receipt permits a conservative elapsed deadline.
The existing 30-second maximum clock uncertainty remains scoped to catalog
receipts. Clock failures refuse only affected operations.

The old CSP0.4 numeric profile remains historical evidence.
The verifier now uses a current fixture and rejects relaxed limits,
wall-clock gateway expiry, and an unconditional local receipt TTL.
All 89 verifier unit tests pass after the correction.

## Final acceptance run

`TEST_DEFAULT_CACHE_CAPACITY=1 TEST_SHARED_CACHE_URL=valkey://127.0.0.1:50507 TEST_SHARED_CACHE_FAULT_URL=valkey://127.0.0.1:50507 GOWORK=/tmp/csp121-host-integration.work bash scripts/verify-catalog-product.sh --starport-root /Users/jack/src/github.com/agentstation/starport-cache-storage --task CSP12.1 --json`
passes all seventeen subcases. The [complete report](all-acceptance.json.gz)
contains 49 distinct commands and 131 passing race-test events, without skips.
The disposable Valkey service stopped after the run.

The [first capacity run](default-capacity.jsonl.gz) retains the measurements
above. The [current profile](performance-profile.json) matches the maintained
Starport profile. The [unit-test log](clock-profile-tests.log) records all
89 verifier tests. Lint, prose lint, and diff whitespace checks pass.

## Remaining qualification

Production-cache discovery isolation and withdrawals remain open.
Full gateway resources, native platform evidence, repository gates,
review, published dependency qualification, and merges remain required.
No implementation task is complete.
