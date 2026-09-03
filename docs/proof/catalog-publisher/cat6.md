# CAT6 proof: application defaults

CAT6 owns the application composition. The CLI and the server compose the CAT5
connected runtime through `starmap.Open`. One settings table maps every
canonical name onto exactly one runtime option. The Docker Compose example and
the environment example carry the same names with their defaults.

CAT5 owns the runtime. CAT7 owns the Starmap cascade source. CAT9 owns the
operator documentation.

## Fail before

The base commit `fb328d0a` read no canonical catalog name. This command finds
every Go file on that commit that names one:

```console
$ git grep -l "STARMAP_CATALOG" fb328d0a -- '*.go'
fb328d0a:internal/ciworkflow/release_workflow_test.go
fb328d0a:pkg/catalogs/artifact/attestation_workflow_test.go
```

Both files are release-workflow tests. Neither reads a process setting, so the
CLI accepted no source URL. An operator who set `STARMAP_CATALOG_SOURCE_URL` to
a remote server URL got the embedded catalog and no error.

The Compose example carried the same gap:

```console
$ git show fb328d0a:docker-compose.yml | grep -c STARMAP_
0
$ git show fb328d0a:.env.example | grep -c STARMAP_
0
```

The stack sent no source request. It served the embedded catalog forever.

The plan-wide capture in `docs/proof/catalog-publisher/cat2-fail-before.txt`
records the three named CAT6 conditions:

```text
FAIL CAT-V27 the catalog payload handler resets a write deadline before each chunk.
FAIL CAT-V28 the SSE frame write deadline defaults to two minutes.
FAIL CAT-V29 the admin update returns an accepted asynchronous operation.
```

All three pass now.

## The settings map

`internal/catalog/settings` owns the table. Each row names one setting, one
kebab-case flag, and one runtime option. No other catalog setting name exists.

| Setting | Flag | Runtime option |
| --- | --- | --- |
| `STARMAP_CATALOG_SOURCE` | `--catalog-source` | `WithCatalogSource` |
| `STARMAP_CATALOG_SOURCE_URL` | `--catalog-source-url` | `WithSourceURL` |
| `STARMAP_CATALOG_SOURCE_API_KEY` | `--catalog-source-api-key` | `WithSourceAPIKey` |
| `STARMAP_CATALOG_SOURCE_REPOSITORY` | `--catalog-source-repository` | `WithSourceRepository` |
| `STARMAP_CATALOG_SOURCE_CHANNEL` | `--catalog-source-channel` | `WithSourceChannel` |
| `STARMAP_CATALOG_SOURCE_SIGNER_WORKFLOW` | `--catalog-source-signer-workflow` | `WithSourceSignerWorkflow` |
| `STARMAP_CATALOG_SOURCE_TOKEN` | `--catalog-source-token` | `WithSourceToken` |
| `STARMAP_CATALOG_SOURCE_POLL_INTERVAL` | `--catalog-source-poll-interval` | `WithSourcePollInterval` |
| `STARMAP_CATALOG_SOURCE_STARTUP_POLICY` | `--catalog-source-startup-policy` | `WithSourceStartupPolicy` |
| `STARMAP_CATALOG_SOURCE_MAX_AGE` | `--catalog-source-max-age` | `WithSourceMaxAge` |
| `STARMAP_CATALOG_SOURCE_MAX_HOPS` | `--catalog-source-max-hops` | `WithSourceMaxHops` |
| `STARMAP_CATALOG_ACQUISITION_ENABLED` | `--catalog-acquisition-enabled` | `WithAcquisitionEnabled` |
| `STARMAP_CATALOG_ACQUISITION_INTERVAL` | `--catalog-acquisition-interval` | `WithAcquisitionInterval` |
| `STARMAP_CATALOG_COALESCE_WINDOW` | `--catalog-coalesce-window` | `WithCoalesceWindow` |
| `STARMAP_CATALOG_WORKSPACE_PATH` | `--catalog-workspace-path` | `WithCatalogPath` |
| `STARMAP_CATALOG_STARTUP_SPREAD` | `--catalog-startup-spread` | `WithStartupSpread` |
| `STARMAP_CATALOG_TRANSFER_IDLE_TIMEOUT` | `--catalog-transfer-idle-timeout` | `WithTransferIdleTimeout` |
| `STARMAP_CATALOG_TRANSFER_MAX_DURATION` | `--catalog-transfer-max-duration` | `WithTransferMaxDuration` |
| `STARMAP_CATALOG_REFRESH_TIMEOUT` | `--catalog-refresh-timeout` | `WithRefreshTimeout` |
| `STARMAP_STATE_DIR` | `--state-dir` | `WithStateDirectory` |
| `STARMAP_SCHEDULER_IDENTITY` | `--scheduler-identity` | `WithSchedulerIdentity` |

