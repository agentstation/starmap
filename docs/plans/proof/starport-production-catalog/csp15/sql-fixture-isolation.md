# SQL contract fixture isolation

Starport commit `9328f07` repairs the fixture reset assigned to CSP15.
The worktree is `starport-sql-contract-isolation`, on branch
`codex/sql-contract-isolation`, from reviewed cache parent `c1c272b`.
The reviewed cache worktree remains unchanged.

The [first run](mysql-before.jsonl.gz) reproduces MySQL error 1060.
The reset removed the migration ledger but retained the audit table.
The next migration tried to add its existing `request_id` column.

Each network fixture now owns a separate PostgreSQL schema or MySQL database.
Cleanup removes that namespace after the test closes its connections.
The fixture retains the configured service database and its existing tables.
The new isolation contract migrates two fixtures, writes to each, and checks
that neither inherits the other fixture's row.
The README states the privileges required to create and remove namespaces.
No production migration changes.

The [combined run](sql-pair-after.jsonl.gz) uses Go 1.27.1 with race detection
and three shuffled repetitions. All 63 test events pass, without skips.
It covers SQLite, PostgreSQL 16.15, and MySQL 8.4.11.
The earlier [MySQL run](mysql-after.jsonl.gz) passes 54 events across three repetitions.
Service queries report zero fixture namespaces after cleanup.
Docker stopped and removed both disposable containers.
Vet, package lint, and prose lint pass.

The [verification record](sql-fixture-isolation.json) owns counts and commands.
This repair does not qualify MySQL production support or the complete storage recipe.
Review, native evidence, and merge remain open. CSP15 remains incomplete.
