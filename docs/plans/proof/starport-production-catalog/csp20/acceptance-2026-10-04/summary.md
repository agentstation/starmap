# CSP20 acceptance run (2026-10-04)

Starmap registry head `da7e88539` against Starport main `e17ea25a7`.
Install evidence: the format 4 capture of Starport PR run 37222309299 (PR #414) in `docs/proof/catalog-native`.
Run start `2026-10-04T19:56:19Z`, task exit 0 at `2026-10-04T19:58:01Z`.
Fixtures: PostgreSQL 16.15, Valkey 7.2.14 (plaintext, private network), MySQL, MinIO. Race operator binary from Starport main.

## Result

CSP20 subcases: PASS 10 of 10.
The whole gate status is `None`, because a `--task` run reports the 50-case gate.

| Subcase | Status |
| --- | --- |
| `A34.temporary_persistent` | PASS |
| `A34.catalog_before_keys` | PASS |
| `A34.credential_roles` | PASS |
| `A34.recipe_links` | PASS |
| `A35.each_advertised_native_install` | PASS |
| `A35.clean_catalog_no_keys` | PASS |
| `A35.documented_inference` | PASS |
| `A34.persistence_paths_and_cache_roles` | PASS |
| `A34.qualified_performance_claims` | PASS |
| `A50.current_artifact_evidence` | PASS |

## Go evidence

| File | Exit | Tests | Command |
| --- | --- | --- | --- |
| `go-run-7q652mik.json` | 0 | 4 | `go test -race -count=1 -timeout 300s -json -run ^(TestDeclaredRecipePages|TestDevelopmentRejectsPersistentEnvironmentSelections|TestLoaderOwnsIndepend` |
| `go-run-h26g7czq.json` | 0 | 1 | `go test -race -count=1 -timeout 300s -json -run ^(TestDiscoveryRouteWithoutProviderCredentials)$ ./internal/server` |
| `go-run-jdz3s3zc.json` | 0 | 1 | `go test -race -count=1 -timeout 300s -json -run ^(TestConfigPathsTextIncludesRuntimeAndStorageLocations)$ ./internal/cli` |
| `go-run-mozp9w8b.json` | 0 | 12 | `go test -race -count=1 -timeout 720s -json -run ^(TestModelsSearchAndShowWithoutCredentialsOrNetwork|TestModelsSearchAnswersJSON|TestReadmeFollowsTheF` |
| `go-run-thncm_0a.json` | 0 | 3 | `go test -race -count=1 -timeout 300s -json -run ^(TestDevKeepsCatalogStateInScratch|TestDevUsesInMemoryBadger|TestDocumentedInferenceRequestStreamsThr` |
