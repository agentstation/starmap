# Cache acceptance registration

Starport `7dd4b3c` adds durable-key and unknown-lifetime checks.
Starmap `445e2d130` registers twelve subcases.
The verification used their final file contents before these commits.
Go 1.27.1, darwin/arm64. Workspace:
`/tmp/csp121-host-integration.work`.

Command from the Starmap worktree:
`TEST_SHARED_CACHE_URL=valkey://127.0.0.1:58834 TEST_SHARED_CACHE_FAULT_URL=valkey://127.0.0.1:58834 GOWORK=/tmp/csp121-host-integration.work bash scripts/verify-catalog-product.sh --starport-root /Users/jack/src/github.com/agentstation/starport-cache-storage --task CSP12.1 --json`.

The [report](registered-acceptance.json.gz) records twelve PASS and five
UNVERIFIED subcases. The gate exits 1 as required.
All 36 distinct behavior commands pass, with 81 passing race-test events
and no skips. Registry validation, lint, and diff whitespace checks pass.
The disposable Valkey service stopped after verification.

The cleanup check reads the authoritative API key after model invalidation
and cache shutdown. The real-service check reads an entry with no TTL,
deletes it at the service, and verifies the next read misses.
The current shared cache has no local refill layer. Removed adapter tests
remain historical evidence only.

## Open subcases

| Subcase | Required next evidence |
| --- | --- |
| A42.cache_outage_admission | Cache failure and pressure must preserve required permission and budget admission through the production path. |
| A42.enablement_by_cache_kind | Add configuration and composition controls for each cache kind. Verify the master switch disables extraction caching too. |
| A42.memory_budget_measurement | Measure the declared default cache budgets and concurrent encoding and stream overhead. The smaller capacity fixture is insufficient. |
| A48.cache_semantic_mode_matrix | Cover disabled, exact, semantic, and streaming behavior, including opt-in and entry expiry. |
| A48.admission_and_expiry_preserved | Exercise admission and expiry together during cache hits and failures. |

At `7dd4b3c`, application composition enables chat, embedding, model, and
provider caches together. It opens extraction caching separately, regardless
of the master switch. This is an implementation gap for the existing
per-kind enablement contract, not permission to weaken that contract.

Remaining discovery evidence must use the production cache for caller
isolation and known withdrawals. Existing mock checks do not complete that
matrix. CSP12.1 remains incomplete and unmerged.
