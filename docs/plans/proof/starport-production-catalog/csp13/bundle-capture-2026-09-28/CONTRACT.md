# Backup bundle capture

CSP13 owns bundle capture and verification in `internal/recovery`.
The source adapters must represent a stopped, externally fenced deployment.
The coordinator checks the supplied closed SQL epoch before capture and before manifest publication.
A changed epoch leaves partial output without a complete manifest.

The bundle binds KV, relational, and blob snapshots with selected configuration and state files.
It records exact artifact sizes and digests, capture times, the build, and external recovery requirements.
It writes the manifest last. It does not replace an existing destination.
Known empty publication locks remain outside the payload inventory.
Unexpected control contents and pending publication files cause refusal.

Verification requires a manifest digest retained independently of the backup.
It rejects missing, changed, additional, non-regular, and unsupported artifacts.
An encrypted challenge verifies access to the key selected during capture.
Credential owners must still verify their historical encrypted records.

The race and pure-Go bundle cohorts each pass sixteen results without failures or skips.
The full recovery package passes 66 race results against real PostgreSQL, Valkey, and versioned object storage.
The local recipe also uses real Badger, SQLite, and filesystem adapters.
The retained failed runs explain the publication-lock inventory correction and the test-runner URL correction.

The caller still owns the complete product file inventory and cross-store reference checks.
Operator commands must bind source identities and preserve external dependency requirements.
Restore must reconcile later permission withdrawals, spending, and uncertain work against independent evidence.
Capture and verification never open the recovery gate.
Native checks, full task verification, review, and merge remain open.
