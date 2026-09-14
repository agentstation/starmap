# Catalog publication preparation

This command collects configured sources and prepares one catalog artifact, run receipt, and private replay checkpoint.
It does not advance a public channel or merge an embedded catalog update.
The scheduled workflow still needs the separate promotion integration.

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
| `state-<digest>.json` | Original baseline, retained source observations, and accepted artifact. |
| `prepared-<run digest>.json` | Private retry record written before artifact staging. |
| `rejected-<digest>.json` | Private admission result for a rejected acquisition. |

The output root restricts access to its owner. Checkpoints, receipts, and retry records use the private file policy.
Checkpoints and retry records contain private scope selectors. Do not publish them with the catalog assets.
The receipt excludes credential values and raw provider errors.
Its binding checksum does not authenticate the publisher.

## Resume and retry

After successful promotion, supply the resulting checkpoint and its separately trusted digest:

```sh
go run ./cmd/starmap-catalog-publish \
  -profile publication.yaml \
  -publisher-id starmap-public \
  -run-id example-002 \
  -state /private/state-ACCEPTED_DIGEST.json \
  -state-checksum sha256:ACCEPTED_DIGEST \
  -output-dir ./publication-output
```

Read the digest from accepted private state or verified publisher provenance.
Do not use the checkpoint's own digest as proof of trust.
The decoder validates original observation receipts and replays their payloads before accepting the checkpoint.

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
The command does not compact history or automatically adopt a prepared checkpoint.

## Verify

```sh
go test -race ./internal/catalog/publication ./cmd/starmap-catalog-publish
```

These tests use a local provider HTTP server and exercise acquisition, retention, checkpoint validation, artifact staging, and restart retries.
The native CI matrix runs the same command tests on Linux, macOS, and Windows.
