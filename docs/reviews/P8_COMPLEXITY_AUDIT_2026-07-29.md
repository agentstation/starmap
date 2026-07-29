# P8 Complexity Audit

Date: 2026-07-29  
Baseline: `85b16d60d0cf030818bfaf43b8fd15354bc483c9`  
Tool: repository-pinned `golangci-lint v2.12.2`

## Decision

Use cognitive complexity above 30 as a review inventory, not as a mechanical
rewrite target. Extract a helper only when it names and contains a coherent
domain operation, owns an invariant, or removes shared mutable control flow.
Retain cohesive state machines, exhaustive codecs, and validation tables when
splitting them would create pass-through modules or scatter one invariant.

This review reduced the inventory from 20 functions to 10 without adding a
package, interface, public symbol, dependency, or compatibility wrapper.
Every removed hotspot was split along an existing domain boundary. The
remaining functions have explicit locality rationales below.

## Metric Selection

Three read-only measurements were run on the exact baseline:

```text
gocyclo, complexity >30: 0
gocognit, complexity >30: 20
cyclop, default maximum 10: 50
```

The same measurements on the candidate are:

```text
gocyclo, complexity >30: 0
gocognit, complexity >30: 10
cyclop, default maximum 10: 50
```

`gocyclo` at 30 found no large control-flow outlier. `cyclop` at its default
10 reports 50 functions, including small exhaustive validators, transport
dispatch, and feature inference. It did not change after the material
refactors and is too sensitive to legitimate branch enumeration to serve as an
architecture gate. `gocognit` at 30 correctly identified the mixed
responsibilities addressed here while retaining a small reviewable tail.

The repository should continue using full lint as the merge gate. This audit
does not add a complexity quota that would reward moving branches into shallow
helpers.

## Extracted Concepts

| Baseline function | Score | Candidate disposition |
| --- | ---: | --- |
| `(*catalogs.Builder).MergeWith` | 79 | Strategy dispatch now delegates to replace, enrich-empty, and append-only implementations; provider/model enrichment is separately named and mutation remains on `Builder` |
| `validateObservationLinks` | 51 | Link identity and revision-shape validation are separate invariants |
| `validateModelConsistency` | 45 | Command orchestration delegates provider-scoped validation and model issue collection |
| `deriveReadViews` | 45 | Authored candidates, provider offerings, definitions, and lineage normalization are explicit construction stages; the offering stage cannot mutate the authored-only definition candidate set |
| `(*catalogs.Builder).saveTo` | 44 | Filesystem transaction orchestration delegates index, provider-model, and authored-model projections through one private writer |
| `CopyAuthorLogos` | 43 | Per-author copy behavior delegates provider/author identity lookup and deterministic logo discovery |
| `processFetch` | 40 | Source orchestration delegates provider processing, issue collection, bounded model conversion, and deterministic key ordering |
| `mergeModelsDevProviderMetadata` | 38 | Catalog endpoint/docs, environment variables, and source extensions retain independent fill-empty rules |
| `buildDecodedCatalog` | 38 | Provider and author payload roles decode separately before one strict immutable build |
| `observe` | 38 | Per-source observation and failed-observation construction are isolated; goroutines return bounded results instead of sharing mutable slices under a mutex |

Every resulting function is below the review threshold. The source-observation
change also removes a concurrency seam: workers no longer coordinate through
shared `errs` and `observations` slices.

## Retained Hotspots

| Function | Score | Rationale |
| --- | ---: | --- |
| `starmap.NewContext` | 31 | One staged construction transaction owns option application, bootstrap/store initialization, validation, and cleanup on failure; its score is one point above the review threshold and splitting would obscure constructor rollback |
| `fetchProviderModels` | 31 | One CLI operation owns selected-provider acquisition, dry-run behavior, result reporting, and typed failure aggregation |
| `testProvidersConcurrent` | 40 | One CLI diagnostic owns bounded concurrent provider probes and their user-facing aggregate; it is not the production acquisition path and its concurrency behavior is tested as a unit |
| `(legacyLayoutMigrator).migrate` | 31 | One transactional migration state machine owns detection, staging, backup, rename, rollback, and failure preservation |
| `completion.Install` | 34 | One platform dispatch table owns shell detection and shell-specific installation destinations; extracting each branch would be pass-through indirection |
| `ParseModelFilter` | 37 | One compatibility parser exhaustively maps query fields while intentionally ignoring malformed legacy values; strict validation remains a separate API |
| `mergeFields` | 39 | One recursive reflection visitor exhaustively defines merge behavior by Go kind; splitting kind cases would scatter the codec |
| `postProcessModelYAML` | 39 | One line-oriented formatter owns section spacing, author-field cleanup, and scalar normalization; its existing suppression states the exhaustive-formatting rationale |
| `(*differ.Differ).model` | 36 | One exhaustive model diff algorithm preserves field-path locality and uses already-extracted field-family comparators; P8.6 independently decides whether the public differ surface is still used |
| `(*remote.Subscriber).run` | 36 | One joinable reconnect/catch-up/poll-fallback state machine owns transition ordering; splitting transitions would risk hiding the mandatory catch-up invariant |

These are review dispositions, not permanent exemptions. A future product
change that adds another responsibility should reopen the relevant item rather
than merely increasing its score.

## Behavioral Evidence

The candidate passed three race-enabled repetitions of every affected package:

```text
go test -race ./pkg/catalogs ./pkg/catalogstore \
  ./internal/catalog/pipeline ./internal/sources/modelsdev \
  ./cmd/starmap/cmd/validate -count=3

pkg/catalogs                         108.751s
pkg/catalogstore                      2.711s
internal/catalog/pipeline             12.412s
internal/sources/modelsdev           185.348s
cmd/starmap/cmd/validate               1.660s
```

The source-observation package separately passed ten race-enabled repetitions
in `38.998s`. Focused ordinary catalog, catalog-store, validation, and
models.dev suites also passed during extraction.

The exact final candidate passed the complete ordinary repository suite with
root `57.648s`, acquisition `20.252s`, catalog pipeline `4.419s`, models.dev
`18.175s`, catalogs `26.280s`, catalog store `0.448s`, remote `14.102s`, and
server `10.716s`. Its affected race gate passed catalogs `56.211s`, catalog
store `3.956s`, catalog pipeline `13.232s`, models.dev `84.602s`, and validation
`3.268s`. Full vet, repository-pinned lint with zero issues, generated
documentation, the Go file-size gate, and `git diff --check` passed.
