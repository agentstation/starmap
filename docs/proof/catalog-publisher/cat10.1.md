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

## Correction to the first CAT10.1 report

The first report claimed that the banned-import check for the server-embed
consumer still passed. That claim was false. The script
`scripts/verify-consumer-deps.sh` exits at the budget check, so it never
reaches the banned check. Nothing observed that result.

The measured closure at the base commit holds 84 forbidden packages. One of
them is `internal/sources/github`. Eighteen are OpenTelemetry packages, and 65
are gRPC packages.

The first report also blamed a stale `SERVER_MAX_PACKAGES` from commit
`5e0eb9bd`. That attribution was wrong. The same measurement on `origin/main`
at commit `d632a1a2` returns 242 packages and zero forbidden imports. The
breach is a campaign regression, and this task repairs it.

## The consumer boundary repair

The plan branch added `server.WithRuntime(*runtime.Runtime)`. The public
`server` package therefore imported `runtime`. It inherited the attested
GitHub source, the Sigstore verification, gRPC, and OpenTelemetry. The
`remote` package implements the cascaded source, so it imported `runtime` for
the same reason.

Two leaf packages now carry the shared names.

| Package | Owns | Imports |
| --- | --- | --- |
| `runtime/status` | `Status`, `Health`, `Freshness`, `SourceKind`, `SourceHop` | `pkg/sources` and the standard library |
| `runtime/source` | `Source`, `Watcher`, `IdentityAdopter`, `Read` | `pkg/catalogs`, `runtime/status`, and the standard library |

The file `runtime/vocabulary.go` keeps every published name. Each moved type
becomes an alias, and each moved constant becomes a re-export. The runtime
package still owns `ParseSourceKind`, `SourceKinds`, the startup policy, and
the freshness policy. No public name changed. Starport therefore needed no
edit, and its gates confirm that.

The option `server.WithRuntime` now takes the interface
`server.ConnectedRuntime`. That interface holds two methods, and they are
everything the server reads from a connected runtime.

| Method | Use |
| --- | --- |
| `Status() status.Status` | Readiness and the source chain render this report. |
| `Close() error` | The server shutdown joins the runtime shutdown. |

The type `*runtime.Runtime` satisfies the interface. The option refuses a nil
runtime, and it also refuses a nil pointer inside the interface value. A
concrete pointer no longer protects the caller once an interface holds it.

The five `internal/server` files that render the status now import
`runtime/status`. The `remote` package imports `runtime/source` and
`runtime/status`. Neither the `server` package nor the `remote` package
reaches `runtime` any more. Test files still import `runtime`.

## Consumer closure counts

Three trees carry the measurements. Commit `d632a1a2` is `origin/main` before
the campaign. Commit `9b552208` is the plan branch base. The head is this
branch. Each measurement ran `GOWORK=off go list -deps` in the consumer module
after `go mod tidy`. The server-storage row adds `-test`, because its own gate
does.

Each cell reads the total count, then the non-standard count, then the
forbidden count.

| Consumer | Main `d632a1a2` | Base `9b552208` | Head | Budget |
| --- | --- | --- | --- | --- |
| read-only | 153 / 31 / 0 | 578 / 362 / 86 | 153 / 31 / 0 | 32 non-standard |
| store-only | 153 / 31 / 0 | 578 / 362 / 86 | 153 / 31 / 0 | forbidden families only |
| pinned-artifact | 161 / 32 / 0 | 578 / 362 / 86 | 161 / 32 / 0 | 32 non-standard |
| server-embed | 242 / 49 / 0 | 594 / 378 / 84 | 246 / 52 / 0 | 260 total |
| remote-subscriber | 225 / 33 / 0 | 579 / 363 / 84 | 230 / 37 / 0 | 240 total |
| server-storage | 335 / 94 / 0 | 676 / 94 / 84 | 340 / 94 / 0 | 350 total |

Every consumer passes its budget, and every forbidden count is zero. One budget
moved, and no banned pattern changed.

Three closures stay above the `origin/main` count. Six packages explain every
difference.

| Package | Reason | Consumers |
| --- | --- | --- |
| `internal/fleet` | The runtime derives one replica instance identity. | all three |
| `hash/fnv` | The fleet package hashes that identity. | all three |
| `runtime/status` | The status vocabulary leaf. | all three |
| `runtime/source` | The source contract leaf. | remote-subscriber, server-storage |
| `internal/server/operations` | The server renders the operations surface. | server-embed, server-storage |
| `pkg/sources` | The status report names one provider attempt. | remote-subscriber |

The server-storage closure reaches 340 packages on Darwin and 341 on Linux,
because Linux adds three standard-library packages and Darwin adds two. The
hosted gate at PR #116 failed on that Linux count against the former budget of
340. The orchestrator raised `SERVER_STORAGE_MAX_PACKAGES` to 350 on purpose.
The five packages above the `origin/main` count are Starmap-owned or standard
library, and the change adds no third-party dependency. The budget stays a
prompt for that question, not a hard rule.

## Commands run

Every command ran with `GOTOOLCHAIN=go1.26.6` exported. Each row reports the
final run on the head of this branch.

| Command | Result |
| --- | --- |
| `make lint` | PASS, 0 issues |
| `make test` | PASS |
| `go tool ago -stale-ignores -format json ./...` | PASS, status 0, no findings |
| `make technical-writing-check` | PASS, 773 files, 0 diagnostics |
| `bash scripts/verify-catalog-package-ownership.sh` | PASS, 13 of 13 |
| `make docs-check` | PASS |
| `shellcheck scripts/*.sh` | PASS, status 0 |
| `bash scripts/verify-consumer-deps.sh` | PASS, all six consumers |
| `make test-pure-go` | PASS |
| `make verify` | PASS |
| `bash scripts/verify-catalog-distribution.sh` | PASS, 68 of 68 |

The distribution verifier ran with `CATALOG_DISTRIBUTION_STARPORT_ROOT` set to
the Starport worktree `agent-ad6e217c4e16a7520`.

Starport reported these results. Each Starport run used a temporary replace at
this Starmap worktree.

| Command | Result |
| --- | --- |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test ./...` | PASS |
| `make lint` | PASS, 0 issues |
| `bash scripts/verify-starmap-ownership.sh` | PASS, 12 of 12 |
| `bash scripts/verify-catalog-driven-providers.sh` | PASS, 19 of 19 |
| `bash scripts/verify-catalog-performance.sh` | PASS, 20 of 20 |

The ownership gate reads `STARMAP_OWNERSHIP_STARMAP_ROOT`, and the
catalog-driven gate reads `CATALOG_DRIVEN_STARMAP_ROOT`. Both named this
Starmap worktree. The first CAT10.1 run also passed
`scripts/verify-dependency-direction.sh` at 6 of 6 and
`scripts/verify-package-layout.sh`. This run did not repeat those two.

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
