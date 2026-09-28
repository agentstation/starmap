# Reporting and budget operation qualification

The reporting tests use the production HTTP router, provider connector, budget owner, Badger, and SQLite.
They reject optional usage commits, reject HTTP exports, and combine both faults.
Each fault runs with complete measured usage and missing usage.
All six cases retain the required charge or reservation and refuse another provider dispatch when capacity is insufficient.

The shared test uses real Valkey and PostgreSQL with account, key, and team budgets.
Five meters apply. One request warms authorization before the measured request.
Both requests settle measured usage into all five meters.
The test counts native budget authority calls and PostgreSQL driver queries without recording their arguments.
A separate native test confirms one synchronous EVAL for each authority read, clock read, or conditional write.

| Phase | Native reads | Native clock | Native writes | SQL approval reads | Total calls |
| --- | ---: | ---: | ---: | ---: | ---: |
| Before dispatch | 12 | 1 | 2 | 4 | 19 |
| Settlement | 6 | 0 | 1 | 2 | 9 |
| Complete budget operation | 18 | 1 | 3 | 6 | 28 |

These counts describe successful uncontended operations with five meters and warm authorization.
They exclude startup, optional reporting, unrelated KV operations, connection setup, and packet fragmentation.
They do not measure elapsed latency, cold authorization, conflicts, recovery, or capacity.
No application maintenance worker runs in this fixture.

The six SQL queries enforce independent approval before and after three native budget mutations.
Removing these checks without an equivalent authority contract would weaken recovery safety.
Individual meter and history reads account for eighteen native calls.
CSP12.2 must evaluate bounded grouped reads before performance qualification.
The optimization must retain budget identity, byte bounds, uncertain capacity, conflict handling, and native incarnation checks.

Manual review checked fault isolation, provider dispatch counts, durable state assertions, test cleanup, and measurement scope.
All test keys use isolated deployments or random identities.
No production behavior or payload version changes in this increment.
Pre-PR review, capacity qualification, native CI, dependency qualification, and paired merges remain required.
