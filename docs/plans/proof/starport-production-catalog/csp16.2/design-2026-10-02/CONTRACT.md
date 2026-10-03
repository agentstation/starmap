# CSP16.2 explicit baseline promotion contract

Date: 2026-10-02. Base: Starmap `80de6d830`, Starport `a4e7e8fd`.
This file is the shared input for the Starmap runtime PR, the Starmap contract PR, and the Starport promotion PR.
Both repositories use the registry test names below. A test name change needs a change in both repositories.

## Scope

Fleet mode only. An established fleet retains its baseline across binary upgrades (CSP11).
One explicit operation promotes the packaged baseline of the running binary into the fleet publication.
Single-node local mode keeps its current behavior. The plan ledger records that limit.

## Subcase contract text

Register this text in `acceptance-map.json` under `subcase_contracts`.
Add the five subcases to `required_subcases.A14` and to `task_checks` for CSP11, CSP16.2, CSP22, and CSP24. The roster validator requires each A14 subcase in all four tasks.

| Subcase | Contract |
| --- | --- |
| `A14.explicit_baseline_promotion` | A fleet binary upgrade keeps the retained baseline. One explicit operation under the publication lease adopts the packaged baseline at the expected head with an increasing revision. Replay after promotion restores the promoted baseline on every replica. |
| `A14.promotion_exact_retry` | A promotion binds a stable operation ID to the deployment, the expected head, and the packaged generation. An exact retry returns the original receipt without a second publication. A stale expected head, a reused operation ID with a different target, or a packaged generation that equals the retained baseline refuses. |
| `A14.promotion_preserves_pins_and_authority` | Promotion refuses while a generation pin is active or a source authority is configured. Retained removal targets survive promotion. A target absent from the promoted baseline stays recorded, stays inert, and appears in the receipt. |
| `A14.software_rollback_keeps_promoted_baseline` | After promotion, a binary with an older packaged baseline replays the promoted baseline and never falls back to its packaged baseline. A recovery record with an unsupported version refuses acquisition and names the version. The record format does not change for promotion. |
| `A14.promotion_request_path_memory` | During promotion and the following replay, request admission reads the accepted catalog in memory. A counting store proves zero request-path storage calls. |

## Registry entries

Register each subcase in `scripts/catalog-product-checks.json` with `kind: all` over these `go_test` checks.
Every check uses `timeout: 5m` unless the table names another value.

| Subcase | Repository | Package | Test |
| --- | --- | --- | --- |
| `A14.explicit_baseline_promotion` | starmap | `./runtime` | `TestPromoteEmbeddedBaselineAdoptsPackagedBaselineUnderLease` |
| `A14.explicit_baseline_promotion` | starmap | `./runtime` | `TestFleetReplayRestoresPromotedBaseline` |
| `A14.explicit_baseline_promotion` | starport | `./internal/catalog` | `TestFleetPromotionAdvancesHeadWithIncreasingRevision` |
| `A14.promotion_exact_retry` | starmap | `./runtime` | `TestPromoteEmbeddedBaselineRefusesStaleHeadAndEqualBaseline` |
| `A14.promotion_exact_retry` | starport | `./internal/catalog` | `TestPromotionExactRetryReturnsOriginalReceipt` |
| `A14.promotion_exact_retry` | starport | `./internal/catalog` | `TestPromotionRefusesReusedOperationID` |
| `A14.promotion_exact_retry` | starport | `./internal/cli` | `TestCatalogPromoteBaselineCommandReportsReceipt` |
| `A14.promotion_preserves_pins_and_authority` | starmap | `./runtime` | `TestPromoteEmbeddedBaselineRefusesPinnedGeneration` |
| `A14.promotion_preserves_pins_and_authority` | starmap | `./runtime` | `TestPromoteEmbeddedBaselineRefusesConfiguredAuthority` |
| `A14.promotion_preserves_pins_and_authority` | starmap | `./runtime` | `TestPromoteEmbeddedBaselineRetainsInertRemovalTargets` |
| `A14.software_rollback_keeps_promoted_baseline` | starmap | `./runtime` | `TestFleetReplayAfterPromotionIgnoresOlderPackagedBaseline` |
| `A14.software_rollback_keeps_promoted_baseline` | starmap | `./runtime` | `TestFleetRecoveryRefusesUnsupportedRecordVersion` |
| `A14.software_rollback_keeps_promoted_baseline` | starport | `./internal/catalog` | `TestFleetRuntimeRollbackKeepsPromotedBaseline` (timeout `10m`) |
| `A14.promotion_request_path_memory` | starport | `./internal/app` | `TestCatalogPromotionKeepsRequestPathInMemory` |

