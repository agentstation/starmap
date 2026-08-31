# Testing and Verification

This document defines the verification model for Starmap. High line coverage
alone is not the goal. Tests must exercise important modules through stable
interfaces and repeat checks of production reliability properties.

## Primary Gate

Run the full deterministic repository verification gate before merging architecture, sync, catalog, provider, reconciliation, server, or transport changes:

```bash
make verify
```

`make verify` runs:

- `go test ./...`
- `go test ./... -race -short -timeout=20m`
- `go vet ./...`
- exact pinned `golangci-lint` verification
- critical boundary coverage thresholds
- `make docs-check` (generated Go documentation and embedded OpenAPI schemas.
  OpenAPI reproduction uses the Makefile-pinned Swag version and does not
  require an ambient `swag` binary)
- `git diff --check`
- binary build plus local CLI smoke checks

The smoke checks do not call provider APIs. They verify that the binary starts, `starmap validate catalog` works against the embedded catalog, provider listing works, and model listing works.

## Fast Local Checks

Use these while iterating:

```bash
go test ./...
go test ./... -race -short -timeout=20m
go vet ./...
make docs-check
make test-critical-coverage
```

Use focused packages while editing a module:

```bash
go test ./internal/catalog/pipeline ./pkg/sync .
go test ./internal/sources/providers ./internal/providers/clients ./pkg/sources
go test ./internal/catalog/query ./internal/server/params ./internal/server/handlers
go test ./pkg/catalogs/authority ./internal/catalog/reconciler
go test ./internal/server/sse ./internal/server/middleware ./internal/server
go test ./pkg/catalogs -race
```

## Critical Boundary Coverage

Global coverage is intentionally not the primary trust metric. CLI command constructors, generated packages, and optional integrations dilute the signal. Starmap instead enforces coverage on modules where correctness and production reliability concentrate:

| Module | Minimum |
| --- | ---: |
| `internal/catalog/pipeline` | 70% |
| `internal/catalog/query` | 75% |
| `internal/providers/clients` | 80% |
| `internal/sources/providers` | 75% |
| `internal/server/middleware` | 90% |
| `internal/server/openrouter` | 85% |
| `internal/server/params` | 95% |
| `internal/server/response` | 95% |
| `internal/server/sse` | 90% |
| `internal/transport` | 40% |
| `pkg/catalogs/authority` | 90% |
| `pkg/catalogs` | 55% |
| `pkg/errors` | 80% |
| `internal/catalog/reconciler` | 75% |
| `pkg/sources` | 35% |

The immutable catalog build derives author membership. The `pkg/catalogs` gate
and behavior-focused tests cover that derivation. It has no separate package
threshold because there is no separate
runtime attribution module.

Raise these thresholds when a module gets stronger tests. Do not lower them to pass a change without documenting the reason.

## Boundary Expectations

Tests should cross the same interface callers use:

- Catalog ownership: use public collection methods and mutate returned values to prove deep-copy boundaries.
- Sync pipeline: inject fake source/store adapters and assert ordering, persistence, error policy, and dry-run behavior.
- Provider source: inject fake provider clients and assert credential loading, bounded concurrency, partial failures, and catalog association.
- Provider clients: use `httptest` and testdata. Never call external APIs from ordinary unit tests.
- Query modules: test filtering, provider alias membership, pagination, and sorting without HTTP or Cobra.
- HTTP handlers: test request/response translation, cache behavior, and error mapping without retesting query internals.
- Reconciliation: assert field-rule coverage, authority resolution, provenance names, and resource-specific merge behavior.
- SSE publication: test serialized writes, flushed heartbeats, write deadlines,
  disconnect-on-backpressure, and cleanup.
- SSE transport: use real HTTP when flushing behavior affects race safety.

## Source Completeness Tests

Source completeness is a schema contract, not a best-effort parser behavior.
Classify each source attribute as canonical, extension-preserved, or ignored
with a stated reason.

Use these focused checks when changing provider clients, models.dev parsing, reconciliation rules, or catalog schema:

```bash
go test ./internal/sources/modelsdev ./internal/providers/...
go test ./pkg/catalogs ./internal/catalog/reconciler ./pkg/catalogs/authority
go test ./internal/catalog/query ./internal/server/params ./cmd/starmap/cmd/models
```

The source-shape tests normalize JSON paths, collapse array indexes to `[]`, and fail when a fixture contains an unclassified path. Mapping tests then prove important fields survive conversion, deep copy, YAML/JSON round-trip, reconciliation, and query/detail output.

## Catalog accessor performance

Run `make test-catalog-performance` to verify the public `Client.Catalog()` fast
path. The gate runs `BenchmarkClientCatalog` three times and requires every run
to remain at zero bytes and zero allocations per operation. Each run has a 10
microsecond latency ceiling. That ceiling is wider than the measured
nanosecond-scale result. It remains portable across CI hosts while detecting
a regression to full-catalog copying.

Run race tests separately. Race
instrumentation is not valid allocation-budget evidence.

Live shape investigation and governed fixture refresh are separate opt-in
workflows. Store exploratory provider or models.dev payloads under `/tmp` and
commit only reviewed field classifications. Use
`make testdata PROVIDER=<provider-id>` only when the full response must remain
as governed replay evidence for a current wire or mapping contract. The command
loads the selected provider record and catalog-acquisition credential metadata
from embedded YAML. Ordinary tests never call provider APIs.

## Catalog generation safety

Run `make catalog-generation-check` before changing embedded catalog tooling.
The gate exercises an HTTP-error response. It verifies that failure preserves
the current embedded models.dev payload. It requires typed and semantic source
validation before an atomic file promotion and command-spies the public CLI.

The only supported update shape is a positional provider plus `--catalog-path`.
The generation workflow must finish with the actual `validate catalog`
subcommand. Provider fixture refresh failures and successful no-op refreshes
must both propagate nonzero.

The refresh contract also proves that a selected
provider update does not change sibling fixtures.

`make update-catalog` and `make update-catalog-provider PROVIDER=<id>` use the
same checked workflow. The models.dev download uses curl's HTTP failure mode.
The command first writes it to a temporary sibling. Syntactically valid JSON
alone does not cause promotion.

Run `make embedded-catalog-budget-check` to emit the versioned embedded-catalog
release policy and current measurements. The command reports generation age,
canonical payload bytes, compressed artifact bytes, provider count, and model
count. It rejects a future generation as a hard correctness failure. It reports
age and size review thresholds without rejecting the release. The command has
no environment override.

## Live Provider Verification

Live provider checks require credentials and are not part of the deterministic gate:

```bash
starmap deps check
starmap providers --test
starmap providers openai --test
make testdata PROVIDER=openai
make update PROVIDER=openai
```

Use live checks when changing provider clients, authentication, transport
behavior, or embedded catalog update workflows. Treat fixture payload and
metadata diffs as review artifacts. Do not accept a fixture diff until each
source field has a canonical, controlled-extension, or intentional-ignore
disposition.

## Release Readiness

Before release, run:

```bash
make verify
make test-pure-go
make release-check
```

`make test-pure-go` executes the external library, store, server, remote, and
CLI compositions with `CGO_ENABLED=0`. It also verifies local binary linkage.
`make verify` includes that gate, then runs the race suite separately with
`CGO_ENABLED=1`. `make release-check` adds release-specific CLI and exact
GoReleaser checks.
