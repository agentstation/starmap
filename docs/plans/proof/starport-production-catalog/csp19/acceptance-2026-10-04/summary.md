# CSP19 acceptance run (2026-10-04)

Starmap registry head `4f58972d4` against Starport main `9f0ad3735`.
Recipe image `starport-storage-recipe:csp19-main`, id `sha256:7304fdf881f7415e646323730680fdefa172ccbafc9f3f31d97c0d4f09ffe4f3`.
Run start `2026-10-04T17:55:11Z`, task exit 1 at `2026-10-04T18:07:49Z`.
Fixtures: PostgreSQL 16.15, Valkey 7.2.14 (plaintext, private network), MySQL, MinIO. Race operator binary from Starport main.

## Result

Gate status `None`. Subcases: PASS 9.

| Subcase | Status |
| --- | --- |
| `A31.each_declared_recipe` | PASS |
| `A31.upgrade_restore_workload` | PASS |
| `A32.authority_restore` | PASS |
| `A31.container_recreation_records` | PASS |
| `A31.readonly_container_mounts` | PASS |
| `A31.compose_complete_storage` | PASS |
| `A03.service_container_roots` | PASS |
| `A16.complete_backup_manifest` | PASS |
| `A31.recipe_latency_profile` | PASS |

## Go evidence

| File | Exit | Tests | Command |
| --- | --- | --- | --- |
| `go-run-1vqldn72.json` | 0 | 4 | `go test -race -count=1 -timeout 300s -json -run ^(TestManualRunByANonOwnerReturnsAConflict|TestOfflineBaselineStorageFailureRefusesStartup|TestOffline` |
| `go-run-5bsug783.json` | 0 | 1 | `go test -race -count=1 -timeout 300s -json -run ^(TestApplicationUsesExplicitDeploymentRootsWithoutHome)$ ./internal/cli/app` |
| `go-run-jputukb2.json` | 0 | 2 | `go test -race -count=1 -timeout 300s -json -run ^(TestStoredBaselineReuseStillValidatesGeneration|TestStoredBaselineReusesVerifiedCatalog)$ ./` |
| `go-run-njcjqu5j.json` | 0 | 3 | `go test -race -count=1 -timeout 300s -json -run ^(TestLocalToSharedPromotionAfterMove|TestOfflineEmbeddedCandidateHasDurableGeneration|TestRecoveryTop` |
| `go-run-rptdgak7.json` | 0 | 1 | `go test -race -count=1 -timeout 300s -json -run ^(TestProjectionPresencePreservesRecoverySelection)$ ./internal/catalog/workspace` |
| `go-run-uj4mvhkk.json` | 0 | 12 | `go test -race -count=1 -timeout 720s -json -run ^(TestBackupInventoryAccountsForEveryCanonicalRole|TestBackupInventoryPreservesSelectedWorkspaceAndRec` |
| `go-run-zo781pqv.json` | 0 | 9 | `go test -race -count=1 -timeout 540s -json -run ^(TestBackupApplicationCapturesExistingDeployment|TestBackupApplicationRefusesInvalidCatalog|TestBacku` |
| `go-run-ztkwkgaa.json` | 0 | 5 | `go test -race -count=1 -timeout 300s -json -run ^(TestBackupBundleBindsStoresFilesAndKeyAccess|TestBackupBundleDetectsChangedArtifacts|TestBackupBundl` |

## First run

The first run on 2026-10-04 at 16:44Z failed `A16.complete_backup_manifest` only.
The Go linker reported `no space left on device` on a 926 GiB volume with 323 MiB free.
The failure came from the host disk, not from the product. The `run1/` directory keeps that log and evidence.
This rerun followed the owner-approved disk cleanup decision and used the same registry and source heads.
