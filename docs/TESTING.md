# Testing and Verification

This document defines the verification model for Starmap. The goal is not only high line coverage; the goal is proof that the important modules are testable through stable interfaces and that production reliability properties are checked repeatedly.

## Primary Gate

Run the full deterministic enterprise gate before merging architecture, sync, catalog, provider, reconciliation, server, or transport changes:

```bash
make verify
```

`make verify` runs:

- `go test ./...`
- `go test ./... -race -short`
- `go vet ./...`
- `golangci-lint run` when available
- critical seam coverage thresholds
- `make docs-check`
- `git diff --check`
- binary build plus local CLI smoke checks

The smoke checks do not call provider APIs. They verify that the binary starts, `starmap validate catalog` works against the embedded catalog, provider listing works, and model listing works.

## Fast Local Checks

Use these while iterating:

```bash
go test ./...
go test ./... -race -short
go vet ./...
make docs-check
make test-critical-coverage
```

Use focused packages while editing a module:

```bash
go test ./internal/catalog/pipeline ./pkg/sync .
go test ./internal/sources/providers ./internal/providers/clients ./pkg/sources
go test ./internal/catalog/query ./internal/server/params ./internal/server/handlers
go test ./pkg/authority ./pkg/reconciler
go test ./internal/server/events ./internal/server/sse ./internal/server/websocket
go test ./pkg/catalogs -race
```

## Critical Seam Coverage

Global coverage is intentionally not the primary trust metric. CLI command constructors, generated packages, and optional integrations dilute the signal. Starmap instead enforces coverage on modules where correctness and enterprise reliability concentrate:

| Module | Minimum |
| --- | ---: |
| `internal/attribution` | 85% |
| `internal/attribution/matcher` | 75% |
| `internal/catalog/pipeline` | 70% |
| `internal/catalog/query` | 75% |
| `internal/providers/clients` | 80% |
| `internal/sources/providers` | 75% |
| `internal/server/events` | 70% |
| `internal/server/middleware` | 90% |
| `internal/server/params` | 95% |
| `internal/server/response` | 95% |
| `internal/server/sse` | 90% |
| `internal/server/websocket` | 85% |
| `internal/transport` | 40% |
| `pkg/authority` | 90% |
| `pkg/catalogs` | 55% |
| `pkg/errors` | 80% |
| `pkg/reconciler` | 75% |
| `pkg/sources` | 35% |

Raise these thresholds when a module gets stronger tests. Do not lower them to pass a change without documenting the reason.

## Seam Expectations

Tests should cross the same interface callers use:

- Catalog ownership: use public collection methods and mutate returned values to prove deep-copy boundaries.
- Sync pipeline: inject fake source/store adapters and assert ordering, persistence, error policy, and dry-run behavior.
- Provider source: inject fake provider clients and assert credential loading, bounded concurrency, partial failures, and catalog association.
- Provider clients: use `httptest` and testdata; never call external APIs from ordinary unit tests.
- Query modules: test filtering, provider alias membership, pagination, and sorting without HTTP or Cobra.
- HTTP handlers: test request/response translation, cache behavior, and error mapping without retesting query internals.
- Reconciliation: assert field-rule coverage, authority resolution, provenance names, and resource-specific merge behavior.
- Event fan-out: test backpressure policy, initialization order, and race safety.

## Source Completeness Tests

Source completeness is a schema contract, not a best-effort parser behavior. Each source attribute must be classified as canonical, extension-preserved, or intentionally ignored with a reason.

Use these focused checks when changing provider clients, models.dev parsing, reconciliation rules, or catalog schema:

```bash
go test ./internal/sources/modelsdev ./internal/providers/...
go test ./pkg/catalogs ./pkg/reconciler ./pkg/authority
go test ./internal/catalog/query ./internal/server/params ./cmd/starmap/cmd/models
```

The source-shape tests normalize JSON paths, collapse array indexes to `[]`, and fail when a fixture contains an unclassified path. Mapping tests then prove important fields survive conversion, deep copy, YAML/JSON round-trip, reconciliation, and query/detail output.

## Catalog accessor performance

Run `make test-catalog-performance` to verify the public `Client.Catalog()` fast
path. The gate runs `BenchmarkClientCatalog` three times and requires every run
to remain at zero bytes and zero allocations per operation with a 10 microsecond
latency ceiling. The ceiling is deliberately much wider than the measured
nanosecond-scale result so it is portable across CI hosts while still detecting
a regression to full-catalog copying. Run race tests separately; race
instrumentation is not valid allocation-budget evidence.

Live shape refreshes are opt-in. Store raw provider or models.dev payloads under `/tmp`, load credentials without printing them, and commit only minimized fixtures or normalized path classifications after deciding each field outcome.

## Catalog generation safety

Run `make catalog-generation-check` before changing embedded catalog tooling.
The gate exercises an HTTP-error response, verifies the current embedded
models.dev payload is retained on failure, requires typed and semantic source
validation before an atomic file promotion, and command-spies the public CLI.
The only supported update shape is a positional provider plus `--output-dir`;
the generation workflow must finish with the actual `validate catalog`
subcommand. Provider fixture refresh failures and successful no-op refreshes
must both propagate non-zero.

`make update-catalog` and `make update-catalog-provider PROVIDER=<id>` use the
same checked workflow. The models.dev download uses curl's HTTP failure mode,
is first written to a temporary sibling, and is never promoted merely because
the response body is syntactically valid JSON.

Run make embedded-catalog-budget-check to emit the exact embedded generation
age, uncompressed payload bytes, compressed artifact bytes, and provider/model
coverage. Threshold breach fixtures cover each stable failure code. Any CI
threshold override must carry a review reason that is recorded in the report.

## Live Provider Verification

Live provider checks require credentials and are not part of the deterministic gate:

```bash
starmap deps check
starmap providers --test
starmap providers openai --test
make testdata PROVIDER=openai
make update PROVIDER=openai
```

Use live checks when changing provider clients, authentication, transport behavior, or embedded catalog update workflows. Treat generated testdata diffs as review artifacts.

## Release Readiness

Before release, run:

```bash
make verify
make release-check
```

`make release-check` adds release-specific CLI and GoReleaser checks. `make verify` remains the broader correctness gate.
