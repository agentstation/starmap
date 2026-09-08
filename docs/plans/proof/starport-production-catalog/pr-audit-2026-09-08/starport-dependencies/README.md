# Starport dependency qualification

The local dependency tree matches PR head `9a642d9` exactly.
The [verification record](verification.json) binds source commits, test captures, and dependency inputs.
The diff from main contains only `go.mod` and `go.sum`.

The focused race suite passes 307 test events and six packages.
It covers credentials, cloud chains, SQLite, Markdown parsing, S3 blob operations, and catalog integration.
The initial Valkey contract skip passes separately with two test events and one package result.

The AWS overlay exercises the production factory and SDK through local TLS fixtures.
Its six flows cover environment credentials, shared files, precedence, web identity, missing credentials, and denied reads.
The overlays add no production behavior and do not appear in attached CI.
Live cloud IAM policy remains outside this evidence.

The prior SQLite dependency writes a file that the new dependency reopens.
Migration, retained data, and integrity checks pass, with one test and one package result in each invocation.
Module verification and module tidiness pass.
Goldmark serves the tested Markdown link parser, which does not render the console.

## MySQL isolation defect

The combined MySQL suite fails with both old and new dependencies.
Each run records thirteen passing tests, two failed tests, and one failed package.
The test reset deletes `schema_migrations` but leaves `audit_log` with its `request_id` column.
The next test repeats the migration and fails with duplicate column error 1060.

The migration/read/write and transaction rollback contracts pass with separate disposable databases.
Each invocation passes three tests and one package.
CSP15 owns the existing fixture isolation repair before shared-backend qualification.
This dependency result does not qualify that complete backend recipe.

MySQL 8.4.11 and Valkey 7.2.14 ran in disposable containers bound to loopback ports.
The executor removed both containers after the checks.
All test data and credential material were synthetic.

## Replay

Extract the captures into a private temporary directory.
Rewrite each overlay prefix for the chosen checkout and extracted source paths.
Use the recorded source commit and Go 1.26.6.
The baseline module pair selects the previous dependency versions without changing production source.
The SQLite probe requires `CSP_SQLITE_UPGRADE_FILE` and either `write` or `read` in `CSP_SQLITE_UPGRADE_MODE`.

Run the writer before the reader against the same private fixture path.
Supply a separate disposable MySQL database to each successful isolated contract invocation.
Do not use an operator database for these tests.

## Merge

Starport PR #368 merged at `857bd85426c6cecc0148a7cd9c4ecda1e71f114d`.
All ten CI checks passed at head `9a642d91049106e1f21ff9cfbc7858bc87937924`.
The merge preserves branches and does not create a release.
