# P9 Distribution Convergence Review

Date: 2026-07-29

Scope: P9.6, with the P9.7 offline composition used as an executable boundary
check.

## Outcome

The P2.8 production decision is now one composed system rather than several
competing distribution products:

| Need | Canonical path | Production entry point |
| --- | --- | --- |
| Online publication | Current manifest, addressed immutable payload, and one SSE hint | `server` |
| Reactive online consumption | Verified initial/catch-up fetch, SSE, atomic activation | `remote` |
| Portable generation | Deterministic archive, checksum, and detached statement | `pkg/catalogartifact` |
| Human-aware portable import | Publisher verification, authority reconciliation, CAS, projection | `acquisition.Syncer.ImportRelease` |
| Offline pinned startup | Publisher/pin verification and exact generation activation | `catalogartifact.VerifyRelease` plus `Client.Activate` |

`pkg/catalogremote` remains a justified deep wire-protocol module shared by
server manifest encoding, server SSE framing, and the public reactive
subscriber. It does not expose a second hosted service, mutable channel, or
consumer lifecycle. Ordinary consumers use `remote`; protocol implementers may
use the versioned wire vocabulary directly.

## Deleted alternatives

The production tree contains no:

- `pkg/catalogdistribution`;
- `pkg/catalogscheduler`;
- WebSocket server package or route;
- dev/canary/stable mutable catalog channel;
- in-library ticker, lease, or run-ledger scheduler; or
- second hosted latest-pointer/archive protocol.

The only `/api/v1/updates/ws` reference is the regression proving that route is
absent. Catalog fields describing a model provider's WebSocket capability are
domain data, not Starmap distribution transport. The indirect
`github.com/gorilla/websocket` module is reachable only through the optional
Google GenAI provider dependency and is absent from the read-only, server,
remote, and pinned-artifact consumer closures.

The OCI mirror is not another catalog protocol. It mirrors the exact
content-addressed release archive and statement, requires equality with the
trusted archive digest, and grants no authority to a mutable tag.

## Executable compositions

The external consumer gate now runs six real `GOWORK=off` modules:

1. read-only embedded library;
2. caller-owned generation store;
3. offline pinned artifact;
4. embeddable server;
5. reactive remote subscriber; and
6. filesystem/S3 server storage plus reactive restart.

The pinned-artifact module has a compile-time archive digest, blanks common
provider credentials, performs no HTTP operation, imports neither acquisition
nor online server/remote implementations, and verifies then activates the
exact embedded generation. Its closure is 31/32 non-standard packages on the
review machine. The reactive external module starts a real Starmap server,
performs verified initial fetch, establishes SSE, reads the immutable catalog,
and joins both lifecycles. The storage module repeats publication/restart
against filesystem and conditional S3-compatible stores.

The human-aware release-import integration separately proves that manual
overrides and manual-only records survive while release-only facts arrive,
that publisher/checksum failure changes no state, and that rollback restores
the exact prior generation.

## Verification commands

```text
test ! -d pkg/catalogdistribution
test ! -d pkg/catalogscheduler
test ! -d internal/server/websocket
git grep -n 'catalogdistribution\|catalogscheduler' -- '*.go' ':!**/*_test.go'
git grep -n 'gorilla/websocket' -- '*.go'
go mod why -m github.com/gorilla/websocket
bash -n scripts/verify-consumer-deps.sh
go test ./internal/ciworkflow -run \
  'TestPinnedArtifact|TestPureGoAndRace|TestReadOnlyConsumerDependency|TestExternalStore|TestExternalServerStorage'
./scripts/verify-consumer-deps.sh
```

The directory and production-source absence checks passed. `go mod why`
reported only `internal/providers/google → google.golang.org/genai →
github.com/gorilla/websocket`. The structural suite passed. The complete
consumer gate reported:

```text
read-only: 30/32 non-standard packages; forbidden families absent
pinned artifact: 31/32 non-standard packages; online/acquisition families absent
server embed: 240/260 packages; acquisition families absent
remote subscriber: 224/240 packages; forbidden families absent
server storage: 332/340 packages; filesystem/S3 reactive restart passed
```

## Disposition

P9.6 is complete. F-025 is resolved by wiring the selected server/remote and
release paths to real external consumers while retaining one shared wire module
and deleting—not adapting—the unused competing products. No hosted deployment
or catalog release is required or authorized by this review.
