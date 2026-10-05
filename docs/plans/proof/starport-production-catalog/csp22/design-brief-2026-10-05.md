# CSP22 design brief (2026-10-05)

Task: qualify the complete release candidate. Scope: checks for the ten unregistered subcases, fresh native and publication captures, the discovery audit, and the candidate gate run. The explorer survey is in `explorer-survey-2026-10-05.md`. The fail-before record is in `fail-before-2026-10-04/`.

## Current state

- Starport main is `c5246216`. Starmap main is `f52b214c0`.
- The fail-before gate reported 265 PASS, 45 UNVERIFIED, and 2 FAIL of 312 subcases.
- Reruns with the toggles, the recipe image, the public fixture, and the brew Python pass every local group. A07, A17, A42, A46, A48, and the registered A31 subcases all PASS.
- The fresh `A50.full_path_baseline` at `c5246216` measured a median paired added latency of 0.46 ms for plain requests. Streams measured 0.60 ms. The p99 values are 0.84 ms and 2.00 ms. The csp0.4 baseline from 2026-09-05 measured 56 to 59 ms.
- Starport PR #419 holds the native capture from the `c5246216` tree at head `84b0bd8b`. Its CI checks still run.
- Starmap PR #221 holds the csp6 hosted capture, the native capture from run 37239363730, and the numeric profile review. It merged as `178976f51` on 2026-10-05 with 62 of 62 checks.
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
- Each subtest asserts the measured milestone against the controlled delay without subtracting the whole connector call.
- The lead corrected this decision on 2026-10-05. The `performanceSample` fields do not carry these milestones. Production records no milestone for the five boundaries. The `execution.OverheadTimer.TrackUpstream()` call at `internal/router/router.go:294` wraps the whole connector call.
- The test measures the milestones with `net/http` trace events on the real request path. Each subtest injects a 600 ms delay at one boundary. The delay milestone must be in the range from 300 ms to 900 ms. Each other milestone must stay below 300 ms.
- UNVERIFIED limits: the milestones come from the test and not from production telemetry. Codec time is inside the gateway processing value. The `dns` subtest covers only the Go resolver.

### D4. A50.connection_reuse_lifecycle

- The registry binds an `all` entry with the nine existing connector and HTTP client tests from the survey.
- The slice adds `TestDispatchTransportGenerationChangeSaturation` in `./internal/providers/connectors`. It saturates the dispatch capacity, changes the runtime generation, and asserts that pooled connections and waiters recover.
- The contract names runtime generation changes. The entry is not honest without the new test.
- UNVERIFIED limit: the new test covers HTTP/1.1 only.

### D5. A50 measurement subcases

- The four measurement subcases need a measurement record graded against `docs/performance-targets-v1.json`.
- The profile requires three runs, 100,000 samples per variant, at least 600 seconds, and open-loop arrivals. It requires 95 percent confidence intervals. It requires a dedicated runner with eight physical cores and 16 GiB.
- This host is not a dedicated runner. No existing CI runner meets the dedicated requirement.
- The lead offers the owner three routes. Route A documents the four subcases as an UNVERIFIED limit of this task. It creates a sibling task for the dedicated measurement. Route B authorizes a dedicated or paid runner now. Route C runs the harness on this host and records the result as UNVERIFIED evidence.
- Owner decision of 2026-10-05: route A. The four subcases stay UNVERIFIED in this task. The sibling task CSP22.1 owns the dedicated measurement.
- The owner asked about the host `nimbus@minicloud`. A read-only probe found two physical cores and 7 GiB of memory. The host does not meet the profile.

### D6. A06 adapters

- The subcase `A06.promoted_checkout` needs a Starmap adapter. It reads the `catalog/v1` channel document and resolves the promoted generation. It compares that generation with the current main embedding.
- The subcase `A06.old_pinned_bytes_unchanged` needs a Starmap adapter. The accepted design reads the Starmap pin and its `h1:` line from the Starport `go.mod` and `go.sum`. It downloads the module zip and recomputes the Go directory hash. A mismatch is a FAIL. The download changes no remote state.
- The adapter reads the embedded generation from the zip. It reports PASS when the pin generation differs from the checkout generation and the hash matches. It reports UNVERIFIED when the generations are equal, when a `replace` directive applies, or when the module is unavailable.
- The adapter `promoted_checkout` reads `channel.json` on the `catalog/v1` branch and verifies its attestation against the generation workflow on `refs/heads/main`. It reports PASS when the channel generation and digest equal the main embedding.
- Limit for CSP23: at the final released pair the Starport pin can equal the checkout. CSP23 then needs an explicit older version input for this subcase.
- Today the main embedding declares the 2026-09-28 generation `whisper-operation-correction-47190c7a…`. The channel declares the 2026-09-26 generation `bindings-ef621d85…`. The promoted checkout adapter reports FAIL until a new promotion lands.
- The adapters land with the Starmap slice. Their candidate results are honest, and the gate counts them as the publication-dependent cases that CSP23 completes.

