# P2 Production Composition Decisions

Date: 2026-07-27  
Task: P2.8  
Code tree inspected: `892589f790f4a7b3b9c88d913924486017854fed`

## Outcome

Starmap will have one live distribution protocol and one offline artifact
format:

1. A Starmap server exposes the current manifest, immutable generation
   payloads, and post-commit SSE publication hints.
2. The public `remote` Go package performs a verified initial fetch, follows
   SSE hints, catches up after reconnect, and atomically activates immutable
   generations.
3. The existing deterministic catalog artifact remains the pinned,
   offline/air-gap, and GitHub Release representation of the same committed
   generation.

The unused hosted channel/promotion protocol, scheduler subsystem, and
WebSocket transport are deleted in their owning implementation phases. This is
an intentional simplification decision, not a compatibility migration:
Starmap is pre-1.0 and no production consumer of those surfaces was found.

## Named production compositions

| Composition | Owner | Retained boundary | Excluded implementation |
| --- | --- | --- | --- |
| In-process catalog library | Embedding Go program | Root Starmap client plus immutable catalog and generation store | CLI, provider clients, scheduler, server, remote HTTP |
| Explicit source acquisition | CLI or embedding deployment | Explicit `Sync(ctx, ...)` and the opt-in provider-client composition selected in P6.3 | Constructor-owned cadence or provider credentials in read-only consumers |
| Embeddable Starmap server | Embedding Go program or the Starmap server binary | Public `server` package over an already constructed Starmap client | Internal application interface, implicit acquisition, second hosted protocol |
| Reactive remote catalog | Starport or another Go process | Public `remote` package over manifest, immutable generation payload, and SSE | WebSocket, event payload catalogs, normal polling |
| Pinned catalog artifact | Release tooling, air-gapped operator, or explicit importer | Deterministic archive, checksum, detached statement, and publisher/provenance verification | Mutable channels, runtime freshness, implicit application-version coupling |

## Decisions

### D-P2.8-1: one live server protocol

Retain and deepen the already wired Starmap-to-Starmap flow:

- current manifest;
- immutable generation-addressed payload;
- one post-commit `catalog.published` SSE hint carrying generation ID and
  monotonic sequence; and
- verified catch-up after initial connection and every reconnect.

The public packages selected by P2.7 remain
`github.com/agentstation/starmap/server` and
`github.com/agentstation/starmap/remote`. The current
`pkg/catalogremote` implementation is a useful verified-fetch primitive, but
its final public capability moves behind `remote`; snapshot vocabulary and the
temporary `pkg/catalogremote` public path do not survive as ordinary consumer
DX.

If `starmap.agentstation.ai` is deployed later, it is an ordinary deployment
of this server contract. It does not introduce a second latest-pointer,
channel, promotion, or archive-serving protocol.

### D-P2.8-2: delete `pkg/catalogdistribution`

Delete the complete `pkg/catalogdistribution` handler, client, in-memory
repository, dev/canary/stable channel policy, promotion state, tests, and
prescriptive documentation in P6.5/P9.6.

Evidence:

- The package contains 767 production Go lines and 1,307 Go lines including
  tests.
- No production package imports it.
- Its only production import of another catalog distribution primitive is
  inward to `pkg/catalogartifact`.
- It competes with the already wired `pkg/catalogremote` plus internal server
  manifest/payload flow by defining different routes, response shapes,
  archive semantics, and mutable channel pointers.

Any durable trust, same-origin, body-limit, digest, compatibility, or rollback
rule that is still applicable is preserved in the selected server/remote or
artifact boundary before the old package is removed. No hosted deployment is
required to prove this plan.

### D-P2.8-3: retain `pkg/catalogartifact`

Retain the deterministic artifact implementation as the one immutable portable
encoding of a committed generation. It has real production callers in
`cmd/starmap-catalog-release` and `internal/embeddedbudget`.

GitHub Releases may be the canonical public source of record for catalog
artifacts when separately authorized. A catalog release is keyed by generation
ID or payload/archive digest and carries the archive, checksum, detached
statement, and GitHub provenance attestation. Import verifies compatibility and
publisher identity before activation and supports rollback to an already
verified generation.

