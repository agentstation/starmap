# Restricted SQL restore

The relational importer publishes imported records, recovery restrictions, and an import receipt in one transaction.
The receipt binds the operation, restriction policy, and original snapshot.
An exact retry validates the source and receipt without applying restrictions twice.
Another operation or policy causes refusal. A failed restriction callback rolls back the rows and receipt.

`PrepareSQLRestore` validates the complete bundle before importing SQL.
It advances retained epochs and closes every recovery gate, bootstrap permission, and unused team initialization grant.
Ordinary startup and backup capture refuse the persistent import barrier.
A successful prepare result grants no admission.

The baseline adapter delegates to the original one-shot importer.
The exact-retry assertion fails because the target contains the first import.
The new receipt contract resolves that outcome without changing the old API's empty-target requirement.

The race suite passes 66 results across relational transfer, recovery, and application checks.
It includes all nine SQL transfer pairs and receipt checks on SQLite, PostgreSQL, and MySQL.
The pure-Go suite passes 22 results. Five final race results cover the added backup-capture barrier.
These cohorts have no failures or skips. Counts include parent and subtest outcomes.

Target writers must remain fenced. A receipt does not prove that every target record remains unchanged after import.
The preparation coordinator rechecks gate restrictions on retry.
Complete target verification, independent later history, and activation remain required.
KV, blob, selected-file restore, and operator commands remain open.
Fifteen CSP13 acceptance subcases remain UNVERIFIED.