### D7. A05 publication evidence

- The subcases `A05.bot_required_checks` and `A05.same_bytes_retry` need a hosted publication capture that matches the current source. The csp6 capture in PR #221 names source commit `45afaed5d`. Main moved past it, and `verify()` refuses it.
- A fresh capture needs one successful scheduled publication from the candidate source. Decision D8 blocks that until the outage ends.
- The brief records A05 as a publication-dependent limit of the candidate gate until the pipeline recovers.
- The `after` phase of the capture needs a retry run of the completed receipt. The documented retry is a manual workflow dispatch with the receipt checksum. That dispatch needs owner authority.

### D8. Publication outage

- The last successful scheduled run is 36243865909 at `71c5c8c72` on 2026-09-26. Every scheduled run since then failed.
- Runs from 2026-09-26 through 2026-09-28 stopped at `catalog-generation-check`. The test `test_interrupted_promotion_merges_exact_artifact_before_both_channels` commits in a fixture repository without a git identity at `scripts/test_catalog_publication.py:879`. The runner has no global identity, and `git commit` exits 128.
- Runs from 2026-09-29 onward report only `catalog acquisition failed with exit status 1`. The `retain_acquisition_corrections` function keeps only the whitespace correction events from stderr. The retained log holds no error message. This is an operator-visibility defect.
- The lead reproduced the acquisition failure with no credential. The publish tool at `f52b214c0` with `-baseline-embedded` and no `-state` prepares a catalog. The same tool with the accepted `catalog/v2` checkpoint and its checksum fails with `validation failed for field publication_admission.state.catalog: retained inputs do not reproduce the accepted catalog`.
- The function `RestoreState` replays the retained inputs of the checkpoint against the checkpoint baseline with the current code. It compares the rebuilt semantic checksum with the accepted catalog. The bisect in `publication-outage-2026-10-05/` shows that `024c24c22` restores the checkpoint and that `4586667a3` fails. Pull request #187 raised the catalog schema version from 10 to 19 and changed derived billing. The current code cannot reproduce a schema 10 catalog.
- The fix slice has three parts. Add the fixture identity to the test commit. Retain the publish tool stderr tail in the validation artifact. Resolve the checkpoint replay break by the route the owner selects.
- The replay break has two routes. Route B2 skips the equality check after a schema version increase and records the supersession in the receipt. Route C resets the channel state and loses the retained inputs and the accepted lineage. Exact reproduction needs a return to schema version 10, which is not an option. Route B is a protocol change and needs an owner decision.
- Owner decision of 2026-10-05: route B2 with regression tests.
- The stderr fix retains only the typed error field and its constant message. Raw stderr can hold provider addresses or credentials, and the validation artifact is public.
- Lead decision of 2026-10-05 on the supersession record: option A. The receipt and checkpoint decoders reject unknown fields, and Starport pins a Starmap version with the same strict decoder. A new receipt field breaks the pinned consumer. The fix adds a `schema_supersession` object to the publish tool report and the workflow validation artifact. The signed receipt format does not change.
- Option B raises the receipt schema version and breaks pinned consumers. Option C first ships a tolerant decoder. The task uses neither. The lead reported both to the owner as a possible later task.
- The scheduled cron publishes by itself after the fix merges. A manual dispatch or receipt replay is a separate authority boundary.

### D9. Recapture sequencing

- A native capture binds one commit. Any later non-evidence change in the same repository invalidates it.
- The Starport code slice from D2, D3, and D4 invalidates the #419 capture. The lead holds #419 as a draft and force-updates its branch with the capture from the code slice CI run.
- The Starmap code slice from D1, D4, D6, and D8 invalidates the #221 native capture. The lead merges #221 for the csp6 capture and the numeric review. A new capture follows the code slice CI run.
- The final candidate gate runs against the last merged commit of each repository.

