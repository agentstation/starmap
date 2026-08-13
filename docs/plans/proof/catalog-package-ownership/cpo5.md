# CPO5 remote protocol package proof

Date: 2026-08-13 UTC  
Work commit: [`cfad0530`](https://github.com/agentstation/starmap/commit/cfad0530)

## Fail-before

The CPO4 structural run proved these conditions:

- The versioned wire client still used `pkg/catalogremote`.
- `pkg/catalogs/remote` did not exist.
- CPO-V09 failed because the approved package was absent.
- Current server, subscriber, tests, consumer guards, and docs used the flat path.

CPO2 had already removed the storage dependency and changed the value to `catalogs.Generation`. The original CPO5 fail-before combined two defects. Only the flat package path remained when CPO5 started. The durable plan now records that sequence.

## Ownership result

`pkg/catalogs/remote` now owns the versioned manifest, payload, and SSE wire client. Its package name is `remote`. It depends on the catalog root for immutable generation contracts and does not import storage.

The top-level `github.com/agentstation/starmap/remote` package remains separate. It owns reactive activation, retry, catch-up, polling fallback, durable publication, health, and lifecycle. Its implementation imports the wire package as `protocol` to make the two concepts explicit in each file.

The direct breaking move updated:

- all internal server handlers and server protocol tests.
- the server SSE publication aliases and generated documentation.
- the reactive subscriber and its lifecycle test matrix.
- read-only, store-only, and pinned-artifact dependency guards.
- current architecture, protocol, package, and README documentation.
- the generated package documentation and generator path.

The dependency guards now reject `pkg/catalogs/remote` where an offline or read-only composition must not link online protocol code.

A mechanical selector rename initially changed three `remote.*` validation fields in the reactive subscriber. Review caught and restored those exact strings before verification. The package move does not alter typed error fields or observable behavior.

The old package tree is absent. No wrapper, forwarding alias, or compatibility package remains.

## Structural verifier

The final CPO5 run reported `Summary: 12 passed, 1 failed`.

- CPO-V01 through V10 passed.
- CPO-V12 and CPO-V13 passed.
- CPO-V11 failed only because CPO6 owns the missing `docs/MIGRATING_TO_V0.5.md` guide.

## Verification

| Command | Result |
|---|---|
| `go test -race -count=1 ./pkg/catalogs/remote ./remote ./internal/server ./internal/server/sse` | PASS. Protocol `1.270s`, subscriber `18.521s`, server `110.214s`, and SSE `1.230s`. |
| `(cd testdata/consumers/remote-subscriber && GOWORK=off go test ./...)` | PASS in `3.120s`. |
| `make test-consumer-deps` | PASS for all six modules and dependency-policy checks. |
| `bash scripts/verify-catalog-package-ownership.sh` | Expected partial failure. 12 passed and 1 failed. |
| `go test ./...` | PASS across the main module. Root `40.549s`, handlers `3.972s`, public server `3.793s`, and CI workflow checks `1.018s`. |
| Focused `go vet` for the affected package set | PASS. |
| `make docs-check` | PASS. |
| Stale current-authority scan for `catalogremote` and its import path | PASS. |
| `git diff --check` and consumer-script syntax | PASS. |
| SHA-256 checks for `go.mod` and `go.sum` | PASS. Both files are unchanged. |

All Go commands used normal scheduling. No command set `GOFLAGS=-p`.
