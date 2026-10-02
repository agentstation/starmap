# CSP16.2 fail-before capture

Date: 2026-10-02. Starmap `80de6d830`. Starport `a4e7e8fd`.

## Acceptance verifier

`scripts/catalog_product_verify.py --task CSP16.2 --json` exits 2 with `Unknown task: CSP16.2`.
The record is `task_csp16.2_fail_before.json.gz`.
`acceptance-map.json` has no `CSP16.2` entry under `task_checks`, and `required_subcases.A14` lists four CSP11 subcases only.
`scripts/catalog-product-checks.json` has no promotion check.

## Starmap runtime

- The fleet recovery record retains the baseline generation (`runtime/fleet_recovery.go:47-58`). Replay adopts that baseline and ignores the packaged one (`runtime/fleet_replay.go:72-73`).
- No exported runtime method changes the retained baseline. The exported mutations are `UpdateAcquisition`, `UpdateObservations`, `PublishObservations`, `ReplaceRemovalTargets`, `RefreshFleet`, `Refresh`, `Sync`, and `CompleteDirectoryMigration`.
- `FleetStatus` (`runtime/fleet_runtime.go:46-61`) reports the head and replay readiness. It does not report the packaged generation or whether it differs from the retained baseline.
- The compatibility checksum binds the baseline identity and checksum (`runtime/fleet_replay.go:25-32`). A baseline change needs a new publication with a new compatibility value.
- The record decoder rejects unknown members and any version other than 2 (`runtime/fleet_recovery.go:150-161`). A new record field or version makes every released binary refuse replay.
- A manual run requires the publication grant captured before acquisition (`runtime/fleet_runtime.go:142-170`). `ReplaceRemovalTargets` (`runtime/removal_update.go:20-35`) is the exemplar for an operator mutation inside `r.execute(runKindManual)`.
- `validateGenerationMutation` refuses every manual mutation while a generation pin is active.
- Single-node local mode without a recovery checkpoint adopts the packaged baseline at each start (`runtime/runtime.go:377-379`). The owner limited condition 13 to fleet mode on 2026-10-02.

## Starport

- Starport exports the embedded baseline at startup (`internal/catalog/runtime.go:158-161`) and never offers it for promotion.
- `internal/catalog/runtime_fleet.go` wraps `FleetStatus` and `RefreshFleet` only.
- Fleet acceptance checks the head compare-and-set and the increasing revision (`internal/catalog/fleet_acceptance.go:49-118`).
- The CLI has `fleet init` only (`internal/cli/fleet.go:16-22`). No `catalog` command exists.
- The operation registry (`internal/catalog/operations.go`) has one kind, `catalog_update`, and keeps closed operations in memory only.
- No receipt store exists for a catalog operator operation. The configuration operation journal (CSP16.1) is the nearest exemplar.
