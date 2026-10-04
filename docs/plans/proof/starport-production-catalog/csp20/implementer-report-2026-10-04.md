# CSP20 implementer report (Starport slice)

Worktree: `/private/tmp/starport-csp20-20261004`. Branch: `csp20-readme`. Base: main `9f0ad373`. Head: `aeae7664`.
The worktree is clean. Nothing is pushed. No PR exists. No commit has an AI attribution or a co-author trailer.

All logs are in the scratchpad with the prefix `csp20-`. Scratchpad: `/private/tmp/claude-501/-Users-jack-src-github-com-agentstation/ae63335d-d56c-4ec6-acb3-5e12280c9e6b/scratchpad`.

## Commits

| Commit | Subject | Decision |
| --- | --- | --- |
| `915fa80c` | release: verify a candidate archive natively without a published release (CSP20) | D6 |
| `a85a63f6` | scripts: qualify native candidate installs as format 4 evidence (CSP20) | D7 |
| `c0ead7a3` | ci: install and run the release snapshot on each native runner (CSP20) | D6 |
| `c03c1c7a` | docs: align the README with the first-use sequence (CSP20) | D1, D2 |
| `36d37540` | test: share the markdown section parser for documentation tests (CSP20) | D3 |
| `bf35b76e` | docs: prove the README sequence, roles, and claims (CSP20) | D3, D2 (local-persistent link) |
| `fd784867` | cmd: prove the catalog commands need no credential or network (CSP20) | D5 |
| `ec8e678a` | app: stream the README request through the gateway (CSP20) | D4 |
| `ebbb1a76` | docs: record CSP20 in TASKS (CSP20) | D11 |
| `a02b35a2` | test: confine the markdown reader to the repository root (CSP20) | D3 (gosec G304 fix) |
| `aeae7664` | scripts: bind native install evidence to its pull request run (CSP20) | D6 decision from the lead |

## D6 decision: PR-only install evidence

The lead accepted plan (a), (b), and (c). `release-snapshot` stays PR-only. `aeae7664` adds the three extra requirements and the record of the limit:

1. The capture records the run event and the PR number.
   - `gh run view` now also requests `event`, so `capture.json` has `run.event`.
   - A format 4 capture also has a top-level `pull_request` number.
   - The capture finds the number with `gh api repos/agentstation/starport/commits/<headSha>/pulls`. It keeps only a PR whose `head.sha` is the run head SHA. Exactly one PR must match.
   - I checked on GitHub that the run API is not usable for this. After a merge, its `pull_requests` list is empty: runs 34273871732 and 34254145029 return `[]`. The commit API still returns merged PR #366 for `2973fd08`.
   - The capture refuses install evidence from a run that is not a PR run. It also refuses when zero PRs or more than one PR match. In each case it stops before it makes the evidence directory.
   - `validate_install` requires `run.event == "pull_request"` and a positive integer `pull_request`.
   - A PASS install result from `verify` contains `pull_request`.
2. A format 3 capture fails an install entry with a clear message. The result is `UNVERIFIED` with the reason "Native install evidence requires a format 4 capture of a pull request run, but this capture is format 3." A test asserts this exact result.
3. `candidate-install` now has the same condition as its parent: `if: github.event_name == 'pull_request'`. It still downloads with `gh run download` and `actions: read`, and it uses the pinned `setup-python` with the native architecture.
4. The TASKS.md entry records the limit. A main push run has no install evidence, so the evidence comes from the final PR run. That run's source commit matches the squash-merged tree.

I also corrected the wording of the capture message for a run without recovery shards. The message is now "Install evidence requires a run that has the recovery shards." This check also moved before the evidence directory is made.

Mutation proof (`csp20-d6-pr-mutations.log`): 6 of 6 mutations make the tests fail, and the restored file is byte-identical (`cmp`).

- P01: `validate_install` accepts any event and no PR number.
- P02: the format message omits the capture format.
- P03: the capture does not record the PR number.
- P04: the capture ignores the PR head SHA.
- P05: the capture accepts a push run.
- P06: `validate_install` accepts format 3.

