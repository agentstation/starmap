# Catalog publication preparation

This command collects configured sources and prepares one catalog artifact, run receipt, and replay checkpoint.
It does not advance a public channel or merge an embedded catalog update.
The [scheduled publisher](../../docs/SCHEDULED_CATALOG_GENERATION.md) owns promotion and channel updates.

## Prepare a catalog

Create an explicit source profile:

```yaml
policy_version: example-v1
scopes:
  - source: embedded_catalog
    required: true
    enabled: true
    allow_missing: false
    max_retained_age: 0s
    disabled_action: preserve
```

This example works offline and makes no claim of fresh provider acquisition.
Provider scopes require a complete acquisition binding and a configured catalog credential profile.
Missing credentials cannot make a required scope optional.

```sh
go run ./cmd/starmap-catalog-publish \
  -profile publication.yaml \
  -publisher-id starmap-public \
  -run-id example-001 \
  -output-dir ./publication-output
```

The first run uses the compiled baseline.
The publisher identity must remain stable across runs and binary updates.
Use `-workspace` to select a local source workspace.
The command uses Starmap's catalog acquisition credential resolver.

The JSON success report has status `prepared` and names these outputs:

| Output | Purpose |
| --- | --- |
| `artifacts/catalog-generations/<generation directory>/` | Immutable catalog archive, detached checksum, and statement. |
| `receipt-<digest>.json` | Run outcome, admitted source evidence, and exact artifact binding. |
| `state-<digest>.json` | Selected baseline, retained source observations, and accepted artifact. |
| `prepared-<run digest>.json` | Private retry record written before artifact staging. |
| `rejected-<digest>.json` | Private admission result for a rejected acquisition. |

The output root restricts access to its owner. Checkpoints, receipts, and retry records use the private file policy.
Checkpoints retain the scope selectors and model data from their configured sources.

The [public GitHub profile](../../.github/catalog-publication.yaml) selects models.dev over HTTP and twelve provider APIs.
models.dev must supply complete evidence from this run or the previous 24 hours.
A provider failure does not stop publication. The receipt reports missing credentials, failure, and retained evidence separately.
Evidence older than 24 hours cannot satisfy source admission, but its catalog facts remain until an explicit removal.

The public profile uses public model data and basic provider API keys.
Its checkpoints can live on GitHub without encryption or a private object store.
The API keys stay in Actions secrets. The checkpoint encoder does not serialize credential values.

Public bindings contain no account or project selectors.
Each binding limits membership changes to its own scope and uses the provider's default catalog endpoint.
The `default-endpoint` region identifies that endpoint selection, not every region the provider supports.
DeepInfra uses its declared public acquisition profile and needs no API key for catalog reads.

Enterprise profiles can contain private models or account selectors. Store those checkpoints in the deployment's configured private storage.
Retry records stay in the local work directory.
The receipt excludes credential values and raw provider errors.
Its binding checksum does not authenticate the publisher.

## Resume and retry

After successful promotion, supply the resulting checkpoint and its separately trusted digest:

```sh
go run ./cmd/starmap-catalog-publish \
  -profile publication.yaml \
  -publisher-id starmap-public \
  -run-id example-002 \
  -state ./state-ACCEPTED_DIGEST.json \
  -state-checksum sha256:ACCEPTED_DIGEST \
  -output-dir ./publication-output
```

Read the digest from accepted publisher state or verified publisher provenance.
Do not use the checkpoint's own digest as proof of trust.
The decoder validates original observation receipts and replays their payloads before accepting the checkpoint.

Add `-baseline-embedded` when the resumed publication must apply authored changes from the current compiled catalog.
The command validates model and alias state before acquisition and binds that baseline to the saved run request.
An embedded copy of the publisher's own accepted output keeps the existing acquisition baseline.
This prevents published provider facts from becoming permanent when an operator later removes their source scope.
Alias removal keeps its historical rename record with the removed state.

The scheduled workflow must finish a pending promotion before it starts another publication against a changed baseline.
This command does not identify pending GitHub promotions or authorize a source commit.

Retry with the same arguments, run identity, and output directory after a staging failure.
A saved preparation reuses its exact bytes without another source request.
A changed profile, baseline, checkpoint, publisher, or workspace requires a new run identity.

Failed or partial provider replies can use retained complete evidence within the profile's age limit.
The receipt preserves the original observation time and reports retention.
An unchanged semantic catalog can reuse its artifact while receiving a new run receipt.

The run receipt carries current review candidates and their original observation evidence.
Consumers must use that receipt for current review results instead of treating historical artifact reviews as current.
Complete omission preserves the visible offering and records absence within its provider account scope.
An explicit disabled-source removal discards that scope's local input history. It preserves the baseline.

Replay currently accepts at most 4096 observations and 64 MiB of retained payloads.
The checkpoint byte limit is 256 MiB.
Capacity errors preserve the previous accepted state and require operator recovery.
Replay compacts repeated equivalent evidence when the smaller history preserves catalog bytes and current reviews.
Distinct source changes can still reach the capacity limits.
The command does not automatically adopt a prepared checkpoint.

## Restore an interrupted publication

Use the accepted run receipt and checkpoint to recover exact publication assets in a new working directory:

```sh
go run ./cmd/starmap-catalog-publish \
  -publisher-id starmap-public \
  -run-id example-002 \
  -restore-receipt ./starmap-catalog-run.json \
  -restore-receipt-checksum sha256:TRUSTED_RECEIPT_DIGEST \
  -state ./starmap-catalog-state.json \
  -state-checksum sha256:TRUSTED_CHECKPOINT_DIGEST \
  -output-dir ./recovered-publication
```

The success report has status `restored`. The restored archive, receipt, and checkpoint preserve the original bytes.
This operation makes no source or provider request and needs no provider API key.
It rejects a changed publisher, run identity, digest, source profile, workspace, or baseline selection.
Both digests must come from accepted state or verified publisher provenance.

The output directory remains private. Public publication requires the separate workflow checks and verified promotion.

## Verify

```sh
go test -race ./internal/catalog/publication ./cmd/starmap-catalog-publish
```

These tests use a local provider HTTP server and exercise acquisition, retention, checkpoint validation, artifact staging, and restart retries.
The native CI matrix runs the same command tests on Linux, macOS, and Windows.
