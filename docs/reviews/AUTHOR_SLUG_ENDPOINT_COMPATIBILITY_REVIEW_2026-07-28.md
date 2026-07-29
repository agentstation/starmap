# Author/Slug and Endpoint Compatibility Review

Date: 2026-07-28  
Reviewed head: `76dd317810815604b6c796814bce5b8887aaadd0`

## Trigger

The user identified a material omission after P5: a provider is the serving
operator, not necessarily the model author. Therefore provider identity cannot
be used to manufacture the `{author}/{slug}` identity required by Starport's
OpenRouter-compatible model APIs.

This is a new review finding. It does not rewrite the P5 evidence that the old
author-model files behaved as stale denormalized copies. User steering now
requires restoring the human-facing author/model structure with explicit,
non-overlapping ownership and using it in endpoint generation.

## Required compatibility

The reviewed OpenRouter contracts are:

- `GET /api/v1/model/{author}/{slug}` returns one model identified by author
  and slug, exposes a canonical author/slug ID, and resolves known aliases:
  <https://openrouter.ai/docs/api/api-reference/models/get-a-model-by-its-slug>
- `GET /api/v1/models/{author}/{slug}/endpoints` returns the provider endpoints
  that currently serve that model, with provider identity, provider model ID,
  pricing, limits, parameters, status, and optional operational metrics:
  <https://openrouter.ai/docs/api/api-reference/endpoints/list-all-endpoints-for-a-model>

These routes make two identities load-bearing:

1. **canonical model identity** — the author/lab and stable model slug; and
2. **provider offering identity** — the provider plus the exact opaque model ID
   accepted by that provider.

They must not be collapsed.

## Verified current state

### Retained data

- `internal/embedded/catalog/authors.yaml` remains embedded and contains author
  entities, aliases, and attribution rules.
- All 611 provider-model YAML files remain embedded.
- 539 of those 611 files carry an explicit `authors:` field.
- Cross-lab examples already disprove provider-to-author inference:
  Alibaba's `kimi-k2.5` and `kimi-k2.7-code` identify `moonshot-ai` as author,
  while Alibaba's `glm-5.1` and `glm-5.2` identify `zhipu-ai`.
- 187 provider model IDs contain a slash. Examples such as DeepInfra's
  `moonshotai/Kimi-K2.5` demonstrate that a provider-native namespace is not
  automatically the canonical Starmap author ID or slug.

### Current derived model

- `ModelDefinition.AuthorIDs` and `Catalog.AuthorModels` preserve derived
  authorship membership.
- `ProviderOffering` already retains the facts needed for a static endpoint
  row: provider ID, exact provider model ID, definition link, provider pricing,
  limits, availability, regions, endpoint, lifecycle, modes, and request
  behavior.
- Provider pricing is already selected per provider offering and remains the
  authoritative price when valid.

### Missing behavior

- `ModelDefinition.ID` is currently derived directly from the provider model
  ID. Consequently `kimi-k2.5`, `moonshotai/Kimi-K2.5`, and a provider-specific
  `accounts/.../kimi-k2p5` can become separate definitions even when they
  represent one authored model.
- The immutable catalog has no precomputed `(author, slug)` or author/slug
  alias index.
- It has no definition-to-all-provider-offerings index or public read method.
- `FindModel` accepts only a definition ID and does not define ambiguity
  behavior for a provider-native alias used by more than one author.
- The server exposes `/api/v1/models` and `/api/v1/models/{id}` only. It does
  not expose either OpenRouter compatibility route.
- `internal/embedded/catalog/endpoints.yaml` is empty. The existing generic
  `catalogs.Endpoint` type contains only ID, name, and description and is not an
  OpenRouter provider endpoint row.
- The deleted 322-file author-model mirror was never an endpoint generator. It
  duplicated model payloads and did not preserve a one-to-many link from one
  canonical authored model to every provider offering.

## Decision

### 1. Restore the author/model workspace as authored model truth

Restore `authors/{author}/models/{slug}.yaml` as the human-readable home for
canonical model identity and intrinsic authored metadata: display name,
description, release and knowledge dates, lineage, architecture, weights, and
intrinsic capabilities.

Do not restore the old semantics in which `Author.Models` was a provider model
copy populated by attribution and then saved as a denormalized mirror. Author
model files are independent authored records. They must not contain provider
price, provider limits, availability, provider endpoint configuration, request
overrides, or provider source extensions.

The 322 files deleted by PR #53 are restored as review input. A deterministic
migration strips provider-serving facts and gives every record a terminal
keep, merge/alias, or reject decision. This preserves real authored metadata
without reviving conflicting prices or stale provider state.

### 2. Keep provider models as serving records with an explicit model link

Provider YAML remains the human-readable home for one provider's exact serving
record: opaque provider model ID, provider price and limits, availability,
regions, lifecycle, protocol endpoint, modes, request behavior, provider source
extensions, and provenance.

Provider identity must never imply author identity. Each provider model needs
one minimal, explicit link to its authored model:

```yaml
# Exact ID sent to DeepInfra.
id: moonshotai/Kimi-K2.5

# Stable author/model identity.
model: moonshot-ai/kimi-k2.5
```

`model` is a reference, not another model payload. Multiple provider records
may reference the same authored model. Dynamic provider and models.dev
observations may propose intrinsic facts through the existing authority and
provenance pipeline, but the projected workspace writes the reconciled
intrinsic result to the author model and serving facts to the provider model.
The two trees do not override the same fields.

Import tooling may propose the link from explicit upstream author metadata or a
verified identity mapping. Publication must not guess it from the provider
name. A human can correct the link in the provider YAML without creating an
override file.