Checks after `aeae7664`:

| Check | Exit | Result |
| --- | --- | --- |
| `scripts/test-native-catalog.py` (Python 3.9.6 and 3.14.7) | 0 | 31 tests OK each (was 29) |
| `scripts/test-native-release.py` (Python 3.14.7) | 0 | OK |
| `scripts/test-recovery-application.py` | 0 | OK |
| `actionlint .github/workflows/ci.yml` and Ruby YAML load | 0 | |
| verify-doc-links, readme-quickstart, DX, ENR, CSG, PLG, RNK, v1-release | 0 | |
| technical-writing lint `docs/TASKS.md` | 1 | the same 6 diagnostics as the base. They moved down 4 lines, and none is in the CSP20 lines. |

The commit changes no Go code, so I did not run the Go gates again.

## Decision status

| Decision | Status | Evidence |
| --- | --- | --- |
| D1 README sequence | Done | h2 order: Install, Quick start, Connect a client, Keep the gateway, Configuration, Features, Performance, Develop, Documentation, License and security. Quick start h3 order: Inspect the catalog without credentials, Credential roles, Terminal 1, Terminal 2, Stop the temporary gateway. |
| D2 README facts | Done | Each fact has a test (see D3). The catalog counts and the wrong `STARPORT_CONFIG_DIR` claim are gone. |
| D3 README contract tests | Done | `cmd/starport/readme_test.go`, 10 tests. Shared parser `internal/markdowntest`. `internal/config/recipe_docs_test.go` uses it. 26 of 26 mutations fail. CRLF control passes. |
| D4 inference test | Done | `internal/app/documented_inference_test.go`, `TestDocumentedInferenceRequestStreamsThroughGateway`. 5 of 5 mutations fail. |
| D5 keyless test | Done | `cmd/starport/models_offline_test.go`, `TestModelsSearchAndShowWithoutCredentialsOrNetwork`. 5 of 5 block mutations fail. |
| D6 candidate install | Done | `915fa80c`, `c0ead7a3`, `aeae7664` (PR-only evidence, see below). |
| D7 evidence adapter | Done (earlier) | `a85a63f6`. 10 of 10 Python mutations fail (`csp20-python-mutations.log`). |
| D8 registry entries | Not in this slice | Starmap PR. The README test names for the A34 entries are in "Names for D8". |
| D9 Starmap README | Not in this slice | Starmap PR. |
| D10 E02 and E03 | Not in this slice | The README digest changed, so E02 and E03 stay UNVERIFIED until CSP24. |
| D11 TASKS.md | Done | Two paragraphs after the CSP19.1 entry, before `### Proposed Work`. |

## Names for D8

These are the README contract tests in `cmd/starport/readme_test.go`:

- `TestReadmeFollowsTheFirstUseSequence`: the h2 order and the Quick start h3 order.
- `TestReadmeTemporaryPathPrecedesPersistentPath`: the temporary text, the temporary-development and local-persistent links in that order, and the refused-selector sentence. It also checks that the development page lists the five selector families, and that the README shutdown list equals the development page list.
- `TestReadmeInspectsTheCatalogBeforeAnyCredential`: the two `starport models` commands come before the first credential variable.
- `TestReadmeNamesTheThreeCredentialRolesInOrder`: the three roles, their variables, their order, and the keys-and-roles link.
- `TestReadmeKeepsCredentialRolesApart`: no fenced block mixes the gateway, provider, and acquisition roles.
- `TestReadmeLinksTheStorageAndRecipePages`: the T4 link text (Valkey, PostgreSQL, shared blob bytes, private replica state) and its anchor, the target-table anchor, and the paths, backends, and caches links. It also checks the Badger and Ristretto sentences.
- `TestReadmeStatesThePathAnchors`: the `STARPORT_HOME` and `STARPORT_CONFIG_DIR` sentences. Each sentence that names `STARPORT_CONFIG_DIR` must contain "only" and no "data". The test binds the sentences to the real loader: `STARPORT_HOME` puts the four roots below the home, and `STARPORT_CONFIG_DIR` moves only the configuration root. The README `init` and `config paths` samples must match the labels and order of the real CLI output.
- `TestReadmePerformanceClaimsAreQualified`: both performance links, UNVERIFIED, and CSP22. These must agree with the JSON. Each latency number in the README must be a `latency_ms` value of `docs/performance-targets-v1.json`. The README has no latency number now.
- `TestReadmeLatencyClaimsReadEachUnit`: a self-test of the latency extractor.
- `TestReadmeOmitsCatalogCounts`: no "N providers" or "N models" text, with a self-test of the pattern.

