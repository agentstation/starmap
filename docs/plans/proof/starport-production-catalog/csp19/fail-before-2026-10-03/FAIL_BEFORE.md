# CSP19 fail-before verifier run

Command: `bash scripts/verify-catalog-product.sh --starport-root <starport main> --task CSP19 --json` from the Starmap acceptance worktree at `344eb64cf` against Starport main `e7a58541`, through the private fixture wrapper, on 2026-10-03.

Result: exit 1, `Summary: 0 passed, 50 failed`, gate `FAIL`, qualification `UNVERIFIED`. The gate stays FAIL for CSP19 by design, because the verifier forces qualification for this task. The acceptance target is 9 of 9 subcases PASS.

The recipe image variable `STARPORT_RECIPE_IMAGE` was not set, so `TestContainerRecipePersistence` skipped. The acceptance run must set it to an image built from the same tree.

| Subcase | Status | Reason |
| --- | --- | --- |
| `A03.service_container_roots` | PASS | The named behavior test passed in this invocation. |
| `A16.complete_backup_manifest` | PASS | See the checks in the JSON record. |
| `A31.compose_complete_storage` | UNVERIFIED | See the checks in the JSON record. |
| `A31.container_recreation_records` | UNVERIFIED | No behavior check is registered. |
| `A31.each_declared_recipe` | UNVERIFIED | No behavior check is registered. |
| `A31.readonly_container_mounts` | UNVERIFIED | No behavior check is registered. |
| `A31.recipe_latency_profile` | UNVERIFIED | No behavior check is registered. |
| `A31.upgrade_restore_workload` | UNVERIFIED | No behavior check is registered. |
| `A32.authority_restore` | PASS | The named behavior test passed in this invocation. |

Record: `task_csp19_fail_before.json.gz` (registry sha256 `641e04e513d8a1cdef09e6bbc11f8d6eff6bb96fcbf0333649ef37f627696cfc`, roster sha256 `c6acdcc057c848d7c7c1a8d10a39fb99d248c4aea27ed93b2d525f2c5e1e33c6`).
