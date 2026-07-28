# P2 Public Composition Baseline

Measured: 2026-07-27

Code tree: `fa088d97d639d716f0593bbdff140d0def255718`

Toolchain and host: `go1.26.5 darwin/arm64`, Apple M2 Max

This is the P2.6 pre-refactor baseline for P6 dependency inversion and P8
modularity work. It records compile closures, not only direct imports. Counts
come from `go list -deps`; `stdlib`, `local`, and `external` are mutually
exclusive package classifications.

## Compile closures

| Consumer/package | Total | Standard library | Starmap-local | External |
| --- | ---: | ---: | ---: | ---: |
| Root library `.` | 472 | 214 | 33 | 225 |
| Canonical catalog `./pkg/catalogs` | 145 | 121 | 8 | 16 |
| Current internal server `./internal/server` | 488 | 214 | 48 | 226 |
| Google provider client `./internal/providers/google` | 448 | 213 | 12 | 223 |

The Google provider closure is a strict subset of the root closure: all 448
Google packages are present in an ordinary root-package build, accounting for
94.9% of its 472-package closure. Only 24 root packages are outside the Google
closure, and most are Starmap orchestration packages plus `database/sql`,
`github.com/gofrs/flock`, and `github.com/google/uuid`.

The largest root closure contributors by package count are:

| Module/category | Packages |
| --- | ---: |
| Standard library | 214 |
| `google.golang.org/grpc` | 64 |
| Starmap module | 33 |
| `google.golang.org/protobuf` | 33 |
| `github.com/google/s2a-go` | 21 |
| `cloud.google.com/go/auth` | 16 |
| `go.opentelemetry.io/otel` | 14 |
| `github.com/goccy/go-yaml` | 9 |
| `golang.org/x/net` | 8 |
| `golang.org/x/crypto` | 7 |
| `github.com/googleapis/gax-go/v2` | 7 |

The current server adds 16 packages beyond the root closure: 14 server/query
Starmap packages, `pkg/catalogscheduler`, and
`github.com/patrickmn/go-cache`. It therefore inherits the entire acquisition
stack before adding its own composition.

### Frozen dependency budgets

- Until P6.2 lands, the root closure may not exceed 472 packages.
- The canonical catalog closure may not exceed 145 packages without a recorded
  product reason.
- The server closure may not exceed 488 packages before its public composition
  is separated.
- P6.2 is not successful merely by staying below 472: the root read-only
  consumer closure must also contain none of GenAI, gRPC, OpenTelemetry,
  WebSocket, SQLite, Cobra, scheduler, or server implementations. The budget is
  a regression ceiling; the banned-implementation assertion is the
  architectural gate.

Reproduction:

```bash
go list -deps -f '{{.ImportPath}}' . | wc -l
go list -deps -f '{{.ImportPath}}' ./pkg/catalogs | wc -l
go list -deps -f '{{.ImportPath}}' ./internal/server | wc -l
go list -deps -f '{{.ImportPath}}' ./internal/providers/google | wc -l
go list -deps -f \
  '{{if .Standard}}stdlib{{else if eq .Module.Path "github.com/agentstation/starmap"}}local{{else}}external{{end}}' \
  . | sort | uniq -c
```

## Binary and accessor performance

| Measurement | Result |
| --- | ---: |
| `go build -trimpath ./cmd/starmap` darwin/arm64 | 37,552,946 bytes |
| Same build with `-ldflags='-s -w'` | 27,687,346 bytes |
| `BenchmarkClientCatalog` latency, five runs | 9.159–10.75 ns/op |
| `BenchmarkClientCatalog` allocation, all runs | 0 B/op, 0 allocs/op |

Benchmark command:

```bash
go test -run '^$' -bench '^BenchmarkClientCatalog$' -benchmem -count=5 .
```

The accessor is already comfortably inside the 10 µs/op public budget. Later
composition work must preserve zero allocation and must not replace O(1)
generation access with a catalog copy.

## Repository scale

| Measurement | Result |
| --- | ---: |
| Go packages (`go list ./...`) | 89 |
| Repository-authored Go files | 466 |
| Total Go lines | 86,051 |
| Non-test Go lines | 47,900 |
| Test Go lines | 38,151 |
| Embedded catalog files | 966 |
| Embedded catalog bytes | 2,514,088 |

The line counts use `rg --files -g '*.go'` and `wc -l`; they include generated
repository files because they still affect review, compile, and file-size
policy. Embedded bytes are the exact sum of regular files under
`internal/embedded/catalog`.

## File-size inventory

| Lines | File | Required disposition |
| ---: | --- | --- |
| 2,059 | `pkg/reconciler/merger_test.go` | Hard failure; split in P8.2 |
| 1,206 | `internal/providers/google/client.go` | Review/extract by concept in P8.3 |
| 1,183 | `internal/providers/openai/client.go` | Review/extract by concept in P8.3 |
| 1,134 | `pkg/reconciler/merger.go` | Deep-module review and conceptual split decision in P8.3 |

No other repository-authored Go file exceeds 1,000 lines at this baseline.

Reproduction:

```bash
go list ./... | wc -l
rg --files -g '*.go' -0 | xargs -0 wc -l
rg --files -g '*_test.go' -0 | xargs -0 wc -l
rg --files -g '*.go' -g '!*_test.go' -0 | xargs -0 wc -l
find internal/embedded/catalog -type f -exec stat -f '%z' {} + |
  awk '{sum += $1} END {print sum}'
rg --files -g '*.go' -0 | xargs -0 wc -l | sort -nr
```

## Architectural reading

The immutable catalog core is materially smaller than the root library, while
the root and internal server both compile almost the complete Google/cloud
acquisition graph. P6.2 therefore must invert the `pkg/sources` to internal
provider-client dependency behind the existing injected factory and move
acquisition into explicit composition. The server must then depend on the
narrow catalog/publication roles it actually serves, not inherit acquisition
merely by importing the root package.

The current binary and embedded catalog sizes are acceptable baselines, not
optimization targets by themselves. Dependency closure, startup composition,
and audit surface are the immediate trust problems. Any size reduction must
follow removal of unused or wrongly owned code rather than compression tricks
that obscure behavior.