Known route aliases need one canonical alias representation owned by the model
identity in its author model file. Do not overload provider aliases or
routing-target aliases.

### 3. Generate one endpoint projection from the join

`endpoints.yaml` is a deterministic generated projection, not a third editable
source of truth. Each row joins:

- one authored model from `authors/{author}/models`;
- one provider serving record that explicitly references it; and
- optional runtime metrics supplied at response time, never persisted as
  invented catalog facts.

The file uses a versioned deterministic schema and carries the input generation
ID or digest. It is rebuilt off to the side, validated, and atomically projected
with the rest of the human workspace. A hand edit is detected as projection
drift and is never silently overwritten; the CLI points the user to the owning
author or provider record.

The generated file is useful for inspection, export, and server startup, but
the immutable catalog may build the identical index directly without reparsing
its own output.

### 4. Make the immutable catalog index the relationship

Publication builds and validates these immutable indexes off to the side:

- canonical model ID to `ModelDefinition`;
- `(canonical author, canonical slug)` to canonical model ID;
- accepted author/slug alias to canonical model ID;
- canonical model ID to sorted `[]OfferingKey`; and
- existing provider ID plus provider model ID to `ProviderOffering`.

The catalog should expose a narrow read API such as:

```go
model, err := catalog.FindModelBySlug("moonshot-ai", "kimi-k2.5")
offerings, err := catalog.ModelOfferings(model.ID)
```

`FindModel("gpt-4o")` remains useful as a compatibility/convenience alias when
the alias is unique. Ambiguous bare or provider-native aliases return a typed
ambiguity error; they never select by map order, alphabetical order, or
provider priority.

### 5. Project compatibility responses at the server edge

The P7 public server owns OpenRouter-specific DTOs and adapters. Canonical
catalog types remain provider-neutral.

- The model route resolves author/slug and projects intrinsic definition data.
- The endpoint route resolves the same definition and maps every eligible
  `ProviderOffering` to one endpoint row.
- Valid current provider pricing wins for each endpoint.
- Static catalog facts and operational telemetry remain separate. Latency,
  throughput, and uptime may be joined from a runtime health/metrics role when
  available; the catalog must not invent them or persist rapidly stale samples
  in human YAML.
- A provider offering without a public URL may still be a routable logical
  endpoint when Starport owns dispatch; the server adapter must define this
  explicitly rather than fabricating a provider URL.

### 6. Fail safely

- Unknown authorship keeps an offering provider-addressable but excludes it
  from author/slug compatibility routes and increments an observable
  `unroutable_models` health count.
- Untrusted source records with malformed canonical IDs are quarantined with
  provenance while valid siblings survive.
- A collision that maps one canonical author/slug or alias to two different
  definitions is a typed validation error. A candidate containing such a
  collision is not published, so readers retain the previous complete
  generation.
- Strict/release validation requires every publicly distributed model to have
  one canonical author/slug and at least one eligible provider offering.
- Unknown telemetry remains absent. Unknown price never displaces a valid
  provider price.

## Historical corpus follow-up

The P5 review found 121 deleted author-model IDs without an exact provider-model
ID: 32 alias/content overlaps, 25 models.dev-only records, and 64 presumed stale
orphans. PR #53's terminal disposition is superseded for the authored-model
corpus:

- 32 alias/content overlaps are reviewed as identity/alias evidence and merged
  into one canonical author model where proven;
- 25 models.dev-only records may remain authored models without endpoints when
  current authorship and model evidence validate them;
- 64 presumed orphans are individually retained, corrected, or rejected with
  evidence rather than being restored blindly.

No author model manufactures a provider endpoint. An endpoint exists only when
a provider serving record explicitly links to that author model.

## Verifiable success criteria

### Core catalog

- A test proves Alibaba offerings for Moonshot and Zhipu resolve under their
  lab authors, never `alibaba/{slug}`.
- Two differently named provider model IDs linked to one canonical model
  produce one definition and two exact offerings with independent prices and
  limits.
- `(author, slug)`, author aliases, and model aliases resolve deterministically.
- Alias and canonical-ID collisions fail candidate publication with typed
  errors while the previous generation remains current.
- Every immutable index is built before publication and is mutation-isolated
  and race-safe.
- Strict/release validation reports zero public definitions without canonical
  author/slug and zero endpoint rows without a valid offering link.
- All 322 historical author models receive an exact keep/merge/reject map.

### Server compatibility

- Contract tests exercise both exact OpenRouter route shapes.
- A model served by at least two providers returns at least two endpoint rows,
  each with the correct provider, provider model ID, price, and limits.
- Missing operational telemetry is omitted or null according to the documented
  compatibility schema and is never invented.
- Provider-specific pricing remains authoritative in each endpoint row.
- Server DTOs do not leak into `pkg/catalogs`.

### Persistence and DX

- Author model YAML owns intrinsic authored facts; provider model YAML owns
  serving facts; their field sets do not overlap.
- `endpoints.yaml` is generated from explicit links and is never an independent
  editable authority.
- A human adds a new authored model once, then adds any number of provider
  serving records using the same `model: author/slug` reference.
- A human adding a new provider for an existing third-party model edits only
  that provider record.
- Generated workspace projection round-trips canonical identity and provenance.
- README, schema, examples, GoDoc, and architecture docs distinguish author,
  canonical model, provider, and provider offering.

## Execution consequence

P5 is explicitly reopened for the missing canonical identity/read-index
contract before P6 composition continues. P6.1 evidence remains valid. P6.2 is
paused, not discarded, and resumes only after the core catalog success criteria
above are green. P7 owns the HTTP compatibility adapters after the core read
model is complete.