The default source is `public`. A configured `starmap`, `github`, `file`, or
`embedded` source replaces it. The parser accepts
`STARMAP_CATALOG_SOURCE=starmap` with `STARMAP_CATALOG_SOURCE_URL`. Only
`Composition.Open` rejects that source. It returns a typed `ConfigError` that
names the CAT7 handoff:

```text
configuration error in catalog source: the starmap source kind is not yet available in this build
```

`starmap update` stays the manual acquisition command. It runs against the
client of the connected runtime, so a manual run reaches the same `Sync` and
`Refresh` paths that the scheduler reaches. The brief also named a `starmap
sync` spelling. `TestRootCommandHasOneCanonicalPublicSpelling` forbids a command
alias, so this task kept the one canonical spelling and dropped the second one.

## Tests

| Test | Rule |
| --- | --- |
| `TestEveryCanonicalNameMapsToOption` | every name selects one runtime option |
| `TestFlagsMirrorCanonicalNames` | each flag is the kebab-case name |
| `TestFlagOverridesEnvironment` | a flag wins over the environment |
| `TestDefaultSourceIsPublic` | the default source is public |
| `TestConfiguredSourceReplacesTheDefault` | a configured source replaces the default |
| `TestStarmapSourceParsesAndCompositionRejectsIt` | the parser accepts the starmap source and composition rejects it |
| `TestInvalidValueReturnsTypedError` | an invalid value returns a typed error |
| `TestNilLookupIsRejected` | a nil lookup returns a typed error |
| `TestApplicationPullsSyntheticChannelAndRetainsState` | the package pulls a synthetic channel, detects eligible credentials, and retains state |
| `TestServerServesEmbeddedStateThenPullsChannel` | the server serves first, reports runtime readiness, and joins the runtime on shutdown |
| `TestComposeExampleParsesAndPullsThePublicChannel` | the Compose example parses and sets no catalog setting |
| `TestDeploymentFilesDocumentEveryCanonicalName` | both deployment files name every canonical setting and no other |
| `TestCatalogPayloadResetsWriteDeadlinePerChunk` | CAT-V27 |
| `TestSSEFrameWriteDeadlineDefaultsToTwoMinutes` | CAT-V28 |
| `TestAdminUpdateReturnsAcceptedOperation` | CAT-V29 |
| `TestRouteTimeoutsRunBeforeTheHandler` | split route timeouts sit in middleware |
| `TestRegistryReportsProgressAndCompletion` | a status read reports progress and completion |
| `TestRegistryReportsBoundedFailureReason` | sanitization runs before serialization |
| `TestRegistryCancelsALiveOperation` | a client can cancel |
| `TestRegistryEvictsTerminalHistoryOnly` | eviction keeps a live status readable |
| `TestRegistryMetricsUseBoundedLabels` | metrics carry only closed-set labels |
| `TestRegistryCloseCancelsLiveOperations` | shutdown ends live operations |
| `TestRegistryStampsTheInjectedClock` | the registry reads an injected clock |
| `TestRegistryRejectsAnUnknownKind` | an unknown kind returns a typed error |

`internal/test/channel` owns the synthetic upstream. It serves a channel
document and a provider model list. It counts channel reads and model reads, so
a test proves that a provider without a credential sent no request. Every test
runs with `-race` and passes with `HTTPS_PROXY=127.0.0.1:1`.

## Mutation evidence

Each row names one source change and the test that caught it. Each run
restored its file afterward.