### D10. Delivery order

- PR A on Starport lands the single-region text, the three new tests, and the five-archive wording in `docs/PERFORMANCE.md`. It runs the full roster and autoreview.
- PR P on Starmap lands the D8 publication fixes. PR R on Starmap lands the registry entries from D1 through D4, the A06 adapters, and the registry test pins. Each runs its unit tests, `make technical-writing-check`, and autoreview.
- The split keeps the publication fix reviewable alone, because the scheduled run depends on it.
- Proof PRs land the recaptures after both code PRs merge.
- PR R is Starmap pull request #223 at head `4458f6146`. It registers six subcases, and the registry holds 313 checks. Its 184 unit tests pass, and `make technical-writing-check` passes. Autoreview with Sol 6.1 at high effort returned clean in one pass.
- The file `scripts/test_catalog_product_verify.py` holds 1,944 lines. It owns the tests of one verifier module. The next addition must split it by adapter.
- PR A is Starport pull request #423 at head `9512f754`. It changes tests and documentation only. The full roster passes at `96ff2ed6`, and the timing test passes five race runs at `9512f754`. Autoreview with Sol 6.1 at high effort returned clean in one pass.
- The roster needs `CATALOG_DRIVEN_STARMAP_ROOT` set to a Starmap worktree at current main. Two of its checks need Python 3.12.
- PR A also fixes a test defect that failed the #419 CI run on `ubuntu-24.04-arm`. The helper test `TestDevelopmentScratchProcess` let its session become unreachable. The collector finalized the lock file and released the lock. The fix adds `runtime.KeepAlive(session)`. Production holds the session until it closes it.
- The first CI run of #223 failed on `Native windows-2025 / client`. The `go list` command exceeded its 120 second limit. The pull request changes only Python and JSON. The lead reran the failed jobs of run 37323139597.
- Pull request #223 merged as `a4d24306b` on 2026-10-05 with 62 of 62 checks after the rerun.
- PR P is Starmap pull request #229 at head `00a4878c2`. The implementer stopped before its commit. The lead ran the checks and made the commit.
- The lead replayed the accepted checkpoint with the fixed tool and no credential. The tool exits 0 and reports the supersession from schema 10 to schema 19.
- The race tests of the command package pass. The race tests of the publication package pass without `TestPublicPublicationProfileRetainsBoundedState`. That capacity test exceeded the 30 minute local limit under the race detector. The change does not touch it, and CI owns its result.
- Autoreview of PR P with Sol 6.1 at high effort returned clean in one pass.
- Pull request #229 merged as `51e09ff56` on 2026-10-05 with 62 of 62 checks. The next scheduled publication starts at 20:17 UTC.
- The first CI run of #423 failed four test jobs. On both Linux runners the `client_backpressure` milestone measured 1.65 s against the fixed 900 ms ceiling. The small socket buffers slow the transfer after the client pause, and the kernel sets that rate.
- Commit `60db1856` sets the ceiling of that milestone to the time that the client took to receive the stream. The floor and the ceilings of the other milestones do not change. Autoreview returned clean in one pass.
- The second CI run 37344667799 passed 47 of 47 checks. The macOS and Windows failures of the first run did not occur again.
- Pull request #423 merged as `3234997a6e9ccc3e312559aa0cd42bb1c290eaac`. The directory `merge-423/` holds the merge proof.
- The lead captured the Starport native evidence from run 37344667799 into pull request #419 at head `17776031`. The capture test passed 31 tests. The install evidence verified on Windows, Linux, and macOS.
- The macOS job stopped at the one hour limit of the `internal/catalog` package. The Windows job failed in `TestComposeStorageRecipes` before the native suite. The pull request changes neither path. The second CI run tests both again.
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

1. D5: answered on 2026-10-05 with route A.
2. D8: answered on 2026-10-05 with route B2.
3. Disk cleanup: answered on 2026-10-05. The owner cleaned the Go build cache and the Docker build cache. The free space is 82 GiB.

## Limits known before implementation

- The four A50 measurement subcases stay UNVERIFIED on this host under any route except B.
- A05 and `A06.promoted_checkout` stay UNVERIFIED or FAIL until a successful publication runs from the candidate source.
- D40 physical suspend stays UNVERIFIED.
- No MinIO version test exists.
- The local baseline numbers are serial loopback measurements and not qualification results.
