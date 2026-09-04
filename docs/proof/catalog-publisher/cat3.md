# CAT3 proof: canonical publication channel

CAT3 owns the publisher workflow, the release command, the channel schema, the
GoReleaser tag policy, and the workflow tests. CAT-V06 and CAT-V07 belong to
other tasks and stay out of this record.

## Fail before

`docs/proof/catalog-publisher/cat2-fail-before.txt` holds the plan-wide capture
on the base commit. The four CAT3 conditions read:

```text
FAIL CAT-V01 the workflow publishes catalog-latest and no catalog-semantic tag.
PASS CAT-V02 the workflow schedule runs every four hours at minute 17.
FAIL CAT-V03 the workflow parses with the generate job at 90 minutes and the Refresh candidate catalog step at 75 minutes.
PASS CAT-V04 the workflow serializes runs and cancels no run in progress.
FAIL CAT-V05 the channel document advances channel_updated_at without a new catalog generation.
```

The base tree also carried one broken test.
`TestScheduledGenerationWorkflowPublishesOnlyValidatedChangedPayload` asserted
the retired daily cron `17 3 * * *` while the workflow already requested
`17 */4 * * *`. CAT3 repairs that test.

## Canonical names

| Concept | Name |
| --- | --- |
| Immutable release tag | `catalog-<catalog-digest>` |
| Immutable release title | `Catalog <generation-id>` |
| Stable channel release | `catalog-latest` |
| Channel asset | `catalog-latest.json` |
| Channel title | `Catalog latest` |

The workflow holds no `catalog-semantic-` string. The retired
`catalog-semantic-*` and `catalog-payload-*` namespaces moved into
`cmd/starmap-catalog-release/rollback.go`. The `--rollback-candidates` mode
reads a GitHub release listing and returns every readable immutable release,
newest first, across all three namespaces. New publication uses the canonical
namespace alone.

## Channel document schema

`pkg/catalogs/artifact/channel.go` owns the schema. `Channel` carries:

| Field | JSON name | Purpose |
| --- | --- | --- |
| SchemaVersion | `schema_version` | Channel document schema version |
| Name | `channel` | The channel name, `catalog-latest` |
| Sequence | `sequence` | Monotonic run counter |
| ChannelUpdatedAt | `channel_updated_at` | Time of the last successful run |
| GenerationID | `generation_id` | Selected generation identifier |
| Tag | `tag` | Selected immutable release tag |
| CatalogDigest | `catalog_digest` | Selected facts-only catalog digest |
| PublishedAt | `published_at` | Publication time of the selected release |
| Assets | `assets` | Selected release assets |

Each `ChannelAsset` carries `name`, `media_type`, `checksum`, and `size_bytes`.

`EncodeChannel` sorts the assets and normalizes the times to UTC. It writes
canonical indented bytes. `DecodeChannel` bounds the input size and rejects an
unknown field. It then validates the result.

## Promotion ordering

`Channel.Advance` in `pkg/catalogs/artifact/channel.go` owns the ordering
check. It takes a `Candidate` that carries a `ReleaseVerification` with
`assets_present`, `checksums_match`, and `attestation_verified`. An incomplete
verification returns a typed validation error, so the channel cannot select an
immutable release before that release verifies.

`TestChannelRejectsPromotionBeforeImmutableVerification` covers the four
incomplete combinations. The scheduled workflow test in
`pkg/catalogs/artifact/scheduled_workflow_test.go` proves the workflow order.
The immutable readback and provenance check precede the channel staging step.

## Heartbeat

`Channel.Advance` returns `AdvanceHeartbeat` when the candidate digest equals
the selected digest. The heartbeat raises `sequence` and `channel_updated_at`
and keeps `generation_id`, `tag`, `catalog_digest`, and `published_at`.
`AdvanceKind.CreatesGeneration` reports false for a heartbeat, so an unchanged
catalog creates no generation and no immutable release.

## Nested timing bounds

The workflow gives the `generate` job `timeout-minutes: 90` and the
`Refresh candidate catalog` step `timeout-minutes: 75`.
`TestCatalogGenerationWorkflowNestsTimeoutLimits` parses the workflow with
`github.com/goccy/go-yaml`. It reads both limits at their structural positions.
It then asserts that 75 exceeds the 60-minute transfer bound, and that 90
exceeds 75.

## Verification

| Command | Result |
| --- | --- |
| `go test ./internal/ciworkflow/... ./pkg/catalogs/artifact/... ./cmd/starmap-catalog-release/... -race -count=1` | 3 packages ok |
| `go test ./internal/ciworkflow -run TestCatalogGenerationWorkflowNestsTimeoutLimits` | PASS |
| `go test ./pkg/catalogs/artifact/... -run TestChannelAdvancesUpdatedAtWithoutNewGeneration` | PASS |
| `make test` | 65 packages ok, 0 failed |
| `make lint` | 0 issues |
| `go tool ago -stale-ignores -format json ./...` | 0 findings, 0 stale ignores |
| `actionlint` | 0 diagnostics over every workflow |
| `make catalog-generation-check` | pass |
| `make verify-action-pins` | 8 references resolve |
| `make technical-writing-check` | PASS, 682 files, 0 diagnostics |
| `make docs-check` | all documentation up to date |

Every command ran with `GOTOOLCHAIN=go1.26.6`.

## Pass after

```text
PASS CAT-V01 the workflow publishes catalog-latest and no catalog-semantic tag.
PASS CAT-V02 the workflow schedule runs every four hours at minute 17.
PASS CAT-V03 the workflow parses with the generate job at 90 minutes and the Refresh candidate catalog step at 75 minutes.
PASS CAT-V04 the workflow serializes runs and cancels no run in progress.
PASS CAT-V05 the channel document advances channel_updated_at without a new catalog generation.
```

## Decisions for review

1. `--clobber` now appears once, in the channel upload step. The mutable channel
   replaces its document in place. The test forbids `--clobber` in the immutable
   publish step and requires exactly one occurrence in the workflow.
2. The classify step no longer exits early when the catalog does not change. It
   publishes when GitHub holds no canonical release tag. The namespace
   migration needs that path. Every current release carries a retired tag.
3. `.goreleaser.yaml` `ignore_tags` collapses to `catalog-*`. The single pattern
   covers the canonical namespace, both retired namespaces, and the
   `catalog-latest` channel.
4. The channel steps run on every successful run, with no `publish` condition.
   The heartbeat needs that. The release command reads the verified directory
   from the fresh publication or from the existing release.
