# Cache kind enablement

Starport `51fb7d7` implements controls. Starmap `7043f61f6` registers
`A42.enablement_by_cache_kind`.
Go 1.27.1, darwin/arm64, workspace
`/tmp/csp121-host-integration.work`.

The master switch previously left extraction caching active.
The [before test](enablement-before.jsonl.gz) fails.
The master switch now disables every optional cache kind.
Five independent flags control chat, embeddings, models, providers, and
extractions.

Semantic caching also requires chat caching.
Discovery-only configurations open no shared response-cache connection.
Disabling extraction caching leaves the parser's existing uncached path active.
The operator contract is `docs/OPTIONAL-CACHES.md` in Starport.

## Evidence

The initial full configuration/application race run records 479 passes,
four service-dependent skips, and three failure events in the
[initial log](enablement-after.jsonl.gz).
All failures belong to the new composition fixture and its parent.
Its chat and embedding cases lacked a canonical deployment ID.
The corrected fixture loads settings through the production loader.

Focused application checks pass ten events in the
[corrected log](enablement-final-app.jsonl.gz).
Proxy bypass checks pass seven events in the
[proxy log](enablement-proxy.jsonl.gz).
The stronger final [master-switch check](master-final.jsonl.gz) passes.

The final [registered check](enablement-registration.json.gz) runs all four
named commands through the catalog verifier. All 22 race events pass without
skips. These commands use the final file contents before commits.
Final lint, new-document prose lint, and diff whitespace checks pass.
The existing operator guide reports unrelated prose diagnostics. Its contents
remain unchanged. The new operator page links its semantic-cache section.

## Remaining work

Thirteen of seventeen subcases now have passing component evidence.
The other four remain unverified: cache-outage admission, declared memory
budgets, the semantic-mode matrix, and combined admission and expiry.
Production-cache discovery isolation and withdrawal checks also remain.
Repository gates, native evidence, review, published dependency qualification,
and merge still apply. CSP12.1 remains incomplete.
