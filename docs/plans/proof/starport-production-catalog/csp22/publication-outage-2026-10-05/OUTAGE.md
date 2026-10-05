# Starmap scheduled catalog publication outage (2026-10-05 diagnosis)

The CSP22 lead found this outage while it prepared the A05 and A06 publication evidence. The lead changed no source and made no publication. `bisect.json` holds the local replay runs.

## Timeline

- The last successful scheduled run is 36243865909 at `71c5c8c72` on 2026-09-26 at 13:03 UTC.
- Every scheduled run since then failed. The workflow runs every four hours.
- From 2026-09-26 at 19:12 UTC through 2026-09-28, each run stopped with `candidate catalog-generation-check failed`.
- From 2026-09-29 at 00:21 UTC through 2026-10-04 at 23:07 UTC, each run stopped with `catalog acquisition failed with exit status 1`.
- The run head moved from `8122081e0` through `d721eb88b` to `f52b214c0` over that period.

## Defect 1: test commit without a git identity

- The test `test_interrupted_promotion_merges_exact_artifact_before_both_channels` commits in a fixture repository at `scripts/test_catalog_publication.py:879`.
- The command passes no `user.name` or `user.email`. The runner has no global identity. The command exits 128.
- Other helpers in the same file pass `-c user.name=fixture -c user.email=fixture@example.invalid`.
- The retained `catalog-generation-check.log` of run 36265236394 shows the traceback.
- A later run reached acquisition, so a source change after 2026-09-28 masked this defect without a fix. The test still has no identity at `f52b214c0`.

## Defect 2: the workflow discards the acquisition error

- The publish tool prints its error to stderr at `cmd/starmap-catalog-publish/main.go:59`.
- The function `retain_acquisition_corrections` in `scripts/catalog_publication.py` keeps only the `display_name_whitespace_trimmed` events from stderr.
- The retained `acquisition-corrections.log` of run 37242581843 holds `process_status: failed` and nothing else.
- An operator cannot see the acquisition error from the workflow artifacts.

## Root cause of the acquisition failure

- The lead built the publish tool at `f52b214c0` and ran it with no provider credential and no checkpoint. It prepared a catalog.
- The lead ran the same tool with the accepted `catalog/v2` checkpoint and its trusted checksum. It failed with `validation failed for field publication_admission.state.catalog: retained inputs do not reproduce the accepted catalog`.
- The checkpoint carries a baseline at schema version 10 and 51 retained inputs. The current embedded catalog is at schema version 19.
- The function `RestoreState` in `internal/catalog/publication/state.go` replays the retained inputs against the checkpoint baseline with the current code. It requires the rebuilt semantic checksum to equal the accepted one.
- The bisect shows that `71c5c8c72`, `8122081e0`, and `024c24c22` restore the checkpoint. The next commit `4586667a3`, pull request #187, fails.
- Pull request #187 raised `CurrentCatalogSchemaVersion` from 10 to 19 and added derived billing contracts. The current code derives schema 19 content from the same inputs, so the checksum differs.
- The same pull request regenerated the embedded catalog inside the repository. The main embedding now declares a generation that no channel promoted.

## Design intent

- `docs/CATALOG_PUBLICATION_QUALIFICATION.md` states that a replacement uses the last accepted channel checkpoint and the current embedded baseline.
- `docs/CATALOG_STORE_CONTRACT.md` states that a checkpoint preserves the original payloads for replay against a replacement baseline or a changed source configuration.
- The restore check contradicts that intent after a schema change. No test covers a checkpoint from an older schema version.

## Routes

- Route B2: narrow the restore check. Skip the equality check when the accepted catalog declares an older schema version than the current code. Record the supersession in the run receipt. Keep the checksum, canonical byte, and attestation checks. This is a protocol change and needs an owner decision.
- Route C: reset the channel state. Start the next publication from the embedded baseline with no accepted checkpoint. This loses the retained inputs and the accepted lineage. It needs publication authority.
- Restoring exact reproduction is not possible without a return to schema version 10.
- The lead recommends route B2 with a regression test that restores a schema 10 checkpoint under the current code.

## Effect on CSP22

- `A05.bot_required_checks` and `A05.same_bytes_retry` need a hosted capture from the candidate source. No publication from the candidate source can succeed before the fix merges.
- `A06.promoted_checkout` compares the main embedding with the promoted channel generation. They differ until the next promotion.
- The fix slice belongs in the Starmap code pull request of this task. The scheduled run publishes by itself after the merge.