## Fail-before

- At `9f0ad373`, the new test names do not exist (`csp20-fail-before.log`). The README has the wrong `STARPORT_CONFIG_DIR` claim at lines 285 to 287.
- I ran the D3 tests against the base README in a scratch root (`csp20-readme-failbefore.log`). Eight of 10 tests fail. `TestReadmeKeepsCredentialRolesApart` passes, because the base README did not mix roles. `TestReadmeLatencyClaimsReadEachUnit` is a self-test. The base README fails `TestReadmeOmitsCatalogCounts` on "17 providers" and "511 models".
- D4 and D5 are regression guards. They pass on the base code, because the curl body and the keyless command behavior did not change. The mutation proofs below show that each guard detects a defect.

## Mutation proofs

### D3 README (`csp20-readme-mutations.log`)

26 of 26 mutations make at least one `TestReadme*` test fail. After each mutation, `git checkout README.md` restored the file. A CRLF copy of the README passes as a control (C01, rc=0).

| ID | Mutation | Failing test |
| --- | --- | --- |
| M01 | remove the keys-and-roles link | NamesTheThreeCredentialRolesInOrder |
| M02 | remove the temporary-development link | TemporaryPathPrecedesPersistentPath |
| M03 | remove the local-persistent links | TemporaryPathPrecedesPersistentPath |
| M04 | remove "shared blob bytes" from the T4 link text | LinksTheStorageAndRecipePages |
| M05 | remove the target-table anchor | LinksTheStorageAndRecipePages |
| M06 | remove the caches link | LinksTheStorageAndRecipePages |
| M07 | remove the performance JSON link | PerformanceClaimsAreQualified |
| M08 | remove the paths link | LinksTheStorageAndRecipePages |
| M09 | reverse Install and Quick start | FollowsTheFirstUseSequence |
| M10 | reverse Terminal 1 and Terminal 2 | FollowsTheFirstUseSequence |
| M11 | reverse credential roles 1 and 2 | NamesTheThreeCredentialRolesInOrder |
| M12 | move Keep the gateway before Quick start | FollowsTheFirstUseSequence, InspectsTheCatalogBeforeAnyCredential, TemporaryPathPrecedesPersistentPath |
| M13 | name a credential before the catalog commands | InspectsTheCatalogBeforeAnyCredential |
| M14 | add `STARPORT_API_KEY` to the Terminal 1 block | KeepsCredentialRolesApart |
| M15 | add "The local p99 target is 3 ms." | PerformanceClaimsAreQualified |
| M16 | change CSP22 to CSP23 | PerformanceClaimsAreQualified |
| M17 | remove UNVERIFIED | PerformanceClaimsAreQualified |
| M18 | add "511 models" | OmitsCatalogCounts |
| M19 | widen the `STARPORT_CONFIG_DIR` claim to the data root | StatesThePathAnchors |
| M20 | change the init label `Data:` | StatesThePathAnchors |
| M21 | reorder the config paths sample | StatesThePathAnchors |
| M22 | remove one shutdown item | TemporaryPathPrecedesPersistentPath |
| M23 | remove the catalog selector from the refused list | TemporaryPathPrecedesPersistentPath |
| M24 | remove the Ristretto sentence | LinksTheStorageAndRecipePages |
| M25 | remove the `STARPORT_HOME` anchor sentence | StatesThePathAnchors |
| M26 | reverse Terminal 2 and Stop | FollowsTheFirstUseSequence |

