# Testing and Verification

Tests must prove a behavior that callers or operators depend on. Each test needs
a failure that explains which contract broke. Test counts and line coverage
cannot establish correctness alone.

Starmap and Starport use Go 1.27.1. Older Go families have no current support commitment.
Qualify future upgrades across both repositories and update their exact pins together.

## Local development

Run the affected package while editing. Narrow the test name when investigating
one failure. These commands permit Go's build and successful-test caches:

```bash
make test TEST_PACKAGES=./runtime TEST_RUN=TestScopeEvidenceRenewal
make test-race TEST_PACKAGES=./pkg/catalogs
make docs-check
```

Use `-count=1` when the result must prove fresh execution:

```bash
go test -race -count=1 -timeout=5m ./internal/catalog/reconciler
```

`make test` selects all packages by default. Each package has a 30-minute timeout.
Local commands run one package at a time to bound full-catalog memory.
`TEST_TIMEOUT` and `TEST_PARALLEL_PACKAGES` override these bounds.

`make test-integration` is a
compatibility alias for `make test`. The repository has no integration build tag.
Local HTTP servers, temporary files, and in-process adapters run in ordinary tests.
Real provider acquisition remains a separate credentialed operation.

## Verification before merge

Run the local gate after the final change:

```bash
make verify
```

The gate checks generated output before expensive execution. It then checks
package ownership, dependencies, verifier failure behavior, vet, pinned linters,
and prose. Additional checks cover pure-Go operation, allocation budgets, containers,
coverage, and CLI smoke behavior.

The final suites run fresh race tests and the full-catalog capacity test.

`make verify-checks` runs the first phase alone. `make test-all` runs both final
suites. `make ci-test` is a compatibility alias for local verification.

Hosted CI adds native platforms, real storage services,
security checks, and fuzzing. A local pass does not replace those results.

Windows AMD64 also tests native file publication and private files with race instrumentation.
This check covers native structure alignment in the instrumented binary. Pure-Go native startup remains separate.

The required `Verification Gate` checks every result in its dependency graph.
A failed, cancelled, skipped, or absent prerequisite cannot pass that gate.

| Execution | Contract |
| --- | --- |
| Go 1.27.1, race suite | Every package with race instrumentation, except one explicit capacity test |
| Go 1.27.1, capacity suite | `TestPublicPublicationProfileRetainsBoundedState` with the complete corpus |
| Native jobs | Linux, macOS, and Windows behavior on the configured architectures |
| Storage jobs | Valkey and Redis behavior with a real object store and process recovery |

The race suite has no blanket `-short` flag. The catalog concurrency test must
run under the race detector. Only the named full-catalog capacity test runs
without race instrumentation. Smaller publication and ownership tests still
exercise those contracts under the race detector.

`scripts/verification_tests.py` assigns each package from `go list ./...` to
exactly one group. New packages enter a group automatically. The race suite uses eight hosted runners after the verification checks job:

| Group | Packages |
| --- | --- |
| `checks` | CI workflow contracts, executed early inside the verification checks job |
| `runtime-1`, `runtime-2`, `runtime-3` | Disjoint runtime test groups, including child packages |
| `client` | Root library, acquisition, and embedded bootstrap |
| `application-1`, `application-2`, `application-3` | Disjoint command, CLI composition, and HTTP server tests |
| `contracts` | All remaining packages |

Runtime and application runners discover top-level tests, examples, and fuzz seeds with `go test -race -json -list .`.
A stable hash of each name selects one of three groups. New tests enter a group automatically.
Each runner verifies that its completed tests exactly match its selected inventory.
Missing, unexpected, and duplicate results fail verification. CI retains the selected inventory beside the test events.

The package timeout remains 30 minutes. Native jobs retain their complete runtime suites.

Publication recovery, ingestion, and real Git acquisition run in separate native jobs on all six platforms.
The required verification gate also requires every native publication job to pass.
This separates sequential job costs without changing test selection or timeout limits.

The early `checks` group runs once with race instrumentation. Later race groups exclude those packages.
Use `make verify` for complete local qualification. It runs the early checks once, then the runtime groups and remaining suites.

Each runner uses `-p=1` to bound concurrent catalog memory. The release race suite covers the complete package inventory.
It also proves ordinary behavior, so CI does not repeat that suite without instrumentation. Coverage and pure-Go checks remain separate because
they prove different properties.

Run one fresh group with retained JSON evidence:

```bash
make verify-tests TEST_SUITE=race TEST_GROUP=runtime
python3 scripts/verification_tests.py race --group runtime --shard 1
python3 scripts/verification_tests.py race --group application --shard 1
make verify-tests TEST_SUITE=race TEST_GROUP=contracts
make verify-tests TEST_SUITE=capacity
```

The runner prints the evidence path, test outcomes, and slowest tests. CI retains
the JSON stream even after failure. Compare the same toolchain, fixture, test
selection, race mode, and machine class when measuring a change. Report build
and queue time separately from test execution. The hosted gate target is below
30 minutes. A target is not measured qualification.

## Cache and parallel execution

Go's build cache avoids recompilation. Local test-result caching speeds repeated
checks with unchanged inputs. Required suites use `-count=1`, so a cached test
result cannot replace fresh qualification. Hosted jobs retain module and build
caches through `actions/setup-go`.

Cache an immutable fixture only when its bytes and validation policy cannot
change during the process. The embedded bootstrap already verifies its immutable
catalog once per process. Callers receive owned copies of mutable generation
data. Never share a mutable builder, store, environment, clock, or source reply
between independent tests. Never cache a success receipt across changed inputs.

