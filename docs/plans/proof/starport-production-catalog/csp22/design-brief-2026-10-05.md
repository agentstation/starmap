# CSP22 design brief (2026-10-05)

Task: qualify the complete release candidate. Scope: checks for the ten unregistered subcases, fresh native and publication captures, the discovery audit, and the candidate gate run. The explorer survey is in `explorer-survey-2026-10-05.md`. The fail-before record is in `fail-before-2026-10-04/`.

## Current state

- Starport main is `c5246216`. Starmap main is `f52b214c0`.
- The fail-before gate reported 265 PASS, 45 UNVERIFIED, and 2 FAIL of 312 subcases.
- Reruns with the toggles, the recipe image, the public fixture, and the brew Python pass every local group. A07, A17, A42, A46, A48, and the registered A31 subcases all PASS.
- The fresh `A50.full_path_baseline` at `c5246216` measured a median paired added latency of 0.46 ms for plain requests. Streams measured 0.60 ms. The p99 values are 0.84 ms and 2.00 ms. The csp0.4 baseline from 2026-09-05 measured 56 to 59 ms.
- Starport PR #419 holds the native capture from the `c5246216` tree at head `84b0bd8b`. Its CI checks still run.
- Starmap PR #221 holds the csp6 hosted capture, the native capture from run 37239363730, and the numeric profile review. Its head is `ee9ab6598`. Its CI checks still run.
- The registry has no entry for `A06.promoted_checkout`, `A06.old_pinned_bytes_unchanged`, `A31.exact_backend_versions`, `A31.single_region_boundaries`, `A50.gateway_provider_boundaries`, `A50.connection_reuse_lifecycle`, `A50.percentiles_and_load`, `A50.stream_timing_memory`, `A50.allocations_cpu_gc`, or `A50.real_recipe_matrix`.
- Every scheduled Starmap catalog publication run since 2026-09-26 failed. Decision D8 records the diagnosis.

## Decisions

### D1. A31.exact_backend_versions

- Bind an `all` entry with three existing Starport tests.
- The storage test `TestQualifiedValkeyVersion` asserts Valkey 7.2.14 in standalone mode.
- The sqlstore test `TestQualifiedPostgreSQLVersion` asserts PostgreSQL 16.15.
- The config test `TestDeclaredRecipePages` asserts that the targets table names both versions.
- A15 already binds the first two tests. The verifier permits a second binding.
- Starport has no object-store version test. CI installs MinIO by a Go pseudo-version at `ci.yml:477`. The record documents this limit, and the task adds no MinIO version test.

### D2. A31.single_region_boundaries

- Starport has no single-region boundary text. The only region reference is the `Files.ObjectStore.Region` setting.
- The Starport slice adds a boundary statement to the recipe and target pages. It names one region, one PostgreSQL primary, one Valkey authority, and one object-store bucket per fleet. It names cross-region replication and multi-region failover as out of scope.
- The slice adds `TestRecipeSingleRegionBoundary` in `./internal/config`, modeled on `TestDeclaredRecipePages`. The test parses the pages and asserts the boundary statement and the out-of-scope statement.
- The registry binds that test as a `go_test` entry.

### D3. A50.gateway_provider_boundaries

- The Starport slice adds `TestGatewayProviderTimingBoundaries` in `./internal/app`. It uses a controlled upstream with fixed delays for each boundary.
- The registry binds it with the required subtests `connection_queueing`, `dns`, `tcp_tls_setup`, `provider_wait`, and `client_backpressure`.
- Each subtest asserts the measured milestone against the controlled delay without subtracting the whole connector call. The `performanceSample` fields in `performance_test.go` already carry the milestones.

### D4. A50.connection_reuse_lifecycle

- The registry binds an `all` entry with the nine existing connector and HTTP client tests from the survey.
- The slice adds `TestDispatchTransportGenerationChangeSaturation` in `./internal/providers/connectors`. It saturates the dispatch capacity, changes the runtime generation, and asserts that pooled connections and waiters recover.
- The contract names runtime generation changes. The entry is not honest without the new test.

### D5. A50 measurement subcases

- The four measurement subcases need a measurement record graded against `docs/performance-targets-v1.json`.
- The profile requires three runs, 100,000 samples per variant, at least 600 seconds, and open-loop arrivals. It requires 95 percent confidence intervals. It requires a dedicated runner with eight physical cores and 16 GiB.
- This host is not a dedicated runner. No existing CI runner meets the dedicated requirement.
- The lead offers the owner three routes. Route A documents the four subcases as an UNVERIFIED limit of this task. It creates a sibling task for the dedicated measurement. Route B authorizes a dedicated or paid runner now. Route C runs the harness on this host and records the result as UNVERIFIED evidence.
- Pending owner decision. The lead recommends route A because the fresh baseline shows no latency regression and route B is new authority.

### D6. A06 adapters

- The subcase `A06.promoted_checkout` needs a Starmap adapter. It reads the `catalog/v1` channel document and resolves the promoted generation. It compares that generation with the current main embedding.
- The subcase `A06.old_pinned_bytes_unchanged` needs a Starmap adapter. It downloads the pinned release assets of the recorded tag and compares their digests with the recorded digests. The download is read-only.
- Today the main embedding declares the 2026-09-28 generation `whisper-operation-correction-47190c7a…`. The channel declares the 2026-09-26 generation `bindings-ef621d85…`. The promoted checkout adapter reports FAIL until a new promotion lands.
- The adapters land with the Starmap slice. Their candidate results are honest, and the gate counts them as the publication-dependent cases that CSP23 completes.

