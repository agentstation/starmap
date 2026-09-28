# Portable KV transfer contract

CSP13 remains in progress. This component supplies KV images for the complete deployment recovery procedure.

## Storage ownership

`internal/storage` owns bounded record reads and conditional imports.
Each record retains its key, value, and absolute Unix-millisecond expiry. Zero expiry means persistent storage.
Keys have a 65,000-byte limit. Values have a 64-MiB limit.

Badger export uses one native snapshot. Valkey checks the selected process identity with each scan and record read.
The coordinator must stop and fence every source writer. Native expiry can remove records during collection.
A scan does not provide cross-store consistency.

## Archive ownership

`internal/recovery` writes a private SQLite image with no WAL dependency.
It deduplicates identical scan records on disk. A changed duplicate refuses the snapshot.
The receipt binds format, byte size, digest, and unique record count.
The final receipt follows durable image writes. A partial directory has no valid completion receipt.

Import verifies a private copy before accessing the target.
It checks integrity, exact schema, record count, field types, sizes, reserved keys, and expiry bounds.
The deployment manifest must bind the receipt to SQL, blobs, runtime state, deployment identity, and required key access.

## Import restrictions

Import requires an empty target namespace or the same retained operation and archive.
Every record write compares the persistent claim. Existing records must match both bytes and expiry.
Expired records never regain a lifetime. Badger rounds expiry down to whole seconds and reports each adjustment.
Badger import requires persistent storage with synchronized writes.

Valkey checks namespace emptiness before its atomic claim.
The coordinator must fence target writers during this interval and throughout import.
Concurrent import owners compete through native conditional writes. Other deployment namespaces remain outside the selected scope.

The import barrier survives successful writes, cancellation, lost acknowledgments, and process loss.
Ordinary storage startup refuses the barrier. Raw constructors permit explicit recovery access without granting admission.
The complete recovery coordinator must retain the barrier until all components and independent evidence pass verification.

## Remaining acceptance

A snapshot digest proves content identity. It cannot establish current permission or missing post-backup spending.
The SQL witness and catalog approval must describe the same recovered deployment.
Full operator commands, blob transfer, credential verification, independent evidence, external fencing, and measured recovery remain required.
Native CI, required review, and merge remain open. This component does not complete A16, A33, or CSP13.

Valkey supplies absolute expiry through [PEXPIRETIME](https://valkey.io/commands/pexpiretime/).
Its [SCAN contract](https://valkey.io/commands/scan/) permits repeated keys and requires complete cursor traversal.
