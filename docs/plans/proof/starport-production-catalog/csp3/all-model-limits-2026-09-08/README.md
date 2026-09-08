# Model limit presence

Reconciliation now uses all six published model limits. Previously it handled only context, input-token, and output-token limits. New document-page, document-count, and document-token values remained stale, including explicit zeroes.

The runtime regression checks six fields across positive, zero, unknown, and missing states. It checks publication, filesystem persistence, runtime reopen, and a later source refresh. Original receipts retain each source presence state. Known values retain their provider provenance. Unknown and missing values use the accepted fallback.

The [verification record](verification.json) binds source files, commands, and compressed captures. Old code produces four failure events. Each supported Go toolchain passes six runtime events. The full reconciler package passes 274 race events. These counts include test groups and package events.

This repair changes no schema or removal policy. Full field-presence coverage, scoped membership, publication review, and released-pair acceptance remain open.
