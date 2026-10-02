# CSP16.1 fail-before capture

Date: 2026-10-02. Starmap `80de6d830`. Starport `a4e7e8fd`.

## Acceptance verifier

`scripts/catalog_product_verify.py --task CSP16.1 --json` exits 1.
All 10 A26 and A27 subcases report `UNVERIFIED` with the reason `No behavior check is registered.`
The record is `task_csp16.1_fail_before.json.gz`.
`acceptance-map.json` lists the 10 subcases under `task_checks.CSP16.1` and `primary_task`. It has no `subcase_contracts` text for them.
`scripts/catalog-product-checks.json` has no `A26.*` or `A27.*` entry.

## Starport configuration surface

- No HTTP route exists for configuration schema, effective state, validation, connection tests, saves, or operation receipts.
  Configuration-adjacent routes read memory only: `GET /api/v1/admin/info` and `GET /admin/catalog/status`.
- The CLI has four configuration commands in `internal/cli/configuration_authority.go`: `config init --shared`, `config migrate --to`, `config apply [--resume]`, and `config effective`.
- No local file writer exists for field saves.
  The only configuration file write is the first publish during setup (`internal/setup/preparation.go:185`), which requires an absent file.
- `internal/configrevision` holds the shared store. It has an optimistic head, an append-only revision table, and exact retry by operation ID. It writes `audit.RecordTx` in the same transaction.
  `Commit` takes the complete seed from `cfg.SharedSeed()`, not field edits.
- `openConfigurationStoreForWrite` (`internal/app/configuration_operations.go:247`) runs `db.Migrate` inside every write operation.
- No HTTP idempotency key or operation ID exists on any admin route.
- Admin authorization is `HasScope("admin")`. Console sessions resolve to a local operator with scope `*`. No deployment binding exists on any write.
- `ApplyConfiguration` is the only operation that refuses a non-shared mode. `Initialize`, `Migrate`, setup, and the auth-mode switch do not check external management.
- The auth-mode switch (`internal/server/controllers/auth.go:190-193`) is the only write with origin and caller checks.
- HTTP audit is best effort after the write (`controllers/audit.go:60`). `configrevision` is the only transactional audit user.
- `TestRequestsReadAppliedRevisionInMemory` (`internal/app/configuration_authority_test.go:276`) proves request reads survive a closed store and a deleted file.

## Baseline promotion

- No promotion operation exists in Starport or in the pinned Starmap module.
  Starport exports the embedded baseline at startup (`internal/catalog/runtime.go:158`) and reads it only during recovery inspection.
- Starport enforces the generation pin at acceptance (`internal/catalog/acceptance.go:62-69`) and checks source authority at transition approval.
- Starport never calls the Starmap runtime functions `PinAcceptance`, `ReadPermission`, `RefreshPermission`, or `ReplaceRemovalTargets`, and holds no removal store.

## Console

`console/src/lib/api.ts` has no configuration types or fetchers. The console settings experience belongs to CSP17.
