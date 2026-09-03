# CAT10.1 proof: the connected runtime moves to starmap/runtime

CAT10.1 follows CAT-D22. The connected runtime leaves the root
`starmap` package and becomes the top-level package
`github.com/agentstation/starmap/runtime`. The root package returns to its role
as the offline library. The attested public GitHub channel stays the default
source of `runtime.Open`.

## What moved

Every root `runtime*.go` file and its test moved into `runtime/`. Git recorded
each one as a rename.

| Before | After |
| --- | --- |
| `runtime.go` | `runtime/runtime.go` |
| `runtime_chain.go` | `runtime/chain.go` |
| `runtime_layers.go` | `runtime/layers.go` |
| `runtime_lease.go` | `runtime/lease.go` |
| `runtime_options.go` | `runtime/options.go` |
| `runtime_policy.go` | `runtime/policy.go` |
| `runtime_refresh.go` | `runtime/refresh.go` |
| `runtime_scheduler.go` | `runtime/scheduler.go` |
| `runtime_source.go` | `runtime/source.go` |
| `runtime_status.go` | `runtime/status.go` |
| `runtime_test.go` | `runtime/runtime_test.go` |
| `runtime_chain_test.go` | `runtime/chain_test.go` |
| `runtime_lease_test.go` | `runtime/lease_test.go` |
| `runtime_policy_test.go` | `runtime/policy_test.go` |
| `runtime_scheduler_test.go` | `runtime/scheduler_test.go` |
| `runtime_status_test.go` | `runtime/status_test.go` |
| `runtime_watch_test.go` | `runtime/watch_test.go` |
| `runtime_test_helpers_test.go` | `runtime/helpers_test.go` |

The new package also carries `runtime/generate.go` and the gomarkdoc output
`runtime/README.md`. The `docs-check` target now reads `./runtime` beside
`./server` and `./remote`.

## The exported root contract

The moved code used eight root identifiers that no caller outside the root
could reach: `acquire`, `apply`, `defaults`, `hooks`, `newClient`, `nextID`,
`options`, and `requireWritableCatalogStore`. Two new exported names close that
gap. The other six stay unexported, because the already exported
`starmap.NewContext` and the root option constructors cover them.

| Name | Reason |
| --- | --- |
| `func (c *Client) NextID() (string, error)` | The runtime stamps each accepted generation with a client-owned identifier. |
| `func (c *Client) PublishesDurably() bool` | The runtime asks whether the client holds the explicit writable store that durable publication needs. |

`PublishesDurably` is a predicate over the still unexported
`requireWritableCatalogStore`. The store rule itself stays inside the root, so
the runtime reads a verdict and never the invariant.

## Renamed identifiers

| Before | After |
| --- | --- |
| `starmap.RuntimeStatus` | `runtime.Status` |
| `starmap.nextID` | `starmap.NextID` |
| `starmap.runtimeOptions` | `runtime.options` |

Every other exported runtime name keeps its spelling under the new package
path. Examples are `runtime.Runtime`, `runtime.Open`, `runtime.Acquirer`,
`runtime.Lease`, `runtime.SourceKind`, and `runtime.Health`.

Three unexported changes support the move. The `runtime/options.go` receiver
became `r`, because `o` collided with the linter's receiver rule. Test
variables named `runtime` became `connected`, because a local name of `runtime`
shadows the imported package. The root test
`TestNewRejectsRuntimeOptions` now reads the root package with `go/parser` and
asserts the API boundary, because the rejection became a compile-time property.

## The two option types

`starmap.Option` and `runtime.Option` are separate types. The bridge is
`runtime.WithClientOptions(opts ...starmap.Option) Option`. A caller therefore
cannot hand a connected option to `starmap.New`, and the compiler enforces it.
The root package keeps `WithCatalogStore`, `WithCatalogPath`,
`WithEmbeddedBootstrapMaxAge`, and `WithEmbeddedBootstrapMaxSizeBytes`.

## Callers updated

The Starmap CLI and composition code moved to the new import path. The six
consumer modules under `testdata/consumers/` each ran
`GOWORK=off GOTOOLCHAIN=go1.26.6 go mod tidy`, because the grpc 1.83.1 merge
left their `go.mod` files stale.

Ten Starport files moved to the new import path.

