# CSP20 design brief (lead decisions, 2026-10-04)

Source: the CSP20 exploration (Opus 5.5 explorer, read-only) on Starmap main `d2e612f14` and Starport main `149303fc`, plus the lead reads after the CSP19 merge. Starport main is now `9f0ad373` (#413) and Starmap main is `4f58972d` (#218). This file records the lead decisions and the facts that the implementers need.

## Fail-before

`--task CSP20` ran at Starmap `d2e612f14` against Starport `149303fc`. All 10 subcases report UNVERIFIED with "No behavior check is registered." The gate is FAIL, exit code 1. The three Starport README verifiers (`verify-readme-quickstart.sh`, `test-readme-quickstart-verifier.sh`, `verify-doc-links.sh`) exit 0 on that main. The logs are in `fail-before/`.

## Owner decision

A35.documented_inference uses a fake upstream now. CSP24 records the real provider capture. A real provider request is a paid action outside this task.

## Ordered split

Plan line 322 permits an ordered repository split for CSP20. The order is:

1. Starport PR: README rewrite, README contract tests, inference test, candidate-install CI job, native evidence adapter, and `TASKS.md` entry.
2. Starmap PR: the registry entries for the 10 subcases and the Starmap README fix.

The Starport PR merges first. The registry entries name Starport tests, so the Starmap PR waits for the merge.

## Decisions

### D1 README sequence

The Starport README follows the seven steps of `readme-demo-brief.md`, in this order:

1. Outcome and preview.
2. Tested installation per advertised platform.
3. Temporary development and catalog inspection.
4. One provider through the documented path.
5. One request with a gateway key and the streamed answer.
6. The client base URL and the compatibility boundary.
7. The links to persistent, team, and enterprise procedures.

Detailed performance and architecture material moves after first use.

### D2 Facts that the README must state

- Three credential roles: the gateway key, the inference provider credential, and the catalog-acquisition credential. Link `docs/site/start/keys-and-roles.md`.
- `STARPORT_HOME` is the shared path anchor. `STARPORT_CONFIG_DIR` moves only the configuration root (`internal/config/paths.go:99`). The current README claim at lines 285 to 287 is wrong.
- The persistent path shows `starport init` output (`Configuration:` and `Data:` lines, `internal/cli/init.go:42-49`) and `starport config paths`. Link `docs/site/configure/paths.md`.
- Badger holds durable KV records. Ristretto holds disposable memory caches. Link `docs/site/storage/caches.md` and `docs/site/storage/backends.md`.
- `starport dev` is temporary. Name the refused persistent selectors and the state that shutdown loses. Link `docs/site/start/temporary-development.md` and `docs/site/start/local-persistent.md`.
- The fleet link points at the T4 recipe in `docs/site/architecture/recipes.md`. The link text names Valkey, PostgreSQL, shared blob bytes, and private replica state as one recipe. Link the target table at `docs/site/architecture/targets.md#target-table`.
- Each performance claim links `docs/performance-targets-v1.json` and `docs/PERFORMANCE.md`. The text marks the targets UNVERIFIED and names CSP22 as the qualifier. The README removes any number without a qualified record.
- The README omits the catalog counts. The current counts are from 2026-08-29 and no test binds them.

### D3 README contract tests

The tests live in `cmd/starport/readme_test.go`, beside the keyless `models` tests. They read `README.md` from the repository root, normalize CRLF, and prove the structure. Move the markdown section parser from `internal/config/recipe_docs_test.go` into one shared internal test-support package. Both test files use it. The tests prove:

- The temporary section comes before the persistent section. Both link their docs/site pages.
- The catalog inspection commands come before the first credential variable.
- The three credential roles appear by name, in order, with the `keys-and-roles.md` link.
- The recipe links, the paths link, the caches link, and the backends link exist.
- The `STARPORT_HOME` claim exists and the old `STARPORT_CONFIG_DIR` claim does not.
- The performance paragraph links both performance files and states UNVERIFIED and CSP22.
- Each latency number in the README equals a value in `docs/performance-targets-v1.json`, or no latency number exists.

Mutation proof: each assertion fails on a removed link, a reversed order, or a changed value. Record the proof in the implementer report.

### D4 The inference test (A35.documented_inference)

A new `internal/app` test on the `newPerformanceFixture*` base (`performance_test.go:86-228`). It reads the Terminal 2 request from the README code block. It sends that exact body with the gateway key header through the full HTTP path. The upstream is the httptest fake provider. The test asserts HTTP 200, `text/event-stream`, at least one content event, and the `[DONE]` event last.

A changed README request body changes the test input, so the docs and the behavior stay bound. Limit: no real provider. CSP24 records the real capture.

### D5 The keyless test (A35.clean_catalog_no_keys)

A new `cmd/starport` test runs `models search` and `models show` with no provider credential in the environment and with the network unreachable. The implementer verifies how `loadRoutableModels` (`models.go:216`) behaves without network. The implementer then selects the strongest block that the code permits: a proxy variable to a closed loopback port, or the Starmap offline setting. The test asserts the JSON answers and that no file appears under a scratch home.

### D6 The candidate install (A35.each_advertised_native_install)

A new `ci.yml` job `candidate-install` with `needs: release-snapshot`. Its matrix is the five runners of `native-release.yaml` (windows-2025, windows-11-arm, ubuntu-24.04, ubuntu-24.04-arm, macos-15). Each job downloads the `starport-release-snapshot` artifact, selects the archive for its platform, and runs `scripts/verify-native-release.py` in a new candidate mode. The job uploads `native-catalog-install-<runner>/result.json`.

`verify-native-release.py` gains `--archive`, `--checksums`, and `--expected-version`. With `--archive`, the script skips the release download and the published-tag check. It keeps the checksum, the extraction, the version, and the two keyless commands. It also keeps the development readiness, the console assets, the authenticated `/api/v1/models`, the 401, and the clean shutdown. The report records `archive_sha256`, `binary_sha256`, and the catalog generation identity from the `models show` payload. `scripts/test-native-release.py` covers the new mode. The snapshot version comes from `.goreleaser.yaml` (`{{ incpatch .Version }}-next`).

Limit: the snapshot job skips signing and notarization (`--skip=notarize`). The Homebrew cask and the container image stay structural checks. A35.released_installer_paths belongs to CSP24.

### D7 The native evidence adapter

`scripts/native_catalog.py` gains an install evidence kind. `capture` downloads `native-catalog-install-*` and records their digests (format 4). `verify` reads `entry["evidence"]`. With `evidence: install`, it validates per platform. The `Candidate install (<runner>)` job completed with success. The bound `result.json` has verdict PASS, two keyless commands, no user files, and the archive and binary digests. The observations list the archive digest, the binary digest, and the catalog generation per architecture. Without the field, `verify` keeps the current test-event behavior. `scripts/test-native-catalog.py` covers the new kind and the format.

The Starmap verifier passes the registry entry to `adapter.verify(root, entry)` unchanged. The new field needs no Starmap verifier change.

### D8 Registry entries (Starmap PR)

Each A34 subcase is a `kind: all` composition after the CSP18 model. It combines the README contract test for that subcase with the behavior tests from the exploration table:

- `TestDevelopmentRejectsPersistentEnvironmentSelections`, `TestDevKeepsCatalogStateInScratch`, `TestDevUsesInMemoryBadger`
- `TestModelsSearchAnswersJSON`, `TestDiscoveryRouteWithoutProviderCredentials`
- `TestDeclaredRecipePages`, `TestRecipeLatencyProfiles`
- `TestConfigPathsTextIncludesRuntimeAndStorageLocations`, `TestLoaderOwnsIndependentProductRoots`

The A35 and A50 entries:

- `A35.each_advertised_native_install`: `all` over three `native_ci` install entries (linux, darwin, windows).
- `A35.clean_catalog_no_keys`: `all` over the D5 test and the three install entries.
- `A35.documented_inference`: the D4 test.
- `A50.current_artifact_evidence`: `all` over the three install entries.

`unchanged_source` and the digests bind the artifacts and the catalog identity.

### D9 Starmap README

The Starmap PR fixes the dangling sentence at `README.md:170` and adds one lifecycle paragraph. The paragraph explains passive library reads, persistent application startup, acquisition, publication, and server subscription. `make technical-writing-check` passes.

### D10 E02 and E03

Both proofs bind `README.md` by digest. Main reports both as UNVERIFIED today. CSP20 does not renew them. CSP24 records the new first-use review and the new media. The proof records this limit.

### D11 TASKS.md

One CSP20 entry under Active Work, after the CSP19 model: two short paragraphs. The first names the README sections, the owned facts, and the CSP24 handoff. The second names the tested behavior.

## Acceptance procedure

1. The Starport PR runs CI. The implementer captures the PR run with `native_catalog.py --run <id> --output <fresh dir>`. The implementer runs the adapter against the three install entries as a dry run. The capture stays out of the commit.
2. After the merge, the lead waits for the main run and captures it into a fresh directory. The capture moves to `docs/proof/catalog-native` in the main checkout. The old capture (run 36875854901 at `70dbf7de`) moves aside, outside git.
3. The Starmap PR merges. `--task CSP20` at merged Starmap main against merged Starport main expects 10 of 10 PASS.

## Facts the implementers must respect

- `verify-readme-quickstart.sh` needs these strings: "in-memory state", "creates no configuration files", "current Starmap catalog view", and "/api/v1/admin/providers/refresh". Terminal 1 has `starport dev` and no `STARPORT_API_KEY`. Terminal 2 has the reverse. No `STARPORT_BIN`, no `init --provider`, no pinned ghcr version.
- Other gates read the README: DX-DOC-1 needs `brew install --cask agentstation/tap/starport`. ENR-V12 needs `SECURITY-POSTURE.md`. CSG-V16 forbids "sign in" and "log in". `verify-v1-release.sh` forbids `^Status: pre-release`. CAT-V57 in Starmap needs "central Starmap" and `docs/DEPLOYMENT-TOPOLOGIES.md`.
- The Windows checkout converts docs to CRLF. Every docs test normalizes line endings (CSP19 lesson at `4117a4a6`).
- The binary has no provider base URL override in the environment. The inference test runs at the `internal/app` level.
- The native evidence runs validate only a completed successful run on the exact source commit.
- Keep the gateway key separate from the provider credential in every example. Keep the acquisition credential separate from the inference credential.