The catalog acceptance runner groups selected Go tests by repository and package.
Each group runs once per invocation with race detection, `-count=1`, and a
five-minute timeout. This reuses immutable bootstrap data within that process.

Each selected test must run and pass exactly once in a complete package result.
A skipped required subtest leaves its named parent unverified. A failed command
fails the group. Results never carry across verifier invocations.

Use `t.Parallel()` only when each test owns all mutable state. Tests that change
the process environment or current directory must remain serial. Concurrency
tests need explicit channels or barriers that establish the required ordering.
Elapsed sleeps cannot prove that ordering. Fault tests must observe completion,
cleanup, and retained state after the fault.

Hosted groups provide parallel execution without simultaneous full-catalog
packages on one small runner. More workers can increase memory pressure and
repeat compilation. Measure total runner time as well as wall time before
increasing the group count.

## Test ownership and fixture size

| Boundary | Evidence and fixture |
| --- | --- |
| Catalog rules and reconciliation | Small authored catalogs with only the records needed for the rule |
| Provider wire conversion | Governed captured responses through the real parser and HTTP client |
| Runtime composition | Real catalog publication with controlled source and clock adapters |
| Store contract | The same observable storage contract against each implementation |
| Durability and recovery | Real files, processes, and services where the failure depends on them |
| Embedded startup and capacity | The complete embedded or public catalog, including its manifest |
| Public interfaces | External consumers, CLI output, HTTP responses, and OpenAPI reproduction |

A receipt-renewal test needs the observed model and its author. It does not need
to copy every embedded model into its source fixture. Separate bootstrap tests
prove the embedded corpus, manifest, and default startup behavior.

Keep policy tests beside the package that owns the policy. Keep orchestration
in composition packages and test adapters at the interface their caller consumes.
Do not export production hooks solely to avoid expensive fixture setup. Extract
a package only when it owns a coherent contract with independent inputs and
outputs. A new directory alone does not reduce the cost of its dependencies.

Use small test builders within the owning package. Put a fixture in
`internal/test` only when several packages share the same protocol or data
contract. A mock must not replace the behavior the test claims to prove. For
example, a mocked rename cannot qualify crash recovery on Windows.

Before adding a case, name its distinct regression: success, boundary, rejection,
rollback, recovery, or concurrent access. Table rows should change a relevant
input or failure point. Repeating the same contract through every wrapper adds
cost without a new guarantee. Keep a composition test where wiring itself can fail.

## Generated output

`make docs-check` reproduces embedded OpenAPI schemas and checks generated API
README files with pinned tools. Run it immediately after public API or comment
changes. `make godoc` updates API READMEs. `make openapi` updates embedded schemas.
Do not edit generated files by hand.

Generation must be deterministic and fail on stale output or generator errors.
Test handwritten mapping and validation rules through their public result.
A generated file needs a reproduction check, not a second test for each generated
line. Preserve parser, schema, and external-consumer tests that prove compatibility.

## Real Git acquisition fixtures

The Git acquisition fixtures use Git and Bun 1.3.12. They create local repositories
with two pinned revisions and a dependency-free lockfile. The production collector
clones these repositories and builds their metadata with Bun. Tests check changed
catalog facts and exact commit and lockfile receipts.

The verification job and all six native jobs require these tools. The
fixture checks the Bun version, operating system, and architecture against the
native Go test process. A missing tool fails required qualification. A local run reports a skip when
a tool is absent. A skip does not qualify Git acquisition.

Run the source collector check with mandatory tools:

```bash
CATALOG_GIT_FIXTURE_REQUIRED=1 go test -race -count=1 -timeout=5m -run '^TestRealPinnedGitSourceAcquisition$' ./acquisition
```

The application matrix uses both HTTP fixtures and real Git fixtures. It covers
CLI refresh, HTTP update, connected refresh, startup acquisition, and repeated
timer cycles. Git commit changes require a restart. Each timer cycle must report
a new successful source receipt. Unchanged facts can retain earlier provenance.

Publisher cases cover provider-scoped and all-provider acquisition. They reopen
the exact staged artifact and bind changed metadata to its source receipt. The
all-provider case runs in a child process with private directories and a local
proxy. The child preserves the mandatory-tool flag and propagates skips.

Native jobs retain runtime, Git source, and publisher ingestion evidence in
separate JSON files. The A20 registry names both source forms for the CLI,
server, publisher, and artifact-binding checks. Missing-dependency and complete
product-pair acceptance remain separate checks.

The fixture redirects only the models.dev Git URL to its local repository. It
disables system and user Git configuration and limits Git transport to local
files. It does not install dependencies from a package registry. This check
qualifies the source collector. The application ingestion matrix has separate
acceptance cases.

The dependency regression tests use an empty executable path and a small file baseline.
CLI and HTTP updates must fail for unavailable Git acquisition, including `--fresh`.
They preserve the accepted generation and report the selected source. Separate tests reject unrecognized diagnostic values and preserve available local acquisition.
These tests do not complete the source-state inventory or revoked-scope restart contract.

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
go test ./internal/catalog/query ./internal/server/params ./internal/cli/commands/models
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
make release-check
```

`make test-pure-go` executes the external library, store, server, remote, and
CLI compositions with `CGO_ENABLED=0`. It also verifies local binary linkage.
`make verify` includes that gate, then runs the race suite separately with
`CGO_ENABLED=1`. `make release-check` adds release-specific CLI and exact
GoReleaser checks.

Consumer dependency budgets count product and third-party packages. Standard-library package counts follow the selected compiler and do not define a product boundary.
The forbidden-import checks still inspect every package, including standard-library database adapters.
