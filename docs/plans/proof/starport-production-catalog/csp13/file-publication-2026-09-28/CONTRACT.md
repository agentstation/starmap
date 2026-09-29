# Private-file streaming and recovery publication

Starmap owns native private-file access across Linux, macOS, and Windows.
Its new `productfiles.Directory.CopyFile` checks access, file identity, size, and directory identity around a bounded streaming copy.
A caller must discard partial output after an error and must verify content hashes when content identity matters.
The caller must fence writers. The API does not create a snapshot or approve recovery.
Empty locked ownership files require no read, which preserves Windows byte-range lock behavior.

Starport owns the selection, target mapping, component validation, and recovery transaction.
The candidate tree publisher verifies complete inventory and hashes before publication.
It requires a component validator and checks the tree again after that validator returns.
It flushes file data and directory entries before publishing a new private tree without replacement.

An exact retry verifies existing content and native access, then confirms durability.
A conflicting target remains unchanged. An error after publication reports that publication occurred.

Recovery must keep admission closed after any error.
The implementation does not approve identity reuse, external fencing, or independent later history.
Component owners must validate path-bound journals and retained policy before activation.
Process loss before publication can leave an inactive staging tree for the recovery coordinator to reconcile.

The consumer candidate uses a temporary module file or workspace with the local Starmap source.
No local replacement enters either committed module file.
Producer publication and native qualification must precede final consumer qualification.

This additive Starmap prerequisite belongs to CSP13 because the existing bounded byte reader cannot stream large private recovery files.
It does not expand catalog authority or move recovery policy into the filesystem layer.
The task retains every existing acceptance requirement.

Starmap also exposes `CheckNoPendingPublications` and the reserved publication-directory name.
The check delegates to native journal inspection without recovery or state creation.
It does not validate all metadata or replace the caller's complete inventory check.

The candidate inference-policy validator belongs to Starport's credential owner.
It checks retained policy before tree publication and refuses a different recorded replica.
It preserves accepted provider choices and the legacy default.
This check does not approve replica reuse, credential use, or deployment admission.
The [owner proof](../policy-file-owner-2026-09-28/verification.json) records tests and source hashes.
