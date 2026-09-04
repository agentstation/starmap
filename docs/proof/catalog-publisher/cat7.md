# CAT7 proof: Starmap source cascading

CAT7 owns the cascade. A Starmap server reads another Starmap server as its
catalog source. The source-chain manifest travels down every hop, and the
origin channel time grades freshness at each node below the origin.

CAT5 owns the runtime core. CAT6 owns the application composition. CAT8 owns
Starport. CAT9 owns the operator documentation.

## Fail before

The base commit `31cb2ec3` composed no cascade. The `starmap` source kind
parsed and then failed:

```console
$ git show 31cb2ec3:internal/catalog/settings/composition.go | sed -n '49,55p'
	if c.Config.SourceKind == starmap.SourceStarmap && c.Source == nil {
		return nil, &errors.ConfigError{
			Component: "catalog source",
			Message: "the starmap catalog source is not yet available; " +
				"select public, github, file, or embedded",
		}
	}
```

A server start therefore owned no upstream subscriber. The server also served
no chain, so a downstream had nothing to read:

```console
$ git grep -l "source-chain" 31cb2ec3 -- 'internal/server/*'
$ git grep -l "remote.NewSource" 31cb2ec3 -- 'internal/*' 'server/*'
```

Both commands print nothing and exit 1.

`docs/proof/catalog-publisher/cat6.md` records the same gap from the verifier
side. That run reported `Summary: 39 passed, 10 failed, 19 unverified.` and
named CAT-V25, CAT-V32 through CAT-V36, CAT-V38, and CAT-V65 as the CAT7
failures. The CAT6 record quotes the rejection message above at line 84. CAT7
replaces that message, so the quotation is now historical.

## What CAT7 built

`remote.NewSource` adapts the subscriber as a `starmap.Source`. The subscriber
opens its event stream under the transfer response-header bound. It reconnects
with decorrelated jitter from one second to fifteen minutes. It resets that
delay only after a healthy liveness window. It honors `Retry-After` as a hard
not-before, and it waits for a credential change after an authentication
failure. It falls back to conditional polling at the stable phase of its
identity.

`internal/catalog/settings.Composition.cascadeSource` builds that source from
the canonical settings. The root package cannot build it, because the
subscriber imports the root package, so the composition step is the one place
that joins them. The subscriber keeps its verified generations in memory,
because the runtime already retains the accepted generation in its own state
directory.

`Composition` reads `STARMAP_CATALOG_SOURCE_URL`, `..._API_KEY`,
`..._MAX_HOPS`, `..._MAX_AGE`, `..._ALIASES`, and the scheduler identity. A
`starmap` source with no URL returns a `ConfigError` that names the missing
setting.

## Manifest format

`GET /api/v1/catalog/source-chain` serves
`application/vnd.agentstation.starmap.source-chain+json` with `Cache-Control:
no-store`. The document is `pkg/catalogs/remote.SourceChain`:

```json
{
  "schema_version": 1,
  "identity": "middle",
  "health": "ok",
  "upstream_health": "ok",
  "source_identity": "starmap_cascade",
  "generation_id": "0199e0d1-6a3f-7c4a-9a1f-2c1f4f0c9a10",
  "channel_updated_at": "2026-09-01T00:00:00Z",
  "observed_at": "2026-09-01T12:00:00Z",
  "hops": [{"identity": "origin", "health": "ok",
            "published_at": "2026-09-01T00:00:00Z",
            "observed_at": "2026-09-01T11:59:00Z"}]
}
```

The document carries safe identities and bounded detail only. It names no URL,
no host, no credential, and no operator message.

`Validate` rejects another schema version, an identity longer than 128 bytes,
an identity with a control character, and more than 16 hops. It also rejects
any health value outside the closed set `unknown`, `ok`, `degraded`, and
`unavailable`. The serving node truncates its own hop list to the same bound.
One node therefore cannot grow a document without limit by forwarding every
hop it received.

`health` is what the serving node observed while it read its own source.
`upstream_health` is what its upstream reported about itself. The two values
stay independent. A healthy transfer of a degraded upstream catalog still
reports a degraded upstream.

## Cycle rejection rules

`acceptSourceChain` runs before the runtime retains an upstream generation.
URL comparison cannot find a loop, because a load balancer, a DNS name, and a
proxy all reach one node under different addresses. The check therefore runs
on the propagated identities. The rules are four:

1. A chain that names this instance is a self reference. The runtime returns a
   `ConflictError` whose `Actual` is the offending identity.
2. A chain that names a declared alias of this instance (`WithSourceAliases`,
   `STARMAP_CATALOG_SOURCE_ALIASES`) is the same self reference under another
   name.
