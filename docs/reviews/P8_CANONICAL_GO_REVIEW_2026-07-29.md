# P8 Canonical Go Review

Date: 2026-07-29

Scope: P8.8 and findings F-110–F-114

Status: `ACCEPTED`

## Standard

This review closes the Go-quality gate after the storage, package, naming,
complexity, deletion, and test-modularity work. It checks:

- caller-owned configuration and lifecycle instead of accidental process-global
  mutation;
- standard wrapped-error classification and typed public error boundaries;
- caller context propagation and goroutine/resource ownership;
- real release-tool behavior rather than successful no-ops;
- deletion of shallow or mutable surfaces with no production use;
- current GoDoc, generated documentation, package naming, vet, lint, race, and
  exact composition gates.

Starmap has not launched, so each correction is a clean break. No compatibility
alias, deprecated wrapper, or duplicated implementation preserves the defective
surface.

## Findings and dispositions

### F-110 — logging and notification ownership

The public default logger was a mutable package value. Concurrent `SetDefault`
and logging raced; `SetDefault` also changed zerolog's separate global logger,
and logger construction changed zerolog's process-global level. The config
constructor accepted a file path even though it returned no closer, leaving the
file descriptor caller-uncloseable. Its `auto` format path also called `Stat`
through a nil `*os.File` for `discard` output. Separately:

- internal HTTP errors bypassed the logger injected into the embeddable server;
- the CLI notifier exposed an unsynchronized, lazily initialized global despite
  having no production caller; and
- provider tests reassigned process-global `os.Stderr`, launched an unjoined pipe
  copier, and could interfere with unrelated goroutines.

Disposition:

- the default logger is atomically replaced and returned by value, so callers
  cannot mutate published state through a retained pointer;
- Starmap no longer changes either zerolog global;
- the CLI executable explicitly installs its post-flag logger at the process
  composition root so `--quiet` and `--log-level` still govern package
  diagnostics, while constructing an `App` remains side-effect free;
- `logging.New` supports only caller-owned standard streams or discard, never
  opens a file, and safely detects only real terminal files;
- HTTP error rendering receives the server's injected logger;
- the dead notifier globals and all convenience wrappers are deleted; and
- provider testing calls the composed clients directly without process I/O
  redirection.

Race tests exercise concurrent default replacement/read. Regression tests prove
discard/auto construction, absence of global-level mutation and hidden files,
and injected server diagnostics without response disclosure.

### F-111 — wrapped and typed errors

`response.ErrorFromType` used a direct type switch, so an idiomatically wrapped
not-found or provider API error became an incorrect 500. `provenance.Load`
performed a check-then-read filesystem sequence and returned untyped
`fmt.Errorf` values from a public package.

Disposition:

- HTTP classification uses standard `errors.As`, including wrapped typed
  errors;
- provenance performs one read, preserves the optional missing-file contract,
  and returns typed `IOError` or `ParseError`; and
- table-driven tests cover wrapped 404/4xx behavior and all provenance outcomes.

A scan of non-internal production packages finds no other direct untyped error
return; the one local JSON multi-document error is immediately wrapped in a
public `ValidationError`.

### F-112 — release manpage was a no-op

GoReleaser and `make` invoked `starmap man`, but the command returned success
without emitting a manual. Releases could therefore contain an empty compressed
artifact while every hook remained green.

Disposition:

- the hidden command now generates the root Cobra manual to its configured
  stdout and rejects positional arguments;
- a compile-time command test asserts the title, NAME, product description, and
  SYNOPSIS; and
- the exact GoReleaser pipeline shape produces a valid non-empty gzip stream.

The two newly materialized Markdown-renderer modules are CLI/release-only
transitive dependencies of Cobra's man generator. The read-only/root composition
budget remains unchanged.

### F-113 — context and goroutine ownership

Two CLI commands replaced `cmd.Context()` with `context.Background()`, so
cancellation did not reach dependency checks or `gcloud` subprocesses. The
Vertex listing wrapped an already context-aware SDK call in a detached timeout
goroutine.

Disposition:

- dependency checks, authentication probes, login, and project configuration
  propagate the Cobra command context;
- the bounded Vertex operation calls the context-aware SDK directly and
  classifies cancellation separately from deadline expiry; and
- the redundant goroutine is deleted.

The remaining credential-detection goroutine is an explicit adapter boundary:
the upstream `DetectDefault` API accepts no context. It has a single buffered
result, never blocks on delivery, and bounds the caller to two seconds or the
caller context. Long-lived Starmap-owned remote, SSE, and server loops retain
their P7 joinable lifecycle proof; concurrent source/provider workers use
bounded channels plus `WaitGroup` joins; callback panic recovery and HTTP panic
recovery remain deliberate trust boundaries.

