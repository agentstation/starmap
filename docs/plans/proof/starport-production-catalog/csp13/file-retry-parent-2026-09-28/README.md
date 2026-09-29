# File-tree retry identity check

A retry must confirm that the target parent still matches its open directory handle.
The regression replaces the parent and preserves the target leaf identity.
Before the repair, the retry reports success after it syncs the old parent.
The repaired retry refuses the changed parent and preserves existing data.

The focused suite passes 33 race results and 33 pure-Go results.
Lint and vet pass.
Native execution remains UNVERIFIED.
The candidate still uses temporary integration with the local Starmap producer.
The caller must fence writers throughout restoration.

[Verification](verification.json) records source hashes, commands, and raw evidence.
This check does not complete CSP13 or approve gateway admission.
