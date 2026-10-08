# Production catalog plan closure

This [plan](starport-production-catalog-plan.html) completed on 2026-10-08. It covers the Starmap and Starport production catalog.
The user activated the whole-plan goal on 2026-09-05. The plan branch `codex/catalog-qualification` carried the plan and its proof during execution.

## Release pair

| product | tag | tag commit | release |
| --- | --- | --- | --- |
| Starmap | `v0.17.0` | `2b2944be7` | published 2026-10-05 |
| Starport | `v1.3.0` | `b649dbf5` | published 2026-10-05, run 37437368384 |

## Ledger result

- 45 tasks: 44 `done` and CSP22.1 `deferred(owner decision 2026-10-05)`.
- 50 primary cases and 331 required subcases. The final CSP24 roster reports 48 passed, 2 failed, and 0 FAIL against Starmap main `774856e4` and the tree of Starport main `ecde4dfb`. Six subcases stayed UNVERIFIED in that roster, and the capture renewal of 2026-10-08 cleared two of them.
- `A05.bot_required_checks` and `A05.same_bytes_retry` bind the publisher tooling paths to the real promotion #249, merged by the bot as `60cec3dec` on 2026-10-08, and to the single owner-authorized retry run 37825040856. Starmap #253 recorded that capture, and the A05 re-run at main `4d452ebab` passes 6 of 6 subcases.
- The four A50 measurements `percentiles_and_load`, `stream_timing_memory`, `allocations_cpu_gc`, and `real_recipe_matrix` stay UNVERIFIED. CSP22.1 documents that limit. A dedicated or paid runner needs new owner authority, by the owner decision of 2026-10-05.
- The verifier reports `gate_status` FAIL and `qualification` UNVERIFIED while any subcase stays UNVERIFIED. No numeric production target comes from the rosters.

## Final merges

| repository | pull request | merge commit | content |
| --- | --- | --- | --- |
| Starport | #433 | `c63b6284` | the v1.3.0 README recording, the release media review, and the website references |
| Starport | #434 | `ecde4dfb` | the native capture of the #433 head run 37628807468 |
| Starmap | #245 | `c3e38970` | E03 through the v1.3.0 release record, the E02 and E03 proofs |
| Starmap | #246 | `774856e4` | the native capture of the #245 head run 37622348577 |
| Starmap | #247 | `7fba79a84` | metadata snapshot compaction before the retained replay bound |
| Starmap | #248 | `747ac19bb` | the A05 live promotion check and the tooling-path capture binding |
| Starmap | #250 | `875202371` | the capacity suite for the replay payload-bound tests |
| Starmap | #249 | `60cec3dec` | the bot promotion with receipt `sha256:9faf893ae5…` |
| Starmap | #253 | `4d452ebab` | the renewed hosted publication capture |
| Starmap | #TODO-ARCHIVE-PR | recorded on the plan branch after the merge | this archive |

## Proof locations

- This directory holds the plan and the task proofs `proof/csp*.md` with the small record directories `proof/csp23/` and `proof/csp24/`.
- Starmap main holds the verifier inputs under `docs/plans/proof/starport-production-catalog/`: 2,286 files and 247 MB. They include the native qualification capture and the first-use records. The verifier `scripts/catalog_product_verify.py` reads them from that path.
- The plan branch `codex/catalog-qualification` at `TODO-BRANCH-TIP` holds the complete proof tree: 26,071 files and 510 MB. It includes the execution histories and the large captures. Every archive link to a branch-only file names that commit. The branch remains and nobody deletes it.
- The design document `docs/design/catalog-lifecycle/PAID_OPERATION_MATRIX.md` exists only on the plan branch. The archived plan links to it there.

## Indexes

- `docs/plans/README.md` lists no active plan and points at this archive.
- Starport `docs/TASKS.md` records the completion and this archive path.
- The engineering specification, the storage review, and `docs/README.md` point at the archived plan.
- The evidence manifests of 2026-09-05 keep their historical digests of the plan at that date.
