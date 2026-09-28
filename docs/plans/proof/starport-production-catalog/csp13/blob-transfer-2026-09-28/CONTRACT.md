# Portable blob transfer

Decision CSP13-BLOB-01, September 28, 2026. CSP13 owns this change.

The filesystem adapter stores the SHA-256 digest of a logical key.
The former object adapter stores the original key.
A filesystem retirement marker cannot recover that original key after the owner deletes its record.
The previous addresses therefore cannot support complete transfer to object storage.

Both adapters now use the same digest address under `objects` or `retained-v1`.
This changes object-store addresses. Existing filesystem addresses remain valid.
Object storage requires an explicit layout receipt. Existing prefixes without it refuse normal access.

A separate exporter reads the former object layout without changing the source.
Migration targets a new directory or empty prefix while source and target writers remain fenced.

The private tar archive binds bytes, size, digest, object count, and retirement count.
It includes permanent markers without file or job metadata.
It excludes incomplete uploads and noncurrent object versions.

Import validates the complete archive before it changes the destination.
Filesystem publication refuses existing paths. Object-store retries require the same operation and image.
Conditional writes and exact readback refuse conflicting bytes. Final enumeration refuses unexpected objects.

Import barriers remain after success and failure. Ordinary fresh clients refuse them.
Previously opened clients can retain readiness. External writer fencing remains mandatory.

Production activation must verify every deployment component and independent recovery history.
Object-store activation must publish the layout receipt before removing the import barrier.
The component tests remove barriers only to inspect restored bytes and late-publication refusal.
They do not qualify deployment approval.

The rejected alternative infers all retained keys from live file and job records.
It loses unreferenced retirement markers. Reopen that choice only if the storage contract retains a complete, durable reverse-key index.
There is no automatic fallback to the former layout.

The race cohort passes 91 results. The focused pure-Go cohort passes 25 results.
Both use real versioned object storage and contain no failures or skips.
File and job tests pass 460 results and skip 98 optional backend results.

Linux and Windows test binaries compile. Native execution remains UNVERIFIED.
Complete manifests, operator commands, independent history, native CI, review, and merge remain open.
