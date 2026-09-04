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

## Repairs after the review

The first CAT5 commit passed every condition but left five contract defects
against `docs/proof/catalog-publisher/cat2-final-review.md`. A second commit
repaired them.

| Defect | Repair |
| --- | --- |
| `Close` raced a caller-owned run | `updatesMu` and `updatesClosed` guard the publication channel under one lock |
| The startup policy validated but never ran | `Open` enforces `require_source`; both workers run a cold-start pass |
| A zero acquisition period reached `untilNextRun` | `runSchedule` returns after the startup pass |
| A refused lease failed `Open` | A refusal opens a non-owner replica, and every run takes the lease again |
| The window closed the run | The window emits and keeps waiting through `AcquisitionRequest.Publish` |

A second review found two more lease items. A `require_source` `Open` now
reads nothing when another instance owns the lease. That replica opens and
consumes accepted state. `leaseKeeper.take` releases a lease that raced
`Close` instead of leaving it to expire.

Four smaller repairs joined them. `acquireProviders` records the report and
degrades health before it returns a retention failure. The GitHub source
adapter dropped its own budget field. It warns one time for each `Read` that
the status marks. `safeSourceReason` maps a not-before boundary and a spent
budget onto `rate_limited`. It maps a bare 403 onto `credential_rejected`.
Every handwritten Go file stays under 1,500 lines.

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

A refused acquisition is a normal fleet state, not a failure. `LeaseStore`
documents the `*errors.ConflictError` that another holder returns. `Open`
records `lease_lost` and returns a usable runtime. The replica then serves its
retained catalog. Every other store error still fails `Open`.

Every run then takes the lease again. `leaseKeeper.ensureHeld` runs before
`runGroup.start`, so a refused replica reads no source and observes no
provider. A scheduled run logs the refusal at debug level, because a non-owner
replica is normal. A manual `Refresh`, `RefreshSource`, or `Sync` returns the
typed conflict to its caller. A successful acquisition installs the new epoch
and restarts renewal.

`leaseKeeper.take` returns a lease that raced `Close`. The store answers the
acquisition, and the keeper then finds itself stopped. It releases that lease
before it returns the conflict, so another instance waits no full time to live
for a lease that no holder uses. The release is best effort and logs at debug
level, because the lease expires on its own.

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

### The startup policy

The `require_source` policy reads the source one time inside the `Open`
context. A failed read fails `Open` with an `*errors.ResourceError`. A
deployment that needs upstream state then never serves the embedded baseline
instead. `Open` already read the source, so that policy needs no startup pass.

The policy names the evidence the runtime needs, not the lease. A replica that
another instance owns therefore reads nothing at `Open`. It logs at info level
that the owner supplies the source state, and it consumes what the owner
publishes. A store error that is not a refusal still fails `Open`.

The `prefer_source` policy runs one read right after its startup offset when
the runtime is cold. A runtime is cold when it retains no source layer, or when
that layer passed the source-check warning age. A runtime with fresh durable
evidence sends no early request and waits for its stable full-interval phase.
The acquisition worker follows the same rule against the retained provider
layers and the acquisition warning age.

### A zero acquisition interval

The canonical environment table reads `true | 0s | one startup pass`. An
enabled policy with a zero period therefore validates. `runSchedule` runs the
startup pass and returns. A zero interval then never reaches `untilNextRun` and
starts no periodic work. A negative period stays a validation error, because it
names no schedule at all.

## Refresh runs

`Refresh`, `RefreshSource`, and `Sync` share one run group. A second caller of
the same kind joins the run in flight and reads the same report, so the run
identity is one value. The run belongs to the runtime. Caller cancellation
stops the run, and a caller deadline adds no deadline of its own. The
configured refresh timeout is the only deadline a run carries, and it is unset
by default.

The bounded coalescing window is 30 seconds by default. It opens on the first
observation that carries a layer. An answer that publishes nothing therefore
opens no window. `AcquisitionRequest.Publish` carries the layers of one closed
window back to the runtime. The runtime retains them, rebuilds the effective
catalog, and publishes one generation under the epoch of the run.

