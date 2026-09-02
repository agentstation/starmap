# CAT5 proof: catalog-source runtime

CAT5 owns the connected runtime in the root package. It owns the durable
retained layers, the runtime lease, and the refresh scheduler. It also owns
the provider and credential outcomes, the scoped evidence, the source and
acquisition policies, the runtime status, and the consumer fixtures. Two
rules sit outside the root package. They are partial-failure publication in
`acquisition` and the missing-credential skip in `internal/sources/providers`.

CLI, server, and Docker composition stay with CAT6. The Starmap cascade stays
with CAT7. Starport stays with CAT8. Operator documentation stays with CAT9.

## Fail before

`docs/proof/catalog-publisher/cat2-fail-before.txt` holds the plan-wide capture
on the base commit. The twenty CAT5 conditions read:

```text
FAIL CAT-V08 Open returns a connected runtime with embedded state before any network reply.
FAIL CAT-V09 the offline constructors reject runtime options.
FAIL CAT-V10 Catalog, State, and Status reads reach no external system.
FAIL CAT-V11 Refresh, RefreshSource, and Sync change distinct layers.
FAIL CAT-V12 Sync returns an AcquisitionReport.
FAIL CAT-V13 Close joins runtime-owned work within five seconds.
FAIL CAT-V14 acquisition policy is enabled plus interval with no mode or on-start name.
FAIL CAT-V15 source selection accepts public, github, starmap, file, and embedded.
FAIL CAT-V16 a custom source never falls back to public GitHub.
FAIL CAT-V17 a missing acquisition credential skips the provider without a request.
FAIL CAT-V18 the runtime retains layers and rebuilds the effective catalog under race.
FAIL CAT-V19 refresh runs are single-flight with run identity and cancellation.
FAIL CAT-V22 Refresh adds no deadline by default.
FAIL CAT-V49 the runtime lease fences the durable generation commit with its epoch.
FAIL CAT-V60 the runtime status reports usability, freshness, fallback, direct source health, and upstream-reported health as independent values.
FAIL CAT-V61 a partial provider failure publishes the succeeded layers and the failed provider retains its own last-known-good observation.
FAIL CAT-V66 completed provider observations publish through one bounded coalescing window while another provider stays blocked.
```

Every one of those conditions passes now. The distribution verifier reports
`Summary: 36 passed, 13 failed, 19 unverified`. The remaining failures belong to
CAT6, CAT7, and CAT9.

## Files

| Path | Role |
| --- | --- |
| `runtime.go` | `Open`, the public methods, publication, and `Close` |
| `runtime_options.go` | Every runtime option and the canonical setting names |
| `runtime_policy.go` | The source policy, the acquisition policy, and validation |
| `runtime_source.go` | Terminal source selection and the file source |
| `runtime_layers.go` | The four layers, the durable store, and the rebuild |
| `runtime_refresh.go` | Single-flight runs, reports, and provider observation |
| `runtime_scheduler.go` | The instance identity, the phases, and the loops |
| `runtime_lease.go` | The lease contract, renewal, and the commit fence |
| `runtime_status.go` | Usability, freshness, fallback, and the two health values |
| `acquisition/acquirer.go` | Partial-failure publication and the bounded window |
| `internal/sources/providers/attempts.go` | The pre-flight credential check and attempts |
| `pkg/sources/outcome.go` | Provider outcomes, safe reason codes, and attempts |

## Layer design

The runtime holds four layers. Each layer has one owner, and the rebuild reads
them in one order.

| Layer | Source | Durable path |
| --- | --- | --- |
| Embedded baseline | The compiled catalog | The binary |
| Upstream source | The selected catalog source | `<state>/catalog-runtime/source.json` |
| Provider observations | One layer for each provider | `<state>/catalog-runtime/providers/<id>.json` |
| Effective catalog | The rebuild result | The published generation |

`layerSet.build` starts from the embedded baseline. The upstream source layer
replaces the baseline. Each provider layer then enriches the result in stable
identity order. A provider layer that no run replaced stays in place, so one
failed provider keeps its last-known-good records.

The rebuild merges provider records with `MergeEnrichEmpty`. That strategy
merges providers and authors, and it does not merge authored models. The
rebuild therefore calls `mergeAuthoredModels`, which adds the authored records
that a provider layer needs. The rebuilt catalog then resolves every model
reference.

