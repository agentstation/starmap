# CSP24 upgrade and recovery from shipped assets

CSP24 reached its acceptance on 2026-10-07. The [final roster](csp24/final-roster-2026-10-07.json) ran all 50 cases from Starmap main `774856e4` against the Starport tree of main `ecde4dfb`. It reports 48 passed, 2 failed, and 0 FAIL with 6 open subcases of 331. Four open subcases belong to the CSP22.1 documented limit. Two belong to the hosted publication binding that this proof documents below.

## Shipped assets under test

- Starmap `v0.17.0` points at `2b2944be7`. Starport `v1.3.0` points at `b649dbf5`. [CSP23](csp23.md) records both releases.
- The README recording ran the released archive `starport_1.3.0_darwin_arm64.tar.gz`. Its checksum and its attestation passed before the recording.
- The archive digest is `3b6dd19b…`, and the binary digest is `b17f6cf4…`. The release record `docs/assets/first-use-v1.3.0/record.json` binds both.
- Starport #433 merged the v1.3.0 recording as `c63b6284`. Starport #434 merged the native capture of the #433 head run as `ecde4dfb`.
- Starmap #245 merged the E02 and E03 proofs as `c3e38970`. Starmap #246 merged the native capture of the #245 head run as `774856e4`. The #245 head `94e63d6d` and `c3e38970` have the same tree.

## Advertised installers

Starmap #245 merged the [first-use review](../../../plans/proof/starport-production-catalog/csp24/first-use/first-use-demo-review.json) and its captures as `c3e38970`. That directory lives on Starmap main, where the verifier reads it.
The review covers all 10 advertised methods with a PASS verdict for each one.

| method | evidence |
| --- | --- |
| `homebrew-macos`, `homebrew-linux` | `homebrew-macos.json`, `homebrew-cask.json` |
| five release archives | native run `37475148346`, one result file per platform |
| `source-build`, `compose-source` | `source-install.json`, `compose-after.json`, `compose-review.json` |
| `container-release` | `container-install.json` |

The source build and the compose build identify the tag commit `b649dbf5`. A35 passes 4 of 4 in the provisional roster and the final roster against the renewed Starport capture.

## Recovery recipes

The roster ran with `--recipes` against the image `starport-storage-recipe:csp24-6cf8dc1c`, built from the Starport head under test. A31 passes 9 of 9. A16 passes 5 of 5 in the final roster with the renewed native capture.
The recipe image from CSP22 was stale because Starport #430 changed `internal/blob`. The roster rebuilds the image from each head under test.

## README demonstration

The recording ran on 2026-10-07 at 05:08 UTC against the released darwin-arm64 archive. The edited animation runs 41.07 seconds in 17 frames at 1280 by 800 pixels. The uncut source keeps 15.10 seconds of original timing.
The real inference interval runs from 9.227 to 10.093 seconds. The four cuts lie outside that interval. The provider path is `openai`, and the answer stream returned status 200 with a `[DONE]` event.

Reviewer Jack recorded the pacing verdict PASS on 2026-10-07. The owner supplied the provider credential in the process environment. No file, capture, or proof holds it.

- [E02](csp24/e02-2026-10-07.json) passes against the review record `first-use-demo-review.json`.
- [E03](csp24/e03-2026-10-07.json) passes 15 of 15 checks against the release record of tag `v1.3.0`.
- [A37, A38, and A39](csp24/cases-a37-39-2026-10-07.json) pass 12 of 12 subcases.
- The real inference capture `real-inference.json` sent one streamed request of at most 32 tokens. The lead judged that request to be inside the recording authority. It gives no full production acceptance credit.

## Rosters

All three rosters ran `catalog_product_verify.py --task CSP24 --released-assets --recipes --backends all` through the acceptance wrapper. The task selects all 331 subcases.

