# Catalog payload bounds

The encoder now rejects catalogs that exceed the decoder byte or nesting limits.
D26 gives canonical catalog JSON a 32 MiB limit.
Raw source JSON retains 16 MiB, and JSON nesting retains 64 levels.
The owner preference remains pending.
This is an engineering default with measured catalog-size evidence.

The original runtime published a 23,683,266-byte catalog that its own decoder could not reopen.
The [fixture descriptor](accepted-fixture.json) preserves its generation, digest, manifest, and byte count.
The large payload remains outside Git.
The 32 MiB candidate reopens that generation without changing its generation ID or digest.

A first repair rejected encoding above 16 MiB.
That repair prevented unreadable writes but failed four existing acquisition tests.
The final repair separates complete catalog capacity from raw source capacity.
Both catalog decoders use the new canonical limit.

| Check | Passed events | Failed events | Result |
| --- | --- | --- | --- |
| Original encoder against byte and nesting boundaries | 4 | 5 | Captures the missing encoder guard. |
| First 16 MiB repair, full runtime/acquisition/reconciler suite | 998 | 5 | Rejected candidate. Four tests and the acquisition package fail. |
| Final 32 MiB repair, full runtime/acquisition/reconciler suite | 1,003 | 0 | Pass. Three package results are included. |
| Final catalog and source-payload packages | 608 | 0 | Pass. Two package results are included. |
| Go 1.25.12 boundary checks | 17 | 0 | Pass. Two package results are included. |
| S3 backend package | 10 | 0 | Pass. One package result is included. |
| Oversized runtime publication with the old encoder | 0 | 2 | The old encoder permits publication. |
| Oversized runtime publication with the final encoder | 2 | 0 | Refusal preserves the accepted head and inputs through restart. |

Counts include test, subtest, parent, and package events.
All listed runs have zero skip events.
The [verification record](verification.json) separates test events from package events and records commands.
Ago reports zero findings, stale ignores, and errors.
Package lint reports zero issues.

The four tests that reject the first repair are:

- `TestPublishObservationsCommitsTenantOfferingGeneration`
- `TestPublishObservationsPersistsReviewCandidateWithoutCatalogChange`
- `TestSyncPublishesStoreOnlyWithoutWorkspaceProjection`
- `TestSyncTimeoutContextReachesDurableCommit`

## Source and reproduction

The [source record](source.json) binds all six applied Go files by SHA-256.
The original encoder matches commit 33728b3f.
Intervening commits change documentation and proof only.
The sibling [prototype archive](../legacy-origin-prototype-2026-09-08/archives.json) retains raw outputs, overlay maps, and probe sources.
Compressed files preserve their original bytes.

Run the four commands in the verification record against the applied source snapshots.
For the runtime refusal probe, use `runtime_oversize_bound_test.go` with the `oversize-bound-final.json` overlay.
Set `CSP3_LEGACY_FIXTURE` to a new private temporary directory for each run.
The before overlay replaces only the encoder with `payload-before.go`.
It keeps the final 32 MiB reader, which isolates the missing producer check.

To reproduce the accepted fixture, use the original runtime and `runtime_upgrade_test.go`.
Run `TestWriteAcceptedLegacyUpgradeFixture`, then retain its fixture directory.
Run `TestOpenAcceptedLegacyUpgradeFixture` against the same directory with the final codec.
Rewrite absolute overlay paths for the selected checkout and extracted capture directory.
The overlay maps identify every replacement file.

## Qualification limits

The encoder still uses JSON marshaling before the resource guard.
The byte limit bounds returned payloads, not peak encoding memory.
Encoding belongs to catalog preparation and publication, outside the inference request path.

The S3 backend checks its configured limit during both writes and reads.
Its default follows the canonical catalog limit.
Transport envelopes, serialized layers, manual history, and embedded review budgets retain their independent limits.

Older 16 MiB readers cannot consume larger catalogs with the same schema version.
The manifest schema range does not declare reader capacity.
Released-pair and downgrade qualification remain open.
The unapplied legacy-recovery prototype still refuses an older accepted history during startup.
This repair grants no CSP3 component acceptance or source publication approval.