`TestFleetRuntimeRollbackKeepsPromotedBaseline` runs two real gateway processes against the Valkey and PostgreSQL fixtures, like `TestFleetRuntimeProcessesRecoverAfterLeaderDirectoryLoss`.
It skips with `UNVERIFIED` in its message when the fixture variables are unset. Every other test runs without fixtures.

## Starmap runtime API

```go
// BaselineStatus compares the packaged baseline with the retained fleet baseline. It reads memory only.
type BaselineStatus struct {
    Packaged  catalogs.GenerationIdentity // GenerationID and payload checksum of the running binary
    Retained  catalogs.GenerationIdentity // GenerationID and payload checksum in the fleet record
    Head      FleetHead
    Promotable bool // true when the identities differ, no pin is active, and no authority is configured
    Refusal   string // the reason when Promotable is false
}

func (r *Runtime) BaselineStatus() (BaselineStatus, bool)

type BaselinePromotion struct {
    ExpectedHead         FleetHead
    PackagedGenerationID string
}

type BaselinePromotionResult struct {
    Head          FleetHead                       // the new head
    Previous      catalogs.GenerationIdentity
    Promoted      catalogs.GenerationIdentity
    InertRemovals []catalogs.CatalogRemovalTarget // retained targets absent from the promoted baseline
}

func (r *Runtime) PromoteEmbeddedBaseline(ctx context.Context, request BaselinePromotion) (BaselinePromotionResult, error)
```

- `PromoteEmbeddedBaseline` runs inside `r.execute(runKindManual)`. It commits through `prepareFleetCommitWithPin` with the grant from the run.
- It refuses with a conflict in three cases. The expected head differs from the current head. The packaged generation differs from the running binary. The packaged identity equals the retained identity.
- It refuses with a conflict when a generation pin is active. It refuses when the runtime requires authority or has a source authority origin.
- The new publication retains the source layer, provider layers, manual checkpoint, and removal policy. Only the embedded layer, its manifest, and the compatibility value change.
- The recovery record keeps version 2 and gains no field. A released binary replays the promoted record without change.
- `catalogs.GenerationIdentity` is a new exported pair `{GenerationID, PayloadChecksum}`. Reuse an equivalent type when one exists.

## Starport promotion operation

- Operation identity: `operation_id` from the operator, bound to `deployment_id`, the expected head revision, and the packaged generation ID.
- Receipt store: a KV record `catalog:promotion:{<deployment digest>}:v1:<operation_id>`. It shares the hash tag of the lease key. Starport writes it after the Starmap result in one native transaction with the live lease grant.
- Exact retry: a stored receipt with the same binding returns without a runtime call. A stored receipt with a different binding refuses with a conflict.
- Request record: the CLI writes one pending request `catalog:promotion-request:{<deployment digest>}:v1` with a bounded lifetime. It carries the operation ID, the expected revision, the packaged generation ID, the actor, and the request time. One pending request exists per deployment. A different pending operation ID refuses the write and names it.
- Leader rule: only the lease holder executes a promotion. The leader reads the request at each lease renewal. It runs the promotion as a manual run under its lease. It replaces the request with the outcome. A follower never executes. The CLI never takes the lease and never opens the gateway state directory.
- Binding check by the leader: a leader whose packaged generation differs from the request refuses and names both generations. An old binary cannot promote a request from a new binary.
- Acceptance: the promoted head passes the existing fleet acceptance (`fleet_acceptance.go`) with an increasing revision before the receipt reports `applied`.
- CLI: `starport catalog promote-baseline --operation-id <id> [--expected-revision <n>] [--wait <duration>] [--json]` and `starport catalog baseline-status [--json]`. The promote command records the request and names the current lease holder. It polls the outcome inside the wait bound and prints the receipt. On a wait timeout it exits with status 1. It says that the request stays recorded. It says that the same operation ID continues the wait.
- CLI without a leader: the command says that no gateway leads. The operator then starts a gateway with the new binary.
- The receipt and the status never carry credentials. They carry generation IDs, checksums, revisions, the operation ID, the actor, and timestamps.

## Receipt

```json
{
  "operation_id": "…",
  "deployment_id": "…",
  "status": "applied|refused",
  "previous": {"generation_id": "…", "checksum": "…", "revision": 0},
  "promoted": {"generation_id": "…", "checksum": "…", "revision": 0},
  "inert_removals": [],
  "refusal": "…",
  "actor": "…",
  "created_at": "…"
}
```

## Boundaries

- Request admission reads memory during the whole operation (D41 condition 17).
- Local mode has no promotion. The command refuses in local mode and names the fleet-only scope.
- Catalog rollback to a previous baseline is a separate explicit operation outside this task.
- The console surface for promotion belongs to CSP17.
