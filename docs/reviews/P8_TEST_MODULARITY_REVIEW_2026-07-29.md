# P8 Test Modularity Review

Date: 2026-07-29

Reviewed commit: `0dd842ee`

Status: `ACCEPTED`

## Outcome

Every repository-authored Go test file is below the 2,000-line hard limit.
Only two test files cross the 1,000-line review threshold, and neither crosses
the 1,500-line durable-justification threshold:

| File | Lines | Tests/benchmarks | Disposition |
| --- | ---: | ---: | --- |
| `internal/catalog/reconciler/merger_test.go` | 1,446 | 27 | Retain as the cohesive core model-merger contract |
| `internal/providers/openai/client_test.go` | 1,036 | 20 | Retain as the cohesive OpenAI-compatible adapter contract |

The next-largest test is `remote/subscriber_test.go` at 982 lines. No hidden
hard-limit exception or generated-file waiver is involved.

## Reconciler Merger

P8.2 already split the former 2,050-line merger test by actual behavior:

- provider reconciliation is in `merger_provider_test.go`;
- atomic provider pricing is in `merger_pricing_test.go`; and
- provider/model-scoped provenance is in `merger_provenance_test.go`.

The retained file now owns the one remaining concept: how the core model merger
combines fields, nested metadata, limits, source timestamps, lineage, modes,
extensions, baseline state, and explicit presence. Its concurrency test and
benchmark exercise that same algorithm. Splitting those field cases into
arbitrary size-based files would separate one invariant table from its edge,
copy, and baseline cases without creating a new production seam.

Its four setup helpers construct models/providers or publish a test catalog.
The other small helpers perform membership, string-pointer, or snapshot
conversion. They hide repetitive construction only. Expected winners,
provenance, presence states, deep-copy isolation, timestamps, extensions, and
errors remain asserted directly in each test.

## OpenAI-Compatible Provider

The 1,036-line provider test follows one vertical adapter contract:

1. bounded OpenAI-compatible response decoding;
2. provider-configured field, author, feature, presence, price, extension, and
   lineage normalization;
3. the real authenticated list-models request; and
4. schema-drift and format-change rejection.

That vertical coverage is intentionally local to the concrete adapter. It has
one client constructor, two testdata loaders, and one modality membership
helper. None assert product behavior on behalf of a test. At only 36 lines over
the review threshold, another file boundary would increase navigation and
duplicate imports without isolating an independent fixture, protocol, or
production concept.

## Fixture Policy

The reviewed helpers:

- accept explicit inputs;
- return concrete catalog/provider/client values;
- fail immediately when setup cannot be constructed;
- do not choose expected winners or suppress error branches; and
- do not contain shared assertion loops that could make many tests pass for
  the same mistaken reason.

All behavior assertions remain at the call site. Provider, pricing, provenance,
wire-decoding, and resilience tests already live in their named files where a
real independent concept exists.

## Verification

The repository file-size gate reports exactly the two reviewed files and
passes:

```text
1446  internal/catalog/reconciler/merger_test.go
1036  internal/providers/openai/client_test.go
Go file-size verification passed: review >1000, justify >1500, fail >=2000 lines
```

The complete generated-document-inclusive `./scripts/verify.sh` gate on the
reviewed code passed before this evidence-only review was recorded, including
ordinary tests, cgo-off compositions, the full cgo-enabled race suite, vet,
zero-issue lint, all coverage floors, documentation, diff checking, catalog
validation, and CLI smokes.

Ten focused race-enabled repetitions also passed:

```text
CGO_ENABLED=1 go test -race \
  ./internal/catalog/reconciler ./internal/providers/openai -count=10

reconciler 2.035s; OpenAI-compatible provider 5.460s
```
