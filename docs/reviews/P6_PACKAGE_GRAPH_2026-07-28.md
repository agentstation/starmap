# P6 Go Package Graph Review

Date: 2026-07-28

Reviewed commit:
`76dd317810815604b6c796814bce5b8887aaadd0`

Status: `ACCEPTED` — this review is the P6.1 package inventory and the input to
P6.2–P6.8. It records current facts and explicit dispositions; a `KEEP`
decision does not exempt a package from the later P8 residual-surface audit.

## Outcome

The repository contains 90 Go packages, one more than the P2 baseline of 89.
The increase is explained by three required lifecycle boundaries replacing two
obsolete attribution packages:

- added `cmd/starmap/cmd/migrate` for explicit typed workspace migration;
- added `internal/catalog/workspace` for the human-YAML lifecycle and atomic
  projection boundary;
- added `internal/sources/embedded` so the embedded catalog is a distinct
  lowest-authority observation;
- removed `internal/attribution` and `internal/attribution/matcher` after
  author membership became a derived catalog view.

`go list -deps ./...` succeeds, proving that the current graph has no import
cycles. The number of consumer-facing library paths is unchanged at 23: the
root package plus 22 `pkg/*` packages.

The root package remains much too broad for read-only use. Its 472-package
closure includes provider acquisition through this path:

```text
starmap
└── pkg/sources
    └── internal/providers/clients
        └── internal/providers/google
            └── google.golang.org/genai
                ├── github.com/gorilla/websocket
                └── cloud auth / gax
                    ├── google.golang.org/grpc
                    └── go.opentelemetry.io/otel
```

Removing only `pkg/sources -> internal/providers/clients` is insufficient. A
simulated graph cut still contained 244 packages because the root package also
compiles the acquisition pipeline, models.dev, remote HTTP, and `net/http`.
P6.2 and P6.3 must therefore form one coherent composition change: the root
owns catalog state and atomic publication, while an explicit opt-in acquisition
package owns source execution and concrete provider clients.

## Measured Dependency Budget

All measurements were taken from an isolated `git archive` of the reviewed
commit so uncommitted worktree changes could not affect the result.

| Closure | P2 baseline | Reviewed commit | Change |
| --- | ---: | ---: | ---: |
| Root `starmap` | 472 | 472 | 0 |
| `pkg/catalogs` | 145 | 147 | +2 |
| `pkg/catalogs` + `pkg/catalogstore` union | 149 | 152 | +3 |

The root closure contains 214 standard-library, 33 Starmap-local, and 225
external packages. Its forbidden dependency inventory is:

| Dependency family | Packages present |
| --- | ---: |
| Google GenAI | 1 |
| gRPC | 64 |
| OpenTelemetry | 21 |
| Gorilla WebSocket | 1 |
| SQLite implementations | 0 |
| Cobra | 0 |
| `pkg/catalogscheduler` | 0 |
| `internal/server` | 0 |

The core union remains below the P6 ceiling of 160, with eight packages of
headroom. P6.2 must make the same ceiling and forbidden-family assertions
machine-enforced for the root read-only consumer.

## Consumer-Facing Library Packages

“Consumer” names a current production importer or an explicit planned product
composition. A package with no legitimate consumer receives a terminal
deletion or internalization disposition.