### D4 inference (`csp20-inference-mutations.log`)

Each README mutation makes the test fail. The README was restored after each run.

- I01 `max_tokens` 32 to 64: the fake provider refuses, so the gateway returns 400.
- I02 no `"stream":true`: the content type is `application/json`.
- I03 route `/api/v1/completions`: the path assertion fails.
- I04 `Bearer $OPENAI_API_KEY` in the chat curl: the header assertion fails. The first I04 attempt did not apply because of shell quoting. The log marks that attempt, and the rerun fails as expected.
- I05 model `openai/gpt-absent`: the gateway returns 404.

### D5 keyless (`csp20-offline-mutations.log`)

Each mutation adds one line to the child before it runs the commands. The file was restored after each run (`cmp` with the saved copy).

- N01 `http.Get` through the default transport: "attempted 1 network dials".
- N02 `net.DefaultResolver.LookupHost`: "attempted 16 network dials".
- N03 a private `http.Transport` with `ProxyFromEnvironment`: "opened 1 proxy connections".
- N04 a file under the home directory: "created .../created".
- N05 a file left in TMPDIR: "created .../tmp/left...".

## D5 network-block choice

`loadRoutableModels` calls `starmap.NewContext(ctx)` with the default options. That loads only the embedded bootstrap. It reads no environment and opens no connection. Starmap has no offline setting. The strongest block that the code permits is therefore a child test process:

- The parent re-executes the test binary with the marker `STARPORT_MODELS_OFFLINE_CHILD=1`. The marker does not contain `TEST_`.
- The child has a minimal environment with no credentials:
  - HOME, USERPROFILE, the XDG roots, APPDATA, and LOCALAPPDATA point below a scratch home.
  - TMPDIR, TMP, and TEMP point to `home/tmp`.
  - All six proxy variables point to a counting loopback sentinel.
  - SYSTEMROOT, GOCOVERDIR, and GORACE pass through when they are set.
- The child replaces `http.DefaultTransport` and `net.DefaultResolver` with values that refuse and count each dial.
- The child reads the two commands from the README "Inspect the catalog without credentials" block and runs them through `runContext`.
- Assertions:
  - The search lists the shown model.
  - `show` returns that ID with `object: model`.
  - The child counts zero refused dials.
  - The parent sees the child PASS line and zero sentinel connections.
  - The scratch home holds only the empty `tmp` directory.

Limit: a direct dial through a private `net.Dialer` escapes these blocks. The comment in the test states this limit. The test skips in short mode, as `TestModelsSearchReadsTheEmbeddedCatalog` does. Native CI runs without `-short`.

## Commands and results

Environment: `GOTOOLCHAIN=go1.27.1 GOWORK=off GOCACHE=/private/tmp/csp13-go-build-20260929 CGO_ENABLED=1 PATH=/usr/bin:/private/tmp/csp13-lint-tools:$PATH`. TMPDIR is `scratchpad/csp20-tmp`. The runner is `csp20-run-gates.sh`, and its summary is `csp20-final-summary.log`. The Go gates ran again on the final head `a02b35a2`.

