# P10 Production Budgets

Date: 2026-07-29

Measured candidate: `c74bf099` plus the two benchmark-only test files described
below.

Toolchain and host: `go1.26.5 darwin/arm64`, Apple M2 Max, macOS 15.7.2.

## Outcome

The request hot path remains O(1) and allocation-free. A complete catalog
publication and a complete verified remote activation are explicitly measured
as off-request-path control-plane operations. Both operate on the production
2,603,614-byte embedded generation and remain within the review budgets below.

The mutation measurements deliberately include the work required for safety:
deterministic payload processing, manifest and digest validation, defensive
ownership, generation-store compare-and-swap, and atomic immutable activation.
They do not justify moving any of that work onto a request path or weakening an
ownership boundary.

| Boundary | Review budget | Five-run measurement |
| --- | ---: | ---: |
| `Client.Catalog()` | <=10 us/op; 0 B/op; 0 allocs/op | 8.711–9.608 ns/op; 0 B/op; 0 allocs/op |
| Complete `Client.Update` publication | <=250 ms/op; <=64 MiB/op | 63.742–66.386 ms/op; 37,802,096–41,656,008 B/op |
| Complete remote generation activation | <=500 ms/op; <=160 MiB/op | 178.135–180.575 ms/op; 112,821,314–112,839,010 B/op |

The accessor budget is a portable hard CI gate. The two mutation latency limits
are review thresholds rather than cross-runner SLOs because they measure a
complete 2.6 MB catalog on one named host. Their allocation ceilings are
architecture-regression thresholds: exceeding one requires profiling and a
recorded disposition before release. Current activation allocation is
deliberately visible because bounded, infrequent catalog activation trades
temporary memory for validation, quarantine, deep immutable ownership, and
retained durable bytes. It must not become proportional work on
`Client.Catalog()` or model lookup.

## Compile and consumer closures

Counts are unique packages from `go list -deps`.

| Composition | Measured | Enforced ceiling |
| --- | ---: | ---: |
| Root package | 151 total | informational |
| `pkg/catalogs` | 140 total | informational |
| `pkg/catalogs` + `pkg/catalogstore` union | 143 total | informational |
| Internal server | 238 total | informational |
| Google provider | 447 total | informational |
| External read-only consumer | 30 non-standard; 152 total | 32 non-standard |
| External pinned-artifact consumer | 31 non-standard | 32 non-standard |
| External server consumer | 240 total | 260 total |
| External remote consumer | 224 total | 240 total |
| External filesystem/S3 server consumer | 332 total | 340 total |

Every external composition compiled and executed. The read-only and remote
closures contain no forbidden acquisition, server, database, cloud SDK,
WebSocket, Cobra, or other implementation families named by their gates. The
server-storage composition alone opts into the S3-compatible SDK.

## Binary and embedded generation

The CLI was built with `GOWORK=off`, `CGO_ENABLED=0`, and `-trimpath`.

| Measurement | Result |
| --- | ---: |
| Forced clean build time | 12.08 s real |
| CLI binary | 39,741,538 bytes |
| CLI binary with `-ldflags='-s -w'` | 29,523,698 bytes |
| Embedded generation, uncompressed | 2,603,614 bytes |
| Embedded generation, compressed | 121,547 bytes |
| Embedded providers | 11 |
| Embedded canonical definitions | 589 |

Both Darwin binaries are pure-Go builds that require no C compiler. As is
normal on Darwin, `otool -L` reports only operating-system libraries and
frameworks (`libSystem`, `libresolv`, CoreFoundation, and Security); the
cross-target release gate separately verifies target-appropriate linkage for
all release binaries.

## Repository scale and file policy

| Measurement | Result |
| --- | ---: |
| Go packages | 85 |
| Repository Go files | 532 |
| Total Go lines | 99,683 |
| Non-test Go lines | 51,995 |
| Test Go lines | 47,688 |
| Exported declarations in root plus `pkg/*` | 693 |
| Exported declarations including public `server` and `remote` | 724 |
| Largest production Go file | `pkg/catalogs/read_views.go`, 982 lines |
| Largest test Go file | `internal/catalog/reconciler/merger_test.go`, 1,446 lines |

No production file enters the review band, no file enters the 1,501-line
justification band, and no file approaches the 2,000-line hard limit. The
1,446-line reconciliation test and 1,036-line OpenAI client test retain their
recorded modularity dispositions.

## Reproduction

```bash
go version
go env GOOS GOARCH CGO_ENABLED
sw_vers

go test -run '^$' -bench '^BenchmarkClientCatalog$' -benchmem -count=5 .
go test -run '^$' -bench '^BenchmarkClientUpdatePublication$' \
  -benchmem -benchtime=3x -count=5 .
go test -run '^$' -bench '^BenchmarkSubscriberActivation$' \
  -benchmem -benchtime=3x -count=5 ./remote

go list -deps -f '{{.ImportPath}}' . | sort -u | wc -l
go list -deps -f '{{.ImportPath}}' ./pkg/catalogs | sort -u | wc -l
go list -deps -f '{{.ImportPath}}' \
  ./pkg/catalogs ./pkg/catalogstore | sort -u | wc -l
go list -deps -f '{{.ImportPath}}' ./internal/server | sort -u | wc -l
go list -deps -f '{{.ImportPath}}' \
  ./internal/providers/google | sort -u | wc -l
make test-consumer-deps

go list ./... | wc -l
rg --files -g '*.go' | wc -l
rg --files -g '*.go' -0 | xargs -0 wc -l
rg --files -g '*.go' -g '!*_test.go' -0 | xargs -0 wc -l
rg --files -g '*_test.go' -0 | xargs -0 wc -l
make test-file-sizes
make embedded-catalog-budget-check

CGO_ENABLED=0 go build -a -trimpath -o /tmp/starmap-p10 ./cmd/starmap
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' \
  -o /tmp/starmap-p10-stripped ./cmd/starmap
stat -f '%z' /tmp/starmap-p10 /tmp/starmap-p10-stripped
otool -L /tmp/starmap-p10 /tmp/starmap-p10-stripped
```