3. A chain that repeats any identity closes a cycle between two or more nodes.
4. A chain longer than the hop budget returns a `ValidationError`, because an
   unbounded cascade multiplies the origin latency.

A refused read leaves the runtime on its last good generation. The runtime
then reports `source_health = unavailable` with the reason `chain_rejected`,
and it keeps the fallback flag.

## Freshness across hops

Every hop passes `channel_updated_at` through unchanged, and every hop grades
that value instead of its own last check. The runtime keeps the propagated
time in `RuntimeStatus.ChannelUpdatedAt` and grades it with the channel
thresholds. It grades its own check separately as `source_check_freshness`.
Readiness publishes `channel_freshness`, `channel_age_seconds`,
`source_check_freshness`, `instance_identity`, and `chain_hops`.

Local acquisition on a hop changes the served bytes. The hop then derives a
new effective generation identity from the upstream identity and the served
digest (`upstream+local.<12 digest characters>`). It never reuses the upstream
identity. Two different catalogs under one identity would let a downstream
treat them as one generation.

## Repairs after review

The first review found three defects. Each repair carries its own test.

The cascade now wakes the runtime. An optional source role,
`starmap.SourceWatcher`, reports one wake for each upstream change through
`Changes`. The cascaded source fills that role from the subscriber stream.

The source worker in `runtime_scheduler.go` selects on the wake channel and on
its interval boundary. It runs the same refresh work under the same coalesce
window. A closed channel drops the worker back onto its poll interval. A
streamed delta crosses one hop in seconds. The earlier end-to-end test hid the
gap, because it called `RefreshSource` in a loop. That test now reads status
alone and calls no refresh.

The composed subscriber carries the canonical bounds. Its settings keep the
transfer idle timeout, the transfer maximum duration, and the startup spread.
The composition step maps all three onto the subscriber config. A second
optional role, `starmap.SourceIdentityAdopter`, takes the derived instance
identity. Open hands that identity to the source right after it initializes
the schedule. A deployment that configures no identity still spreads its
cascade on the identity of its runtime.

A refused start is no longer sticky. The source held a terminal error under
`sync.Once`. One 401 therefore disabled the source for the life of the
process. The start now runs under a mutex and keeps no error. A later read
opens the lifecycle again, so the runtime recovers after a key rotation.

The review named three cheap items too. This repair closed two of them. The
cascaded read drops the dead `ErrNotFound` branch, because a served status
always arrives as an `APIError`. The served chain now warns when it names
fewer hops than the runtime observed.

The third item stays open. The refusal helper reads the wall clock inside
`pkg/catalogs/remote`. That package holds no clock, so a test clock there
needs a wider change than this repair.

## Tests

| Test | Condition |
| --- | --- |
| `remote.TestSubscriberOpenHonorsResponseHeaderTimeout` | CAT-V25 |
| `remote.TestSubscriberReconnectDelayCapsAtFifteenMinutes` | CAT-V32 |
| `remote.TestSubscriberResetsBackoffAfterHealthyWindow` | CAT-V33 |
| `remote.TestSubscriberHonorsRetryAfterNotBefore` | CAT-V34 |
| `remote.TestSubscriberWaitsForCredentialChange` | CAT-V35 |
| `remote.TestFallbackPollingUsesStablePhase` | CAT-V36 |
| `sse.TestSourceAdmissionReturnsRetryAfter` | CAT-V38 |
| `sse.TestAdmissionRetryAfterStaysJitteredAndWhole` | CAT-V38 |
| `server.TestCascadedFreshnessPropagatesChannelUpdatedAtThroughHops` | CAT-V65 |
| `server_test.TestServerCascadesVerifiedCatalogSource` | the whole cascade |
| `starmap.TestRuntimeRefusesSelfAliasAndCyclicSourceChains` | chain rules 1 to 3 |
| `starmap.TestRuntimeRefusesAnOverlongSourceChain` | chain rule 4 |
| `starmap.TestRuntimeGradesThePropagatedChannelTime` | propagated grading |
| `starmap.TestDerivedEffectiveGenerationNeverReusesTheUpstreamIdentity` | derived identity |
| `settings.TestStarmapSourceComposesTheCascadeSource` | composition |
| `settings.TestStarmapSourceWithoutURLIsRejected` | missing URL |
| `settings.TestCascadeSubscriberCarriesTheCanonicalBounds` | the mapped bounds |
| `settings.TestCascadeSubscriberNeedsAnUpstreamURL` | the named missing URL |
| `starmap.TestRuntimeRefreshesOnAnUpstreamWake` | the reactive wake |
| `starmap.TestRuntimeHandsItsInstanceIdentityToTheSource` | the adopted identity |
| `remote.TestSourceRetriesAStartThatTheUpstreamRefused` | the retried start |

