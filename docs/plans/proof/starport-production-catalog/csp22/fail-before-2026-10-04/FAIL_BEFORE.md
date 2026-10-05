# CSP22 fail-before candidate gate (2026-10-04)

The lead ran `bash scripts/verify-catalog-product.sh --starport-root /private/tmp/starport-csp21-20261004 --gate candidate --json` from the Starmap main tree at `f52b214c0`.
The Starport root was the PR #418 head `f0f63a45`. That tree equals the later main merge `c5246216`.
The wrapper derived the fixture endpoints from the five CSP13 containers.
The run started at `2026-10-05T02:46:26Z` and ended at `2026-10-05T04:21:16Z`, after 1 h 35 min.
The run had no value for `TEST_DEFAULT_CACHE_CAPACITY`, `TEST_AUTHORIZATION_CAPACITY`, `TEST_AUTHORIZATION_POSTGRES_URL`, `TEST_SHARED_CACHE_FAULT_URL`, or `STARPORT_RECIPE_IMAGE`.

## Result

Gate status FAIL. The summary reports 31 of 50 cases passed.
Subcases: 265 PASS, 45 UNVERIFIED, and 2 FAIL of 312.
`report.json.gz` holds the trimmed verifier report. The lead removed the pyenv hashlib warnings from the stdout and stderr fields.

## Failures

| Subcase | Cause |
| --- | --- |
| `A30.instruction_contrast` | The console vitest `src/styles/contrast.test.tsx` could not resolve `./routeTree.gen` in the fresh worktree. |
| `A50.partial_complete_metric_surfaces` | The Go check passed. The second check is the same console failure. |

The repository ignores the generated file `console/src/routeTree.gen.ts`, and `pnpm -C console build` creates it.
The lead ran the build after the gate, and the contrast test passed on a rerun.

## UNVERIFIED inventory

| Group | Subcases | Cause |
| --- | --- | --- |
| Stale native evidence | `A03.*` (6), `A04.*` (6), `A16.concurrent_migration_owner`, `A16.sqlite_wal_backup`, `A35.clean_catalog_no_keys`, `A35.each_advertised_native_install`, `A50.current_artifact_evidence` | Both native captures name older commits. Starmap holds run 34193810413 at `7e591262`. Starport holds run 37222309299 at `aeae7664`. |
| Missing hosted proof | `A05.bot_required_checks`, `A05.same_bytes_retry` | Starmap main has no `csp6/hosted-publication/capture.json`. The plan branch holds the capture. |
| No registered check | `A06.promoted_checkout`, `A06.old_pinned_bytes_unchanged`, `A31.exact_backend_versions`, `A31.single_region_boundaries`, `A50.gateway_provider_boundaries`, `A50.percentiles_and_load`, `A50.stream_timing_memory`, `A50.allocations_cpu_gc`, `A50.real_recipe_matrix`, `A50.connection_reuse_lifecycle` | The registry `scripts/catalog-product-checks.json` has no entry for these ten subcases. |
| Public catalog fixture | `A07.checksum_size_schema`, `A07.invalid_signature`, `A07.public_no_provider_keys`, `A07.replay_rejection`, part of `A31.compose_complete_storage` | Starmap `runtime` tests skip without the fixture from `scripts/prepare_public_catalog_fixture.py`. |
| Recipe image | `A31.compose_complete_storage`, `A31.container_recreation_records`, `A31.readonly_container_mounts`, `A31.upgrade_restore_workload` | The recipe tests skip without `STARPORT_RECIPE_IMAGE`. |
| Cache toggles | `A42.cache_outage_admission`, `A42.memory_budget_measurement`, `A48.bounded_cache_read_deadline` | The tests skip without `TEST_DEFAULT_CACHE_CAPACITY=1` and a disposable `TEST_SHARED_CACHE_FAULT_URL`. |
| Authorization toggles | `A46.bounded_tenant_working_set`, `A46.validity_skew_withdrawal`, `A46.fleet_receipt_vs_enforcement` | The tests skip without `TEST_AUTHORIZATION_CAPACITY=1` and `TEST_AUTHORIZATION_POSTGRES_URL`. |
| SDK roster parse | `A17.official_sdk_roster` | The pyenv Python prints hashlib warnings before each Python SDK line, so the matcher misses the three Python transition lines. |
| Numeric profile review | `A50.reviewed_numeric_profile` | The review record `csp0.4/numeric-profile-review.json` binds an older `docs/performance-targets-v1.json` digest. |

All 24 Go checks inside the native and toggle groups that ran reported PASS. The subcase status follows the skipped or unmatched check.

## Next

- Capture native evidence from the candidate CI runs for both repositories.
- Add the csp6 capture and the native capture to Starmap main through a proof pull request.
- Prepare the local fixtures and toggles, then rerun the affected subcases.
- Register checks for the ten unregistered subcases and refresh the numeric profile review.
