# CSP20 lead review, Starport slice (2026-10-04)

Branch `csp20-readme` at `aeae7664` on Starport main `9f0ad373`. Eleven commits, 13 files. Draft PR #414 opened on 2026-10-04 after this review. The implementer report is in `implementer-report-2026-10-04.md`. The design brief is in `design-brief-2026-10-04.md`.

## Review result

- The README follows the first-use order from D1. The h2 and h3 sequence matches the brief. Each fact binds to a test in `cmd/starport/readme_test.go`.
- `TestReadmeStatesThePathAnchors` runs `starport init` and `starport config paths` through the CLI with stub dependencies. The sample output and the loader paths agree.
- `TestReadmePerformanceClaimsAreQualified` binds the README to `docs/performance-targets-v1.json`. The README states no latency number and no catalog count.
- `internal/markdowntest` owns the shared section parser. `ReadFile` opens through `os.OpenRoot` at the repository root, which closes the gosec G304 finding.
- `TestDocumentedInferenceRequestStreamsThroughGateway` parses the Terminal 2 request from the README and streams it through the composed gateway to a fake provider. Real provider inference belongs to CSP24 by the owner decision of 2026-10-04.
- `TestModelsSearchAndShowWithoutCredentialsOrNetwork` runs the two catalog commands in a child process. The child refuses every dial and counts proxy connections.
- The `candidate-install` job runs the release snapshot archive on the five native runners. The adapter `scripts/native_catalog.py` qualifies the result as format 4 evidence and binds it to the pull request run and number.
- The lead checked three facts in the source. The `/api/v1/catalog/discovery` route and its `generation_id` field exist in `internal/server`. No repository file references the three removed README anchors. The capture records the run event.
- No defect found. No protocol, storage layout, or public API changed. The CI workflow gained one job and one output.

## Decision D6: pull request only install evidence

The `release-snapshot` job runs only on pull requests. The lead kept that condition. The `candidate-install` job carries the same condition, and the capture refuses install evidence from a push run. After the merge, the lead captures the final pull request run into a new evidence directory. Its source commit tree equals the squash-merged tree, which `unchanged_source` checks with `git diff`.

## Lead verification at `aeae7664`

| Check | Result |
| --- | --- |
| `scripts/test-native-catalog.py` | 31 tests OK |
| `actionlint .github/workflows/ci.yml` | rc 0 |
| technical-writing lint `docs/TASKS.md` | 6 diagnostics, all on the base lines |
| Autoreview `--gate pre-pr --mode auto` | Sol 6.1 high, one pass, no findings, "patch is correct (0.98)" |
| Pre-PR roster | all 36 checks rc 0 (`goago` exits 2 by design), head `aeae7664`, dirty 0 |
| PR #414 checks | 51 of 51 SUCCESS at `aeae7664`, which include the five `Candidate install` jobs |

## Addendum: merge and capture (2026-10-04)

PR #414 merged as `e17ea25a` at 19:45:51Z with the protected squash at the exact head. The merge proof in `starport414-merge-2026-10-04/` records 52 of 52 checks, the reviewed tree equal to the merged tree, strict protection, and zero unresolved threads. The lead deleted the remote branch `csp20-readme`.

The lead captured the final pull request CI run `37222309299` from the updated main checkout with `scripts/native_catalog.py --run 37222309299 --output docs/proof/catalog-native`. The capture is format 4, bound to pull request 414 and head `aeae7664`, with 51 file digests across the app, recovery, and install shards. The stale format 3 capture at `70dbf7de` now rests outside git at `/private/tmp/starport-native-capture-stale-70dbf7de-20261004`.

## Starmap slice review at `da7e88539` (2026-10-04)

Branch `csp20-registry` on Starmap main `4f58972d`, two commits. The implementer report is in `starmap-implementer-report-2026-10-04.md`.

- The ten registry entries match decision D8 and the acceptance map order. Each `go_test` leaf names one test that PR #414 added. Each of the three install leaves binds format 4 evidence for one platform.
- The lead read the README lifecycle paragraph against the Starmap source. In `runtime/policy.go`, lines 31 to 33 keep the embedded baseline active until the first verified upstream generation. Lines 147 to 157 set the default source poll of one hour. Lines 268 to 270 set the default acquisition interval of four hours. In `internal/cli/app/catalog_runtime.go`, line 135 connects the source acquirer and line 177 connects the provider acquirer. The `serve` command passes only the listen address, and the implementer report cites each other sentence.
- The lead acceptance run at `da7e88539` against Starport main `e17ea25a` with the format 4 capture gave 10 of 10 CSP20 subcases PASS. The record is in `acceptance-2026-10-04/`.
- `make technical-writing-check`: PASS, 1975 files, 0 diagnostics. The verifier tests: 139 OK in the lead run, 151 OK with the component suite in the implementer run.
- Autoreview skipped the model call, because the slice holds only documentation and registry data.
- Draft PR agentstation/starmap#219 opened at `da7e88539`.

Limits from the Starmap slice:

- No gate binds the Starmap README lifecycle paragraph by digest. A later change to it fails no check.
- `app.CatalogAcquisition` turns both schedules off when it opens the runtime first. `serve` opens the runtime first, and no test binds that order.
- `make technical-writing-check` needs the pyenv Python. With `/usr/bin` first in PATH it exits 2.

## Implementer evidence (reported, not repeated by the lead)

- `go vet ./...`, `make lint` (0 issues), `make build`, and the eight `verify-*` scripts: rc 0.
- `go test -race` on the new tests and the touched packages: 25 PASS, 0 FAIL, 0 SKIP.
- Mutation proofs fail the tests in every case: README 26, inference 5, keyless 5, adapter 10, pull request binding 6.
- Fail-before: 8 of the 10 README tests fail on the base README.
- `scripts/test-native-release.py`: 17 tests OK on Python 3.14. The Python 3.9 tarfile filter failure exists on the base.

## Limits recorded

- Install evidence exists only for pull request runs. A main push run has none.
- The snapshot skips signing and notarization. CSP24 owns the released installer paths.
- E02 and E03 bind the README by digest and stay UNVERIFIED until CSP24 renews the recording.
- The keyless test blocks the default transport and resolver only. A private dialer escapes the block.
- A pre-existing gofmt finding in `internal/recovery/catalog_expiring_preparation_test.go` is outside CSP20.
- The Starmap registry entries for A34, A35, and A50 follow in the second PR.

## Addendum: Starmap merge (2026-10-04)

PR #219 merged as `b6c30e0a7` at 21:04:10Z with the protected squash at the exact head `da7e88539`. The merge proof in `starmap219-merge-2026-10-04/` records 62 of 62 checks, the reviewed tree equal to the merged tree, strict protection, and zero unresolved threads. GitHub deleted the head branch. CSP20 is complete on both repositories.