| Command | Exit | Result |
| --- | --- | --- |
| `gofmt -l` on the changed Go files | 0 | clean |
| `gofmt -l cmd internal scripts` | 1 | pre-existing: `internal/recovery/catalog_expiring_preparation_test.go:41`, which CSP20 does not change (last change #395) |
| `go vet ./...` | 0 | |
| `make lint` | 0 | 0 issues on `a02b35a2` (`csp20-final-make-lint-rerun.log`). The `make-lint rc=2` line in `csp20-final-summary.log` is from the gate run before the fix. That run found gosec G304 in `markdowntest`, and `a02b35a2` fixes it. |
| `make build` | 0 | `./starport` (ignored) |
| `bash scripts/test-readme-quickstart-verifier.sh` | 0 | |
| `bash scripts/verify-readme-quickstart.sh` | 0 | |
| `bash scripts/verify-doc-links.sh` | 0 | |
| `bash scripts/verify-v1-architecture.sh` | 0 | |
| `bash scripts/verify-dependency-direction.sh` | 0 | |
| `STARMAP_OWNERSHIP_STARMAP_ROOT=... bash scripts/verify-starmap-ownership.sh` | 0 | |
| `/usr/bin/python3 scripts/test-native-release.py` (3.9.6) | 1 | 17 tests, 1 error: `test_exact_archives_extract (kind='tar')` because `tarfile.extractall(filter=)` is absent in 3.9. The base `9f0ad373` has the same error on 3.9 (10 tests, 1 error). |
| `/opt/homebrew/bin/python3 scripts/test-native-release.py` (3.14.7) | 0 | 17 tests OK |
| `/usr/bin/python3 scripts/test-native-catalog.py` | 0 | 29 tests OK (also OK on 3.14.7) |
| `bash scripts/verify-developer-experience.sh` | 0 | DX-DOC-1 included |
| `bash scripts/verify-enterprise-readiness.sh` | 0 | ENR-V12 included |
| `bash scripts/verify-console-session-grants.sh` | 0 | CSG-V16 included |
| `bash scripts/verify-document-parser.sh` | 0 | PLG-V19 included |
| `bash scripts/verify-reranking.sh` | 0 | RNK-V19 included |
| `bash scripts/verify-v1-release.sh` | 0 | runs without a release |
| `ruby -ryaml` on `ci.yml` and `release.yaml` | 0 | |
| `actionlint ci.yml release.yaml` | 0 | |
| `go test -race -count=1 -v ./cmd/starport/ ./internal/config/ ./internal/app/ ./internal/markdowntest/ -run '<new tests>\|TestDeclaredRecipePages\|TestRecipeLatencyProfiles\|TestFullPathMeasurement.*\|TestModelsSearch.*\|TestModelsShow.*'` | 0 | 25 tests PASS, 0 FAIL, 0 SKIP; 4 packages ok |
| `go test -count=1 ./cmd/starport/ ./internal/markdowntest/ ./internal/config/` | 0 | full packages ok |

## Technical-writing lint

| File | Before | After |
| --- | --- | --- |
| README.md | 1 (long_paragraph at line 128) | 0 |
| docs/TASKS.md | 6 | 6. These are the same six diagnostics as the base file. None is in the CSP20 lines. |

## Deviations

- `make build` runs `pnpm install --frozen-lockfile`. pnpm 11 ignored the `npm_config_store_dir` redirect and used the global store `~/Library/pnpm/store/v11`. It downloaded nothing (793 reused) but added one project link: `projects/de756fee7004facb12f1058d4f219055 -> /private/tmp/starport-csp20-20261004/console`. This is a write outside the permitted locations. I did not remove the link. It is safe to delete after the worktree goes away.
- `make lint` runs golangci-lint through `go run`, which reads the shared module cache.
- YAML validation used Ruby, because `/usr/bin/python3` has no PyYAML.
- The gosec fix is a separate commit (`a02b35a2`), because interactive rebase is not available. Squash it into `36d37540` if you want.
- `ReadFile` now reads through `os.OpenRoot` at the repository root. A docs test therefore cannot read outside the repository. This matches the `os.OpenRoot` use elsewhere in `internal/`.

## Checks not run

- The structural part of `verify-release-archives.sh`: it needs a `dist/` from a GoReleaser snapshot, and the brief does not authorize that build. CI `release-snapshot` runs it.
- The full repository roster, `make verify`, and autoreview: the lead owns them.
- No Docker. No `TEST_*` variable. No network-dependent gate.

## Open questions

- Closed: the PR-only trigger of `candidate-install`. The lead accepted it, and `aeae7664` implements it. After the merge, the lead captures the final PR run into a new directory.
- The pre-existing gofmt finding in `internal/recovery/catalog_expiring_preparation_test.go` is outside CSP20. It needs a separate fix.