| Package | Role | Named production consumer | Disposition |
| --- | --- | --- | --- |
| `starmap` | Concrete immutable-catalog client and atomic publication owner | CLI composition, current server, external Go library consumers | `KEEP`; remove acquisition imports in P6.2/P6.3 and complete construction contracts in P6.8 |
| `pkg/authority` | Executable field authority policy | Catalog read-view derivation and reconciler | `KEEP`; prefer concrete policy values and interfaces at algorithm inputs |
| `pkg/catalogartifact` | Deterministic catalog archive and attestation format | Catalog release tool and embedded-budget verifier | `KEEP` through P9/P11 release and import proof |
| `pkg/catalogdistribution` | Alternative hosted repository/client protocol | None outside its own package | `DELETE` in P6.5 |
| `pkg/catalogmeta` | Shared source, revision, resource, and generation vocabulary | Root, catalogs, sources, scheduler, provenance, reconciler | `KEEP` as foundational vocabulary |
| `pkg/catalogremote` | Current verified online generation fetch protocol | Root update path and server catalog handler | `KEEP/REWORK` in P7 into the sole verified reactive remote composition |
| `pkg/catalogs` | Mutable construction and concrete immutable canonical catalog | Root client, store, reconciliation, sources, server, CLI | `KEEP` as the core product |
| `pkg/catalogscheduler` | Lease, retry, run-ledger, and operational projection | CLI app only | `DELETE` in P6.5; deployment scheduling stays above the library and P7 owns server operational health |
| `pkg/catalogstore` | Immutable generation CAS storage | Root client, bootstrap, workspace, artifact, remote | `KEEP`; Memory, Filesystem, SQL, and Object stores are real adapters |
| `pkg/constants` | Cross-package defaults and implementation constants | 26 repository implementation packages | `REVIEW/SPLIT` in P8.4/P8.6; retain only genuine public contract constants |
| `pkg/convert` | Catalog-to-OpenAI/OpenRouter presentation conversion | CLI models command only | `INTERNALIZE` under its consumer in P8.6 unless Starport becomes a direct package consumer |
| `pkg/differ` | Catalog changesets | Root publication, pipeline, reconciler, sync result | `KEEP`; remove dead exported operations in P8.3/P8.6 |
| `pkg/enhancer` | Optional post-reconciliation enrichment pipeline | Reconciler constructs only an empty pipeline; no enhancer is configured | `DELETE` in P6.5 |
| `pkg/errors` | Typed public failure contract | Root and 40+ implementation packages | `KEEP` |
| `pkg/logging` | Zerolog and context integration | Root, CLI, pipeline, providers, transport | `KEEP`; trim global or test-only exports in P8.6 |
| `pkg/provenance` | Durable field evidence and reporting | Catalogs, store, reconciler, pipeline, CLI | `KEEP`; localize its tracker interface in P6.4 |
| `pkg/reconciler` | Multi-observation authoritative merge | Internal catalog pipeline | `KEEP` for the current algorithm boundary; P8.6 must prove public customization or internalize it |
| `pkg/save` | Path, format, and writer option pass-through | Root persistence, workspace projector, catalogs | `DELETE` in P6.5; move options to the APIs that own them |
| `pkg/sourceevidence` | Replayable normalized/raw source archive | None outside its own package | `DELETE` in P6.5 |
| `pkg/sourcepayload` | Bounded tolerant upstream decoding | Provider clients, models.dev, transport, store, sources | `KEEP` through P6.2/P6.3; P8.6 decides public plugin helper versus internal acquisition utility |
| `pkg/sources` | Source observation contract plus provider acquisition façade | Pipeline, reconciler, embedded/local/models.dev/provider sources | `KEEP` the observation contract; move provider acquisition and defaults to explicit composition in P6.2/P6.3 |
| `pkg/sync` | Public synchronization options and result | Root/acquisition path, CLI update, scheduler until deletion | `KEEP` |
| `pkg/types` | Compatibility aliases to `catalogmeta` | None | `DELETE` in P6.5 |

High-signal deletion evidence:

- `pkg/catalogdistribution`, `pkg/sourceevidence`, and `pkg/types` have zero
  non-test importers.
- `sources.NewSources`, `RegisterProviderClientFactory`, and
  `RegisterProviderRawFetcher` have zero non-test callers.
- No production code constructs an enhancer or passes one to reconciliation.
- `pkg/catalogscheduler` supplies only the CLI operational-state projection;
  no library or server consumer requires its scheduling machinery.

## Importable Command Packages

Command code is currently importable because its directory path is not
`internal`. These paths are not supported library APIs. P8.6 will internalize
them while preserving CLI behavior.

| Package | Role and named consumer | Disposition |
| --- | --- | --- |
| `cmd/starmap/app` | CLI composition root imported by `cmd/starmap` | Split the broad application role in P6.4; internalize in P8.6 |
| `cmd/starmap/cmd/auth` | Auth command imported by the CLI app | Preserve behavior; internalize in P8.6 |
| `cmd/starmap/cmd/authors` | Author query command imported by the CLI app | Preserve behavior; internalize in P8.6 |
| `cmd/starmap/cmd/completion` | Shell completion command imported by the CLI app | Preserve behavior; internalize in P8.6 |
| `cmd/starmap/cmd/deps` | Source dependency command imported by the CLI app | Preserve behavior; internalize in P8.6 |
| `cmd/starmap/cmd/embed` | Embedded-catalog command imported by the CLI app | Preserve behavior; internalize in P8.6 |
| `cmd/starmap/cmd/migrate` | Explicit transactional workspace migration imported by the CLI app | Preserve behavior; internalize in P8.6 |
| `cmd/starmap/cmd/models` | Model query/export command imported by the CLI app | Preserve behavior; internalize in P8.6 |
| `cmd/starmap/cmd/providers` | Provider query command imported by the CLI app | Preserve behavior; internalize in P8.6 |
| `cmd/starmap/cmd/serve` | Server command imported by the CLI app | Preserve the CLI adapter; P7 supplies the canonical public server |
| `cmd/starmap/cmd/update` | Synchronization command imported by the CLI app | Preserve behavior; internalize in P8.6 |
| `cmd/starmap/cmd/validate` | Validation command imported by the CLI app | Preserve behavior; internalize in P8.6 |