| roster | Starmap tree | Starport head | result | open subcases |
| --- | --- | --- | --- | --- |
| [pre-merge](csp24/pre-merge-roster-2026-10-07.json) | main `c3e38970` | #433 head `ac7baf1e` | 43 passed, 7 failed, 0 FAIL | 27 |
| [provisional](csp24/provisional-roster-2026-10-07.json) | main `c3e38970` | #434 head `6cf8dc1c` | 46 passed, 4 failed, 0 FAIL | 15 |
| [final](csp24/final-roster-2026-10-07.json) | main `774856e4` | #434 head `6cf8dc1c`, the tree of main `ecde4dfb` | 48 passed, 2 failed, 0 FAIL | 6 |

The pre-merge roster took 110 minutes. Its 27 open subcases have three causes.

- 18 subcases in A03, A04, A05, A16, A35, and A50 reported a native evidence mismatch. The Starport capture bound to the #427 head, and the Starmap capture bound to the #239 head. Starport #434 renews the Starport capture from the #433 head run `37628807468`.
- 4 subcases in A07 skipped because the public catalog fixture was absent in a fresh worktree. The script `prepare_public_catalog_fixture.py` downloads the fixture of 411,974 bytes. It checks a pinned digest. A07 then passes [4 of 4](csp24/case-a07-2026-10-07.json).
- 4 subcases in A50 have no registered check. They are the CSP22.1 documented limit, and A50 also carries `current_artifact_evidence`, which binds to the native capture.
- No subcase reported FAIL.

The provisional roster ran from the same Starmap tree against the #434 head. A03 `starport`, A16, A35, and A50 `current_artifact_evidence` passed with the renewed Starport capture. The 11 open Starmap entries in A03, A04, and A05 still bound to the #239 head `9dd83561`. Starmap #241 through #245 changed the embedded catalog and the verifier scripts after that capture. Starmap #246 renews the Starmap capture from the #245 head run `37622348577`, whose five `Runtime` jobs passed. In that worktree `native_catalog.verify` passes 6 of 6 Starmap entries.

The final roster started at 20:00 UTC from Starmap main `774856e4`, before Starport #434 merged. The #434 branch sat on main `c63b6284`, so the tree at its head `6cf8dc1c` equals the tree of the squash merge `ecde4dfb`. The [environment record](csp24/environment.json) states both heads and both tree equalities. The roster took 108 minutes. Its six open subcases were the four A50 measurements and the two A05 hosted publication checks. The capture renewal of 2026-10-08 cleared the two A05 subcases, so four A50 measurements stay open.

The verifier reports `gate_status` FAIL and `qualification` UNVERIFIED while any subcase stays UNVERIFIED. The reason text reads `Real environment and final artifact qualification remain unimplemented`. That is the CSP22.1 limit, which the plan accepts.

## Hosted publication binding

`A05.bot_required_checks` and `A05.same_bytes_retry` verify the capture `csp6/hosted-publication` on Starmap main. That capture binds the real promotion #230, merged on 2026-10-05 as `2fba452ab`. Until Starmap #248, the verifier required the current source tree to equal that publication tree. The old binding ignored only `docs/plans/` and `docs/design/`.

- The CSP22 roster of 2026-10-05 ran at that tree, and both subcases passed with five attested subjects.
- Since then, promotions #236, #238, and #241 and the verifier changes #243 through #245 moved main. Eighteen source files differed from `2fba452ab`, so both subcases reported a mismatch in every CSP24 roster.
- On 2026-10-07 the lead re-ran both checks in a detached worktree at `2fba452ab` with the proof files from main. Both passed again: the release assets matched the captured digests and the attestations verified. The record is `csp24/a05-hosted-at-2fba.txt`.
- Starmap #248 (merged as `747ac19bb`) binds the capture to the publisher tooling paths only. Those paths are the workflow, the publication profile, `scripts/catalog_publication.py`, both catalog commands, and `internal/catalog/publication`. The verifier also checks the current publication live. A live run from the #248 branch passed in 138 seconds with current pull 241 and three current attested subjects. The record is `csp24/a05-live-binding-2026-10-07.json`.
- Starmap #247 (merged as `7fba79a84`) changed three bound tooling paths. They are the error grammar in `scripts/catalog_publication.py`, a publication history test, and the publish command README. The A05 re-run at main `747ac19bb` on 2026-10-08 passes 4 of 6 subcases. The two capture subcases report `Publication tooling differs from the captured publication source.` The record is `csp24/case-a05-2026-10-08.json`.

