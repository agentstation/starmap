# Shared catalog storage boundary

The [probe record](shared-storage-boundary-2026-09-13/verification.json) binds one diagnostic race test to Starport `ba9b0d8e`.
The Go overlay adds the probe without changing tracked source or the pinned Starmap module.
The probe uses actual in-memory Badger through the production `GenerationStore` adapter.

After 33 accepted generations, the history index contains 32 entries. Storage still contains 33 generation descriptors and 33 payload chunks.
The oldest generation remains readable. The adapter exposes neither `Collect` nor `AcquireGeneration`.
The history index limit therefore does not implement catalog retention.

Starport stores catalog descriptors and payload chunks in its configured KV backend.
Accepted and candidate heads share those immutable records while keeping separate pointers.
The object-storage backend under `internal/blob` stores uploaded file bytes.
Starmap's optional `storage.Object` catalog adapter is a separate storage path.

CSP5 owns Starmap retention and object-store coordination. CSP8 owns Starport adoption and the A22/A23 consumer checks.
Starport retention must protect both heads, required generations, active readers, and chunks shared by retained generations.
It must also protect writers that store chunks before they publish a generation descriptor.
The existing KV batch comparison contract supplies atomic conditional writes, but the catalog adapter does not yet coordinate deletion through it.

The diagnostic test passes one race event. It measures existing behavior and supplies no product acceptance credit.
Valkey execution, persistent Badger recovery, concurrent collection, and safe deletion remain UNVERIFIED.
The pending S3-only coordination decision does not settle the Starport KV adapter work.