- `internal/app/catalog_operations.go`
- `internal/app/remote_catalog_test.go`
- `internal/app/runtime.go`
- `internal/catalog/lease.go`
- `internal/catalog/runtime.go`
- `internal/catalog/runtime_acquisition_test.go`
- `internal/catalog/settings.go`
- `internal/catalog/settings_test.go`
- `internal/catalog/status.go`
- `internal/catalog/status_test.go`

`starmap.New`, `starmap.NewContext`, `starmap.CatalogState`, and
`starmap.EmbeddedBuilder` stay on the root import in Starport.

## Consumer closure counts

The base is commit `9b552208`. Each count comes from
`GOWORK=off go list -deps` in the consumer module after `go mod tidy`.

| Consumer | Measure | Before | After | Budget |
| --- | --- | --- | --- | --- |
| read-only | non-standard packages | 362 | 31 | 32 |
| pinned-artifact | non-standard packages | 362 | 32 | 32 |
| server-embed | total packages | 594 | 595 | 260 |

The read-only consumer fell by 331 packages. The Sigstore, Rekor, gRPC,
protobuf, and OpenTelemetry families left both offline closures. Neither budget
moved, and the banned-import patterns keep their original text.

## The server-embed budget

`make test-pure-go` and `make verify` fail on one condition that CAT10.1 did
not cause. The server-embed consumer closure is 595 packages, and the budget
allows 260 packages. The same measurement at the base commit `9b552208`
returns 594, so the breach predates this task. The move adds exactly one package, which is the new
`runtime` package itself.

`SERVER_MAX_PACKAGES=260` entered `scripts/verify-consumer-deps.sh` in commit
`5e0eb9bd` and never moved. The banned-import check for that consumer still
passes, so no acquisition family reaches it. The gap is a stale total count
after later dependency bumps. CAT10.1 leaves the budget alone rather than hide
the drift.

## Commands run

Every command ran with `GOTOOLCHAIN=go1.26.6` exported.

| Command | Result |
| --- | --- |
| `make lint` | PASS, 0 issues |
| `make test` | PASS |
| `go tool ago -stale-ignores -format json ./...` | PASS, status 0, no findings |
| `make technical-writing-check` | PASS, 767 files, 0 diagnostics |
| `bash scripts/verify-catalog-package-ownership.sh` | PASS, 13 of 13 |
| `make docs-check` | PASS |
| `shellcheck scripts/*.sh` | PASS, status 0 |
| `bash scripts/verify-catalog-distribution.sh` | PASS, 68 of 68 |
| `make test-pure-go` | FAIL, server-embed budget only |
| `make verify` | FAIL, server-embed budget only |

The distribution verifier ran with
`CATALOG_DISTRIBUTION_STARPORT_ROOT` set to the Starport worktree
`agent-ad6e217c4e16a7520`.

Starport reported these results.

| Command | Result |
| --- | --- |
| `make lint` | PASS, 0 issues |
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| `bash scripts/verify-starmap-ownership.sh` | PASS, 12 of 12 |
| `bash scripts/verify-dependency-direction.sh` | PASS, 6 of 6 |
| `bash scripts/verify-catalog-driven-providers.sh` | PASS, 19 of 19 |
| `bash scripts/verify-catalog-performance.sh` | PASS, 20 of 20 |
| `bash scripts/verify-package-layout.sh` | PASS |

## Starport commit state

The ten Starport files carry the finished edits, and every Starport gate above
ran against them. They stay uncommitted. This agent runs under worktree
isolation, and the harness refuses every version-control command that targets a
tree outside its own Starmap worktree. The orchestrator owns the Starport
commit.

The Starport `go.mod` replace points back at the plan worktree
`/Users/jack/src/github.com/agentstation/starmap-catalog-publisher`. The gate
runs used a temporary replace at this Starmap worktree. Starport therefore
compiles once the runtime move reaches the plan worktree.

## The verifier retarget

`scripts/verify-catalog-distribution.sh` names a package path for each Go test
row. Fifteen rows moved from `.` to `./runtime`, and one row moved its explicit
`go_test_passes` call the same way. The row for
`TestNewRejectsRuntimeOptions` stays in the root, because that test guards the
root constructor. No condition text changed, and the count stays at 68.
