# Video publication review

Status: local review. CSP12.2 and its pre-PR review remain open.

The audit reproduced five failed native results across memory, Badger, and Valkey.
A delayed asset write could recreate expired content after losing its acknowledgment.
A repeated acceptance callback could replace the first stored billing receipt.
A separate provider-job probe reproduced an unreferenced asset after a lost write reply.

All job receipts and assets now use immutable blob publication.
An exact native callback retry preserves the first receipt timestamp.
A conflicting callback cannot replace its content or measured usage.
Recovery verifies existing native asset bytes before another external download.

Provider-polled jobs retain an asset identity, digest, measured size, and expiry before publication.
The pending record survives ambiguous write replies. Refresh and startup recovery verify the retained bytes.
A replacement configuration cannot change the prepared size bound or deadline.
Repository replacement preserves this identity. Record deletion requires completed asset retirement.

Job schema 5 changes the byte namespace. Schema 4 and older records require coordinated migration.
CSP13 owns migration. There is no fallback to mutable blobs.
Content-route fixtures now use the current byte namespace. Their HTTP and expiry assertions remain unchanged.

Expiry confirms backend retirement before clearing pending state.
Native retirement also covers the preallocated receipt and asset identities.
Late administrator evidence remains durable after private receipt expiry.
The existing budget and audit rules remain mandatory.

Real MinIO tests cover versioned storage with all three metadata backends.
A separate process-loss test kills workers after native receipt, native asset, and provider asset publication.
Recovery reopens Badger and filesystem storage without a provider runner or downloader.
It verifies the original bytes and retained asset identity.

## Required remaining work

Construction does not contact the bucket. Define operation-specific conditional-write readiness before paid dispatch.
Keep independent gateway operations and diagnostics available when file storage is unavailable.
Readiness must cover single-part and multipart conditions without issuing inference requests.

Current retirement markers need permanent retention until a protocol proves old writers cannot resume.
Native platform, restore, staging, incomplete multipart, and noncurrent-version cleanup qualification remain open.
An API retention window does not specify when backends delete backups or retained object versions.

Complete stable batch aggregates, interrupted-run policy, audited correction, the paid-operation matrix, A47, capacity, review, CI, and paired merges.
