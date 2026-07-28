# P2 Golden User Journeys

Frozen: 2026-07-27

These five fixtures define consumer-visible success before implementation
changes package boundaries or storage behavior. They are intentionally small:
each names one actor, one composition, and the observable result. Later phases
must turn the golden source files into external compile/integration tests
rather than replace them with a different product story.

## 1. In-process Go library

Fixture: `testdata/journeys/in_process_library.go.golden`

The ordinary consumer imports only the root Starmap package, constructs a
client, receives initialization errors from `New`, gets the concrete immutable
catalog through non-failing `Catalog()`, and performs a read. `Snapshot` never
appears. The client starts no scheduler, server, provider acquisition, or
remote goroutine merely to serve this read.

Success is externally compiled in P5.7/P6.2/P6.6 and must remain within the
160-package dependency budget with a zero-allocation catalog accessor.

## 2. CLI human workspace

Fixture: `testdata/journeys/cli_workspace.json`

The fixture freezes the complete operator loop: atomic first seed, semantic
hand edit, metadata-only manual model, later dynamic observation, field
authority, atomic generation publication, and failure preservation. The only
human model tree is `~/.starmap/catalog/providers/...`; definitions, offerings,
and overrides are forbidden persisted paths.

The fixture becomes the P3/P4 CLI end-to-end test. The current command and docs
that still describe separate input/output or export directories are
characterized behavior, not the target contract.

## 3. Embedded E1 to E2 upgrade

Fixture: `testdata/journeys/embedded_upgrade.json`

Installing E2 alone performs no write. One later explicit update distinguishes
unchanged E1-derived facts from actual human edits, applies E2 only where its
evidence still owns the field, adds new facts, publishes once, and supports
restart plus exact rollback. Formatting is not evidence.

This fixture becomes the lifecycle suite shared by P3.3–P3.10, P4 authority,
and P9 embedded-integrity work.

## 4. Embeddable Go server

Fixture: `testdata/journeys/embeddable_server.go.golden`

The selected public import is `github.com/agentstation/starmap/server`.
`server.New(sm)` accepts an already constructed Starmap dependency and returns
a concrete server. `Serve(ctx, listener)` makes listener and context ownership
explicit, blocks for the owned lifecycle, and returns only after its loops are
joined. The embedding program imports neither `internal/application` nor the
CLI and provider acquisition remains opt-in.

P7.1/P7.5 may add validated options without invalidating this minimal path.
Health, readiness, manifest, immutable generation, query, and SSE routes are
integration-tested through the resulting handler/server.

## 5. Reactive remote Go consumer

Fixture: `testdata/journeys/remote_reactive_consumer.go.golden`

The selected opt-in import is `github.com/agentstation/starmap/remote`, keeping
HTTP/TLS and stream machinery outside the <=160-package local read closure.
`remote.New(Config)` validates configuration and starts no goroutine.
`Start(ctx)` performs the verified initial manifest/payload fetch before
returning success, then owns SSE/reconnect/catch-up under the caller context.
`Catalog()` returns the same concrete immutable catalog product, and `Close()`
cancels and joins the lifecycle. Polling is not part of the normal path.

P7 turns this syntactic golden into a real server-to-consumer test covering
generation hints, heartbeats, reconnect catch-up, corruption, authorization,
slow consumers, and shutdown.

## Fixture gate

`TestP2UserJourneyGoldenFixtures` parses all three Go golden files, rejects
`internal`/CLI imports, checks the selected public import paths, validates both
machine-readable lifecycle fixtures, and requires the three forbidden
persisted trees. This test validates the durable contract artifacts; the later
phase tests validate their runtime semantics.
