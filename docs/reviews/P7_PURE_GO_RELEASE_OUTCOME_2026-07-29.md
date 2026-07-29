# P7 Pure-Go and Static Release Outcome

Date: 2026-07-29

Status: `ACCEPTED` — supported Starmap library, store, server, remote, CLI,
application-release, container, and Homebrew compositions now have an explicit
cgo policy and executable verification.

## Contract

Starmap is a pure-Go product and does not require a C compiler or a
deployment-provided C runtime:

- repository-authored Go source has no `import "C"`;
- the SQLite test adapter is `modernc.org/sqlite`, not a cgo driver;
- read-only, store-only, server-embed, and remote-subscriber external modules
  compile and execute with `CGO_ENABLED=0`;
- the CLI builds and executes with `CGO_ENABLED=0`;
- GoReleaser archive and container builds explicitly set `CGO_ENABLED=0`;
- every raw release binary records `CGO_ENABLED=0` in Go build metadata;
- Linux release binaries have neither an ELF program interpreter nor dynamic
  `NEEDED` libraries;
- Windows release binaries import no MSVC/UCRT, libgcc, or libstdc++ runtime;
  and
- Darwin/Homebrew binaries may use only Apple system ABI libraries under
  `/usr/lib` and `/System/Library`, never a separately distributed C runtime.

The race suite is deliberately separate and explicitly uses `CGO_ENABLED=1`
because Go's race detector normally requires cgo. Passing the race gate does
not define the release build mode.

## Dependency-budget relationship

On `linux/amd64`, enabling cgo adds exactly one package to the read-only
consumer closure: standard-library `runtime/cgo`. There is no cgo-off-only
package. Both modes contain exactly 31 non-standard packages, so the
platform-independent 31/32 architecture budget is unchanged. The complete
closure still drives the existing forbidden-family scan.

## Verification implementation

`scripts/verify-pure-go.sh`, exposed as `make test-pure-go` and invoked by
`scripts/verify.sh`, performs the focused product gate:

1. reject any repository Go source importing `C`;
2. execute all four external consumer modules with `CGO_ENABLED=0`;
3. build and run the CLI with `CGO_ENABLED=0`;
4. inspect its Go build metadata; and
5. verify Linux static linkage or Darwin system-only dynamic linkage on the
   host platform.

`scripts/verify-release-binaries.sh` inspects the six raw GoReleaser binaries,
requires the exact Darwin/Linux/Windows amd64/arm64 matrix, verifies cgo-disabled
build metadata, proves Linux static linkage, rejects Windows C/C++ runtime
imports, and checks Darwin system-only linkage when run on macOS.

The release workflow runs that verifier before checksums, attestations, or
draft publication. Stable Homebrew verification independently inspects the
installed binary's cgo-disabled metadata and Darwin library set.

## Exact local evidence

```text
./scripts/verify-pure-go.sh
  read-only: 31/32 non-standard packages
  store-only: publication passed
  server-embed: 247/260 packages
  remote-subscriber: 231/240 packages
  starmap dev
  pure-Go verification passed

go test ./internal/ciworkflow -race -count=1
  PASS

make goreleaser-check
  GoReleaser 2.17.0
  1 configuration file validated

make release-snapshot-devbox
  six Darwin/Linux/Windows amd64/arm64 binaries
  six archives and six archive SBOMs
  Homebrew cask generated

./scripts/verify-release-binaries.sh dist
  verified 6 cgo-disabled release binaries
  Linux is static
  Windows imports no C/C++ runtime
```

The exact pre-portability correction candidate also completed
`./scripts/verify.sh`: ordinary root tests passed in `89.859s`; the cgo-enabled
root race suite passed in `494.523s`; the catalog accessor measured
`8.278–8.735 ns/op`, `0 B/op`, and `0 allocs/op`; vet, pinned lint with zero
issues, every coverage floor, documentation, diff, build, 610-record catalog
validation, and CLI smoke tests all passed.

The final local candidate repeated `./scripts/verify.sh` with the new ordering:
ordinary root tests passed in `64.340s`; the cgo-off composition gate passed;
the explicitly cgo-enabled root race suite passed in `278.626s`; the accessor
measured `8.862–11.60 ns/op`, `0 B/op`, and `0 allocs/op`; vet, pinned lint
with zero issues, every coverage floor, documentation, diff, build, all 610
catalog records, and CLI smoke tests passed. Pinned govulncheck v1.6.0 found
zero reachable and zero imported-package vulnerabilities; one required module
contains an unreachable vulnerability.

The final committed head must now pass both hosted required checks.