`STARMAP_STATE_DIR` names the state directory. Each layer write goes to a
temporary file and then renames, so a restart reads a complete layer or none.

## Lease contract

| Property | Value |
| --- | --- |
| Time to live | 90 s (`LeaseTTL`) |
| Renewal interval | 30 s (`LeaseRenewInterval`) |
| Fence | The epoch of the acquisition that the run started under |
| Loss | The keeper cancels every runtime-owned run |

A deployment without a lease store commits without a fence, and `Status`
reports `lease_not_required`. A deployment with a lease store takes the lease
at `Open`. Each run captures the current epoch when it starts. The durable
commit calls `leaseKeeper.fence` with that epoch inside the update
transaction. A stale epoch and a lost lease both return an
`*errors.ConflictError`, so two instances never overwrite one another.

## Scheduler contract

| Property | Value |
| --- | --- |
| Startup spread | 15 min (`fleet.DefaultStartupSpread`) |
| Phase | `fleet.StablePhase` for each controller |
| Identity | `sha256(seed, host, listen address)`, first 16 hexadecimal characters |
| Seed | `<state>/catalog-runtime/instance-seed` |

The seed is durable, so one instance keeps its phase across a restart. The host
name and the listen address join the seed, so a copied state directory produces
two identities instead of one. The source controller and the acquisition
controller derive separate phases from one identity.

## Refresh runs

`Refresh`, `RefreshSource`, and `Sync` share one run group. A second caller of
the same kind joins the run in flight and reads the same report, so the run
identity is one value. The run belongs to the runtime. Caller cancellation
stops the run, and a caller deadline adds no deadline of its own. The
configured refresh timeout is the only deadline a run carries, and it is unset
by default.

A provider that does not answer inside the bounded coalescing window keeps its
retained layer. The window is 30 seconds by default, and it opens on the first
answer. The providers that answered publish one generation, and the report
names the retained providers.

A missing catalog credential skips the provider before any request. The attempt
carries the `skipped_not_configured` outcome and a safe reason code. The
pre-flight check and the fetch share one credential memo, so each run resolves
the material of one provider one time.

## Status

`RuntimeStatus` reports five independent values.

| Value | Meaning |
| --- | --- |
| `Usable` | The runtime serves a catalog |
| `Freshness` | The age of the published generation |
| `Fallback` | The runtime serves the embedded baseline |
| `SourceHealth` | The health of the direct transfer |
| `UpstreamHealth` | The health that the upstream reported |

A failed source leaves the runtime usable and reports the embedded fallback. A
healthy transfer of a degraded upstream reports `ok` and `degraded` together.
Time alone moves freshness, and it moves neither health value nor the fallback.

## Tests

| Test | Condition |
| --- | --- |
| `TestOpenReturnsEmbeddedStateBeforeSourceReply` | CAT-V08 |
| `TestNewRejectsRuntimeOptions` | CAT-V09 |
| `TestRuntimeReadsReachNoExternalSystem` | CAT-V10 |
| `TestRuntimeRefreshMethodsChangeDistinctLayers` | CAT-V11 |
| `TestRuntimeSyncReturnsAcquisitionReport` | CAT-V12 |
| `TestRuntimeCloseJoinsWithinFiveSeconds` | CAT-V13 |
| `TestAcquisitionPolicyFromEnabledAndInterval` | CAT-V14 |
| `TestSourceSelectionNames` | CAT-V15 |
| `TestCustomSourceNeverFallsBackToPublic` | CAT-V16 |
| `TestMissingCredentialSkipsProviderWithoutRequest` | CAT-V17 |
| `TestSkippedProviderDegradesObservationWithSafeReason` | CAT-V17 |
| `TestRuntimeRetainsLayersUnderRace` | CAT-V18 |
| `TestRefreshJoinsActiveRunAndCancels` | CAT-V19 |
| `TestRefreshAddsNoDeadlineByDefault` | CAT-V22 |
| `TestRuntimeLeaseRejectsStaleEpochAtCommit` | CAT-V49 |
| `TestRuntimeLeaseFenceRejectsALostLease` | CAT-V49 |
| `TestRuntimeStatusKeepsUsabilityFreshnessFallbackAndHealthIndependent` | CAT-V60 |
| `TestSyncPartialFailurePublishesAndRetainsProviderLastKnownGood` | CAT-V61 |
| `TestSyncPublishesCompletedProvidersWhileAnotherBlocked` | CAT-V66 |
| `TestStartupSpreadDefaultsToFifteenMinutes` | The startup spread default |
| `TestStablePhaseSurvivesRestart` | The durable phase |
| `TestSchedulerIdentityDivergesAcrossClonedState` | The cloned state directory |

