# CSP5 deterministic author YAML

Commit `5160414e4d113343cebb11397cb7d79e565e76be` fixes author index comments without changing catalog records.
The [verification record](yaml-roundtrip-2026-09-12/verification.json) binds both changed files, commands, test events, and the original repository failure.
The isolated worktree is `starmap-catalog-update-controls-roundtrip`, on branch `codex/catalog-update-controls-roundtrip`.
The original qualification worktree retains its tested source.

## Failure and correction

The repository race suite reports different `authors.yaml` bytes between callback serialization and filesystem serialization.
The original test fails in 29 of 100 isolated repetitions. Seventy-one repetitions pass.

The formatter puts root comments and the first author's comments in the YAML comment map.
The pinned encoder assigns those paths to the same sequence position. Map iteration lets one overwrite the other.
The added regression shows missing file headers or a missing first author heading.
Its empty-input case passes before the correction. Both nonempty cases fail.

The formatter now writes the fixed file header separately from author comments.
The existing byte-equality test remains unchanged.
The new test checks both header lines, each author heading, sorted decoded records, empty input, and repeated output.

## Verification

| Check | Result |
|---|---|
| Original round-trip test, 100 repetitions before the fix | 71 passing and 29 failing events |
| New header regression before the fix | One passing and three failing events, including the parent |
| Both regressions, 100 repetitions on Go 1.26.6 | 500 passing race events |
| Both regressions, 100 repetitions on Go 1.25.12 | 500 passing race events |
| Complete `pkg/catalogs` suite on Go 1.26.6 | 1,282 passing race events |
| Package lint and repository policy | Pass |
| Repository prose | 1,658 files pass with zero diagnostics |
| CSP5 task verification on the corrected commit | All twelve selected subcases pass, with 42 race events across 21 commands |

The repeated selections overlap the complete package suite. No passing check reports failed or skipped test events.
The minimum-toolchain check covers the regressions, rather than the complete package.
Generated API signatures and exported declaration locations remain unchanged.

The task check completed at 17:51:44 UTC on September 12, with exit status zero.
All 21 commands passed. No recorded test event failed or skipped.
Four A19 subcases carry product evidence. Eight A22 and A23 subcases carry producer component evidence and retain Starport consumer requirements.

The complete product report retains 49 unverified primary cases. The selected task result does not qualify the complete product.

## Remaining qualification

Original repository session `42639` ended at 18:02:38 UTC with exit status two.
It reports the author-serialization failure and a runtime package timeout after 1,800.573 seconds.
The timeout dump contains recently started tests and runnable catalog and filesystem operations.
This snapshot does not establish a deadlock or prove that resource contention caused the timeout.
The verification record preserves its terminal logs, command, and timeout analysis.
That run cannot qualify the corrected commit.

Corrected repository session `58714` completed twelve phases, including 85 passing ordinary packages.
It was then interrupted for the [verification cache correction](verification-cache-2026-09-12.md), with exit status 143.
Task session `9470` completed successfully on `5160414e4`.
Current repository verification continues from the linked correction before required review and publication.
Native CI and merge remain required. Shared/object collection remains part of the full CSP5 contract.
