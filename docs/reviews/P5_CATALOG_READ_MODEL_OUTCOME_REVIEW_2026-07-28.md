# P5 Catalog Read Model Outcome Review

Date: 2026-07-28

## Scope

The reviewed outcome is the complete P5 provider-model and derived-read-view
simplification on `codex/catalog-read-model-simplification` against
`origin/main@9609f4f4a74281a7f9692a97cc4926df5978d754`.

The phase:

- keeps provider YAML and `provider_models` as the only persisted model records;
- removes the embedded author-model mirror and top-level `author_models`;
- derives immutable definitions, provider offerings, and author membership at
  `Builder.Build`;
- removes the prelaunch flattened catalog and legacy schema adapters;
- preserves provider identity through exact offering, query, hook, HTTP, CLI,
  JSON, and YAML paths;
- deletes unused prelaunch options, flags, aliases, and duplicate schema
  spellings; and
- retains a concrete immutable `*catalogs.Catalog` with O(1), non-failing root
  access.

The review prioritized duplicate truth, provider identity, authority,
provenance, nil safety, deterministic ordering, immutable ownership, public
consumer DX, and schema/output consistency.

## Method

The structured autoreview helper used Claude Fable with maximum reasoning and
repository tools. The deletion-heavy branch required three chunks per run.

1. The first complete bundle was 1,172,673 bytes. It found lossy bare-ID hook
   and query adapters, stale CLI text, an incomplete GoDoc sentence, and a
   digest-stale generation fixture. One claim that the package example omitted
   `Builder.Build` was disproven by direct inspection; that step already
   existed.
2. Provider identity remediation materially changed prelaunch HTTP and CLI
   response semantics, so Operating Rule 19 required a repeat. The 1,200,206
   byte bundle found mixed-provider compatibility export, YAML shape, canonical
   history-provider identity, partial-metadata provenance nil safety, and
   embedded-corpus proof gaps.
3. Those fixes changed public output and construction failure behavior, so one
   final repeat reviewed a 1,212,013 byte bundle. It found no production defect.
   It found one test-proof error: the new panic regression used `Metadata`
   rather than the persisted evidence path `metadata`. The fixture now asserts
   that exact evidence lookup before construction and passes ten race-enabled
   repetitions.

No review was rerun for the final test-only spelling correction, documentation,
or ledger evidence.

## Findings and dispositions

### F-055 — provider identity was lost outside the canonical offering index

Severity: high

Status: fixed in P5

The first implementation keyed transitional model-hook diffs and unfiltered
query rows by bare model ID. Same-ID offerings from two providers could hide
changes, produce arbitrary winners, or become ambiguous after export.

The remediation:

- keys hook diffs by `(ProviderID, modelID)` with stable provider/model order;
- returns internal `ModelRecord` values that carry canonical `provider_id`;
- preserves that field in flat JSON and YAML objects;
- includes Provider in CLI tables;
- keeps HTTP list/search records provider-scoped;
- uses provider ID as the stable tie-breaker after model sort fields; and
- rejects compatibility export when results span providers because OpenAI and
  OpenRouter model-list schemas cannot represent provider ownership.

Focused tests cover duplicate IDs, deterministic ordering, JSON/HTTP identity,
flat YAML, provider-scoped hook updates, and mixed-provider export rejection.
F-019 remains assigned to P7 because P7 deletes per-model events and fixes
cross-generation ordering, overload, and SSE publication semantics.

### F-056 — partial metadata evidence could panic read-view construction

Severity: critical

Status: fixed in P5

Architecture extractors safely filtered candidate records but reused the same
closures for decoded provenance. A valid `metadata` evidence object with no
architecture could therefore dereference nil during `Builder.Build`,
generation decode, or startup.

All architecture leaves now use one nil-safe accessor. The regression binds a
partial metadata record to the exact persisted `metadata` evidence key, proves
that lookup is populated, and verifies that construction preserves the valid
candidate architecture without panic.

### F-057 — closeout evidence and public text had inconsistent identifiers

Severity: medium

Status: fixed in P5

The review found a generation fixture whose ID retained the prior payload
digest suffix, stale CLI alias/testing guidance, an incomplete `FindModel`
GoDoc sentence, and insufficiently explicit full-corpus publication
assertions. The fixture ID now matches the schema-v2 payload digest; current
CLI documentation has one command/flag vocabulary; GoDoc is complete; and the
embedded bootstrap gate builds the complete corpus, requires nonzero
definitions and offerings, and validates every offering.

The review's package-example claim was rejected: the example already called
`Build` before `Definitions`. Its related GoDoc observation was valid and
fixed.

### History provider alias claim

Severity: medium

Status: fixed with corrected diagnosis

Starmap has provider aliases but no model-alias schema. The review's proposed
model-alias regression therefore did not exist. Inspection did expose a real
provider-alias issue: validation resolved the alias, then returned the requested
alias for provenance lookup rather than the canonical provider ID. History now
validates through `Catalog.Offering` and returns its canonical `ProviderID`;
the alias case has direct coverage.

## Result

No structured-review production finding remains unresolved. Exact complete
local verification, exact-head hosted checks, protected review/merge, and
protected-main ledger readback remain the authoritative P5 phase gate.
