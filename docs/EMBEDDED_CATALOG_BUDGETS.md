# Embedded Catalog Budgets

The checked-in offline catalog is a reliability fallback, not an unbounded copy
of every upstream response. Run make embedded-catalog-budget-check to rebuild
the verified embedded generation and deterministic distribution artifact, print
a JSON report, and fail when any reviewed threshold is exceeded.

| Measurement | Default gate |
| --- | --- |
| Generation age | At most 30 days and never future-dated |
| Canonical uncompressed payload | At most 16 MiB |
| Deterministic compressed archive | At most 8 MiB |
| Provider coverage | At least 5 providers |
| Canonical model coverage | At least 100 models |

The report records generation identity/time, measurement time and age, payload
checksum, both byte sizes, provider/model counts, applied limits, violations,
and pass/fail state. The release CI job runs the same Make target so these
values remain visible in hosted logs. Required PR CI adopts the target in P11
rather than treating release-only execution as branch protection evidence.

Threshold changes use the following environment variables:

- STARMAP_EMBEDDED_BUDGET_MAX_AGE;
- STARMAP_EMBEDDED_BUDGET_MAX_UNCOMPRESSED_BYTES;
- STARMAP_EMBEDDED_BUDGET_MAX_COMPRESSED_BYTES;
- STARMAP_EMBEDDED_BUDGET_MIN_PROVIDERS;
- STARMAP_EMBEDDED_BUDGET_MIN_MODELS.

Any override requires a non-empty
STARMAP_EMBEDDED_BUDGET_OVERRIDE_REASON. The reason is emitted in the report so
a temporary exception cannot be indistinguishable from the checked-in policy.
CI configuration changes and override reasons require ordinary code review; a
missing reason is a typed validation failure before measurement.
