# Captured SQL identities and account grants

`OpenRelationalSnapshot` verifies a private SQL image before exposing read-only queries.
The relational importer uses the same image contract.
Verification closes the connection before removing its owned copy.
Writes through the query interface fail at SQLite's read-only boundary.

Identity validation checks user and team payloads against their SQL identities and revisions.
Memberships require retained users and teams.
Account grants require a retained user or team.
A grant referencing a deleted account remains diagnostic data without granting access.
Account owners validate captured account records and template revisions.

User, team, and account records retain their 64 KiB authorization-record bound.
Templates use the 64 MiB portable-record bound.
A regression preserves valid templates larger than the authorization-record limit.
The SQL recovery epoch must match the manifest's closed boundary with bootstrap permission disabled.

The focused cohorts pass 35 race results and 35 pure-Go results without failures or skips.
The broader cohort passes 318 results across SQL, recovery, account, and identity packages.
It includes all nine SQL backend pairs without skips.
That cohort precedes the template-bound correction, which the final focused cohorts cover.
Lint, affected-package vet, six dependency checks, and Linux and Windows cross-compilation pass.
Cross-compilation does not qualify native execution.

Gateway API-key references, budget records, independent later history, and full restore remain required.
These checks cannot approve admission or authorize retries.
No additional CSP13 acceptance subcase is complete.
