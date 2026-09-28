# CSP13 recovery preparation

This note records preparation at Starport `d6107baa` while CSP12.2 awaits native CI.
CSP13 remains `todo`. The acceptance map and engineering specification retain their contracts.
No product code changes implement these proposals yet.

## Current gaps

The [failure evidence](verification.json) reproduces two defects against the preceding source.
The fixture-only change at `d6107baa` does not alter either implementation.

- `internal/storage/badger.go:Restore` closes the database and deletes its directory before validating the backup.
- `internal/sqlstore/migrate.go` checks migration history before opening the migration transaction. Separate connection pools can apply the same file concurrently.
- `internal/catalog/runtime_migration.go` moves the runtime directory. It does not transfer all deployment stores.
- `internal/cli/fleet.go` exposes fresh initialization. Existing recovery primitives do not provide the complete operator recovery procedure.

## Ownership and implementation sequence

| Concept owner | Required behavior | Required evidence |
| --- | --- | --- |
| `internal/sqlstore` | Serialize migration history reads and schema changes across processes. Retain backend-specific transaction rules. | Concurrent startup, canceled wait, failed migration, owner loss, and incompatible schema history. |
| `internal/storage` | Validate imported state before replacing committed Badger state. Preserve absolute expirations during transfer. | Corrupt backup leaves committed state usable. Interrupted replacement recovers without lost state or extended expiry. |
| Deployment transfer coordinator | Bind KV, SQL, blobs, runtime files, deployment identity, and key-access requirements in one manifest. | Missing components, changed checksums, lost references, wrong deployment, and unavailable decryption keys prevent publication. |
| `internal/recovery` | Keep admission closed until independent evidence permits a new epoch. | Restore older state after revocation and spending. Missing later history keeps affected access restricted. |
| Application and CLI | Expose bounded operations and durable receipts through the existing concept owners. | Run actual commands through interruption, exact retry, conflict, and restart. |

Start with SQL ownership and non-destructive storage import.
Then implement the complete transfer and recovery procedure over those contracts.
Use separate procedures to initialize fresh storage and adopt populated storage.

## Backend constraints

SQLite `BEGIN IMMEDIATE` starts the write transaction before migration-history reads.
It can report `SQLITE_BUSY`, so ownership acquisition needs bounded cancellation and real contention tests.
This is a candidate implementation, pending repository tests. [SQLite transactions](https://www.sqlite.org/lang_transaction.html)

SQLite's backup API and `VACUUM INTO` can produce consistent copies of a live database.
A consistent SQLite copy alone does not establish a matching KV and blob boundary.
The deployment procedure must still stop relevant writes and validate references. [SQLite backup](https://www.sqlite.org/backup.html)

PostgreSQL session advisory locks last until explicit release or session termination.
A migration implementation that selects this mechanism must reserve the same connection for ownership and migration operations.
Returning that connection to a pool without releasing ownership is unsafe. [PostgreSQL locks](https://www.postgresql.org/docs/18/explicit-locking.html#ADVISORY-LOCKS)

MySQL named locks also belong to a session and survive transaction commit or rollback.
A MySQL implementation must use database-specific lock names and release ownership on every exit.
DDL can commit implicitly, so locking alone cannot prove recovery from partial schema changes.
MySQL production qualification remains separate. [MySQL locks](https://dev.mysql.com/doc/refman/8.4/en/locking-functions.html), [MySQL implicit commits](https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html)

## Recovery limits

A backup checksum proves content identity. It does not prove that the backup contains later permission withdrawals or acknowledged spending.
The approval record and its sole recovery evidence must not both come from the older backup.
Keep unknown history restricted until independent reconciliation supplies the missing boundary.

Stopping reachable gateways does not fence an unreachable gateway or former primary.
The operator procedure must bind external fencing evidence before reopening admission.
A test fixture that stops every child process establishes only its own controlled test boundary.

Measure recovery point and elapsed recovery time in the actual drill.
Do not infer a production RPO or RTO from a unit test or a successful import.
Retain uncertain reservations and their original accounting identities throughout transfer.
