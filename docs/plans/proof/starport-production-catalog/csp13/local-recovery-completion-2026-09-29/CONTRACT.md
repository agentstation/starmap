# Strict local recovery completion

This component proves native local completion. The parent coordinator owns complete recovery approval.

## Native target

`storage.OpenLocalRecoveryTarget` accepts the actual writable Badger handle and the independently accepted native target digest.
It rejects wrappers, memory mode, unsynchronized writes, readonly mode, closed handles, changed paths, and replaced native directories.
The sealed target retains the original directory identity. It cannot adopt a replacement.

The native digest identifies the persistent Badger directory. It is not a remote incarnation or a permission grant.
The local backend reference is `badger-target-v1:<native target SHA256>`.

`CheckActivatedImportAt` reads the exact native version-two completion claim, decision, and replay cursor.
It checks target continuity before and after that read. It creates or repairs no records.

## Atomic SQLite boundary

`Witness.CompleteImportedLocalAt` requires a deadline, SQLite, and positive final KV and SQL replay positions.
The caller supplies the original closed record, original relational import identity, snapshot, native claim, and independent decision digest.
The coordinator separately validates history, selected catalog, permission, blobs, current inputs, and external fences.

The method first checks native Badger completion.
Within the SQL activation transaction, it locks the exact original closed record and checks native Badger completion again.
It replaces that record with the typed local backend approval. It checks Badger completion again before commit.
The native SQL owner publishes its exact completion receipt and removes the import barrier in that same transaction.

A lost SQL reply does not imply rollback.
An exact completed retry reads the immutable receipt and checks current approval. It preserves later domain data.
A later withdrawal makes completion retry and current approval inspection refuse. The historical SQL receipt remains valid.

`CheckImportedLocalCompletionAt` checks current local approval and native completion.
It does not grant admission or renew catalog permission. Historical receipt inspection has a separate meaning.

## Ownership and remaining integration

This seam creates no fleet authority, remote incarnation, lease, clock qualification, or permission renewal.
Ordinary APIs and local request-path behavior remain unchanged.
External fences must stop former owners before recovery. No local request-path Witness read was added.

The parent owns final decision, retained history, blob proof, catalog selection, current permission, current configuration, fencing, and startup.
The component fixture does not prove that whole graph. It proves native cursor binding and the exact local completion boundary.

The request is caller-owned data with exported fields. Supported value diagnostics use its redacted formatter.
Invalid format verbs can bypass Go formatters. Log the sealed target or checked result instead of raw requests.

## Evidence

Go 1.27.1 native macOS arm64 race and pure-Go suites each pass seven top-level tests and thirty cases.
Both runs have no failures or skips. Vet, pinned lint, and strict prose checks pass.
The tests use actual Badger and SQLite. The lost-reply driver reports failure after a real successful SQLite COMMIT.
Linux and Windows execution remain parent-owned native CI qualification.