`TestServerCascadesVerifiedCatalogSource` runs the whole cascade on loopback
with the synthetic verified channel of `internal/test/channel`. An origin
runtime serves the channel, a middle runtime consumes the origin through the
`starmap` source kind, and a leaf runtime consumes the middle.

The test proves the shared payload checksum and the propagated origin channel
time at both hops. It also proves one more chain entry at each hop, the three
refusal rules, and a streamed delta. The origin publishes new bytes, and the
middle moves onto them with no refresh call, so the wake path carries them.

Every test is hermetic and runs with `-race`. The package runs above used
`HTTPS_PROXY=127.0.0.1:1`.

## Mutation evidence

This task applied each mutation alone and then reverted it.

| Mutation | Result |
| --- | --- |
| `namesInstance` ignores the alias list | `--- FAIL: TestRuntimeRefusesSelfAliasAndCyclicSourceChains/alias_of_this_instance`: `RefreshSource error = <nil>, want a ConflictError` |
| `acceptSourceChain` skips the repeated-identity check | `--- FAIL: TestRuntimeRefusesSelfAliasAndCyclicSourceChains/two_node_cycle`: `RefreshSource error = <nil>, want a ConflictError` |
| the source layer records the local check time as `ChannelUpdatedAt` | `--- FAIL: TestRuntimeGradesThePropagatedChannelTime`: `channel time = 2026-09-01 12:00:00 +0000 UTC, want the propagated origin time 2026-09-01 00:00:00 +0000 UTC` |
| `deriveEffectiveGenerationID` returns the upstream identity | `--- FAIL: TestDerivedEffectiveGenerationNeverReusesTheUpstreamIdentity`: `derived identities "generation-upstream" and "generation-upstream" reuse the upstream identity` |
| `refuse` writes no `Retry-After` header | `--- FAIL: TestSourceAdmissionReturnsRetryAfter`: `refusal carried no Retry-After header` |
| `sourceChanges` returns nil for every source | `--- FAIL: TestRuntimeRefreshesOnAnUpstreamWake`: `source reads = 1, want 2`, and `--- FAIL: TestServerCascadesVerifiedCatalogSource`: `middle payload = "sha256:948a6164..." want the streamed delta "sha256:6115c41c..."` |
| `Source.start` keeps its first start error | `--- FAIL: TestSourceRetriesAStartThatTheUpstreamRefused`: `second Read: API error from starmap-server (status 401): unexpected event stream response status` |
| `cascadeSubscriber` drops the idle timeout and the startup spread | `--- FAIL: TestCascadeSubscriberCarriesTheCanonicalBounds`: `idle timeout = 2m0s, want 7s` |
| `Open` hands no identity to the source | `--- FAIL: TestRuntimeHandsItsInstanceIdentityToTheSource`: `adopted identity = "", want the runtime identity "8a86ff98e4d8c579"` |

## Commands

| Command | Result |
| --- | --- |
| `make lint` | pass, `0 issues`, strict prose `PASS: 754 file(s), 0 diagnostic(s)` |
| `make test` | pass, exit 0, no `FAIL` line |
| `go tool ago -stale-ignores -format json ./...` | `"findings": []`, `"staleIgnores": []` |
| `make technical-writing-check` | pass |
| `bash scripts/verify-catalog-package-ownership.sh` | `Summary: 13 passed, 0 failed` |
| `make godoc` | pass |
| `make openapi` | pass, the new route entered the embedded spec |
| `make docs-check` | `All documentation is up to date` |
| `shellcheck scripts/*.sh` | pass, no output |
| `bash scripts/verify-catalog-distribution.sh` | `Summary: 47 passed, 2 failed, 19 unverified.` |

Every command ran with `GOTOOLCHAIN=go1.26.6`. `go.mod` keeps `go 1.25.0`, and
CAT7 added no direct module.

The eight CAT7 conditions all report PASS:

```text
PASS CAT-V25 the SSE stream open honors the response-header bound.
PASS CAT-V32 the subscriber reconnect delay uses decorrelated jitter up to 15 minutes.
PASS CAT-V33 the subscriber resets backoff only after a healthy liveness window.
PASS CAT-V34 the subscriber honors Retry-After as a not-before.
PASS CAT-V35 an authentication failure waits for credential change instead of stopping.
PASS CAT-V36 fallback polling uses a stable jittered phase.
PASS CAT-V38 source server admission returns Retry-After on refusal.
PASS CAT-V65 freshness measures the propagated channel_updated_at through two Starmap hops with local acquisition on each hop.
```

The two remaining failures, CAT-V59 and CAT-V64, belong to CAT9. Nineteen
conditions stay unverified because this repository holds no Starport tree.