| Mutation | Result |
| --- | --- |
| default source becomes `embedded` | `TestDefaultSourceIsPublic`: `default source = "embedded", want "public"` |
| the source URL flag becomes `catalog-source-uri` | `TestFlagsMirrorCanonicalNames`: `flag for STARMAP_CATALOG_SOURCE_URL = "catalog-source-uri"` |
| the payload handler sets one deadline for the whole body | `TestCatalogPayloadResetsWriteDeadlinePerChunk`: `write deadlines = 1, want one per chunk (127)` |
| the SSE write deadline default becomes 30 seconds | `TestSSEFrameWriteDeadlineDefaultsToTwoMinutes`: `DefaultWriteTimeout = 30s, want 2m0s` |
| the admin update answers 200 instead of 202 | `TestAdminUpdateReturnsAcceptedOperation`: `status = 200, want 202` |
| the operation reason carries the raw error | `TestRegistryReportsBoundedFailureReason` and `TestRegistryMetricsUseBoundedLabels` both fail on the leaked text |
| the provider preflight stops gating on the credential | `TestApplicationPullsSyntheticChannelAndRetainsState`: `unconfigured outcome = "failed", want "skipped_not_configured"` |
| the server shutdown skips the runtime close | `TestServerServesEmbeddedStateThenPullsChannel`: `the server shutdown did not close the runtime` |
| the Compose service sets `STARMAP_CATALOG_SOURCE` | `TestComposeExampleParsesAndPullsThePublicChannel`: `the running service sets "STARMAP_CATALOG_SOURCE"` |

## Two defects that CAT6 found and repaired

### The retained provider layer failed to decode

`layerSet.build` decoded each retained provider layer with
`catalogs.DecodeCatalogPayload`. That decoder demands resolved canonical
authorship inside one payload. A provider layer holds serving records that name
an authored model of the baseline, so the layer alone resolves no authorship.
The application test failed on the first sync:

```text
Sync: failed to decode retained provider layer provider-configured: failed to
index catalog read views: authored model with ID test-author/model-observed not
found
```

`runtime_layers.go` now calls `catalogs.DecodeSourceObservationPayload`. That
decoder reads a source candidate without the authorship demand. The final
`builder.Build` still validates the merged result, so the contract holds.
Reverting the one line reproduces the failure above.

### The Compose state volume arrived owned by root

The Compose example mounted the state volume at `/home/nonroot/.starmap`. That
directory does not exist in the digest-pinned static base. Docker creates a
missing mount point as `root:root`, so the unprivileged process could not write
it. The container smoke check failed:

```text
opening the catalog runtime: IO error during create of
/home/nonroot/.starmap/state/runtime/catalog-runtime: mkdir
/home/nonroot/.starmap/state: permission denied
```

The volume now mounts at `/home/nonroot`. That directory exists in the base and
user 65532 owns it, so Docker copies the ownership onto the volume. The catalog
workspace path and the state directory both sit under the mount.

## Commands

| Command | Result |
| --- | --- |
| `make lint` | pass |
| `make test` | pass |
| `go tool ago -stale-ignores -format json ./...` | no finding |
| `make technical-writing-check` | pass |
| `bash scripts/verify-catalog-package-ownership.sh` | pass |
| `make docs-check` | pass |
| `shellcheck scripts/*.sh` | pass |
| `bash scripts/verify-catalog-distribution.sh` | `Summary: 39 passed, 10 failed, 19 unverified.` |
| `bash scripts/verify-container-smoke.sh` | `PASS the image serves with a read-only root and a writable state volume.` |

CAT-V27, CAT-V28, and CAT-V29 all report PASS. Ten conditions still fail. The
run above lists them, and each one belongs to a later task. CAT7 owns CAT-V25,
CAT-V32 through CAT-V36, CAT-V38, and CAT-V65. CAT9 owns the other two failures.
Nineteen conditions stay unverified because this repository holds no Starport
tree.

The container smoke check needs Docker. A host without Docker gets this report
and the exact command to run later:

```console
$ bash scripts/verify-container-smoke.sh
UNVERIFIED the container smoke check needs Docker.
Run this exact command on a host with Docker: bash scripts/verify-container-smoke.sh
```

This run had Docker, so the check reports PASS.