Every test is hermetic. `WithClock`, `WithRandom`, `WithSource`,
`WithAcquirer`, `WithLeaseStore`, and `WithAcquirerCoalesceTimer` inject every
dependency that a test needs to control. No test sleeps for a real interval,
and no test reaches a network. The whole suite passes behind a dead proxy.

## Mutation evidence

This record disables each rule below in turn. The named test then fails, and
the rule returns.

| Disabled rule | Failing test | Reported failure |
| --- | --- | --- |
| The lease fence in `Runtime.commit` | `TestRuntimeLeaseRejectsStaleEpochAtCommit` | `RefreshSource published under a stale lease epoch` |
| The pre-flight credential check | `TestMissingCredentialSkipsProviderWithoutRequest` | `outcome = "failed", want "skipped_not_configured"` |
| The terminal cascade selection | `TestCustomSourceNeverFallsBackToPublic` | `Open selected the fallback source "starmap_cascade"` |
| The retained provider layers | `TestSyncPartialFailurePublishesAndRetainsProviderLastKnownGood` | `the catalog holds no provider "beta"` |
| The durable instance seed | `TestStablePhaseSurvivesRestart` | the identity and both phases moved across the restart |
| The bounded coalescing window | `TestSyncPublishesCompletedProvidersWhileAnotherBlocked` | `test timed out after 1m0s` |

## Commands

Every command ran with `GOTOOLCHAIN=go1.26.6`.

| Command | Result |
| --- | --- |
| `make lint` | PASS |
| `make test` | PASS |
| `go tool ago -stale-ignores -format json ./...` | PASS, zero findings and zero stale ignores |
| `make technical-writing-check` | PASS, 721 files and zero diagnostics |
| `bash scripts/verify-catalog-package-ownership.sh` | PASS, 13 passed and 0 failed |
| `make godoc` | PASS |
| `make docs-check` | PASS |
| `shellcheck scripts/*.sh` | PASS |
| `bash scripts/verify-catalog-distribution.sh` | 36 passed, 13 failed, 19 unverified |
| `HTTPS_PROXY=127.0.0.1:1 go test -race .` and three more packages | PASS |

## Decisions for the orchestrator

### The root package now holds the Sigstore verification stack

`runtime_source.go` builds the attested GitHub source, so the root package
imports `internal/sources/github` and its attestation engine. Every consumer of
`github.com/agentstation/starmap` therefore records those modules. The six
consumer fixtures under `testdata/consumers` carry the tidied requirements, and
`scripts/verify-catalog-package-ownership.sh` passes again.

The alternative is a driver registry. The root would then name each source kind
and reject a kind that no imported package supplies. That choice moves the
attested source out of the library graph, and it moves the failure from build
time to start time. The plan asks `Open` to select the source by name, so this
record keeps the direct import.

### The acquirer is an injected role

`acquisition.NewAcquirer` supplies the concrete provider observation, and the
runtime holds the `Acquirer` interface. The root package therefore imports no
provider client. A deployment that never observes providers imports neither the
acquirer nor any provider SDK.

### The interface method names carry a suffix

`LeaseStore.AcquireLease` and `Acquirer.AcquireProviders` name the object they
act on. The bare verb is a banned word in the strict writing mode, so the
suffix keeps both the godoc convention and the prose gate.

### The runtime reads no environment variable

Every setting arrives through an option. The option names map onto the
canonical `CATALOG_SOURCE_*`, `CATALOG_ACQUISITION_*`, `CATALOG_WORKSPACE_PATH`,
`CATALOG_STARTUP_SPREAD`, `CATALOG_TRANSFER_*`, and `CATALOG_REFRESH_TIMEOUT`
names. CAT6 owns the parsing of those names.

### The lease renewal uses a resetting timer

`TestRootClientExposesNoCadenceLifecycle` rejects `time.NewTicker` in the root
package. The lease keeper renews on a timer that it resets after each renewal,
so the cadence rule and the lease contract hold together.
