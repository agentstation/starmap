# Description recovery evidence

Source commit `97857661` preserves missing, unknown, empty, and nonempty descriptions during reconciliation and recovery.
Known values take precedence over unknown values. Each selection checks source eligibility and the original receipt.
Rejected local claims lose their value and active source receipt. Missing local data preserves the baseline without a new receipt.

The [verification record](verification.json) binds source files and test captures by SHA-256.

The final race suite passes 1,032 test results and three package results, with zero failures or skips.

Focused Go `1.26.6` and Go `1.25.12` suites each pass 25 test results and two package results.

Ago and package lint report zero findings.

The original-source overlay records failures for description presence and rejected receipts.
That capture precedes the final missing-local baseline test.
The earlier empty-history assertion was too strict because the provenance contract retains source-empty clear markers.
The final assertion rejects any remaining source, observation ID, or value claim.

Six runtime scenarios verify publication, restart, source refresh, manifest receipts, degraded state, and retained observation history.
The repair changes reconciliation only. It changes no public API, wire format, storage schema, or inference request path.
CSP3 remains in progress. Legacy admission, other field presence, scoped membership, review, and merge remain open.

## Publication checks

The initial verifier passed 31 stages, then failed on stale generated catalog documentation.
Commit `a342d2e9` updates the generated API reference.
The continuation passes the failed documentation gate and all seven remaining stages.
All 39 required stages now have passing evidence against unchanged production Go source.
The ordinary and race commands each pass 79 package suites.
Review, publication, and merge remain open.
