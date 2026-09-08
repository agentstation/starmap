# Architecture presence implementation

Commit `9b6403eb` preserves missing, unknown, false, and true architecture claims.
`Quantized` and `FineTuned` retain those states through codecs, definition views, reconciliation, publication, and recovery.
Known values retain precedence over unknown values. Rejected receipts cannot retain a claim.
Use the explicit setters to record false values or replace previously decoded claims.

The [prototype evidence](../architecture-presence-prototype-2026-09-08/README.md) retains codec failures, source-policy probes, and real filesystem upgrade checks.
The new reader preserves old recorded bytes and observation checksums.
It cannot reconstruct distinctions that an older writer already lost.

The generated OpenAPI JSON and YAML now accept Boolean or null values for both fields.
A field annotation declares that contract. The generation tool normalizes both specifications and rejects inconsistent inputs.
It preserves unrelated YAML bytes and exact numeric bounds.
Directory-scoped handles confine reads and writes to the selected file directories.
Both generation and freshness checks run the same tool.

The [verification record](verification.json) retains all results and hashes.
The initial affected suite passes 1,754 test results and five packages.
Later command repairs and equivalent view switches pass the final focused checks.
Both supported Go toolchains pass 129 test results and four packages.
The final command suite passes seventeen test results and one package.

Lint and Ago report zero findings. Documentation freshness passes, and prose checks pass across 1,251 files.
The command cross-compiles for Windows AMD64. This build does not establish native Windows execution.

The full publication gate passes all 39 stages against the committed source.
Ordinary and race suites each pass 80 packages.
Structured review, publication, merge, and the remaining CSP3 contracts stay open.
No primary acceptance case or task becomes complete from this implementation alone.
