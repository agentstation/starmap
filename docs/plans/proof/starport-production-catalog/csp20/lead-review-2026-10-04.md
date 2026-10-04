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
| Pre-PR roster | pending, see the addendum |
| PR #414 checks | pending, see the addendum |

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
