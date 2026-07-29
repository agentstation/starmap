# P6 Go Composition Outcome

Date: 2026-07-29

Comparison baseline:
`6993b1e7c72b196508dc7321a20c5f277262afb2`

Measured candidate:
`67330be0` plus this evidence report only

Toolchain and host: `go1.26.5 darwin/arm64`, Apple M2 Max, macOS 15.7.2

## Outcome

The P6 composition work is materially smaller than the post-P5 catalog
restoration baseline. The ordinary root library no longer compiles acquisition,
provider clients, server, scheduler, remote transport, CLI, or their large
third-party dependency families. Seven unused packages and the alternate
distribution/scheduling designs are deleted rather than retained as speculative
extension points.

All numeric measurements improved. Forced full-build wall time is effectively
unchanged within measurement noise and did not regress.

## Compile closures

Counts are unique packages from `go list -deps`.

| Composition | P6 baseline | Candidate | Change |
| --- | ---: | ---: | ---: |
| Root `starmap` | 472 | 158 | -314 |
| External read-only consumer | not yet implemented | 159 | <=160 budget |
| `pkg/catalogs` | 147 | 146 | -1 |
| `pkg/catalogs` + `pkg/catalogstore` union | 152 | 151 | -1 |
| Internal server | 488 | 484 | -4 |
| Google provider client | 449 | 448 | -1 |

The candidate root closure is 128 standard-library, 13 Starmap-local, and 17
external packages. The real `GOWORK=off` read-only consumer contains none of
the acquisition, provider, pipeline, remote, server, scheduler, GenAI, gRPC,
OpenTelemetry, WebSocket, SQLite, Cobra, or CLI families enforced by
`scripts/verify-consumer-deps.sh`.

## Repository and public surface

| Measurement | P6 baseline | Candidate | Change |
| --- | ---: | ---: | ---: |
| Go packages | 90 | 83 | -7 |
| Repository Go files | 519 | 498 | -21 |
| Total Go lines | 98,985 | 92,599 | -6,386 |
| Non-test Go lines | 53,082 | 48,782 | -4,300 |
| Test Go lines | 45,903 | 43,817 | -2,086 |
| Documented public functions, methods, and types in root + `pkg/*` | 1,017 | 817 | -200 |

The exported-declaration measurement deliberately uses the same reproducible
`go doc -all` rule for both trees. It counts documented exported functions,
methods, and types; it does not estimate API size from raw capitalized tokens.
The 19.7% reduction comes from deleting zero-consumer packages and broad
contracts. It does not remove the immutable catalog, source observation,
generation-store, acquisition, artifact, or verified remote primitives retained
by the control plane.

## Build and access performance

Both binaries were built with the same toolchain and host. `go build -a` forces
package recompilation while reusing the same module download cache.

| Measurement | P6 baseline | Candidate | Change |
| --- | ---: | ---: | ---: |
| Forced `go build -a -trimpath ./cmd/starmap`, real time | 8.94 s | 8.76 s | -0.18 s |
| CLI binary, `-trimpath` | 39,338,130 bytes | 39,213,106 bytes | -125,024 |
| CLI binary, `-trimpath -ldflags='-s -w'` | 29,243,202 bytes | 29,141,106 bytes | -102,096 |
| `BenchmarkClientCatalog`, five runs | 10.35–10.97 ns/op at P6.2 | 10.40–10.91 ns/op | no regression |
| `BenchmarkClientCatalog` allocation | 0 B/op, 0 allocs/op | 0 B/op, 0 allocs/op | unchanged |

The CLI still intentionally includes explicit acquisition and concrete provider
clients, so the large root-closure reduction is not expected to produce a large
binary reduction. The small binary improvement is consistent with deleting
unused scheduler, hosted-distribution, evidence-archive, enhancer, compatibility,
and pass-through code while retaining CLI acquisition.

## Reproduction

```bash
go version
go env GOOS GOARCH

go list -deps -f '{{.ImportPath}}' . | sort -u | wc -l
go list -deps -f '{{.ImportPath}}' ./pkg/catalogs | sort -u | wc -l
go list -deps -f '{{.ImportPath}}' \
  ./pkg/catalogs ./pkg/catalogstore | sort -u | wc -l
go list -deps -f '{{.ImportPath}}' ./internal/server | sort -u | wc -l
go list -deps -f '{{.ImportPath}}' \
  ./internal/providers/google | sort -u | wc -l

go list ./... | wc -l
rg --files -g '*.go' | wc -l
rg --files -g '*.go' -0 | xargs -0 wc -l
rg --files -g '*.go' -g '!*_test.go' -0 | xargs -0 wc -l
rg --files -g '*_test.go' -0 | xargs -0 wc -l

for package in $(go list . ./pkg/...); do
  go doc -all "$package"
done | awk \
  '/^func [A-Z]/ || /^func \([^)]*\) [A-Z]/ || /^type [A-Z]/ { count++ }
   END { print count+0 }'

/usr/bin/time -p env GOWORK=off \
  go build -a -trimpath -o /tmp/starmap-p6-current ./cmd/starmap
go build -trimpath -ldflags='-s -w' \
  -o /tmp/starmap-p6-current-stripped ./cmd/starmap

make test-consumer-deps
go test -run '^$' -bench '^BenchmarkClientCatalog$' -benchmem -count=5 .
```

The baseline was extracted with `git archive` and measured with the same
commands outside the active worktree.