The five `main` packages (`cmd/starmap`,
`cmd/starmap-bootstrap-manifest`, `cmd/starmap-catalog-release`,
`cmd/starmap-embedded-budget`, and `cmd/starmap-modelsdev-promote`) are
non-importable programs. Each has a named CLI, verification, release, or
acquisition use and remains subject to the P8 residual review.

## Required P6 Composition

P6.2 and P6.3 use this target boundary:

```text
read-only Go program
        |
        v
starmap.Client ------> catalogs + catalogstore
        ^
        | publish verified immutable generation
        |
acquisition.Syncer --> source protocol + pipeline + concrete provider clients
        ^
        |
CLI / embeddable server composition
```

The explicit acquisition package must:

1. own concrete provider-client and raw-transport defaults;
2. inject the provider factory into the pipeline and provider source without a
   mutable global registry;
3. own provider, models.dev, and HTTP source execution;
4. return typed configuration failures when acquisition dependencies are
   absent;
5. leave construction and `Catalog()` complete without provider creation,
   network access, CLI, scheduler, or server dependencies.

Whether acquisition is a concrete `Syncer` wrapping `*starmap.Client` or an
injected root sync engine is decided by the smallest coherent API. The
acceptance criteria are behavioral and graph-based, not tied to either shape.

## Interface and Construction Inputs

The package graph audit also found concrete P6.4/P6.8 work:

- replace the 10-method `internal/application.Application` interface with
  consumer-local roles; its four build-metadata methods have no interface
  consumer;
- remove `Source.Name()` from the required source role because production code
  never calls it;
- retain the three-method `catalogstore.Store` interface and concrete
  `catalogremote.Client`; both already have useful, narrow seams;
- define and test nil-receiver behavior for `(*Client).Catalog()`;
- make storage-backed construction caller-cancellable through
  `NewContext(ctx, ...)`, with `New(...)` retaining the canonical convenience
  experience;
- replace AST-only external journey checks with real nested consumer modules
  compiled using `GOWORK=off`.

## Verification Commands

```bash
git rev-parse HEAD
go list ./... | wc -l
go list -deps ./... >/dev/null
go list -deps -f '{{.ImportPath}}' . | sort -u | wc -l
go list -deps -f '{{.ImportPath}}' ./pkg/catalogs | sort -u | wc -l
go list -deps -f '{{.ImportPath}}' \
  ./pkg/catalogs ./pkg/catalogstore | sort -u | wc -l

go list -deps -f \
  '{{if .Standard}}stdlib{{else if and .Module (eq .Module.Path "github.com/agentstation/starmap")}}local{{else}}external{{end}}' \
  . | sort | uniq -c

go mod why -m google.golang.org/genai
go mod why -m google.golang.org/grpc
go mod why -m go.opentelemetry.io/otel
go mod why -m github.com/gorilla/websocket
```

P6.2 must add a CI assertion that compiles an external
`starmap.New().Catalog()` consumer and rejects a root closure above 160 or any
GenAI, gRPC, OpenTelemetry, WebSocket, SQLite, Cobra, scheduler, or server
implementation dependency.

## Measurement correction

The 160-package statement above is retained as the historical P6.1 measurement
and decision. PR #55 later proved that a total-package ceiling includes
platform-specific standard-library internals: the same consumer measured
159 packages on Darwin and 163 on Linux with CGO, while its non-standard
closure remained exactly 31 on Darwin, Linux, and Windows.

The current prescriptive gate therefore enforces a ceiling of 32 non-standard
packages and retains the complete dependency closure for the unchanged
forbidden-family scan. See
[`P7_CONSUMER_DEPENDENCY_BUDGET_CORRECTION_2026-07-29.md`](P7_CONSUMER_DEPENDENCY_BUDGET_CORRECTION_2026-07-29.md)
for the exact failed hosted run, cross-target evidence, rationale, and
structural regression gate.
