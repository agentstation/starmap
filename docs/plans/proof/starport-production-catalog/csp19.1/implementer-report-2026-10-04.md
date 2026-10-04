# CSP19.1 implementer final report (csp191-migration, 2026-10-04T00:41Z)

Branch `csp191-migration` at `1528da52`, rebased onto `1f18b216`, 8 commits, clean worktree. Not pushed. No PR.

## Commits (oldest first)

- 7692b88c blob: type the populated object store import refusal
- a65d404f recovery: type the backup encryption-key refusal
- 82187297 app: qualify the populated local-to-shared migration
- d6399d3c recovery: qualify the local-to-shared move with the recipe image
- 213d0ae3 docs: add the local-to-shared migration procedure (CSP19.1)
- 3ada33e9 catalog: prove baseline promotion over a head that a local-to-shared move carried
- 4747f190 app: promote the packaged baseline after the local-to-shared move
- 1528da52 docs: record promotion and process-scoped keys after the local-to-shared move

`git diff 491d9281 1f18b216 -- '*.go'` is empty (the rebase brought in no Go change).

## Fixture isolation

Separate Valkey DB 13 (DBSIZE 0 before and after each run), a fresh PG schema and a fresh bucket per test. Endpoints from `docker inspect` into a mode-600 env file. No container stopped, restarted, or removed. Logs have 0 matches for the fixture URL or MinIO secret patterns.

## Full fixture suite (rebased code, -race)

`go test -race -count=1 -timeout 60m -run '^TestLocalToSharedMigration' ./internal/app/ -v`: ok in 534.324s, exit 0.

| Test | Result |
| --- | --- |
| TestLocalToSharedMigrationPopulatedWorkload | PASS (144.41s) |
| TestLocalToSharedMigrationRefusals (7 subtests) | PASS (46.51s) |
| TestLocalToSharedMigrationPromotionAfterMove | PASS (141.22s) |
| TestLocalToSharedMigrationSecondReplicaJoins | PASS (136.98s) |
| TestLocalToSharedMigrationRollback | PASS (62.84s) |

Parity: `keys=3 accounts=2 credentials=2/2 files=2 users=2 teams=1 memberships=2 grants=2 budget_windows=2 budget_records=11 usage=4 audit=3`. The G9 enumeration passes. The `bootstrap_allowed=0` check passes. Refusal subtests: populated-kv, populated-sql, populated-blob, different-deployment, wrong-master-key, restored-sql-witness, open-boundary. PromotionAfterMove: moved head revision=2, promotion applied previous=2 promoted=4 leader_named=true.

Lease log line "Another instance owns the refresh lease" comes from the `CatalogBaselineStatus` observer (`internal/catalog/baseline_observer.go:75`, `fleet_lease.go:35-37`), not from the running gateway.

## Fail-before evidence

- Without `App.Run`, PromotionAfterMove failed in 265.69s: "baseline promotion is still pending". `executePromotions` starts only in `catalog.Runtime.Start` (`internal/catalog/runtime.go:401`), called by `App.Run` (`internal/app/app.go:1280`). Test-only fix (approved): run the gateway on a free port, wait for `/health/ready`, then promote. Wait and assertions unchanged.
- Catalog `TestLocalToSharedPromotionAfterMove`: first version asserted HeadRevision 1 and got 2 (the leader republishes when it opens). The test now asserts the republication and promotes with ExpectedRevision = carried revision.
- Before the typed errors, the populated-blob, different-deployment, and wrong-master-key refusals carried untyped strings. The tests now check them with `require.ErrorIs`.

## Catalog race run (before the rebase, which changed no Go code)

`go test -race -count=1 -timeout 30m -run 'Promotion|Promote|Topology|Baseline' ./internal/catalog/`: ok in 1600.062s. Includes `TestLocalToSharedPromotionAfterMove` (10.34s alone).

## No-fixture checks at 1528da52

- Skip proof: all 5 app tests SKIP with "UNVERIFIED: local-to-shared migration needs native Valkey, PostgreSQL, and object storage". `TestContainerRecipeLocalToShared` (internal/config/compose_recipe_test.go:134) SKIPs with "UNVERIFIED: STARPORT_RECIPE_IMAGE is required for local-to-shared container qualification".
- config ok 20.151s, blob ok 24.514s.
- gofmt: only the pre-existing `internal/recovery/catalog_expiring_preparation_test.go`.
- go vet clean on app, blob, recovery, config, and catalog.
- make lint: 0 issues.
- verify-v1-architecture 11/0. verify-starmap-ownership 12/0. verify-doc-links PASS.
- console docs vitest: 5 files, 49 tests.

## Skipped by the implementer

- Full no-fixture `go test` of app, recovery, catalog on the rebased head.
- `TestContainerRecipeLocalToShared` with an image in this session (earlier result in d6399d3c still applies).
- Starmap `make verify`.

## G9 result (no stop condition)

- provider-health:instance: and provider-latency:instance: use per-process instance IDs. A gateway writes them only when `Storage.Distributed()`. A local source writes none. A carried record is a peer record bounded by TTL.
- catalog_migration:v1: written only by `starport migrate runtime prepare`. A moved receipt refuses on a fleet with "store identity differs" (fails closed).
- No transfer filter added. PopulatedWorkload enumerates the three prefixes on the target and finds none.

## backend_id and bootstrap_allowed

Activation approves with BackendID = History.ValkeyIncarnation (`recovery_activation_release.go:285`). `replaceWith` writes bootstrap_allowed=0 (`witness.go:142-146`). Only fleet init writes 1. The local-to-fleet compile writes the head, the inventory, both initialized markers, and epoch 1 with no lease. The target therefore never needs bootstrap. Asserted after activation.

## Source retirement (report only)

Nothing retires the local source after the move (`budget.go:31-50`, `recovery_startup.go:54-110`). Observation: `closed_local_source_restarts_unchanged`. The procedure makes the external fence mandatory.

## Limits

- The fixture team has no budget, so team budget history is not tested.
- The test compares usage and audit records separately from the reference report.
- Valkey is plaintext on a private network.
- The image test stops before activation.
- The procedure is not qualified for production.
- Long fixture runs need -timeout 60m under load.

## Open questions

1. CI selects no TestLocalToSharedMigration* test. TestContainerRecipeLocalToShared is outside the Storage Recipes regex.
2. The run used Valkey DB 13 for isolation. One stopped run left 103 keys under the test prefix, deleted afterwards.
3. pyenv python3 prints blake2 warnings. The implementer used a scratch shim (environment only).