- The scheduled run 37735193283 healed the channel at `747ac19bb` and opened the promotion #249 with receipt `sha256:9faf893ae5…`. Starmap #250 fixed the race shard timeout. The bot then updated the promotion base and merged #249 as `60cec3dec`. The completion run 37821919952 published both discovery channels. The lead then dispatched the single owner-authorized retry, run 37825040856, with the receipt checksum.
- Starmap #253 (merged as `4d452ebab`) renews the capture `csp6/hosted-publication` from that completion run and that retry. The task CSP6 verification in the capture worktree passes all six A05 subcases at `60cec3dec` with Starport `6cf8dc1c`. The final A05 re-run at main `4d452ebab` passes 6 of 6 subcases. The record is `csp24/case-a05-2026-10-08b.json`.

The binding, not the behavior, made the two subcases UNVERIFIED until the renewal. The renewed capture binds the publisher tooling at a tree that includes #247.

## Scheduled publication outage

The scheduled `Catalog Generation` workflow failed four times from 2026-10-06T23:47Z at `Refresh candidate catalog` with `catalog acquisition failed with exit status 1`. The lead replayed the step without credentials from the accepted `catalog/v2` checkpoint. Run A reproduced the failure in 110 seconds: `validation failed for field replay.observations: exceeds the retained payload bound`. The retained history held eight distinct `models.dev` snapshots of 80 MiB, and the 64 MiB bound ran before compaction. The record is `csp24/acquisition-replay-2026-10-07.json`.

Starmap #247 compacts superseded metadata snapshots before the retained bound and retains any typed validation error line. Run C replays the real accepted checkpoint with the fixed publisher and exits 0 in 234 seconds with 3 of 9 snapshots retained. Run D restores the compacted state and exits 0 in 175 seconds. The record is `csp24/acquisition-replay-fix-2026-10-07.json`. The owner chose this fix with no channel reset, so the next scheduled run restores the accepted checkpoint and heals the channel.

## Roster environment

The [environment record](csp24/environment.json) lists the toolchain, the fixture containers, and the qualification variables without credential values.

- The acceptance wrapper derives the fixture URLs from `docker inspect` and never prints them.
- Six Docker fixtures serve the roster: PostgreSQL, MySQL, two Valkey instances, and a MinIO object store. A disposable Valkey serves the cache fault test.
- The roster script exports the four CSP22 gate variables: `STARPORT_RECIPE_IMAGE`, `TEST_SHARED_CACHE_FAULT_URL`, `TEST_AUTHORIZATION_CAPACITY`, and `TEST_DEFAULT_CACHE_CAPACITY`. Both capacities are 1.
- The toolchain is `go1.27.1` with `GOWORK=off`, `CGO_ENABLED=1`, and a shared build cache. The wrapper builds the recovery operator binary from the Starport head under test.
- Only one roster runs at a time because the rosters share the fixtures, the build cache, and port 54811.

## Flakes

- The #433 run had a Windows timing failure in `TestNativeRetirementFencesDelayedPublication/badger` and a Go module proxy stream error. A rerun of the failed jobs passed with no source change.
- The first #434 run 37654118086 ended with one job queued without a runner, and GitHub refused a retry. A closed and reopened pull request started run 37672845670, which passed with no source change.

## Limits

- A50 `percentiles_and_load`, `stream_timing_memory`, `allocations_cpu_gc`, and `real_recipe_matrix` stay UNVERIFIED. CSP22.1 documents that limit, and the plan accepts it.
- `A05.bot_required_checks` and `A05.same_bytes_retry` reported UNVERIFIED from #247 until the capture renewal in #253. Both pass at main `4d452ebab`.
- Cited `models.dev` snapshots can accumulate across publisher runs while offering timestamps stamp them. The retained bound now fails closed only after compaction. A later change can lower retention if the accumulation approaches 64 MiB.
- The release gate stays FAIL and the qualification stays UNVERIFIED for the A50 limit. No numeric production target comes from this roster.
- Component test duration is not deployment RTO, and a laboratory zero-loss result is not a blanket production RPO guarantee.