### D7. A05 publication evidence

- The subcases `A05.bot_required_checks` and `A05.same_bytes_retry` need a hosted publication capture that matches the current source. The csp6 capture in PR #221 names source commit `45afaed5d`. Main moved past it, and `verify()` refuses it.
- A fresh capture needs one successful scheduled publication from the candidate source. Decision D8 blocks that until the outage ends.
- The brief records A05 as a publication-dependent limit of the candidate gate until the pipeline recovers.

### D8. Publication outage

- The last successful scheduled run is 36243865909 at `71c5c8c72` on 2026-09-26. Every scheduled run since then failed.
- Runs from 2026-09-26 through 2026-09-28 stopped at `catalog-generation-check`. The test `test_interrupted_promotion_merges_exact_artifact_before_both_channels` commits in a fixture repository without a git identity at `scripts/test_catalog_publication.py:879`. The runner has no global identity, and `git commit` exits 128.
- Runs from 2026-09-29 onward report only `catalog acquisition failed with exit status 1`. The `retain_acquisition_corrections` function keeps only the whitespace correction events from stderr. The retained log holds no error message. This is an operator-visibility defect.
- The lead reproduced the acquisition failure with no credential. The publish tool at `f52b214c0` with `-baseline-embedded` and no `-state` prepares a catalog. The same tool with the accepted `catalog/v2` checkpoint and its checksum fails with `validation failed for field publication_admission.state.catalog: retained inputs do not reproduce the accepted catalog`.
- The function `RestoreState` replays the retained inputs of the checkpoint against the checkpoint baseline with the current code. It compares the rebuilt semantic checksum with the accepted catalog. The bisect in `publication-outage-2026-10-05/` shows that `024c24c22` restores the checkpoint and that `4586667a3` fails. Pull request #187 raised the catalog schema version from 10 to 19 and changed derived billing. The current code cannot reproduce a schema 10 catalog.
- The fix slice has three parts. Add the fixture identity to the test commit. Retain the publish tool stderr tail in the validation artifact. Resolve the checkpoint replay break by the route the owner selects.
- The replay break has two routes. Route B2 skips the equality check after a schema version increase and records the supersession in the receipt. Route C resets the channel state and loses the retained inputs and the accepted lineage. Exact reproduction needs a return to schema version 10, which is not an option. Route B is a protocol change and needs an owner decision.
- Pending owner decision. Route B2 is a protocol change. The lead recommends route B2 with a regression test and asks before the change.
- The scheduled cron publishes by itself after the fix merges. A manual dispatch or receipt replay is a separate authority boundary.

### D9. Recapture sequencing

- A native capture binds one commit. Any later non-evidence change in the same repository invalidates it.
- The Starport code slice from D2, D3, and D4 invalidates the #419 capture. The lead holds #419 as a draft and force-updates its branch with the capture from the code slice CI run.
- The Starmap code slice from D1, D4, D6, and D8 invalidates the #221 native capture. The lead merges #221 for the csp6 capture and the numeric review. A new capture follows the code slice CI run.
- The final candidate gate runs against the last merged commit of each repository.

### D10. Delivery order

- PR A on Starport lands the single-region text, the three new tests, and the five-archive wording in `docs/PERFORMANCE.md`. It runs the full roster and autoreview.
- PR B on Starmap lands the registry entries from D1 through D4 and the A06 adapters. It also lands the registry test pins and the D8 publication fixes. It runs the registry unit tests, `make technical-writing-check`, and autoreview.
- Proof PRs land the recaptures after both code PRs merge.
- The acceptance run is `--gate candidate` at the merged Starmap head against the merged Starport main.

### D11. Discovery audit

- The audit record maps the five criteria to passing subcases of the final gate run.
- HTTP: A09, A10, and A21.
- Browser journeys: A30.
- Caller policy: A11 and A46.
- Replicas: A16 and A31.
- The declared candidate: the subcase `A50.current_artifact_evidence` and the native captures.
- The record names the candidate pair and the gate run directory.

## Fail-before

- `--gate candidate` at Starmap `f52b214c0` against Starport `c5246216`: 265 PASS, 45 UNVERIFIED, 2 FAIL of 312, gate FAIL.
- `--case A46 --case A31` with the toggles and the recipe image: every registered subcase PASS, two unregistered A31 subcases UNVERIFIED.
- The accepted checkpoint replay at `f52b214c0`: exit 1 with the `state.catalog` admission error.

## Pending owner decisions

The owner reported disappearing questions. This section retains each open question until the owner answers it.

1. D5: route A, B, or C for the four A50 measurement subcases.
2. D8: route B2 or route C for the checkpoint replay break, and whether the lead starts the fix slice now.
3. Disk cleanup: the Go build cache at `/private/tmp/csp13-go-build-20260929` holds 65 GB and the Docker build cache holds 21.86 GB. The lead cannot run the cleanup. The owner runs it.

## Limits known before implementation

- The four A50 measurement subcases stay UNVERIFIED on this host under any route except B.
- A05 and `A06.promoted_checkout` stay UNVERIFIED or FAIL until a successful publication runs from the candidate source.
- D40 physical suspend stays UNVERIFIED.
- No MinIO version test exists.
- The local baseline numbers are serial loopback measurements and not qualification results.
