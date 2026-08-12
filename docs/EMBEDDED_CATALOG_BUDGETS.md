# Embedded Catalog Release Policy

The checked-in catalog is an offline reliability fallback. The release policy
measures its exact generation and separates correctness failures from signals
that need review.

Run `make embedded-catalog-budget-check`.

The command rebuilds the verified embedded generation and deterministic
distribution artifact. It writes one versioned JSON report.

## Policy classification

| Measurement | Classification | Boundary | Consequence |
| --- | --- | --- | --- |
| Future generation time | Hard correctness gate | `generated_at` must not be after `measured_at` | Block the release and regenerate the catalog |
| Generation age | Review threshold | Review after 30 days | Record a finding without rejecting the release |
| Canonical payload size | Review threshold | Review above 16 MiB | Record a finding without rejecting the release |
| Compressed artifact size | Review threshold | Review above 8 MiB | Record a finding without rejecting the release |
| Provider count | Measurement only | None | Keep the count visible in the report |
| Model count | Measurement only | None | Keep the count visible in the report |

The old provider and model minimums did not represent an approved availability
or catalog-coverage objective. The release policy does not use them as gates.
A lower count remains visible for release review and catalog validation still
checks structural correctness.

The age and size values are review triggers. They are not hard product budgets.
Crossing one of these values cannot reject a release by itself. A future change
can make one a hard budget only when an approved operational requirement exists.
The hard budget must define its objective, method, unit, limit, consequence,
owner, exception path, and reopen condition.

## Report contract

The report contains:

- the report schema and policy versions.
- every classified rule and its policy fields.
- generation identity and time.
- measurement time and age.
- payload checksum.
- canonical and compressed byte counts.
- provider and model counts.
- classified findings.
- the hard-gate pass state.

The release workflow runs the same Make target. Hosted logs therefore preserve
the policy and exact measurements used for the release decision.

The repository stores and reviews the policy as code. The command does not accept
environment overrides. To change a review threshold or hard gate, change its
policy and tests in one pull request.
