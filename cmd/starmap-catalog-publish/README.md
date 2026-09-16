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
models.dev requires accepted evidence before the first publication. After an outage, the publisher retains that evidence while valid provider updates continue.
The public profile permits isolated invalid records through `allow_record_quarantine: true`.
A provider failure does not stop publication. The receipt reports missing credentials, failure, and retained evidence separately.

The models.dev policy sets `allow_stale_retained: true` and uses 24 hours as its freshness threshold.
Older evidence receives `stale_retained` status. Its original observation time remains unchanged.
This option defaults to false and requires a positive `max_retained_age`. Provider scopes retain their existing 24-hour admission limit.

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

## Record quality and corrections

The models.dev adapter trims surrounding whitespace from display names. It preserves exact model IDs and the original source bytes.
Each correction logs `display_name_whitespace_trimmed` with its run, source, provider, and model identity.
Source formatting rules belong in the adapter. Individual model names do not require YAML exceptions.

Unresolved invalid records remain quarantined. A source policy can permit their valid siblings with `allow_record_quarantine: true`.
This setting defaults to false. It requires accepted records and only classified record failures.
Transport failures, truncation, stale fallback, and source schema failures still require eligible retained evidence.

The run receipt records the partial attempt, degraded observation, accepted and rejected counts, and each rejected record's identifier and reason code.
It excludes raw diagnostic messages. The checkpoint retains the original observation evidence for replay.
Invalid records preserve their last accepted catalog values. An incomplete provider inventory never establishes model absence.

A repaired source update clears its current quarantine report. Historical immutable receipts retain the earlier report.
An outage preserves the retained report and its original observation time under the selected retention policy.
Operators inspect the current receipt for source quality and the acquisition logs for correction and rejection details.

The scheduled workflow retains per-source outcomes, observation times, ages in seconds, and stale flags in `source-status.log`.
Its job summary reports the stale-source count and oldest evidence age. A successful source refresh clears the current stale status.

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
go test -race -timeout=30m ./internal/catalog/publication ./cmd/starmap-catalog-publish
```

These tests use a local provider HTTP server and exercise acquisition, retention, checkpoint validation, artifact staging, and restart retries.
The full public-profile capacity test replays eight scheduled runs, including an outage and checkpoint recovery.
Race instrumentation needs the explicit timeout for this full-size test. Short test runs omit its capacity qualification.
The native CI matrix runs the same command tests on Linux, macOS, and Windows.