The catalog generation does not have to match the Starmap application version.
Compatibility is determined by the manifest schema/consumer range and exact
digests. A binary records the generation it embeds; a remote or pinned artifact
is independently versioned. P9 verifies this flow with fixtures and local
release/import tooling without publishing a release under this plan.

### D-P2.8-4: delete the scheduler subsystem; cadence is deployment-owned

Starmap's core library owns an explicit, context-bound, idempotent
`Sync(ctx, ...)` operation. It does not own a ticker, cron expression, hidden
goroutine, distributed lease, retry loop, SQL run ledger, or startup
background task.

The actual deployment owns cadence:

- a Kubernetes CronJob, systemd timer, CI workflow, or another external
  orchestrator may invoke the CLI;
- an embedding Go application may invoke `Sync` from its own supervised
  scheduler; or
- an operator may use explicit/manual updates.

Delete `pkg/catalogscheduler` in P6.5. The package contains 2,314 production Go
lines and 3,490 Go lines including tests. `NewRunner`,
`NewInitialRunController`, all lease implementations, run ledgers, retry
policy, and freshness monitor have no production caller. The CLI constructs
only `NewOperations()` with no inputs, so its current endpoint always reports
that scheduling is unconfigured; that is not a production scheduler use case.

P7.11 replaces the inert scheduler-shaped operational response with health
derived from real catalog, source, publication, and remote subscriber state.
If a later deployment needs an in-process scheduling helper, it can be proposed
against a named owner and real adapter after the explicit `Sync` composition
has proven insufficient.

### D-P2.8-5: delete WebSocket and collapse transport-general abstractions

Delete the WebSocket route, hub, adapter, dependency, tests, and documentation
in P7. SSE is the sole notification transport because catalog publication is
one-way. No bidirectional consumer exists.

With one transport, generalized multi-transport broker/adapter surfaces must
also pass the deletion test. Keep only a deep internal publication stream
needed to serialize, flush, apply backpressure, heartbeat, and join SSE
connections. Publication correctness remains manifest/payload catch-up, not
event replay.

## Scheduled-sync conclusion

Scheduled catalog refreshes are useful at deployment level, but adding a
second Starmap-owned scheduler would reduce reliability:

- it would duplicate the deployment orchestrator's lifecycle and leader
  election;
- it would pull scheduler, SQL, and acquisition dependencies into otherwise
  small library/server compositions;
- it would create unclear retry and shutdown ownership; and
- it is unnecessary because `Sync` is already the explicit unit of work.

Therefore Starmap supplies the operation and observable results, while the
deployment supplies cadence. Polling remains an explicit fallback only for the
remote subscriber when SSE is unavailable; it is unrelated to provider-source
sync scheduling.

## Verifiable evidence

The following repository queries were run from the recorded code tree:

```bash
rg -n 'pkg/catalogdistribution|catalogdistribution\.' \
  --glob '*.go' --glob '!**/*_test.go' .
rg -n 'pkg/catalogscheduler|catalogscheduler\.|NewRunner\(' \
  --glob '*.go' --glob '!**/*_test.go' .
for symbol in NewRunner NewInitialRunController NewFilesystemLease \
  NewMemoryLease NewSQLRunLedger NewMemoryRunLedger NewFreshnessMonitor \
  WithOperationsRunLedger WithOperationsFreshness WithOperationsInitialRun
do
  rg -n "$symbol" --glob '*.go' --glob '!pkg/catalogscheduler/**' \
    --glob '!**/*_test.go' .
done
```

Results:

- `catalogdistribution` has no production importer.
- None of the listed scheduler construction/wiring symbols has a production
  caller.
- `cmd/starmap/app/app.go` imports `catalogscheduler` only to construct empty
  `Operations` and project its always-unconfigured scheduler state.
- `pkg/catalogartifact` has production callers in release and embedded-budget
  tooling.
- `pkg/catalogremote` is already wired on both client and server sides.

This decision precedes deletion. P6.5 and P7 perform the code removal and
consumer rewiring; P9.6 proves the selected live and artifact flows end to end.