### F-114 — residual mutable and dead surfaces

The public errors package exported `var New = errors.New`, allowing any importer
to replace process-wide error construction. It also retained an entirely unused
`MergeError` type/constructor. Internal transport exposed a mutable
`DefaultHTTPTimeout` variable with no caller. These were leftover convenience
surfaces, not product seams.

Disposition:

- callers use the standard library to construct plain local errors;
- the dead merge-error surface is deleted;
- transport uses the canonical timeout constant directly; and
- no alias or compatibility layer remains.

The broader typed error taxonomy is retained because public store, remote,
server, source, and catalog contracts use `errors.Is`/`errors.As` against those
types and sentinels. This review does not replace useful typed domain errors
with strings.

## Retained boundary decisions

- CLI flag getter panics remain programming-invariant assertions for flags
  declared in the same command constructor. User input follows Cobra validation
  and typed command errors.
- Hook and HTTP middleware recovery remain at callback/request trust boundaries;
  application logic does not use panic/recover for control flow.
- Nil-context normalization remains only on documented public convenience
  boundaries. Internal commands now propagate their caller contexts.
- Test-only stdout/stderr capture remains serialized in one models.dev test and
  is not linked into production.
- Standard sentinel-error variables are retained as the normal Go identity
  pattern; mutable function and configuration variables are not.

## Verification

The final evidence is recorded in the control-plane Evidence Log on the exact
candidate head. Required proof includes:

```text
go test -race <affected packages> -count=10
go vet ./...
golangci-lint run --timeout=10m
make generate
make docs-check
make test-file-sizes
./scripts/verify-pure-go.sh
./scripts/verify.sh
git diff --check
```

The exact GoReleaser manpage shape is also executed:

```text
go run ./cmd/starmap man | gzip -c | gzip -t
```

P8 receives one structured outcome review after this complete coherent
candidate is green. It is repeated only if remediation materially changes
architecture, public API, concurrency, persistence, security, or failure
semantics.

## Structured outcome

The final P8.8 bundle received one independent structured review:

```text
autoreview --mode local --engine claude \
  --model claude-fable-5 --thinking max
```

The 132,283-byte bundle fit one pass. The reviewer found the product patch
correct and accepted one P3 documentation issue: the F-111 ledger row said
“with regressions” where it meant “with regression tests.” That wording is
corrected as F-115. No code or behavior changed, so the approved review cadence
does not call for another run.

A full `origin/main` branch bundle was attempted first. The helper refused it
before model invocation because the diff repeats secret-like but non-secret
fragments from deleted/moved baseline code, including an error message, an API
key type expression, a page-token field, and fixture identifiers. The
fail-closed scanner was not disabled or allowlisted. The earlier P8 tasks remain
covered by their dedicated archived audits and exact gates; this structured
pass covers every P8.8 closeout file.

The first subsequent exact gate passed completely and exposed one diagnostics
composition regression during smoke-output inspection: after global zerolog
mutation was removed, CLI flags no longer configured package-level Starmap
diagnostics. F-116 restores that behavior explicitly in the executable pre-run
through the atomic Starmap logger boundary. Fifty focused race repetitions pass.
This one-line composition-root correction changes no public API, architecture,
persistence, concurrency, security, or failure semantics, so the approved
outcome-oriented cadence does not rerun structured review. The full exact gate
was repeated on the corrected final tree and passed:

- the ordinary repository suite passed, including the root package in
  `44.016s`;
- every external composition executed with `CGO_ENABLED=0`; the read-only
  closure improved to 30 of the 32-package non-standard budget and the S3
  package remained outside ordinary closures;
- the complete cgo-enabled race suite passed, including the root package in
  `265.056s`, the CLI application in `80.778s`, the server in `109.987s`,
  models.dev in `72.977s`, and catalogs in `42.184s`;
- vet and pinned golangci-lint passed with zero issues;
- `BenchmarkClientCatalog` measured `8.954–10.25 ns/op`, `0 B/op`, and
  `0 allocs/op` across three runs;
- every enforced coverage floor passed, including 100% in server response
  classification;
- generated documentation, file-size policy, and `git diff --check` passed;
  the only files above 1000 lines remain the reviewed 1446-line merger contract
  and 1036-line OpenAI adapter contract;
- the CLI built and executed its version, provider, and model-list smokes; and
- the embedded catalog validated all 11 providers, 104 authors, 610 models,
  and their cross-references.

The exact manpage pipeline
`go run ./cmd/starmap man | gzip -c | gzip -t` also passed. P8.8 therefore
satisfies the canonical-Go closeout gate.