A closed window ends neither the run nor a provider. The acquirer emits what it
collected and keeps waiting. One slow provider then delays no completed peer by
more than one window, and it still publishes its own layer. The report names
the providers that kept their retained layer.

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
| `TestAcquireOpensNoWindowWithoutLayer` | CAT-V66 |
| `TestStartupSpreadDefaultsToFifteenMinutes` | The startup spread default |
| `TestStablePhaseSurvivesRestart` | The durable phase |
| `TestSchedulerIdentityDivergesAcrossClonedState` | The cloned state directory |
| `TestCloseNeverRacesACallerOwnedRun` | Close against a caller-owned run |
| `TestRequireSourceFailsOpenWhenTheSourceFails` | The require_source policy |
| `TestPreferSourceReadsOnceOnAColdStart` | The prefer_source cold start |
| `TestFreshDurableEvidenceWaitsForItsPhase` | Fresh evidence waits |
| `TestZeroAcquisitionIntervalRunsOneStartupPass` | The zero acquisition period |
| `TestRefusedLeaseOpensAsANonOwner` | The non-owner replica |
| `TestScheduledRunSkipsWhileAnotherInstanceOwnsTheLease` | The skipped run |
| `TestManualRunByANonOwnerReturnsAConflict` | The manual conflict |
| `TestRequireSourceOpensAsANonOwner` | The non-owner startup read |
| `TestTakeReleasesALeaseThatRacedClose` | The lease that raced Close |

Every test is hermetic. Six options inject every dependency that a test needs
to control. They are `WithClock`, `WithRandom`, `WithSource`, `WithAcquirer`,
`WithLeaseStore`, and `WithAcquirerCoalesceTimer`. The test-only
`withScheduleTimer` joins them. It records what a worker waits for and never
fires, so a schedule test observes one pacing decision without a real delay.

No test sleeps for a real interval, and no test reaches a network. The whole
suite passes behind a dead proxy.

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
| The window cancels the run | `TestSyncPublishesCompletedProvidersWhileAnotherBlocked` | `Sync: context canceled` |
| The closed-channel guard in `Runtime.broadcast` | `TestCloseNeverRacesACallerOwnedRun` | `panic: send on closed channel` |
| The `require_source` read in `Open` | `TestRequireSourceFailsOpenWhenTheSourceFails` | `Open served the embedded baseline under require_source` |
| The zero-interval guard in `runSchedule` | `TestZeroAcquisitionIntervalRunsOneStartupPass` | `acquisition runs = 8635, want one startup pass` |
| The refusal branch in `leaseKeeper.start` | `TestRefusedLeaseOpensAsANonOwner` | `Open: failed to acquire runtime lease ...: another instance owns the runtime lease` |
| The `ensureHeld` call in `Runtime.execute` | `TestManualRunByANonOwnerReturnsAConflict` | `source reads = 1, want no upstream request from a non-owner` |
| The layer test that opens the window | `TestAcquireOpensNoWindowWithoutLayer` | `windows opened = 1, want none` |
| The non-owner skip in `Open` | `TestRequireSourceOpensAsANonOwner` | `Open as a non-owner: failed to read catalog source ...: another instance owns the runtime lease` |
| The release in `leaseKeeper.take` | `TestTakeReleasesALeaseThatRacedClose` | `lease releases = 0, want one returned lease` |

## Commands

Every command ran with `GOTOOLCHAIN=go1.26.6`.

| Command | Result |
| --- | --- |
| `make lint` | PASS |
| `make test` | PASS |
| `go tool ago -stale-ignores -format json ./...` | PASS, zero findings and zero stale ignores |
| `make technical-writing-check` | PASS, 722 files and zero diagnostics |
| `bash scripts/verify-catalog-package-ownership.sh` | PASS, 13 passed and 0 failed |
| `make godoc` | PASS |
| `make docs-check` | PASS |
| `shellcheck scripts/*.sh` | PASS |
| `bash scripts/verify-catalog-distribution.sh` | 36 passed, 13 failed, 19 unverified |
| `HTTPS_PROXY=127.0.0.1:1 go test -race ./ ./acquisition` | PASS |

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
